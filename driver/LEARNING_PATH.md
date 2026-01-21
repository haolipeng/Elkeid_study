# Elkeid Driver 学习路径 (Elkeid Driver Learning Path)

## 项目概述 (Project Overview)

Elkeid Driver 是一个基于 Linux 内核模块（LKM）的主机入侵检测系统（HIDS）驱动，使用 **Kprobe/Kretprobe** 技术实现内核函数钩子，**不使用 eBPF**。

Elkeid Driver is a Host Intrusion Detection System (HIDS) driver based on Linux Kernel Module (LKM), using **Kprobe/Kretprobe** technology for kernel function hooking, **without eBPF**.

---

## 学习前准备 (Prerequisites)

### 基础知识要求 (Required Knowledge)
- C 语言编程能力
- Linux 操作系统基础
- 基本的内核模块概念（可选，会同步学习）
- 英文文档阅读能力

### 环境准备 (Environment Setup)
```bash
# 建议使用虚拟机进行实验（推荐）
# Linux 内核版本：2.6.32 - 6.3
# 发行版：Ubuntu 18.04+, CentOS 7+, Debian 9+

# 安装内核开发包
sudo apt install linux-headers-$(uname -r) build-essential  # Ubuntu/Debian
sudo yum install kernel-devel kernel-headers gcc make        # CentOS/RHEL

# 克隆项目（如果尚未克隆）
git clone https://github.com/bytedance/Elkeid.git
cd Elkeid/driver
```

---

## 第一阶段：基础认知 (Foundation) - 1-2 周

### 目标 (Goals)
理解 Driver 是什么、做什么、为什么需要

### 学习资源 (Learning Resources)

#### 1. 阅读 README 文档
```bash
# 中文版（推荐）
driver/README-zh_CN.md

# 英文版
driver/README.md
```

**关键内容要点：**
- Driver 的作用和架构
- 支持的 Hook 类型列表（30+ 种）
- 数据协议格式（`\x17` 和 `\x1e` 分隔符）
- 性能基准测试结果
- 兼容性矩阵（内核 2.6.32 - 6.3）

#### 2. 理解关键概念
- 什么是 HIDS（主机入侵检测系统）
- 为什么需要在内核层面采集数据
- Kprobe vs eBPF 的区别
- 内核模块（LKM）基础

### 验收标准 (Checkpoint)
- [ ] 能说出 Driver 至少 5 种监控能力
- [ ] 理解数据协议的分隔符（`\x17` 和 `\x1e`）
- [ ] 了解 Driver 支持的 Linux 发行版
- [ ] 能解释为什么使用 Kprobe 而不是 eBPF

### 实践练习 (Exercises)
```bash
# 编译驱动模块
cd driver/LKM
make clean && make

# 查看生成的模块
ls -lh hids_driver.ko
modinfo hids_driver.ko

# 加载模块（可选，谨慎操作）
sudo insmod hids_driver.ko

# 检查模块是否加载成功
lsmod | grep hids

# 查看内核日志
dmesg | tail -20

# 卸载模块
sudo rmmod hids_driver
```

---

## 第二阶段：架构理解 (Architecture) - 2-3 周

### 目标 (Goals)
理解 Driver 的整体架构和组件交互

### 学习资源 (Learning Resources)

#### 1. 模块初始化代码
**文件：** `driver/LKM/src/init.c` (68 行)

**学习重点：**
```c
// 模块初始化入口
module_init(hids_driver_init);

// 三大子系统：
// 1. trace       - 追踪事件处理
// 2. anti_rootkit - 反 Rootkit 检测
// 3. kprobe_hook - Kprobe 钩子注册

// 错误处理模式
KPROBE_INITCALL(trace_init()) ||
KPROBE_INITCALL(anti_rootkit_init()) ||
KPROBE_INITCALL(smith_hook_init());
```

#### 2. 用户空间插件文档
**文件：** `plugins/driver/README.md`

**关键内容：**
- 用户空间插件如何与 Driver 交互
- 数据过滤和补充机制

#### 3. 关键文件结构
```
driver/LKM/
├── src/
│   ├── init.c           # 模块初始化 (68 行)
│   ├── smith_hook.c     # 主要 Hook 实现 (5,101 行)
│   ├── trace_buffer.c   # 环形缓冲区管理 (4,376 行)
│   ├── trace.c          # Trace 事件处理 (503 行)
│   ├── anti_rootkit.c   # 反 Rootkit 检测 (333 行)
│   ├── filter.c         # 白名单过滤 (578 行)
│   ├── memcache.c       # 无锁内存池 (337 行)
│   └── util.c           # 工具函数 (137 行)
└── include/
    ├── kprobe.h         # Kprobe 模板宏定义
    ├── smith_hook.h     # Hook 定义和内核兼容性
    ├── trace_buffer.h   # 环形缓冲区结构
    ├── print_event.h    # 事件打印宏
    └── ...
```

