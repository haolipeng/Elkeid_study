# Elkeid Driver 数据流详解 (Elkeid Driver Data Flow)

## 文档概述 (Document Overview)

本文档详细描述 Elkeid Driver 中数据从内核 Hook 触发到用户空间读取的完整流程。

This document describes in detail the complete flow of data from kernel Hook trigger to userspace reading in Elkeid Driver.

---

## 1. 数据流概览 (Data Flow Overview)

### 1.1 完整数据流图

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           完整数据流 (Complete Data Flow)                  │
└────────────────────────────────────────────────────────────────────────────┘

   内核空间 (Kernel Space)                    用户空间 (User Space)
   ────────────────                            ────────────────

   ┌─────────────────────────────────────────────────────────────────────┐
   │  [1] HOOK TRIGGER                                                   │
   │      用户/系统调用内核函数                                          │
   │      do_sys_open, do_execve, connect, etc.                         │
   └────────────────────────────┬────────────────────────────────────────┘
                                │
                                ▼
   ┌─────────────────────────────────────────────────────────────────────┐
   │  [2] KPROBE/KRETPROBE CALLBACK                                     │
   │      smith_hook.c: KPROBE_HANDLER_DEFINE3()                         │
   │      • 从寄存器/栈读取参数                                          │
   │      • 获取进程上下文信息                                            │
   └────────────────────────────┬────────────────────────────────────────┘
                                │
                                ▼
   ┌─────────────────────────────────────────────────────────────────────┐
   │  [3] FILTER CHECK (filter.c)                                        │
   │      execve_exe_check() / execve_argv_check()                       │
   │      • 红黑树查找                                                   │
   │      • O(log n) 时间复杂度                                         │
   └────────────────────────────┬────────────────────────────────────────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
              在白名单中              不在白名单中
                    │                       │
                    ▼                       ▼
              跳过采集            ┌─────────────────────────────────────┐
                                 │  [4] RESERVE BUFFER SPACE            │
                                 │      trace_reserve()                  │
                                 │      • 分配环形缓冲区空间              │
                                 │      • 使用 GFP_ATOMIC                │
                                 └──────────────────┬──────────────────┘
                                                    │
                                                    ▼
                                 ┌─────────────────────────────────────┐
                                 │  [5] COLLECT EVENT DATA              │
                                 │      • 读取函数参数                   │
                                 │      • 获取进程信息                   │
                                 │      • 获取时间戳                     │
                                 │      • 获取 Namespace 信息             │
                                 └──────────────────┬──────────────────┘
                                                    │
                                                    ▼
                                 ┌─────────────────────────────────────┐
                                 │  [6] FORMAT EVENT (print_event.h)    │
                                 │      print_event_xxx()                │
                                 │      • 格式化为 \x17 分隔的字段       │
                                 │      • 添加通用字段                   │
                                 │      • 添加私有字段                   │
                                 └──────────────────┬──────────────────┘
                                                    │
                                                    ▼
                                 ┌─────────────────────────────────────┐
                                 │  [7] COMMIT TO RING BUFFER            │
                                 │      trace_commit()                   │
                                 │      • 提交到 Per-CPU 环形缓冲区        │
                                 │      • 更新写指针                      │
                                 │      • 唤醒等待的读取者                │
                                 └──────────────────┬──────────────────┘
                                                    │
                                                    ▼
   ┌────────────────────────────────────────────────────────────────────────┐
   │  [8] PER-CPU RING BUFFER (trace_buffer.c)                            │
   │  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐                      │
   │  │ CPU 0  │  │ CPU 1  │  │ CPU 2  │  │ CPU N  │                      │
   │  │ Buffer │  │ Buffer │  │ Buffer │  │ Buffer │                      │
   │  └────────┘  └────────┘  └────────┘  └────────┘                      │
   └────────────────────────────┬────────────────────────────────────────┘
                                │
                                │ 用户空间发起读取请求
                                ▼
   ┌───────────────────────────────────────────────────────────────────────┐
   │  [9] USER READ REQUEST (trace.c)                                      │
   │      用户调用 read() on /proc/elkeid-endpoint                         │
   │      trace_read_pipe()                                                │
   └────────────────────────────┬──────────────────────────────────────────┘
                                │
                                ▼
   ┌───────────────────────────────────────────────────────────────────────┐
   │  [10] WAIT FOR DATA                                                   │
   │      trace_wait_pipe()                                                │
   │      • 检查是否有数据                                                 │
   │      • 如无数据且非阻塞：返回 -EAGAIN                                │
   │      • 如无数据且阻塞：等待 tb_wait()                                │
   └────────────────────────────┬──────────────────────────────────────────┘
                                │
                                ▼
   ┌───────────────────────────────────────────────────────────────────────┐
   │  [11] FIND NEXT ENTRY                                                │
   │      __find_next_entry()                                              │
   │      • 从所有 CPU 缓冲区中查找时间戳最小的事件                        │
   │      • 循环遍历所有 CPU                                              │
   └────────────────────────────┬──────────────────────────────────────────┘
                                │
                                ▼
   ┌───────────────────────────────────────────────────────────────────────┐
   │  [12] FORMAT FOR USER (trace.c)                                       │
   │      print_trace_fmt_line()                                          │
   │      • 查找事件格式化函数                                             │
   │      • 调用 class->format()                                          │
   │      • 写入 trace_seq 缓冲区                                         │
   └────────────────────────────┬──────────────────────────────────────────┘
                                │
                                ▼
   ┌───────────────────────────────────────────────────────────────────────┐
   │  [13] CONSUME ENTRY                                                   │
   │      tb_consume()                                                     │
   │      • 标记事件已消费                                                 │
   │      • 更新读指针                                                     │
   └────────────────────────────┬──────────────────────────────────────────┘
                                │
                                ▼
   ┌───────────────────────────────────────────────────────────────────────┐
   │  [14] COPY TO USER                                                    │
   │      trace_seq_to_user()                                             │
   │      • 使用 copy_to_user()                                           │
   │      • 返回读取的字节数                                              │
   └────────────────────────────┬──────────────────────────────────────────┘
                                │
                                ▼
   ┌───────────────────────────────────────────────────────────────────────┐
   │  [15] USER SPACE RECEIVES DATA                                       │
   │      driver plugin 接收数据                                           │
   │      • 按 \x17 分割字段                                              │
   │      • 按 \x1e 分割记录                                              │
   │      • 解析各个字段                                                  │
   └───────────────────────────────────────────────────────────────────────┘
