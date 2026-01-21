# memcache.c 代码详解 (memcache.c Code Walkthrough)

## 文件概述 (File Overview)

`memcache.c` 实现了一个基于环形数组的无锁 MPMC (Multi-Producer Multi-Consumer) FIFO 队列，用于内核对象的内存池管理。

**文件路径:** `driver/LKM/src/memcache.c`
**代码行数:** 337 行
**作者:** wuqiang.matt@bytedance.com, mhiramat@kernel.org
**主要功能:** 无锁对象池，减少内核内存分配开销

---

## 1. 设计概述 (Design Overview)

### 1.1 核心特性

- **无锁设计** - 使用原子操作实现 MPMC 队列
- **Per-CPU 槽位** - 每个 CPU 有独立的对象槽
- **内存预分配** - 初始化时分配所有对象
- **NUMA 感知** - 对象分配在对应 CPU 节点的内存上

### 1.2 数据结构

```
memcache_head (全局控制结构)
    ├── nr_cpus          # CPU 数量
    ├── obj_size         # 单个对象大小
    ├── capacity         # 每个槽位的容量 (2的幂次)
    ├── gfp              # 分配标志
    ├── nr_objs          # 总对象数
    ├── cpu_slots[nr_cpus]  # Per-CPU 槽位数组
    │   └── memcache_slot (每个 CPU 的槽位)
    │       ├── entries[]    # 环形数组存储对象指针
    │       ├── head         # 消费者索引
    │       ├── tail         # 生产者预留索引
    │       ├── last         # 生产者提交索引
    │       └── mask         # capacity - 1，用于取模
    └── release          # 释放回调
```

---

## 2. Per-CPU 槽位初始化 (Per-CPU Slot Initialization)

### 2.1 单个槽位初始化

```c
static int
memcache_init_percpu_slot(struct memcache_head *pool,
                         struct memcache_slot *slot,
                         int nodes, void *context,
                         memcache_init_obj_cb objinit)
{
    void *obj = (void *)&slot->entries[pool->capacity];
    int i;

    /* initialize elements of percpu memcache_slot */
    slot->mask = pool->capacity - 1;

    for (i = 0; i < nodes; i++) {
        if (objinit) {
            int rc = objinit(obj, context);
            if (rc)
                return rc;
        }
        slot->entries[slot->tail & slot->mask] = obj;
        obj = obj + pool->obj_size;
        slot->tail++;
        slot->last = slot->tail;
        pool->nr_objs++;
    }

    return 0;
}
```

**内存布局解析:**

```
slot 的内存分配:
┌─────────────────────────────────────────────────────────┐
│ struct memcache_slot                                    │
│ ├── entries[] (capacity 个指针)                        │
│ └── 对象数据区 (nodes * obj_size)                      │
│     ├── obj[0]                                         │
│     ├── obj[1]                                         │
│     └── ...                                            │
└─────────────────────────────────────────────────────────┘

obj 指向 entries[] 后的第一个对象位置
每次增加 obj_size 字节来访问下一个对象
```

**索引初始化:**

```
初始化后:
  head = 0    (消费位置)
  tail = nodes    (预留位置)
  last = nodes    (可用位置)

entries[0] = obj[0]
entries[1] = obj[1]
...
entries[nodes-1] = obj[nodes-1]
```

### 2.2 所有槽位分配

```c
static int
memcache_init_percpu_slots(struct memcache_head *pool, int nr_objs,
                          void *context, memcache_init_obj_cb objinit)
{
    int i, cpu_count = 0;

    for (i = 0; i < pool->nr_cpus; i++) {

        struct memcache_slot *slot;
        int nodes, size, rc;

        /* skip the cpu node which could never be present */
        if (!cpu_possible(i))
            continue;

        /* compute how many objects to be allocated with this slot */
        nodes = nr_objs / num_possible_cpus();
        if (cpu_count < (nr_objs % num_possible_cpus()))
            nodes++;
        cpu_count++;

        size = sizeof(struct memcache_slot) + sizeof(void *) * pool->capacity +
            pool->obj_size * nodes;

        /*
         * here we allocate percpu-slot & objs together in a single
         * allocation to make it more compact, taking advantage of
         * warm caches and TLB hits. in default vmalloc is used to
         * reduce the pressure of kernel slab system. as we know,
         * mimimal size of vmalloc is one page since vmalloc would
         * always align the requested size to page size
         */
        if (pool->gfp & GFP_ATOMIC)
            slot = kmalloc_node(size, pool->gfp, cpu_to_node(i));
        else
            slot = vmalloc_node(size, pool->gfp, cpu_to_node(i));
        if (!slot)
            return -ENOMEM;
        memset(slot, 0, size);
        pool->cpu_slots[i] = slot;

        /* initialize the memcache_slot of cpu node i */
        rc = memcache_init_percpu_slot(pool, slot, nodes, context, objinit);
        if (rc)
            return rc;
    }

    return 0;
}
```