#### 4. 两个通信通道
**1. 数据流通道：** `/proc/elkeid-endpoint` (只读)
```bash
# 用户空间读取安全事件
cat /proc/elkeid-endpoint
```

**2. 配置通道：** `/dev/hids_driver_allowlist` (写)
```bash
# 配置白名单和过滤规则
echo "Y/path/to/binary" > /dev/hids_driver_allowlist
echo "N/sensitive/path" > /dev/hids_driver_allowlist
```

### 验收标准 (Checkpoint)
- [ ] 能画出 Driver 的组件交互图
- [ ] 解释两个通信通道的作用
- [ ] 理解 per-CPU 环形缓冲区的设计
- [ ] 能解释模块初始化的三条错误处理路径

### 实践练习 (Exercises)
```bash
# 1. 分析 Makefile 理解编译过程
cat driver/LKM/Makefile

# 2. 查看生成的目标文件和符号
nm hids_driver.ko | grep -E "(init|trace|hook)"

# 3. 使用 strace 观察用户空间读取过程
strace -e read cat /proc/elkeid-endpoint
```

---

## 第三阶段：核心技术深入 (Core Technology) - 3-4 周

### 目标 (Goals)
掌握 Kprobe/Kretprobe 技术和 Hook 实现原理

### 学习资源 (Learning Resources)

#### 1. Kprobe 教程
**文件：** `driver/LKM/KPROBE.md`

**核心内容：**
- Kprobe/Kretprobe API 使用教程
- 宏定义：`KPROBE_HANDLER_DEFINE0-6`
- 代码示例：do_sys_open 钩子
- Tracepoint 使用方法

#### 2. Hook 实现代码
**文件：** `driver/LKM/src/smith_hook.c` (5,101 行)

**学习策略：**
1. 先理解宏定义模板（`include/kprobe.h`）
2. 阅读简单的 Hook 示例（如 do_sys_open）
3. 逐步学习复杂的 Hook 实现

#### 3. Kprobe 技术要点
```c
// Kprobe - 函数入口钩子
KPROBE_HANDLER_DEFINE3(do_sys_open, df, at, flags)
{
    // 参数：
    // df    - 文件名字符串指针
    // at    - 访问模式
    // flags - 标志位

    // 注意：在原子上下文中，不能调用可能睡眠的函数
    // 只能使用 GFP_ATOMIC 分配内存
}

// Kretprobe - 函数返回钩子
KRETPROBE_HANDLER_DEFINE(do_sys_open, df, ret)
{
    // 参数：
    // df  - 入口参数（通过 entry_get 获取）
    // ret - 返回值（文件描述符或错误码）
}
```

#### 4. 40+ Hook 类型分类

| 分类 | Hook 类型 | 数据类型 ID |
|------|-----------|-------------|
| **网络监控** | connect, bind, accept, dns_query | 400-405 |
| **进程监控** | execve, exit, kill, ptrace | 100-105 |
| **文件操作** | open, write, rename, link, unlink | 1-8 |
| **安全相关** | cred_change, cap_raised, module_load | 500-510 |
| **系统操作** | mount, memfd_create, setsid, prctl | 200-210 |
| **反 Rootkit** | syscall_hook_check, hidden_module | 700-703 |

### 验收标准 (Checkpoint)
- [ ] 能解释 Kprobe 和 Kretprobe 的区别
- [ ] 理解在原子上下文中的安全操作限制
- [ ] 能读懂一个 Hook 的实现代码
- [ ] 能说出至少 10 种不同的 Hook 类型

### 实践练习 (Exercises)
```bash
# 1. 查看已注册的 kprobe 探针
cat /sys/kernel/debug/tracing/kprobe_events 2>/dev/null || echo "需要 root 权限"

# 2. 查看 tracepoint 列表
ls /sys/kernel/debug/tracing/events/ 2>/dev/null

# 3. 阅读 smith_hook.c 中的具体实现
# 搜索特定函数的钩子实现
grep -n "do_sys_open" driver/LKM/src/smith_hook.c
grep -n "do_execve" driver/LKM/src/smith_hook.c
```

---

## 第四阶段：数据流与格式 (Data Flow) - 2 周

### 目标 (Goals)
理解 Driver 如何收集、格式化和输出数据

### 学习资源 (Learning Resources)

#### 1. 数据格式文档
**文件：** `docs/ElkeidData/raw_data_desc.md`

**关键内容：**
- 1-612 和 700-703 号数据类型的字段定义
- 13 个通用字段的含义

#### 2. 事件打印系统
**文件：** `driver/LKM/include/print_event.h`