```

---

## 2. Hook 回调阶段 (Hook Callback Stage)

### 2.1 Kprobe 回调流程

```c
// smith_hook.c 中的典型 Kprobe 处理流程

KPROBE_HANDLER_DEFINE3(do_sys_open, df, at, flags)
{
    // === [2.1] 参数提取 ===
    // df    - const char __user* - 文件名
    // at    - struct open_flags* - 访问模式
    // flags - int - 标志位

    // === [2.2] 进程上下文获取 ===
    struct task_struct *task = current;
    pid_t pid = task->pid;
    uid_t uid = task->cred->uid.val;
    // ... 获取更多进程信息

    // === [2.3] 白名单检查 [3] ===
    char exe_path[PATH_MAX] = {0};
    smith_read_exe_path(task, exe_path, PATH_MAX);
    if (execve_exe_check(exe_path, strlen(exe_path))) {
        // 在白名单中，跳过采集
        return;
    }

    // === [2.4] 预留缓冲区空间 [4] ===
    unsigned int len = estimate_event_size();
    struct tb_event *event = trace_reserve(len);
    if (!event) {
        // 缓冲区满，丢弃事件
        return;
    }

    // === [2.5] 收集事件数据 [5] ===
    struct event_context ctx = {
        .data_type = DATA_TYPE_OPEN,
        .timestamp = tb_time_stamp(ring),
        .pid = pid,
        .uid = uid,
        // ... 更多字段
    };

    // 从用户空间读取文件名
    char filename[PATH_MAX] = {0};
    smith_copy_string_from_user(filename, df, PATH_MAX);

    // === [2.6] 格式化事件 [6] ===
    print_event_do_sys_open(event, &ctx, filename, at, flags);

    // === [2.7] 提交到缓冲区 [7] ===
    trace_commit(event);
}
```

### 2.2 Kretprobe 回调流程

```c
// Kretprobe 在函数返回时触发
KRETPROBE_HANDLER_DEFINE(do_sys_open, df, ret)
{
    // 获取入口时保存的参数
    const char *filename = entry_get_df(df);

    // 获取返回值 (文件描述符或错误码)
    long retval = ret;

    // 收集返回值相关的额外信息
    // 例如：文件描述符对应的文件结构
    if (retval >= 0) {
        struct file *file = fget(retval);
        if (file) {
            // 收集文件信息...
            fput(file);
        }
    }

    // 格式化并提交事件...
}
```

---

## 3. 白名单过滤 (Whitelist Filtering)

### 3.1 过滤流程图

```
┌────────────────────────────────────────────────────────────────────────────┐
│                         白名单过滤流程 (filter.c)                          │
└────────────────────────────────────────────────────────────────────────────┘

            Hook 触发，获取 exe_path
                    │
                    ▼
         ┌──────────────────────┐
         │ execve_exe_check()   │
         └──────────┬───────────┘
                    │
                    ▼
         ┌──────────────────────┐
         │ read_lock_irqsave()  │  <- 获取读锁
         └──────────┬───────────┘
                    │
                    ▼
         ┌──────────────────────┐
         │ exist_rb()           │  <- 红黑树查找
         └──────────┬───────────┘
                    │
         ┌──────────┴───────────┐
         │                      │
         ▼                      ▼
   ┌──────────┐          ┌──────────┐
   │  找到     │          │  未找到   │
   └─────┬────┘          └─────┬────┘
         │                     │
         ▼                     ▼
   ┌──────────┐          ┌──────────┐
   │返回 1    │          │返回 0     │
   │(在白名单)│          │(不在白名单)│
   └─────┬────┘          └─────┬────┘
         │                     │
         ▼                     ▼
   ┌──────────┐          ┌──────────┐
   │跳过采集  │          │继续采集  │
   └──────────┘          └──────────┘