**对象分配策略:**

```
假设: nr_objs = 100, num_possible_cpus() = 4

CPU 0: nodes = 100 / 4 + 1 = 26 (100 % 4 = 0, 不额外分配)
       等等，逻辑是:
       nodes = nr_objs / num_possible_cpus() = 25
       if (cpu_count < nr_objs % num_possible_cpus())  // if (0 < 0) false
       所以 CPU 0: 25 个对象

修正后:
CPU 0: cpu_count=0, nr_objs % 4 = 0, nodes = 25
CPU 1: cpu_count=1, nr_objs % 4 = 0, nodes = 25
CPU 2: cpu_count=2, nr_objs % 4 = 0, nodes = 25
CPU 3: cpu_count=3, nr_objs % 4 = 0, nodes = 25

如果是 nr_objs = 102:
CPU 0: cpu_count=0, 102 % 4 = 2, 0 < 2, nodes = 25 + 1 = 26
CPU 1: cpu_count=1, 102 % 4 = 2, 1 < 2, nodes = 25 + 1 = 26
CPU 2: cpu_count=2, 102 % 4 = 2, 2 < 2, nodes = 25
CPU 3: cpu_count=3, 102 % 4 = 2, 3 < 2, nodes = 25
总共: 26 + 26 + 25 + 25 = 102
```

### 2.3 内存分配选择

```c
if (pool->gfp & GFP_ATOMIC)
    slot = kmalloc_node(size, pool->gfp, cpu_to_node(i));
else
    slot = vmalloc_node(size, pool->gfp, cpu_to_node(i));
```

**为什么区分:**

| 分配方式 | 适用场景 | 特点 |
|----------|----------|------|
| `kmalloc_node` | `GFP_ATOMIC` | 物理连续内存，大小有限，适合原子上下文 |
| `vmalloc_node` | 普通 | 虚拟连续内存，大小灵活，可能有 TLB 开销 |

---

## 3. 对象池初始化 (Object Pool Initialization)

```c
int
memcache_init(struct memcache_head *pool, int nr_objs, int object_size,
            gfp_t gfp, void *context, memcache_init_obj_cb objinit,
            memcache_fini_cb release)
{
    int rc, capacity, slot_size;

    /* check input parameters */
    if (nr_objs <= 0 || nr_objs > MEMCACHE_NR_OBJS_MAX ||
        object_size <= 0 || object_size > MEMCACHE_OBJSIZE_MAX)
        return -EINVAL;

    /* align up to unsigned long size */
    object_size = ALIGN(object_size, sizeof(long));

    /* calculate capacity of percpu memcache_slot */
    capacity = roundup_pow_of_two(nr_objs);
    if (!capacity)
        return -EINVAL;

    /* initialize memcache pool */
    memset(pool, 0, sizeof(struct memcache_head));
    pool->nr_cpus = nr_cpu_ids;
    pool->obj_size = object_size;
    pool->capacity = capacity;
    pool->gfp = gfp & ~__GFP_ZERO;
    pool->context = context;
    pool->release = release;
    slot_size = pool->nr_cpus * sizeof(struct memcache_slot *);
    pool->cpu_slots = kzalloc(slot_size, pool->gfp);
    if (!pool->cpu_slots)
        return -ENOMEM;

    /* initialize per-cpu slots */
    rc = memcache_init_percpu_slots(pool, nr_objs, context, objinit);
    if (rc)
        memcache_fini_percpu_slots(pool);
    else
        atomic_set(&pool->ref, pool->nr_objs + 1);

    return rc;
}
```

**参数验证:**

- `nr_objs` - 对象数量 (1 ~ MEMCACHE_NR_OBJS_MAX)
- `object_size` - 对象大小 (对齐到 unsigned long)
- `capacity` - 向上取最近的 2 的幂次 (便于位运算取模)

---

