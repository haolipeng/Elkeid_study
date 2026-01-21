# Elkeid Driver 学习检查清单 (Elkeid Driver Learning Checklist)

## 使用说明 (How to Use)

本检查清单用于跟踪学习进度。每个阶段完成后，在对应的复选框中标记 `[x]`。

This checklist is used to track learning progress. Mark `[x` when each stage is completed.

---

## 第一阶段：基础认知 (Foundation)

### 阅读理解 (Reading)

- [ ] 阅读 `/driver/README.md`
- [ ] 阅读 `/driver/README-zh_CN.md` (中文版)
- [ ] 理解 HIDS 的基本概念
- [ ] 理解 Kprobe vs eBPF 的区别

### 概念验证 (Concept Verification)

- [ ] 能说出 Driver 至少 5 种监控能力
  - 示例: execve, connect, open, mount, load_module

- [ ] 理解数据协议分隔符
  - 字段分隔符: `\x17` (ASCII 23)
  - 记录分隔符: `\x1e` (ASCII 30)

- [ ] 了解 Driver 支持的 Linux 发行版
  - Debian 8-10
  - Ubuntu 14.04+
  - CentOS/RHEL 6-8
  - Amazon Linux 2
  - 内核版本: 2.6.32 - 6.3

### 实践练习 (Exercises)

- [ ] 成功编译 Driver
- [ ] 成功加载 Driver (`insmod`)
- [ ] 成功卸载 Driver (`rmmod`)
- [ ] 查看 `dmesg` 中的日志

---

## 第二阶段：架构理解 (Architecture)

### 源码阅读 (Source Code Reading)

- [ ] 阅读 `/driver/LKM/src/init.c` (68 行)
  - [ ] 理解三大子系统的初始化顺序
  - [ ] 理解错误处理机制
  - [ ] 理解模块元数据

- [ ] 阅读 `/driver/LKM/src/trace.c` (503 行)
  - [ ] 理解 `/proc/elkeid-endpoint` 的创建
  - [ ] 理解事件读取流程
  - [ ] 理解 Per-CPU 缓冲区遍历

- [ ] 阅读 `/driver/LKM/src/filter.c` (578 行)
  - [ ] 理解红黑树实现
  - [ ] 理解白名单过滤机制

### 架构理解 (Architecture Understanding)

- [ ] 能画出 Driver 的组件交互图

```
参考答案:
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Kprobe     │────▶│   Filter     │────▶│ Ring Buffer  │
│   Hooks      │     │   Whitelist  │     │  Per-CPU     │
└──────────────┘     └──────────────┘     └──────┬───────┘
                                                  │
                                                  ▼
                                          ┌──────────────┐
                                          │   Trace      │
                                          │  /proc/...   │
                                          └──────────────┘
```

- [ ] 解释两个通信通道的作用
  - `/proc/elkeid-endpoint`: 数据输出 (只读)
  - `/dev/hids_driver_allowlist`: 配置输入 (只写)

- [ ] 理解 Per-CPU 环形缓冲区的设计
  - 每个 CPU 独立缓冲区
  - 无锁写入
  - 消费者循环读取

### 实践练习 (Exercises)

- [ ] 分析 `Makefile` 的编译流程
- [ ] 使用 `nm` 查看模块符号
- [ ] 使用 `strace` 观察用户空间读取

---

## 第三阶段：核心技术 (Core Technology)

### Kprobe 技术学习 (Kprobe Learning)

- [ ] 阅读 `/driver/LKM/KPROBE.md`
- [ ] 理解 Kprobe vs Kretprobe 的区别
- [ ] 理解原子上下文的限制

### Hook 实现理解 (Hook Implementation)

- [ ] 阅读 `/driver/LKM/include/kprobe.h`
  - [ ] 理解 `KPROBE_INITCALL` 宏
  - [ ] 理解 `KPROBE_HANDLER_DEFINE0-6` 宏

- [ ] 阅读 `/driver/LKM/src/smith_hook.c` (部分)
  - [ ] 找到一个 Kprobe 示例
  - [ ] 找到一个 Kretprobe 示例
  - [ ] 理解数据收集流程

### Hook 类型掌握 (Hook Types)

- [ ] 能说出至少 10 种 Hook 类型

| 类型 | Hook | 数据类型 ID |
|------|------|-------------|
| 进程 | execve | 100/59 |
| 进程 | exit | 60/231 |
| 网络 | connect | 42 |
| 网络 | bind | 49 |
| 网络 | dns_query | 601 |
| 文件 | open | 2 |
| 文件 | write | 1 |
| 文件 | rename | 82 |
| 安全 | load_module | 603 |
| 安全 | update_cred | 604 |

