# anti_rootkit.c 代码详解 (anti_rootkit.c Code Walkthrough)

## 文件概述 (File Overview)

`anti_rootkit.c` 实现 Rootkit 检测功能，基于 Nick Bulischeck 的 Tyton 项目。它检测四种类型的 Rootkit 技术。

**文件路径:** `driver/LKM/src/anti_rootkit.c`
**代码行数:** 333 行
**主要功能:** 检测隐藏模块、Syscall 表钩子、IDT 钩子、Proc 文件系统钩子

---

## 1. 头文件和常量定义 (Headers and Constants)

```c
#define ANTI_ROOTKIT_CHECK 1
#if ANTI_ROOTKIT_CHECK

#include <linux/kthread.h>
#include "../include/util.h"
#include "../include/trace.h"
#include "../include/kprobe.h"
#include "../include/anti_rootkit.h"

#define CREATE_PRINT_EVENT
#include "../include/anti_rootkit_print.h"

#define DEFERRED_CHECK_TIMEOUT (15 * 60)
```

**解析:**

- `ANTI_ROOTKIT_CHECK` - 条件编译开关，设置为 1 启用反 Rootkit
- `linux/kthread.h` - 内核线程支持
- `anti_rootkit_print.h` - 事件打印宏（仅在 CREATE_PRINT_EVENT 定义时包含）
- `DEFERRED_CHECK_TIMEOUT` - 检测间隔：15 分钟 (900 秒)

---

## 2. 数据结构 (Data Structures)

### 2.1 内核符号指针

```c
static int (*ckt) (unsigned long addr) = NULL;  // 检查地址是否在内核代码段
```

### 2.2 IDT (Interrupt Descriptor Table) 相关 (x86)

```c
#ifdef CONFIG_X86
#include <asm/unistd.h>      // NR_syscalls
#include <asm/desc_defs.h>   // gate_desc
static gate_desc *idt = NULL;  // 中断描述符表指针
```

**IDT 结构:**

```
IDT (Interrupt Descriptor Table)
├── Gate Descriptor 0  → Divide Error ISR
├── Gate Descriptor 1  → Debug ISR
├── Gate Descriptor 2  → NMI ISR
├── ...
├── Gate Descriptor 128 → System Call (int 0x80)
└── Gate Descriptor 255 → ISP ISR
```

### 2.3 获取 ISR 地址

```c
static inline unsigned long get_isr_addr(const gate_desc *g)
{
#ifdef CONFIG_X86_64
    return g->offset_low | ((unsigned long)g->offset_middle << 16) |
           ((unsigned long) g->offset_high << 32);
#else
    return g->offset_low | ((unsigned long)g->offset_middle << 16);
#endif
}
```

**解析:**

x86-64 的 Gate Descriptor 格式 (16 字节):

```
┌─────────────────────────────────────────────────────────┐
│ offset_low (16) | selector (16) | ist (3) | type (5)    │
├─────────────────────────────────────────────────────────┤
│ offset_middle (16) | attr (8) | offset_high (32)        │
└─────────────────────────────────────────────────────────┘

ISR 地址 = offset_high << 32 | offset_middle << 16 | offset_low
```

### 2.4 Syscall 表和模块相关

```c
static unsigned long *sct = NULL;           // System Call Table 指针
static struct kset *mod_kset = NULL;        // Module kset 指针
static struct mutex *mod_lock = NULL;       // Module mutex 指针
struct module *(*mod_find_module)(const char *name);      // find_module 函数指针
static struct module *(*get_module_from_addr)(unsigned long addr); // __module_address
```

---

## 3. 隐藏模块检测 (Hidden Module Detection)

### 3.1 地址范围检查宏

```c
#define BETWEEN_PTR(x, y, z) ( \
    ((uintptr_t)x >= (uintptr_t)y) && \
    ((uintptr_t)x < ((uintptr_t)y+(uintptr_t)z)) \
)
```

**解析:**

检查 `x` 是否在范围 `[y, y+z)` 内。

### 3.2 查找隐藏模块