## 4. 对象推送 (Push Operation)

```c
int memcache_push(void *obj, struct memcache_head *pool)
{
    struct memcache_slot *slot;
    uint32_t tail, last;

    get_cpu();

    slot = pool->cpu_slots[raw_smp_processor_id()];

    do {
        /* loading tail and head as a local snapshot, tail first */
        tail = READ_ONCE(slot->tail);
        smp_rmb();
        last = tail + 1;
    } while (cmpxchg_local(&slot->tail, tail, last) != tail);

    /* now the tail position is reserved for the given obj */
    WRITE_ONCE(slot->entries[tail & slot->mask], obj);

    /* make sure obj is visible before marking it's ready */
    smp_wmb();

    /* update sequence to make this obj available for pop() */
    while (cmpxchg_local(&slot->last, tail, last) == tail) {
        tail = last;
        last = READ_ONCE(slot->tail);
        if (tail == last)
            break;
    }

    put_cpu();

    return 0;
}
```

**无锁算法详解:**

### 4.1 预留槽位 (Reserve Slot)

```c
do {
    tail = READ_ONCE(slot->tail);
    smp_rmb();           // 读屏障，确保读取顺序
    last = tail + 1;
} while (cmpxchg_local(&slot->tail, tail, last) != tail);
```

**CAS (Compare-And-Swap) 循环:**

```
目的: 原子地将 tail 增加 1

步骤:
1. 读取当前 tail 值
2. 计算 new_tail = tail + 1
3. 尝试: if (tail == *slot->tail) *slot->tail = new_tail
4. 如果成功，退出循环
5. 如果失败 (其他线程修改了 tail)，重复步骤 1

cmpxchg_local(&slot->tail, tail, last):
  返回: 旧值 (如果成功则等于 tail)
  操作: 如果 *slot->tail == tail，则 *slot->tail = last
```

### 4.2 写入对象

```c
WRITE_ONCE(slot->entries[tail & slot->mask], obj);
```

**位运算取模:**

```
capacity = 8 (1000 binary)
mask = 7 (0111 binary)

tail & mask 等价于 tail % 8，但更快

示例:
  tail = 10 (1010)
  mask = 7  (0111)
  10 & 7 = 2 (0010)

  entries[2] = obj
```

### 4.3 内存屏障

```c
smp_wmb();  // 写屏障，确保对象写入完成后再更新 last
```

**为什么需要屏障:**

```
没有屏障的情况 (可能出现):
  CPU 0                    CPU 1
  entries[i] = obj
                           x = entries[i]  // 可能读到未完全初始化的 obj
  last = i

有屏障的情况:
  CPU 0                    CPU 1
  entries[i] = obj
  smp_wmb()  ═════════════════════════════════╗
  last = i                                         ║
                                                 ║
                           读取 last，确认对象可用
                           x = entries[i]  // 安全
```

### 4.4 提交槽位 (Commit Slot)

```c
while (cmpxchg_local(&slot->last, tail, last) == tail) {
    tail = last;
    last = READ_ONCE(slot->tail);
    if (tail == last)
        break;
}
```

**目的:** 更新 `last` 索引，使对象对消费者可见

---

## 5. 对象弹出 (Pop Operation)

### 5.1 尝试获取对象

```c
static inline void *memcache_try_get_slot(struct memcache_head *pool, int cpu)
{
    struct memcache_slot *slot = pool->cpu_slots[cpu];
    uint32_t head = READ_ONCE(slot->head);

    while (head != READ_ONCE(slot->last)) {
        void *obj;

        /*
         * data visibility of 'last' and 'head' could be out of
         * order since memory updating of 'last' and 'head' are
         * performed in push() and pop() independently
         *
         * before any retrieving attempts, pop() must guarantee
         * 'last' is behind 'head', that is to say, there must
         * be available objects in slot, which could be ensured
         * by condition 'last != head && last - head <= nr_objs'
         */
        if (READ_ONCE(slot->last) - head - 1 >= pool->nr_objs) {
            head = READ_ONCE(slot->head);
            continue;
        }

        /* obj must be retrieved before moving forward head */
        obj = READ_ONCE(slot->entries[head & slot->mask]);

        /* move head forward to mark it's consumption */
        if (cmpxchg(&slot->head, head, head + 1) == head)
            return obj;

        /* reload head */
        head = READ_ONCE(slot->head);
    }

    return NULL;
}
```