```

### 3.2 红黑树查找实现

```c
// filter.c:62-80
static int exist_rb(struct rb_root *root, char *string)
{
    struct rb_node *node = root->rb_node;
    uint64_t hash;

    // 计算字符串的 MurmurHash
    hash = hash_murmur_OAAT64(string, strlen(string));

    // 红黑树查找：O(log n)
    while (node) {
        struct allowlist_node *data = container_of(node, struct allowlist_node, node);
        if (hash < data->hash) {
            node = node->rb_left;
        } else if (hash > data->hash) {
            node = node->rb_right;
        } else {
            return 1;  // 找到，在白名单中
        }
    }
    return 0;  // 未找到，不在白名单中
}
```

---

## 4. 环形缓冲区操作 (Ring Buffer Operations)

### 4.1 缓冲区预留 (Reserve)

```c
// trace_buffer.c 中的预留操作

struct tb_event *trace_reserve(unsigned long length)
{
    struct tb_event *event;

    // [4.1] 计算需要的空间
    // 包括：tb_event 头部 + 数据长度

    // [4.2] 调用 tb_lock_reserve
    event = tb_lock_reserve(trace_ring, length);

    if (!event) {
        // 缓冲区满或分配失败
        return NULL;
    }

    return event;
}

// tb_lock_reserve 实现 (简化版)
struct tb_event *tb_lock_reserve(struct tb_ring *ring, unsigned long length)
{
    struct tb_per_cpu *cpu_buffer;
    unsigned long tail;

    // 获取当前 CPU 的缓冲区
    cpu_buffer = &ring->buffer[smp_processor_id()];

    // 检查空间是否足够
    if (ring_buffer_space(cpu_buffer) < length) {
        if (ring->flags & TB_FL_OVERWRITE) {
            // 覆盖模式：丢弃最旧的事件
            discard_oldest_event(cpu_buffer);
        } else {
            // 非覆盖模式：返回失败
            return NULL;
        }
    }

    // 预留空间，更新写指针
    tail = local_read(&cpu_buffer->tail);
    // ... 分配空间

    return event;
}
```

### 4.2 事件提交 (Commit)

```c
// trace_commit 实现

void trace_commit(struct tb_event *event)
{
    struct tb_ring *ring = trace_ring;

    // [7.1] 提交事件
    tb_unlock_commit(ring);

    // [7.2] 唤醒等待的读取者
    tb_wake_up(ring);
}