```c
static const char *find_hidden_module(unsigned long addr)
{
    const char *mod_name = NULL;
    struct kobject *cur;
    struct module_kobject *kobj;

    if (unlikely(!mod_kset))
        return NULL;

    spin_lock(&mod_kset->list_lock);
    list_for_each_entry(cur, &mod_kset->list, entry) {
        if (!kobject_name(cur))
            break;

        kobj = container_of(cur, struct module_kobject, kobj);
        if (!kobj || !kobj->mod)
            continue;

#if defined(KMOD_MODULE_MEM)
        if (BETWEEN_PTR(addr, kobj->mod->mem[MOD_TEXT].base,
                       kobj->mod->mem[MOD_TEXT].size))
            mod_name = kobj->mod->name;
#elif defined(KMOD_CORE_LAYOUT) || LINUX_VERSION_CODE >= KERNEL_VERSION(4, 5, 0)
        if (BETWEEN_PTR(addr, kobj->mod->core_layout.base,
                       kobj->mod->core_layout.size))
            mod_name = kobj->mod->name;
#else
        if (BETWEEN_PTR(addr, kobj->mod->module_core,
                       kobj->mod->core_size))
            mod_name = kobj->mod->name;
#endif
    }
    spin_unlock(&mod_kset->list_lock);

    return mod_name;
}
```

**原理:**

1. 遍历内核模块 kset 链表
2. 检查地址 `addr` 是否在某个模块的代码段范围内
3. 如果找到，返回模块名

**检测方法:**

```
正常情况:
  /proc/modules 显示模块列表
  通过 module_kset 遍历也能找到

Rootkit 隐藏模块:
  /proc/modules 中不显示
  但 module_kset 链表中仍然存在
  因此可以通过遍历 module_kset 检测到
```

### 3.3 模块列表锁定

```c
static void module_list_lock(void)
{
    if (likely(mod_lock))
        mutex_lock(mod_lock);
}

static void module_list_unlock(void)
{
    if (likely(mod_lock))
        mutex_unlock(mod_lock);
}
```

---

## 4. Syscall 表钩子检测 (Syscall Table Hook Detection)

```c
static void analyze_syscalls(void)
{
    int i;
    unsigned long addr;
    struct module *mod;

    if (!sct || !ckt)
        return;

    for (i = 0; i < NR_syscalls; i++) {
        const char *mod_name = "-1";
        addr = sct[i];

        if (!ckt(addr)) {  // 地址不在内核代码段
            module_list_lock();
            mod = get_module_from_addr(addr);
            if (mod) {
                mod_name = mod->name;
            } else {
                const char* name = find_hidden_module(addr);
                if (IS_ERR_OR_NULL(name)) {
                    module_list_unlock();
                    continue;
                }
                mod_name = name;
            }

            syscall_print(mod_name, i);  // 输出事件
            module_list_unlock();
        }
    }
}
```

**检测原理:**

```
正常情况:
  syscall_table[i] → 内核代码段地址
  ckt(addr) 返回 true

被钩子的情况:
  syscall_table[i] → 恶意模块地址
  ckt(addr) 返回 false
```

**检测流程:**

```
1. 遍历系统调用表 (0 到 NR_syscalls-1)
2. 获取每个系统调用的处理函数地址
3. 使用 ckt() 检查地址是否在内核代码段
4. 如果不在：
   a. 尝试通过 __module_address 找到所属模块
   b. 如果失败，使用 find_hidden_module 查找隐藏模块
   c. 输出检测事件
```

---

## 5. 中断钩子检测 (Interrupt Hook Detection)

```c
static void analyze_interrupts(void)
{
#ifdef CONFIG_X86
    int i;
    unsigned long addr;
    struct module *mod;

    if (!idt || !ckt)
        return;

    for (i = 0; i < IDT_ENTRIES; i++) {
        const char *mod_name = "-1";

        addr = get_isr_addr(&idt[i]);
        if (addr && !ckt(addr)) {  // ISR 地址不在内核代码段
            module_list_lock();

            mod = get_module_from_addr(addr);
            if (mod) {
                mod_name = mod->name;
            } else {
                const char *name = find_hidden_module(addr);
                if (IS_ERR_OR_NULL(name)) {
                    module_list_unlock();
                    continue;
                }
                mod_name = name;
            }

            interrupts_print(mod_name, i);  // 输出事件
            module_list_unlock();
        }
    }
#endif
}
```

**检测原理:**