**安全检查解析:**

```c
if (READ_ONCE(slot->last) - head - 1 >= pool->nr_objs)
```

**目的:** 防止读取未完全写入的对象

```
假设 capacity = 4, nr_objs = 4

正常情况:
  last = 5, head = 1
  last - head - 1 = 5 - 1 - 1 = 3
  3 < 4, 安全，可以读取

异常情况 (last 更新延迟):
  last = 1, head = 0  (push 正在进行)
  last - head - 1 = 1 - 0 - 1 = 0
  0 < 4, 但应该等待

更复杂的情况:
  last 溢出回绕, head 没有跟上
  last = 1, head = 0xFFFFFFFC
  last - head - 1 ≈ 很大的数
  超过 nr_objs，需要重试
```

### 5.2 弹出对象

```c
void *memcache_pop(struct memcache_head *pool)
{
    void *obj = NULL;
    unsigned long flags;
    int i, cpu;

    /* disable local irq to avoid preemption & interruption */
    raw_local_irq_save(flags);

    cpu = raw_smp_processor_id();
    for (i = 0; i < num_possible_cpus(); i++) {
        obj = memcache_try_get_slot(pool, cpu);
        if (obj)
            break;
        cpu = cpumask_next_wrapped(cpu, cpu_possible_mask);
    }
    raw_local_irq_restore(flags);

    return obj;
}
```

**搜索策略:**

```
从当前 CPU 开始，循环遍历所有 CPU 的槽位

假设 4 CPU 系统，当前在 CPU 2:

1. 尝试 CPU 2
2. 尝试 CPU 3
3. 尝试 CPU 0 (回绕)
4. 尝试 CPU 1
5. 如果都空了，返回 NULL

这种策略:
- 优先使用本地 CPU 的对象 (缓存友好)
- 其次使用邻近 CPU 的对象
- 最后才使用远程 CPU 的对象
```

### 5.3 禁用 IRQ 的原因

```c
raw_local_irq_save(flags);
```

**原因:**

1. **防止 CPU 迁移** - 确保 `raw_smp_processor_id()` 在整个操作期间不变
2. **原子性** - 在 Kprobe 回调中可能已经在禁用 IRQ 的上下文中

---

## 6. MPMC 正确性证明 (MPMC Correctness)

### 6.1 单生产者单消费者 (SPSC)

```
时间线:
  t0: push() 预留 tail
  t1: push() 写入对象
  t2: push() 更新 last
  t3: pop() 读取对象，更新 head

始终: head ≤ last ≤ tail
```

### 6.2 多生产者多消费者 (MPMC)

```
生产者 P1, P2; 消费者 C1, C2

关键不变量:
  1. entries[head] 到 entries[last-1] 是有效对象
  2. entries[last] 到 entries[tail-1] 正在被写入
  3. entries[tail] 到 entries[captity-1] 是空闲的

CAS 操作确保:
  - tail: 只有生产者能递增
  - head: 只有消费者能递增
  - last: 生产者递增，但受 tail 限制

内存屏障确保:
  - 对象写入在 last 更新之前可见
  - 对象读取在 head 更新之后
```

---

## 7. 使用示例 (Usage Example)

```c
// 定义对象结构
struct my_object {
    int data;
    char buffer[128];
};

// 初始化回调
int init_obj(void *obj, void *context)
{
    struct my_object *o = obj;
    o->data = 0;
    return 0;
}

// 释放回调
void release_pool(struct memcache_head *pool, void *context)
{
    // 清理操作
}

// 使用
struct memcache_head pool;

memcache_init(&pool, 100, sizeof(struct my_object),
             GFP_ATOMIC, NULL, init_obj, release_pool);

// 获取对象
struct my_object *obj = memcache_pop(&pool);

// 使用对象
obj->data = 42;

// 归还对象
memcache_push(obj, &pool);

// 销毁
memcache_fini(&pool);
```

---

## 8. 学习要点 (Learning Points)

1. **无锁编程** - CAS 循环、内存屏障的正确使用
2. **Per-CPU 设计** - 如何减少跨 CPU 的竞争
3. **环形缓冲区** - 使用位运算实现高效取模
4. **NUMA 感知** - 如何使用 kmalloc_node/vmalloc_node
5. **内存布局** - 槽位和对象数据的紧凑排列

---

**下一步学习:** 阅读 `trace_buffer.c` 了解环形缓冲区的另一种实现