**数据格式详解：**
```
通用数据结构（13 字段）：
├── data_type       # 事件类型 ID (1-612, 700-703)
├── timestamp       # 时间戳（纳秒）
├── uid             # 用户 ID
├── gid             # 组 ID
├── pid             # 进程 ID
├── tid             # 线程 ID
├── session_id      # 会话 ID
├── ppid            # 父进程 ID
├── namespace       # 容器 namespace 信息
├── container_id    # 容器 ID
├── exe_path        # 可执行文件路径
├── cmdline         # 命令行参数
└── cwd             # 当前工作目录

私有数据：事件类型特定字段
└── [根据 data_type 不同而不同]
```

#### 3. 事件处理流程
```
1. Kprobe/tracepoint 触发
   ↓
2. 收集事件数据
   - 从寄存器/栈读取参数
   - 获取进程上下文信息
   ↓
3. 预留环形缓冲区空间
   - trace_reserve()
   ↓
4. 格式化数据并提交
   - print_event_xxx()
   ↓
5. 用户空间读取
   - /proc/elkeid-endpoint
```

### 验收标准 (Checkpoint)
- [ ] 能解析一条事件数据的各个字段
- [ ] 理解事件的产生到消费的完整流程
- [ ] 能识别不同类型事件的私有字段

### 实践练习 (Exercises)
```bash
# 1. 读取原始事件数据
hexdump -C /proc/elkeid-endpoint | head -20

# 2. 使用 Python 解析数据格式
# 参考: docs/ElkeidData/data_import/raw_data_usage_tutorial.md

# 3. 分析数据分隔符
# 查找 \x17 (字段分隔) 和 \x1e (记录分隔)
```

---

## 第五阶段：高级主题 (Advanced Topics) - 3-4 周

### 目标 (Goals)
深入理解内核级安全和性能优化

### 学习资源 (Learning Resources)

#### 1. 反 Rootkit 检测
**文件：** `driver/LKM/src/anti_rootkit.c`

**检测技术：**
1. **隐藏模块检测** - 对比 /proc/modules 与内部模块链表
2. **Syscall 表钩子检测** - 检查系统调用表是否被修改
3. **中断处理钩子检测** - 检查 IDT 完整性
4. **Proc 文件系统钩子检测** - 检测 /proc 目录劫持

#### 2. 环形缓冲区实现
**文件：** `driver/LKM/src/trace_buffer.c`

**技术要点：**
- Per-CPU 无锁设计
- 内存预分配避免动态分配
- 单生产者单消费者模型
- TP99 < 3.5us 的延迟控制

#### 3. 兼容性处理
**文件：** `driver/DOC/Description_of_Elkeid's_Crash_caused_by_fput_in_low_version_Kernel.md`

**关键问题：**
- 不同内核版本的 API 差异
- fput() 在低版本内核中的崩溃问题
- 跨架构支持（x86_64, ARM64, RISC-V）

### 验收标准 (Checkpoint)
- [ ] 理解 Driver 如何检测 Rootkit
- [ ] 了解性能优化的关键技术
- [ ] 能分析内核兼容性问题的原因
- [ ] 理解 per-CPU 设计的优势

### 实践练习 (Exercises)
```bash
# 1. 测试反 Rootkit 功能
# 需要加载测试用的 rootkit 模块（谨慎操作）

# 2. 性能测试
# 使用 ftrace 测量 kprobe 回调延迟

# 3. 阅读兼容性代码
grep -n "LINUX_VERSION_CODE" driver/LKM/src/*.c
```

---

## 第六阶段：实践与实验 (Practice) - 持续

### 实验建议

#### 1. 完整编译流程
```bash
cd driver/LKM
make clean && make
sudo insmod hids_driver.ko
dmesg | tail -10
```

#### 2. 读取事件数据
```bash
# 使用测试程序
cd driver/test
make
./rst

# 或直接读取
cat /proc/elkeid-endpoint | od -c | head -50
```

#### 3. 配置白名单
```bash
# 添加到白名单
echo "Y/usr/bin/ssh" > /dev/hids_driver_allowlist

# 添加黑名单
echo "N/tmp/malware" > /dev/hids_driver_allowlist

# 删除规则
echo "D/usr/bin/ssh" > /dev/hids_driver_allowlist
```

#### 4. 源码阅读顺序
1. `init.c` - 理解模块初始化
2. `trace.c` - 理解用户空间通信
3. `filter.c` - 理解配置管理
4. `KPROBE.md` + `smith_hook.c` - 理解 Hook 实现
5. `trace_buffer.c` - 理解数据结构

---

## 关键文件清单 (Key Files)

