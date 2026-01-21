# init.c 代码详解 (init.c Code Walkthrough)

## 文件概述 (File Overview)

`init.c` 是 Elkeid Driver 的模块初始化入口文件，负责初始化三大子系统并处理错误恢复。

**文件路径:** `driver/LKM/src/init.c`
**代码行数:** 68 行
**主要功能:** 模块初始化和清理

---

## 1. 头文件包含 (Header Includes)

```c
// SPDX-License-Identifier: GPL-2.0
/*
 * init.c
 *
 * Here's the register of kprobes, kretprobes and tracepoints.
 */

#include "../include/kprobe.h"
```

**解析:**
- `SPDX-License-Identifier: GPL-2.0` - 声明代码使用 GPL v2 许可证
- 注释说明这是 kprobes, kretprobes 和 tracepoints 的注册文件
- 只包含 `kprobe.h`，因为需要使用 `KPROBE_INITCALL` 宏定义

---

## 2. 外部初始化函数声明 (External Init Function Declarations)

```c
/* Definions for global init/exit routines */
extern const struct kprobe_initcall KPROBE_CALL(trace);
extern const struct kprobe_initcall KPROBE_CALL(anti_rootkit);
extern const struct kprobe_initcall KPROBE_CALL(kprobe_hook);
```

**解析:**

这声明了三个外部符号，每个都是 `struct kprobe_initcall` 类型：

```c
// kprobe.h 中定义的结构体
struct kprobe_initcall {
    int (*init)(void);    // 初始化函数指针
    void (*exit)(void);   // 清理函数指针
};
```

| 符号 | 子系统 | 源文件 | 描述 |
|------|--------|--------|------|
| `trace` | 通信子系统 | `trace.c` | 环形缓冲区和 proc 接口 |
| `anti_rootkit` | 反 Rootkit | `anti_rootkit.c` | Rootkit 检测 |
| `kprobe_hook` | Hook 子系统 | `smith_hook.c` | Kprobe/Kretprobe 注册 |

**宏展开说明:**

```c
// KPROBE_CALL(trace) 展开为 smith_trace_init_body
// 这是通过 kprobe.h 中的宏定义实现的：
#define KPROBE_CALL(mod) smith_##mod##_init_body
```

---

## 3. 模块入口数组 (Module Entry Array)

```c
static const struct kprobe_initcall *__mod_entry[] =
{
    &KPROBE_CALL(trace),
    &KPROBE_CALL(anti_rootkit),
    &KPROBE_CALL(kprobe_hook),
};
```

**解析:**

这是一个静态数组，存储三个子系统的初始化结构体指针：

```
__mod_entry[0] = &smith_trace_init_body
__mod_entry[1] = &smith_anti_rootkit_init_body
__mod_entry[2] = &smith_kprobe_hook_init_body
```

**初始化顺序的重要性:**

1. **trace 必须最先初始化**
   - 提供环形缓冲区基础设施
   - 其他子系统依赖它来输出数据

2. **kprobe_hook 必须最后初始化**
   - 需要缓冲区已经准备好
   - 一旦初始化就会开始采集数据

3. **anti_rootkit 在中间**
   - 相对独立，但也需要 trace 子系统

---

## 4. 初始化函数 (Initialization Function)

```c
static int __init kprobes_init(void)
{
    int i, rc = 0;

    for (i = 0; i < ARRAY_SIZE(__mod_entry); i++) {
        const struct kprobe_initcall *kic = __mod_entry[i];
        if (kic && kic->init) {
            rc = kic->init();
            if (rc < 0)
                goto exit;
        }
    }

    return 0;

exit:
    while (i-- > 0) {
        const struct kprobe_initcall *kic = __mod_entry[i];
        if (kic && kic->exit)
            kic->exit();
    }

    return rc;
}
```

**详细解析:**

### 4.1 函数签名

```c
static int __init kprobes_init(void)
```