### 实践练习 (Exercises)

- [ ] 编写一个简单的 Hook 伪代码
- [ ] 解释 Kprobe 回调中的安全操作
- [ ] 理解 Tracepoint 的使用

---

## 第四阶段：数据流 (Data Flow)

### 数据格式理解 (Data Format)

- [ ] 阅读 `/docs/ElkeidData/raw_data_desc.md`
- [ ] 理解 13 个通用字段
- [ ] 理解私有字段的结构

### 数据流理解 (Data Flow Understanding)

- [ ] 阅读 `/driver/DATA_FLOW.md`
- [ ] 理解完整的数据流路径

```
Hook 回调 → 过滤检查 → 预留缓冲区 → 格式化数据 → 提交缓冲区
                                                          ↓
                                    用户空间读取 ← ← ← ← ← ←
```

- [ ] 理解事件数据的解析

### 实践练习 (Exercises)

- [ ] 使用 Python 解析事件数据
- [ ] 分析不同事件类型的私有字段
- [ ] 测量事件数据大小

---

## 第五阶段：高级主题 (Advanced Topics)

### 反 Rootkit 学习 (Anti-Rootkit)

- [ ] 阅读 `/driver/CODE_WALKTHROUGH/02-anti_rootkit_walkthrough.md`
- [ ] 理解 4 种 Rootkit 检测技术

| 类型 | 检测方法 | 数据类型 ID |
|------|----------|-------------|
| 隐藏模块 | 遍历 module_kset | 702 |
| Syscall 钩子 | 检查 syscall 表地址 | 701 |
| IDT 钩子 | 检查 IDT 表地址 | 703 |
| Proc 钩子 | 检查 /proc f_op | 700 |

- [ ] 理解内核符号查找 (`kallsyms_lookup_name`)

### 内存管理学习 (Memory Management)

- [ ] 阅读 `/driver/CODE_WALKTHROUGH/03-memcache_walkthrough.md`
- [ ] 理解无锁 MPMC 队列
- [ ] 理解 Per-CPU 内存池

### 性能优化理解 (Performance)

- [ ] 理解 Per-CPU 设计的优势
- [ ] 理解零拷贝优化
- [ ] 理解批量读取优化

---

## 第六阶段：实践与实验 (Practice)

### 编译和测试 (Build and Test)

- [ ] 阅读 `/driver/BUILDING.md`
- [ ] 在测试环境编译成功
- [ ] 运行基础功能测试
- [ ] 运行 LTP 集成测试 (可选)

### 测试脚本 (Test Scripts)

- [ ] 阅读 `/driver/TESTING.md`
- [ ] 运行白名单功能测试
- [ ] 运行事件类型测试
- [ ] 运行性能测试

### 故障排查 (Troubleshooting)

- [ ] 学会使用 `dmesg` 查看日志
- [ ] 学会使用 `lsmod` 查看模块
- [ ] 学会使用 `modinfo` 查看模块信息
- [ ] 学会使用 `ftrace` 进行性能分析

---

## 代码阅读清单 (Code Reading Checklist)

### 核心文件 (Core Files)

- [x] `/driver/LKM/src/init.c` (68 行) - 模块初始化
- [x] `/driver/LKM/src/trace.c` (503 行) - 通信接口
- [x] `/driver/LKM/src/filter.c` (578 行) - 白名单过滤
- [x] `/driver/LKM/src/memcache.c` (337 行) - 内存池
- [x] `/driver/LKM/src/anti_rootkit.c` (333 行) - 反 Rootkit
- [ ] `/driver/LKM/src/trace_buffer.c` (4376 行) - 环形缓冲区
- [ ] `/driver/LKM/src/smith_hook.c` (5101 行) - Hook 实现

### 头文件 (Header Files)

- [x] `/driver/LKM/include/kprobe.h` - Kprobe 宏模板
- [ ] `/driver/LKM/include/smith_hook.h` - Hook 定义
- [ ] `/driver/LKM/include/trace_buffer.h` - 环形缓冲区 API
- [ ] `/driver/LKM/include/print_event.h` - 事件打印宏
- [ ] `/driver/LKM/include/filter.h` - 白名单 API
- [ ] `/driver/LKM/include/memcache.h` - 内存池 API
- [ ] `/driver/LKM/include/anti_rootkit.h` - 反 Rootkit API
- [ ] `/driver/LKM/include/util.h` - 工具函数

### 文档文件 (Documentation)