| 文件 | 用途 | 优先级 | 难度 |
|------|------|--------|------|
| `driver/README.md` | 主文档 | ⭐⭐⭐⭐⭐ | ⭐ |
| `driver/LKM/KPROBE.md` | Kprobe 教程 | ⭐⭐⭐⭐⭐ | ⭐⭐ |
| `driver/LKM/src/init.c` | 初始化入口 | ⭐⭐⭐⭐⭐ | ⭐ |
| `driver/LKM/src/smith_hook.c` | Hook 实现 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| `driver/LKM/include/print_event.h` | 事件系统 | ⭐⭐⭐⭐ | ⭐⭐⭐ |
| `driver/LKM/src/trace_buffer.c` | 环形缓冲区 | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| `driver/LKM/src/anti_rootkit.c` | Rootkit 检测 | ⭐⭐⭐ | ⭐⭐⭐ |
| `driver/LKM/src/filter.c` | 白名单过滤 | ⭐⭐⭐ | ⭐⭐ |
| `driver/LKM/src/trace.c` | 通信机制 | ⭐⭐⭐⭐ | ⭐⭐ |
| `docs/ElkeidData/raw_data_desc.md` | 数据格式 | ⭐⭐⭐⭐ | ⭐⭐ |

---

## 学习检查点 (Learning Checklist)

### 第一阶段检查
- [ ] 能解释 Elkeid Driver 的作用和基本原理
- [ ] 能编译和加载驱动模块
- [ ] 理解 Kprobe 和 eBPF 的区别

### 第二阶段检查
- [ ] 能画出组件交互图
- [ ] 理解通信机制
- [ ] 能阅读 init.c 并理解每行代码

### 第三阶段检查
- [ ] 能读懂并编写简单的 Kprobe Hook
- [ ] 理解原子上下文的限制
- [ ] 能说出至少 10 种 Hook 类型

### 第四阶段检查
- [ ] 能解析事件数据的完整结构
- [ ] 理解数据流全过程
- [ ] 能编写数据解析脚本

### 第五阶段检查
- [ ] 理解内核兼容性处理原理
- [ ] 理解性能优化技术
- [ ] 能分析反 Rootkit 检测机制

### 第六阶段检查
- [ ] 能独立编译、加载和测试 Driver
- [ ] 能分析新 Hook 的实现需求
- [ ] 能参与 Driver 的开发贡献

---

## 学习时间估算 (Time Estimation)

| 学习深度 | 预计时间 | 说明 |
|----------|----------|------|
| **快速浏览** | 2-3 周 | 阅读文档，理解概念，能运行示例 |
| **深入学习** | 2-3 个月 | 源码阅读，实验验证，理解核心实现 |
| **精通掌握** | 6 个月+ | 实际开发，问题排查，贡献代码 |

### 每周学习时间建议
- **快速浏览**：5-10 小时/周
- **深入学习**：10-20 小时/周
- **精通掌握**：20+ 小时/周

---

## 学习资源推荐 (Recommended Resources)

### 官方资源
- [Elkeid GitHub](https://github.com/bytedance/Elkeid)
- [Elkeid 官方文档](https://bytedance.github.io/Elkeid/)

### 内核开发参考
- [Linux Kernel Documentation](https://www.kernel.org/doc/html/latest/)
- [Kprobe Documentation](https://www.kernel.org/doc/html/latest/trace/kprobes.html)

### 推荐书籍
- 《Linux Kernel Development》(Robert Love)
- 《Understanding the Linux Kernel》(Daniel P. Bovet)
- 《Linux Device Drivers》(Jonathan Corbet)

---

## 常见问题 (FAQ)

### Q1: 为什么 Elkeid 不使用 eBPF？
**A:** eBPF 在 Elkeid 开发初期（2019-2020）还不够成熟，对老版本内核（2.6.32）的支持有限。Kprobe 是更成熟、兼容性更好的方案。

### Q2: 学习这个驱动需要什么背景？
**A:** 需要扎实的 C 语言基础，了解 Linux 系统调用，最好有简单的内核模块开发经验。

### Q3: 如何调试内核模块？
**A:** 主要使用 `dmesg` 查看日志，`printk` 输出调试信息，配合 ftrace 进行性能分析。

### Q4: 编译失败怎么办？
**A:** 检查内核头文件版本，确保 `linux-headers-$(uname -r)` 已安装，查看 Makefile 中的编译选项。

### Q5: 如何参与开发？
**A:** 先从阅读源码、修复小 bug 开始，逐步熟悉代码后可以提交 PR 添加新功能。

---

## 版本历史 (Version History)

- **v1.0** (2024) - 初始学习路径文档
- 基于 Elkeid Driver v1.7.0.24

---

## 许可证 (License)

本文档遵循与 Elkeid 项目相同的许可证（Apache License 2.0）。

---

**祝学习顺利！Happy Learning!**