// tb_unlock_commit 实现
int tb_unlock_commit(struct tb_ring *ring)
{
    struct tb_per_cpu *cpu_buffer = &ring->buffer[smp_processor_id()];

    // 更新写指针，使事件对读取者可见
    local_add(length, &cpu_buffer->write);

    return 0;
}
```

### 4.3 Per-CPU 缓冲区结构

```
┌────────────────────────────────────────────────────────────────────────────┐
│                       Per-CPU Ring Buffer Layout                           │
└────────────────────────────────────────────────────────────────────────────┘

  tb_ring
    ├── buffer[0]  ──────────────┐
    │   ├── write                │ CPU 0 的独立缓冲区
    │   ├── read                 │
    │   ├── commit_page          │
    │   └── reader_page          │
    │                            │
    ├── buffer[1]  ──────────────┤
    │   ├── write                │ CPU 1 的独立缓冲区
    │   ├── read                 │
    │   ├── commit_page          │
    │   └── reader_page          │
    │                            │
    ├── buffer[2]  ──────────────┤
    │   ├── write                │ CPU 2 的独立缓冲区
    │   ├── read                 │
    │   ├── commit_page          │
    │   └── reader_page          │
    │                            │
    └── buffer[N]  ──────────────┘
        ├── write                │ CPU N 的独立缓冲区
        ├── read                 │
        ├── commit_page          │
        └── reader_page          │

  特性:
  • 每个 CPU 独立管理，无锁写入
  • Producer 只操作本 CPU 的 write 指针
  • Consumer 循环读取所有 CPU 的缓冲区
```

---

## 5. 用户空间读取 (Userspace Reading)

### 5.1 读取流程详解

```c
// trace.c:271-369 trace_read_pipe() 实现简化版

static ssize_t trace_read_pipe(struct file *filp, char __user *ubuf,
                               size_t cnt, loff_t *ppos)
{
    struct print_event_iterator *iter = filp->private_data;
    ssize_t sret;

    // [9.1] 获取迭代器互斥锁
    mutex_lock(&iter->mutex);

    // [9.2] 尝试返回上次未读完的数据
    sret = trace_seq_to_user_sym(&iter->seq, ubuf, cnt);
    if (sret != -EBUSY)
        goto out;

    // [10] 等待数据
    sret = trace_wait_pipe(filp);
    if (sret <= 0)
        goto out;

    // [11] 查找下一个事件
    while (trace_next_entry_inc(iter) != NULL) {
        // [12] 格式化事件
        ret = print_trace_fmt_line(iter);
        if (ret == TRACE_TYPE_PARTIAL_LINE) {
            // 缓冲区满，停止读取
            break;
        }

        // [13] 标记事件已消费
        tb_consume(iter->ring, iter->cpu, &iter->ts, &iter->lost_events);

        // 检查是否读取了足够的数据
        if (__trace_seq_used(&iter->seq) >= cnt)
            break;
    }

    // [14] 复制数据到用户空间
    sret = trace_seq_to_user_sym(&iter->seq, ubuf, cnt);

out:
    mutex_unlock(&iter->mutex);
    return sret;
}
```

### 5.2 等待数据实现

```c
// trace.c:130-149

static int trace_wait_pipe(struct file *filp)
{
    struct print_event_iterator *iter = filp->private_data;
    int ret;

    // 检查是否有数据
    while (is_trace_empty(iter)) {
        // 非阻塞模式
        if (filp->f_flags & O_NONBLOCK)
            return -EAGAIN;

        // 阻塞模式：等待数据到达
        mutex_unlock(&iter->mutex);
        ret = tb_wait(iter->ring, TB_RING_ALL_CPUS, 0);
        mutex_lock(&iter->mutex);

        if (ret)
            return ret;

        // 检查是否收到致命信号
        if (fatal_signal_pending(current))
            return -ERESTARTSYS;
    }

    return 1;
}
```

### 5.3 多 CPU 事件排序

```c
// trace.c:183-227 __find_next_entry() 实现