与 Syscall 表钩子检测类似，但检查的是 IDT 表中的中断处理函数 (ISR)。

---

## 6. Proc 文件系统钩子检测 (Proc Filesystem Hook Detection)

```c
static void analyze_fops(void)
{
    struct module *mod = NULL;
    unsigned long addr;
    const char *mod_name;
    struct file *fp;

    fp = filp_open("/proc", O_RDONLY, S_IRUSR);
    if (IS_ERR_OR_NULL(fp)) {
        printk(KERN_INFO "[ELKEID] open /proc error\n");
        return;
    }

    if (IS_ERR_OR_NULL(fp->f_op)) {
        printk(KERN_INFO "[ELKEID] /proc has no fops\n");
        filp_close(fp, NULL);
        return;
    }

#if defined(SMITH_FS_OP_ITERATE)
    addr = (unsigned long)fp->f_op->iterate;
#elif defined(SMITH_FS_OP_ITERATE_SHARED)
    addr = (unsigned long)fp->f_op->iterate_shared;
#else
    addr = (unsigned long)fp->f_op->readdir;
#endif

    if (!ckt(addr)) {  // f_op 函数指针被劫持
        module_list_lock();
        if (get_module_from_addr)
            mod = get_module_from_addr(addr);
        mod_name = mod ? mod->name : find_hidden_module(addr);
        if (!IS_ERR_OR_NULL(mod_name))
            fops_print(mod_name);
        module_list_unlock();
    }
    filp_close(fp, NULL);
}
```

**检测原理:**

Rootkit 可能通过替换 `/proc` 文件系统的操作函数来隐藏文件或进程。这里检查 `iterate`/`readdir` 函数指针是否指向内核代码段。

---

## 7. 模块隐藏检测 (Module Hiding Detection)

```c
static void analyze_modules(void)
{
    struct kobject *cur;
    struct module_kobject *kobj;

    if (unlikely(!mod_kset))
        return;

    module_list_lock();
    spin_lock(&mod_kset->list_lock);
    list_for_each_entry(cur, &mod_kset->list, entry) {
        if (!kobject_name(cur)) {
            break;
        }

        kobj = container_of(cur, struct module_kobject, kobj);
        if (kobj && kobj->mod) {
            // 检查模块是否能通过 find_module 找到
            if (mod_find_module && !mod_find_module(kobj->mod->name))
                mod_print(kobj->mod->name);  // 输出隐藏模块事件
        }
    }
    spin_unlock(&mod_kset->list_lock);
    module_list_unlock();
}
```

**检测原理:**

```
正常模块:
  module_kset 链表中有 ✓
  find_module() 能找到 ✓

隐藏模块:
  module_kset 链表中有 ✓
  find_module() 找不到 ✗ (Rootkit 修改了 find_module)
```

---

## 8. 主检测函数 (Main Detection Function)

```c
static void anti_rootkit_check(void)
{
    analyze_fops();        // 1. Proc 文件系统钩子检测
    analyze_syscalls();    // 2. Syscall 表钩子检测
    analyze_modules();     // 3. 隐藏模块检测
    analyze_interrupts();  // 4. IDT 钩子检测
}
```

---

## 9. 工作线程 (Worker Thread)

### 9.1 线程函数

```c
static struct task_struct *g_worker_thread;

static int anti_rootkit_worker(void *argv)
{
    unsigned long timeout = msecs_to_jiffies(DEFERRED_CHECK_TIMEOUT * 1000);

    do {
        /* waiting 15 minutes, or being waken up */
        if (!schedule_timeout_interruptible(timeout)) {
            /* perform rootkit detection */
            anti_rootkit_check();
        }
    } while (!kthread_should_stop());

    return 0;
}
```

**解析:**

- `schedule_timeout_interruptible()` - 可中断的睡眠
- 每 15 分钟执行一次检测
- `kthread_should_stop()` - 检查是否应该停止

### 9.2 线程启动

```c
static int __init anti_rootkit_start(void)
{
    int rc = 0;

    g_worker_thread = kthread_create(anti_rootkit_worker, 0, "elkeid - antirootkit");
    if (IS_ERR(g_worker_thread)) {
        rc = g_worker_thread ? PTR_ERR(g_worker_thread) : -ENOMEM;
        printk("anti_rootkit_start: failed creating anti-rootkit worker: %d\n", rc);
        return rc;
    }

    /* wake up anti-rootkit worker thread */
    if (!wake_up_process(g_worker_thread)) {
        kthread_stop(g_worker_thread);
        g_worker_thread = NULL;
    }
    return rc;
}
```