- [x] `/driver/README.md` - 主文档
- [x] `/driver/LEARNING_PATH.md` - 学习路径
- [x] `/driver/ARCHITECTURE.md` - 架构文档
- [x] `/driver/BUILDING.md` - 编译指南
- [x] `/driver/DATA_FLOW.md` - 数据流详解
- [x] `/driver/TESTING.md` - 测试指南

---

## 自我评估 (Self-Assessment)

### 理论知识 (Theoretical Knowledge)

- [ ] 能解释 Elkeid Driver 的作用和原理
- [ ] 能画出系统架构图
- [ ] 能解释 Kprobe/Kretprobe 的工作原理
- [ ] 能解释 Per-CPU 环形缓冲区的设计
- [ ] 能解释白名单过滤机制
- [ ] 能解释反 Rootkit 检测原理

### 实践能力 (Practical Skills)

- [ ] 能独立编译 Driver
- [ ] 能加载和卸载 Driver
- [ ] 能配置白名单
- [ ] 能读取和解析事件数据
- [ ] 能进行基本的调试

### 源码理解 (Source Code Understanding)

- [ ] 能阅读 init.c 并理解每一行
- [ ] 能阅读一个 Hook 的实现
- [ ] 能理解数据格式化过程
- [ ] 能理解事件从产生到消费的完整流程

---

## 进阶学习路径 (Advanced Learning Path)

完成基础学习后，可以继续以下方向：

### 方向 1: 深入 Hook 实现

- [ ] 深入阅读 `smith_hook.c` (5101 行)
- [ ] 理解各种 Hook 类型的具体实现
- [ ] 学习如何添加新的 Hook

### 方向 2: 性能优化

- [ ] 分析环形缓冲区的性能特征
- [ ] 理解 TP99 < 3.5us 的实现
- [ ] 学习 ftrace 性能分析工具

### 方向 3: 兼容性处理

- [ ] 阅读 `/driver/DOC/Description_of_Elkeid's_Crash_caused_by_fput_in_low_version_Kernel.md`
- [ ] 理解内核版本兼容性处理
- [ ] 学习如何支持新内核版本

### 方向 4: 开发贡献

- [ ] 参与社区讨论
- [ ] 贡献 Bug 修复
- [ ] 添加新功能

---

## 学习资源 (Learning Resources)

### 官方资源

- [Elkeid GitHub](https://github.com/bytedance/Elkeid)
- [Elkeid 官方文档](https://bytedance.github.io/Elkeid/)

### 内核开发

- [Linux Kernel Documentation](https://www.kernel.org/doc/html/latest/)
- [Kprobe Documentation](https://www.kernel.org/doc/html/latest/trace/kprobes.html)
- [Linux Kernel Module Programming Guide](https://sysprog21.github.io/lkmpg/)

### 推荐书籍

- 《Linux Kernel Development》(Robert Love)
- 《Understanding the Linux Kernel》(Daniel P. Bovet)
- 《Linux Device Drivers》(Jonathan Corbet)

---

## 学习时间记录 (Learning Time Log)

| 阶段 | 开始日期 | 完成日期 | 耗时 | 备注 |
|------|----------|----------|------|------|
| 第一阶段：基础认知 | __________ | __________ | ____ | |
| 第二阶段：架构理解 | __________ | __________ | ____ | |
| 第三阶段：核心技术 | __________ | __________ | ____ | |
| 第四阶段：数据流 | __________ | __________ | ____ | |
| 第五阶段：高级主题 | __________ | __________ | ____ | |
| 第六阶段：实践与实验 | __________ | __________ | ____ | |
| **总计** | | | | |

---

## 学习笔记 (Learning Notes)

### 重要概念记录

1. Kprobe vs Kretprobe
   ```
   Kprobe:  在函数入口触发，可以访问参数
   Kretprobe: 在函数返回时触发，可以访问返回值
   ```

2. 数据协议
   ```
   字段分隔符: \x17
   记录分隔符: \x1e
   ```

3. 三大子系统
   ```
   trace: 通信和缓冲区管理
   anti_rootkit: Rootkit 检测
   kprobe_hook: Hook 注册和数据采集
   ```

### 常见问题记录

1. 编译失败
   - 检查内核头文件是否安装
   - 检查内核版本是否匹配

2. 模块加载失败
   - 检查 `dmesg` 日志
   - 检查内核版本兼容性

3. 读取不到数据
   - 确认模块已加载
   - 触发一些系统操作生成事件

---

**检查清单版本**: 1.0
**最后更新**: 2024
**维护者**: Elkeid Team