static struct print_event_entry *__find_next_entry(struct print_event_iterator *iter,
                                                   int *ent_cpu, unsigned long *me,
                                                   u64 *ent_ts)
{
    struct tb_ring *ring = iter->ring;
    struct print_event_entry *ent = NULL;
    u64 ts, min_ts = U64_MAX;
    unsigned long lost_events = 0;
    int cpu, next_cpu = -1;
    int start = ent_cpu ? (*ent_cpu + 1) : 0;

    // 遍历所有可能的 CPU
    cpu = __cpumask_next_wrap(start - 1, cpu_possible_mask, start, 0);
    while (cpu < nr_cpumask_bits) {
        // 检查该 CPU 的缓冲区是否为空
        if (tb_empty_cpu(ring, cpu))
            goto next_cpu;

        // 查看该 CPU 的下一个事件
        ent = peek_next_entry(iter, cpu, &ts, &lost_events);
        if (ent && ts < min_ts) {
            // 找到更早的事件
            min_ts = ts;
            next_cpu = cpu;
        }

next_cpu:
        cpu = __cpumask_next_wrap(cpu, cpu_possible_mask, start, 1);
    }

    // 返回时间戳最小的事件
    if (next_cpu >= 0) {
        ent = peek_next_entry(iter, next_cpu, ent_ts, me);
        if (ent_cpu)
            *ent_cpu = next_cpu;
        return ent;
    }

    return NULL;
}
```

---

## 6. 数据格式详解 (Data Format Details)

### 6.1 事件数据结构

```c
// 通用事件上下文 (所有事件共享)
struct event_context {
    u64  data_type;      // 事件类型 ID
    u64  timestamp;      // 纳秒级时间戳
    u32  uid;            // 用户 ID
    u32  gid;            // 组 ID
    u32  pid;            // 进程 ID
    u32  tid;            // 线程 ID
    u32  session_id;     // 会话 ID
    u32  ppid;           // 父进程 ID
    char namespace[64];  // Namespace 信息
    char container_id[64]; // 容器 ID
    char exe_path[PATH_MAX]; // 可执行文件路径
    char cmdline[PATH_MAX * 2]; // 命令行参数
    char cwd[PATH_MAX];  // 当前工作目录
};
```

### 6.2 格式化示例

```c
// print_event.h 中的事件格式化宏

static int print_event_do_sys_open(struct trace_seq *s,
                                   struct print_event_entry *entry)
{
    struct event_context *ctx = (struct event_context *)entry->data;
    struct do_sys_open_data *data = (void *)(ctx + 1);

    // 输出格式：字段之间用 \x17 分隔
    trace_seq_printf(s, "%llu\x17", ctx->data_type);      // data_type
    trace_seq_printf(s, "%llu\x17", ctx->timestamp);      // timestamp
    trace_seq_printf(s, "%u\x17", ctx->uid);              // uid
    trace_seq_printf(s, "%u\x17", ctx->gid);              // gid
    trace_seq_printf(s, "%u\x17", ctx->pid);              // pid
    trace_seq_printf(s, "%u\x17", ctx->tid);              // tid
    trace_seq_printf(s, "%u\x17", ctx->session_id);       // session_id
    trace_seq_printf(s, "%u\x17", ctx->ppid);             // ppid
    trace_seq_printf(s, "%s\x17", ctx->namespace);        // namespace
    trace_seq_printf(s, "%s\x17", ctx->container_id);     // container_id
    trace_seq_printf(s, "%s\x17", ctx->exe_path);         // exe_path
    trace_seq_printf(s, "%s\x17", ctx->cmdline);          // cmdline
    trace_seq_printf(s, "%s\x17", ctx->cwd);              // cwd

    // 私有字段
    trace_seq_printf(s, "%s\x17", data->filename);        // filename
    trace_seq_printf(s, "%u\x17", data->flags);           // flags
    trace_seq_printf(s, "%u\x17", data->mode);            // mode

    // 记录结束
    trace_seq_printf(s, "\x1e");

    return 0;
}
```

### 6.3 用户空间解析示例

```python
# 用户空间解析事件数据的 Python 示例

def parse_event(data):
    """解析单个事件"""
    # 按记录分隔符分割
    records = data.split(b'\x1e')
    for record in records:
        if not record:
            continue

        # 按字段分隔符分割
        fields = record.split(b'\x17')

        if len(fields) >= 13:
            event = {
                'data_type': int(fields[0]),
                'timestamp': int(fields[1]),
                'uid': int(fields[2]),
                'gid': int(fields[3]),
                'pid': int(fields[4]),
                'tid': int(fields[5]),
                'session_id': int(fields[6]),
                'ppid': int(fields[7]),
                'namespace': fields[8].decode('utf-8', errors='ignore'),
                'container_id': fields[9].decode('utf-8', errors='ignore'),
                'exe_path': fields[10].decode('utf-8', errors='ignore'),
                'cmdline': fields[11].decode('utf-8', errors='ignore'),
                'cwd': fields[12].decode('utf-8', errors='ignore'),
            }

            # 解析私有字段
            if event['data_type'] == 2:  # do_sys_open
                if len(fields) >= 16:
                    event['filename'] = fields[13].decode('utf-8', errors='ignore')
                    event['flags'] = int(fields[14])
                    event['mode'] = int(fields[15])

            yield event