---

## 10. 初始化和清理 (Initialization and Cleanup)

### 10.1 初始化

```c
static int __init anti_rootkit_init(void)
{
    struct kset **kset;

#ifdef CONFIG_X86
    idt = (void *)smith_kallsyms_lookup_name("idt_table");
#endif
    sct = (void *)smith_kallsyms_lookup_name("sys_call_table");
    ckt = (void *)smith_kallsyms_lookup_name("core_kernel_text");
    mod_lock = (void *)smith_kallsyms_lookup_name("module_mutex");
    mod_find_module = (void *)smith_kallsyms_lookup_name("find_module");
    get_module_from_addr = (void *)smith_kallsyms_lookup_name("__module_address");
    kset = (void *)smith_kallsyms_lookup_name("module_kset");
    if (kset)
        mod_kset = *kset;

    /* start rootkit-detection worker thread */
    anti_rootkit_start();

    printk("[ELKEID] ANTI_ROOTKIT_CHECK: %d\n", ANTI_ROOTKIT_CHECK);
    return 0;
}
```

**内核符号查找:**

| 符号 | 作用 |
|------|------|
| `idt_table` | IDT 表地址 |
| `sys_call_table` | 系统调用表 |
| `core_kernel_text` | 检查是否为内核代码段 |
| `module_mutex` | 模块列表互斥锁 |
| `find_module` | 查找模块 |
| `__module_address` | 地址到模块 |
| `module_kset` | 模块 kset |

### 10.2 清理

```c
static void anti_rootkit_exit(void)
{
    /* kthread_stop will wait until worker thread exits */
    if (!IS_ERR_OR_NULL(g_worker_thread)) {
        kthread_stop(g_worker_thread);
    }
}
```

---

## 11. 数据类型定义 (Data Type Definitions)

根据 README.md，反 Rootkit 检测产生的数据类型：

| 数据类型 ID | 检测类型 | 默认状态 |
|------------|----------|----------|
| 700 | proc file hook | ON |
| 701 | syscall table hook | ON |
| 702 | hidden kernel module | ON |
| 703 | interrupt table hook | ON |

---

## 12. 检测流程图 (Detection Flow)

```
┌─────────────────────────────────────────────────────────────────┐
│                  Anti-Rootkit Detection Flow                   │
└─────────────────────────────────────────────────────────────────┘

                          模块加载
                             │
                             ▼
                    ┌─────────────────┐
                    │ kthread_create  │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  工作/睡眠循环    │
                    │  (15分钟间隔)    │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ anti_rootkit_   │
                    │   check()       │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
  ┌──────────┐        ┌──────────┐        ┌──────────┐
  │analyze_  │        │analyze_  │        │analyze_  │
  │fops()    │        │syscalls()│        │modules() │
  └─────┬────┘        └─────┬────┘        └─────┬────┘
        │                   │                   │
        ▼                   ▼                   ▼
  ┌──────────┐        ┌──────────┐        ┌──────────┐
  │检查 /proc │        │遍历 SCT   │        │遍历 mod_  │
  │f_op 指针 │        │检查地址   │        │kset      │
  └─────┬────┘        └─────┬────┘        └─────┬────┘
        │                   │                   │
        └────────────────────┼────────────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  analyze_       │
                    │  interrupts()   │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ 检查 IDT 表     │
                    │ ISR 地址        │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ 输出检测事件     │
                    │ (Data 700-703)  │
                    └─────────────────┘
```

---

## 13. 学习要点 (Learning Points)

1. **内核符号查找** - 如何使用 kallsyms_lookup_name 获取未导出的内核符号
2. **模块隐藏原理** - Rootkit 如何隐藏模块，如何检测
3. **函数钩子检测** - 如何检测 syscall 表、IDT 表、f_op 的钩子
4. **内核线程** - 如何创建和使用内核工作线程
5. **内核兼容性** - 如何处理不同内核版本的结构差异

---

**下一步学习:** 阅读 `memcache.c` 了解无锁内存池实现