- `static` - 内部链接，不导出符号
- `int` - 返回状态码 (0 成功, 负数错误码)
- `__init` - 标记为初始化代码，加载后可释放内存

### 4.2 主循环 (Main Loop)

```c
for (i = 0; i < ARRAY_SIZE(__mod_entry); i++) {
    const struct kprobe_initcall *kic = __mod_entry[i];
    if (kic && kic->init) {
        rc = kic->init();
        if (rc < 0)
            goto exit;
    }
}
```

**执行流程:**

```
i=0: 调用 smith_trace_init()
      ├── 成功: rc = 0, 继续
      └── 失败: rc < 0, 跳转到 exit

i=1: 调用 smith_anti_rootkit_init()
      ├── 成功: rc = 0, 继续
      └── 失败: rc < 0, 跳转到 exit

i=2: 调用 smith_kprobe_hook_init()
      ├── 成功: rc = 0, 返回 0
      └── 失败: rc < 0, 跳转到 exit
```

### 4.3 错误处理 (Error Handling)

```c
exit:
    while (i-- > 0) {
        const struct kprobe_initcall *kic = __mod_entry[i];
        if (kic && kic->exit)
            kic->exit();
    }

    return rc;
```

**错误恢复示例:**

假设 `anti_rootkit_init()` 失败：

```
1. trace_init()         → 成功 (i=0)
2. anti_rootkit_init()  → 失败，rc = -ENOMEM (i=1)
3. 跳转到 exit

   while (i-- > 0):  # i 从 1 开始递减
       i=1: 调用 anti_rootkit_exit()  (但 init 未成功，exit 可能什么都不做)
       i=0: 调用 trace_exit()         (清理已分配的资源)

4. 返回 -ENOMEM
```

**设计模式:** 这是标准的 RAII (Resource Acquisition Is Initialization) 模式，确保部分初始化失败时正确清理已分配的资源。

---

## 5. 清理函数 (Cleanup Function)

```c
static void __exit kprobes_exit(void)
{
    int i;

    for (i = ARRAY_SIZE(__mod_entry) - 1; i >= 0; i--) {
        const struct kprobe_initcall *kic = __mod_entry[i];
        if (kic && kic->exit)
            kic->exit();
    }

    return;
}
```

**解析:**

### 5.1 反向顺序清理

```c
for (i = ARRAY_SIZE(__mod_entry) - 1; i >= 0; i--)
```

**执行顺序 (LIFO - Last In First Out):**

```
1. smith_kprobe_hook_exit()    # 最后初始化的最先清理
2. smith_anti_rootkit_exit()
3. smith_trace_exit()          # 最先初始化的最后清理
```

**为什么要反向清理:**

- `kprobe_hook` 依赖 `trace` 的缓冲区
- 必须先停止数据采集 (kprobe_hook_exit)
- 再清理缓冲区 (trace_exit)

### 5.2 模块元数据 (Module Metadata)

```c
module_init(kprobes_init);
module_exit(kprobes_exit);

MODULE_VERSION("1.7.0.24");
MODULE_LICENSE("GPL");
MODULE_INFO(homepage, "https://github.com/bytedance/Elkeid/tree/main/driver");
MODULE_AUTHOR("Elkeid Team <elkeid@bytedance.com>");
MODULE_DESCRIPTION("Elkied Driver is the core component of Elkeid HIDS project");
```

**宏说明:**

| 宏 | 作用 |
|-----|------|
| `module_init` | 指定模块加载时调用的函数 |
| `module_exit` | 指定模块卸载时调用的函数 |
| `MODULE_VERSION` | 模块版本号 |
| `MODULE_LICENSE` | 许可证 (GPL 是开源内核模块必须的) |
| `MODULE_INFO` | 自定义信息字段 |
| `MODULE_AUTHOR` | 作者信息 |
| `MODULE_DESCRIPTION` | 模块描述 |

---