# 使用示例
with open('/proc/elkeid-endpoint', 'rb') as f:
    data = f.read(4096)
    for event in parse_event(data):
        print(f"Event: {event['data_type']}, PID: {event['pid']}, Exe: {event['exe_path']}")
```

---

## 7. 性能优化 (Performance Optimization)

### 7.1 零拷贝优化

```
┌────────────────────────────────────────────────────────────────────────────┐
│                         零拷贝路径优化                                      │
└────────────────────────────────────────────────────────────────────────────┘

传统路径 (多次拷贝):
  [Kprobe] → [Ring Buffer] → [内核缓冲区] → [用户空间]

优化路径 (减少拷贝):
  [Kprobe] → [Ring Buffer] → [用户空间]
             (直接读取，无需中间缓冲)
```

### 7.2 批量读取优化

```c
// 一次性读取多个事件，减少系统调用

while (trace_next_entry_inc(iter) != NULL) {
    // 格式化事件
    print_trace_fmt_line(iter);

    // 检查是否已读取足够数据
    if (__trace_seq_used(&iter->seq) >= cnt)
        break;  // 批量返回，减少系统调用次数
}
```

### 7.3 Per-CPU 并行写入

```
CPU 0  ──────────────────▶ Buffer 0 ──┐
                                   ├──▶ 用户空间循环读取
CPU 1  ──────────────────▶ Buffer 1 ──┤
                                   │
CPU 2  ──────────────────▶ Buffer 2 ──┤
                                   │
CPU N  ──────────────────▶ Buffer N ──┘

优势:
• 无锁写入 - 每个 CPU 独立操作
• 高并发 - 多个 CPU 同时写入
• 顺序保证 - 按 timestamp 排序输出
```

---

## 8. 错误处理 (Error Handling)

### 8.1 缓冲区满处理

```c
// trace_reserve() 中的缓冲区满处理

event = tb_lock_reserve(ring, length);
if (!event) {
    // 处理策略
    if (ring->flags & TB_FL_OVERWRITE) {
        // 覆盖模式：丢弃最旧的事件，为新事件腾出空间
        // 优点：不阻塞 Hook 回调
        // 缺点：可能丢失重要事件
        return NULL;
    } else {
        // 非覆盖模式：保留所有事件，丢弃新事件
        // 优点：不丢失已采集的事件
        // 缺点：新事件丢失
        return NULL;
    }
}
```

### 8.2 内存分配失败处理

```c
// 在原子上下文中只能使用 GFP_ATOMIC

event = smith_kzalloc(len, GFP_ATOMIC);
if (!event) {
    // 内存分配失败
    // 策略：丢弃事件，不阻塞系统
    atomic_inc(&dropped_events);
    return;
}
```

---

## 9. 总结 (Summary)

### 9.1 数据流关键点

1. **快速路径** - Hook 回调中的快速处理
   - 早期白名单过滤
   - 原子上下文安全操作
   - 零拷贝或最小拷贝

2. **可靠传递** - 环形缓冲区保证
   - Per-CPU 无锁设计
   - 覆盖/非覆盖模式可选
   - 时间戳排序保证顺序

3. **高效读取** - 批量读取优化
   - 一次系统调用读取多个事件
   - 用户空间高效解析
   - 非阻塞/阻塞模式支持

### 9.2 性能指标

| 指标 | 值 |
|------|-----|
| Hook 延迟 (TP99) | < 3.5us |
| 每个事件大小 | ~100-500 bytes |
| 缓冲区大小 (每 CPU) | 4MB (可配置) |
| 最大事件数 (每 CPU) | ~8000 (500字节事件) |

---

**文档版本**: 1.0
**最后更新**: 2024
**维护者**: Elkeid Team