## 6. 完整执行流程 (Complete Execution Flow)

### 6.1 模块加载流程 (insmod)

```
用户空间: insmod hids_driver.ko
    │
    ▼
内核: module_init() → kprobes_init()
    │
    ├──▶ trace_init()           [第1个初始化]
    │       │
    │       ├── tb_alloc()              # 分配环形缓冲区
    │       ├── proc_create_data()      # 创建 /proc/elkeid-endpoint
    │       └── 返回 0
    │
    ├──▶ anti_rootkit_init()   [第2个初始化]
    │       │
    │       ├── kthread_create()        # 创建检测线程
    │       ├── kallsyms_lookup_name()  # 查找内核符号
    │       └── 返回 0
    │
    └──▶ smith_hook_init()     [第3个初始化]
            │
            ├── register_kprobe()       # 注册 kprobe
            ├── register_kretprobe()    # 注册 kretprobe
            └── 返回 0

    返回 0 → 模块加载成功
```

### 6.2 模块卸载流程 (rmmod)

```
用户空间: rmmod hids_driver
    │
    ▼
内核: module_exit() → kprobes_exit()
    │
    ├──▶ smith_hook_exit()     [第1个清理 - 反向]
    │       │
    │       ├── unregister_kprobe()      # 注销 kprobe
    │       ├── unregister_kretprobe()   # 注销 kretprobe
    │       └── 返回
    │
    ├──▶ anti_rootkit_exit()   [第2个清理]
    │       │
    │       ├── kthread_stop()           # 停止检测线程
    │       └── 返回
    │
    └──▶ trace_exit()           [第3个清理]
            │
            ├── remove_proc_entry()      # 删除 /proc/elkeid-endpoint
            ├── tb_free()                # 释放环形缓冲区
            └── 返回

    模块卸载完成
```

---

## 7. 关键设计模式 (Key Design Patterns)

### 7.1 依赖注入模式

```c
// 通过数组顺序隐式定义依赖关系
static const struct kprobe_initcall *__mod_entry[] = {
    &KPROBE_CALL(trace),         // 基础设施，无依赖
    &KPROBE_CALL(anti_rootkit),  // 依赖 trace
    &KPROBE_CALL(kprobe_hook),   // 依赖 trace
};
```

### 7.2 错误处理模式

```c
// 标准的 "C 风格" 错误处理
for (初始化) {
    rc = 子系统_init();
    if (rc < 0)
        goto exit;  // 失败时跳转到清理代码
}

exit:
    while (i-- > 0)  // 反向清理已初始化的子系统
        子系统_exit();
```

### 7.3 模板方法模式

```c
// 使用宏统一初始化结构
#define KPROBE_INITCALL(mod, init_func, exit_func) \
    const struct kprobe_initcall KPROBE_CALL(mod) = { \
        .init = init_func, \
        .exit = exit_func, \
    };
```

---

## 8. 常见问题 (Common Questions)

### Q1: 为什么使用 `extern` 声明而不是直接 include 头文件?

**A:** 因为各子系统使用宏 `KPROBE_INITCALL` 生成符号，这个宏生成的符号名称不是标准的 C 标识符模式，不能通过头文件直接声明。

### Q2: 为什么 `__mod_entry` 是 `static const`?

**A:**
- `static` - 限制符号作用域在本文件
- `const` - 数据只读，可放入 .rodata 段

### Q3: 如果某个子系统的 init 返回正数会怎样?

**A:** 只有 `< 0` 被视为错误，正数会被当作成功处理。

---

## 9. 学习要点 (Learning Points)

1. **理解初始化顺序** - 为什么 trace 必须最先初始化
2. **理解错误恢复** - 如何在部分初始化失败时正确清理
3. **理解反向清理** - 为什么卸载时顺序相反
4. **理解模块元数据** - MODULE_* 宏的作用和意义

---

**下一步学习:** 阅读 `trace.c` 了解通信子系统的实现
