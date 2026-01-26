# Elkeid Plugin 插件端功能拆分与开发计划

> 本文档详细拆分了 Elkeid Plugin 插件端的功能模块，并列出了细粒度的开发任务清单。

## 一、插件端整体架构

```
Elkeid Plugin 体系
├── 1. Collector 插件 (Go) - 资产采集
│   ├── 1.1 进程信息采集
│   ├── 1.2 端口信息采集
│   ├── 1.3 用户账户采集
│   ├── 1.4 软件包采集
│   ├── 1.5 容器信息采集
│   ├── 1.6 应用配置采集
│   └── 1.7 其他资产采集
│
├── 2. Baseline 插件 (Go) - 基线检查
│   ├── 2.1 规则引擎
│   ├── 2.2 检查类型实现
│   └── 2.3 合规性分析
│
├── 3. Scanner 插件 (Rust) - 文件扫描
│   ├── 3.1 ClamAV 引擎集成
│   ├── 3.2 扫描功能模块
│   └── 3.3 勒索防护
│
├── 4. Journal Watcher 插件 (Rust) - 日志监控
│   ├── 4.1 日志源适配
│   └── 4.2 SSH 事件解析
│
├── 5. Driver 插件 (Rust) - 内核驱动交互
│   ├── 5.1 内核模块管理
│   ├── 5.2 事件转换
│   └── 5.3 数据采集
│
├── 6. 内核驱动模块 (LKM) - 系统调用 Hook
│   ├── 6.1 追踪系统
│   ├── 6.2 系统调用 Hook
│   ├── 6.3 反 Rootkit 检测
│   └── 6.4 过滤模块
│
└── 7. Plugin SDK - 公共库
    ├── 7.1 Go SDK
    └── 7.2 Rust SDK
```

---

## 二、详细任务拆分

### 模块 1: Collector 插件 - 资产采集

**代码位置：** `/home/work/openSource/Elkeid/plugins/collector/`
**语言：** Go
**功能：** 定期收集主机各类资产信息并进行关联分析

#### 1.1 进程信息采集

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 1.1.1 | 进程列表采集 | `process.go` | 遍历 /proc 获取进程列表 | 3002 |
| 1.1.2 | 进程详情获取 | `process.go` | 获取 cmdline/exe/cwd 等信息 | 3002 |
| 1.1.3 | 进程 MD5 计算 | `process.go` | 计算可执行文件 MD5 | 3002 |
| 1.1.4 | 容器关联 | `process.go` | 关联进程所属容器 | 3002 |
| 1.1.5 | 进程树构建 | `process.go` | 构建父子进程关系 | 3002 |

---

#### 1.2 端口信息采集

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 1.2.1 | Netlink 端口采集 | `port/netlink.go` | 通过 netlink 获取端口信息 | 3001 |
| 1.2.2 | Procfs 端口采集 | `port/procfs.go` | 通过 /proc/net 获取端口 | 3001 |
| 1.2.3 | 端口数据处理 | `port/port.go` | 端口数据解析和格式化 | 3001 |
| 1.2.4 | TCP 端口采集 | `port.go` | TCP 监听端口列表 | 3001 |
| 1.2.5 | UDP 端口采集 | `port.go` | UDP 监听端口列表 | 3001 |
| 1.2.6 | 服务关联 | `port.go` | 关联端口对应的服务进程 | 3001 |
| 1.2.7 | 外部暴露分析 | `port.go` | 分析端口是否暴露到外网 | 3001 |

---

#### 1.3 用户账户采集

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 1.3.1 | 用户列表采集 | `user.go` | 解析 /etc/passwd 获取用户 | 3003 |
| 1.3.2 | 用户组采集 | `user.go` | 解析 /etc/group 获取组 | 3003 |
| 1.3.3 | 弱密码检测 | `user.go` | 检测弱密码账户 | 3003 |
| 1.3.4 | Sudoers 配置 | `user.go` | 解析 sudoers 权限配置 | 3003 |
| 1.3.5 | SSH 密钥采集 | `user.go` | 采集 authorized_keys | 3003 |
| 1.3.6 | 用户工具函数 | `utils/user.go` | 用户信息处理工具 | - |

---

#### 1.4 软件包采集

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 1.4.1 | RPM 包采集 | `software.go`, `rpm/rpm.go` | RPM 包管理器软件列表 | 3004 |
| 1.4.2 | DEB 包采集 | `software.go` | DPKG 包管理器软件列表 | 3004 |
| 1.4.3 | PyPI 包采集 | `software.go` | Python pip 包列表 | 3004 |
| 1.4.4 | Jar 包采集 | `software.go` | Java Jar 包扫描 | 3004 |
| 1.4.5 | NPM 包采集 | `software.go` | Node.js npm 包列表 | 3004 |
| 1.4.6 | 漏洞关联 | `software.go` | 关联已知漏洞 CVE | 3004 |

---

#### 1.5 容器信息采集

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 1.5.1 | 容器运行时检测 | `container/container.go` | 检测容器运行时类型 | 3005 |
| 1.5.2 | Docker 容器采集 | `container/container.go` | Docker API 容器列表 | 3005 |
| 1.5.3 | CRI 容器采集 | `container/container.go` | CRI 接口容器列表 | 3005 |
| 1.5.4 | Containerd 采集 | `container/container.go` | Containerd 容器列表 | 3005 |
| 1.5.5 | 容器枚举 | `container/enum.go` | 容器信息枚举工具 | 3005 |

---

#### 1.6 应用配置采集

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 1.6.1 | 应用版本检测 | `app.go` | 30+ 常见应用版本检测 | 3006 |
| 1.6.2 | 配置文件解析 | `app.go` | 应用配置文件路径 | 3006 |
| 1.6.3 | Web 服务检测 | `app.go` | Nginx/Apache/Tomcat 等 | 3006 |
| 1.6.4 | 数据库检测 | `app.go` | MySQL/Redis/MongoDB 等 | 3006 |
| 1.6.5 | 中间件检测 | `app.go` | Kafka/RabbitMQ 等 | 3006 |

---

#### 1.7 其他资产采集

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 1.7.1 | 服务采集 | `service.go` | 系统服务列表 (systemd/init) | 3007 |
| 1.7.2 | 定时任务采集 | `cron.go` | Cron 计划任务列表 | 3008 |
| 1.7.3 | 网卡信息采集 | `net_interface.go` | 网络接口信息 | 3009 |
| 1.7.4 | 磁盘信息采集 | `volume.go` | 磁盘卷信息 | 3010 |
| 1.7.5 | 完整性校验 | `integrity.go` | 系统文件完整性验证 | 3011 |
| 1.7.6 | 内核模块采集 | `kmod.go` | 内核模块列表 | 3012 |
| 1.7.7 | ZIP 文件处理 | `zip/zip.go` | 压缩包内容扫描 | - |

---

#### 1.8 采集引擎

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 1.8.1 | 调度引擎 | `engine/engine.go` | 基于 cron 定时调度 | 无 |
| 1.8.2 | 任务缓存 | `engine/engine.go` | 采集结果缓存 | 1.8.1 |
| 1.8.3 | 数据序列化 | `engine/engine.go` | 数据汇聚和序列化 | 1.8.1 |
| 1.8.4 | 并发控制 | `engine/engine.go` | 采集任务并发管理 | 1.8.1 |

---

### 模块 2: Baseline 插件 - 基线检查

**代码位置：** `/home/work/openSource/Elkeid/plugins/baseline/`
**语言：** Go
**功能：** 检测资产基线策略合规性，判断安全配置风险

#### 2.1 核心模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.1.1 | 主入口 | `main.go` | 任务接收、定期扫描 | 无 |
| 2.1.2 | 分析引擎 | `src/check/analysis.go` | 基线分析主逻辑 | 2.1.1 |
| 2.1.3 | 规则引擎 | `src/check/rule_engine.go` | 规则执行引擎 | 2.1.2 |
| 2.1.4 | 规则定义 | `src/check/rules.go` | 检查类型实现 | 2.1.3 |
| 2.1.5 | 系统检测 | `src/linux/os_system.go` | OS 类型检测 | 无 |

---

#### 2.2 检查类型实现

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.1 | 命令检查 | `src/check/rules.go` | command_check 实现 | 2.1.4 |
| 2.2.2 | 文件行检查 | `src/check/rules.go` | file_line_check 实现 | 2.1.4 |
| 2.2.3 | 文件权限检查 | `src/check/rules.go` | file_permission 实现 | 2.1.4 |
| 2.2.4 | 文件存在检查 | `src/check/rules.go` | if_file_exist 实现 | 2.1.4 |
| 2.2.5 | 文件所有者检查 | `src/check/rules.go` | file_user_group 实现 | 2.1.4 |
| 2.2.6 | MD5 检查 | `src/check/rules.go` | file_md5_check 实现 | 2.1.4 |
| 2.2.7 | 功能检查 | `src/check/rules.go` | func_check 实现 | 2.1.4 |
| 2.2.8 | 自定义检查 | `src/check/rules.go` | custom_check 实现 | 2.1.4 |

---

#### 2.3 基础设施

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.3.1 | 日志处理 | `infra/log.go` | 日志记录 | 无 |
| 2.3.2 | YAML 解析 | `infra/yaml.go` | 配置文件解析 | 无 |
| 2.3.3 | 结果格式化 | `infra/` | 检查结果格式化 | 2.3.1 |

---

### 模块 3: Scanner 插件 - 文件扫描

**代码位置：** `/home/work/openSource/Elkeid/plugins/scanner/`
**语言：** Rust
**功能：** 基于 ClamAV 引擎的文件恶意软件静态扫描

#### 3.1 ClamAV 引擎集成

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 3.1.1 | 引擎配置 | `src/model/engine/clamav/config.rs` | 引擎参数配置 | 无 |
| 3.1.2 | 引擎接口 | `src/model/engine/clamav/clamav.rs` | ClamAV 核心接口 | 3.1.1 |
| 3.1.3 | 病毒库更新 | `src/model/engine/clamav/updater.rs` | 病毒库管理 | 3.1.2 |
| 3.1.4 | 扫描结果处理 | `src/model/engine/clamav/` | 结果解析和格式化 | 3.1.2 |

---

#### 3.2 扫描功能模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 3.2.1 | 插件入口 | `src/bin/scanner_plugin.rs` | 插件模式主入口 | - |
| 3.2.2 | CLI 入口 | `src/bin/scanner_cli.rs` | 命令行模式 | - |
| 3.2.3 | 全盘扫描 | `src/model/functional/fulldiskscan.rs` | 全盘扫描功能 | 6000 |
| 3.2.4 | 定时扫描 | `src/model/functional/cronjob.rs` | 计划扫描任务 | 6000 |
| 3.2.5 | 文件监控 | `src/model/functional/fmonitor.rs` | 实时文件监控 | 6001 |
| 3.2.6 | 检测器 | `src/detector.rs` | 检测核心逻辑 | 6001 |
| 3.2.7 | 文件过滤 | `src/filter.rs` | 文件类型过滤 | - |

---

#### 3.3 勒索防护

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 3.3.1 | 勒索检测 | `src/model/functional/anti_ransom.rs` | 勒索软件行为检测 | 6001 |
| 3.3.2 | 文件备份 | `src/model/functional/anti_ransom.rs` | 关键文件备份 | - |
| 3.3.3 | 行为分析 | `src/model/functional/anti_ransom.rs` | 异常行为分析 | 6001 |

---

#### 3.4 核心功能

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 3.4.1 | 数据类型定义 | `src/data_type.rs` | DataType 常量定义 | 无 |
| 3.4.2 | 配置管理 | `src/configs.rs` | 扫描路径、过滤器配置 | 无 |
| 3.4.3 | 工具函数 | `src/lib.rs` | Hash 计算、资源限制 | 无 |
| 3.4.4 | 资源控制 | `src/lib.rs` | cgroup 内存和 CPU 限制 | 无 |

---

### 模块 4: Journal Watcher 插件 - 日志监控

**代码位置：** `/home/work/openSource/Elkeid/plugins/journal_watcher/`
**语言：** Rust
**功能：** 实时监控 SSH 日志事件

#### 4.1 日志源适配

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 4.1.1 | Journalctl 适配 | `src/main.rs` | journalctl -f 监听 | 无 |
| 4.1.2 | Auth.log 适配 | `src/main.rs` | /var/log/auth.log 监听 | 无 |
| 4.1.3 | Secure 日志适配 | `src/main.rs` | /var/log/secure 监听 | 无 |
| 4.1.4 | 日志源选择 | `src/main.rs` | 自动选择可用日志源 | 4.1.1-3 |

---

#### 4.2 SSH 事件解析

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | DataType |
|---------|---------|---------|---------|----------|
| 4.2.1 | PEG 语法定义 | `sshd.pest` | SSH 日志语法规则 | - |
| 4.2.2 | 登录成功解析 | `src/main.rs` | Accepted 事件解析 | 4000 |
| 4.2.3 | 登录失败解析 | `src/main.rs` | Failed 事件解析 | 4000 |
| 4.2.4 | 无效用户解析 | `src/main.rs` | Invalid User 事件解析 | 4000 |
| 4.2.5 | 认证事件解析 | `src/main.rs` | Authorized 事件解析 | 4001 |
| 4.2.6 | JSON 格式处理 | `src/main.rs` | journalctl JSON 解析 | - |
| 4.2.7 | 纯文本格式处理 | `src/main.rs` | 传统日志格式解析 | - |

---

### 模块 5: Driver 插件 - 内核驱动交互

**代码位置：** `/home/work/openSource/Elkeid/plugins/driver/`
**语言：** Rust
**功能：** 与内核驱动交互，采集内核事件并转换为上报格式

#### 5.1 核心模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 5.1.1 | 主入口 | `src/main.rs` | 任务接收、事件发送 | 无 |
| 5.1.2 | 内核模块管理 | `src/kmod.rs` | 驱动加载/卸载 | 5.1.1 |
| 5.1.3 | 配置管理 | `src/config.rs` | 运行时配置 | 5.1.1 |
| 5.1.4 | 核心库 | `src/lib.rs` | 公共函数 | 无 |
| 5.1.5 | 工具函数 | `src/utils.rs` | 辅助工具 | 无 |

---

#### 5.2 事件转换

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 5.2.1 | 事件转换器 | `src/transformer.rs` | 内核事件格式转换 | 5.1.1 |
| 5.2.2 | Schema 定义 | `src/transformer/schema.rs` | 数据结构定义 | 5.2.1 |
| 5.2.3 | 缓存管理 | `src/transformer/cache.rs` | 事件缓存 | 5.2.1 |
| 5.2.4 | 字段映射 | `src/transformer.rs` | 内核字段到上报字段映射 | 5.2.1 |

---

### 模块 6: 内核驱动模块 (LKM)

**代码位置：** `/home/work/openSource/Elkeid/driver/LKM/`
**语言：** C
**功能：** 内核级系统调用 Hook 和安全检测

#### 6.1 追踪系统

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 6.1.1 | 模块初始化 | `src/init.c` | 模块入口和注册 | 无 |
| 6.1.2 | 追踪框架 | `src/trace.c` | 追踪系统主框架 | 6.1.1 |
| 6.1.3 | Proc 接口 | `src/trace.c` | /proc/elkeid-endpoint | 6.1.2 |
| 6.1.4 | 环形缓冲区 | `src/trace_buffer.c` | Per-CPU 环形缓冲 | 6.1.2 |
| 6.1.5 | 缓冲区头文件 | `include/trace_buffer.h` | 缓冲区数据结构 | 6.1.4 |
| 6.1.6 | 事件定义 | `include/kprobe_print.h` | 事件格式定义 | 6.1.2 |

---

#### 6.2 系统调用 Hook

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 默认状态 |
|---------|---------|---------|---------|---------|
| 6.2.1 | Hook 框架 | `src/smith_hook.c` | kprobe/kretprobe 框架 | - |
| 6.2.2 | CONNECT Hook | `src/smith_hook.c` | 网络连接事件 | 启用 |
| 6.2.3 | BIND Hook | `src/smith_hook.c` | 网络绑定事件 | 启用 |
| 6.2.4 | EXECVE Hook | `src/smith_hook.c` | 进程执行事件 | 启用 |
| 6.2.5 | CREATE_FILE Hook | `src/smith_hook.c` | 文件创建事件 | 启用 |
| 6.2.6 | PTRACE Hook | `src/smith_hook.c` | 进程追踪 | 启用 |
| 6.2.7 | MODULE_LOAD Hook | `src/smith_hook.c` | 模块加载 | 启用 |
| 6.2.8 | UPDATE_CRED Hook | `src/smith_hook.c` | 凭证更新 | 启用 |
| 6.2.9 | RENAME Hook | `src/smith_hook.c` | 文件重命名 | 启用 |
| 6.2.10 | LINK Hook | `src/smith_hook.c` | 文件链接 | 启用 |
| 6.2.11 | SETSID Hook | `src/smith_hook.c` | 会话设置 | 启用 |
| 6.2.12 | CHMOD Hook | `src/smith_hook.c` | 权限修改 | 启用 |
| 6.2.13 | MOUNT Hook | `src/smith_hook.c` | 挂载操作 | 启用 |

---

#### 6.3 反 Rootkit 检测

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 6.3.1 | 检测框架 | `src/anti_rootkit.c` | 反 Rootkit 主框架 | 6.1.1 |
| 6.3.2 | 隐藏模块检测 | `src/anti_rootkit.c` | kset 列表比对 | 6.3.1 |
| 6.3.3 | 系统调用表检测 | `src/anti_rootkit.c` | sys_call_table Hook 检测 | 6.3.1 |
| 6.3.4 | 中断表检测 | `src/anti_rootkit.c` | IDT Hook 检测 (x86) | 6.3.1 |
| 6.3.5 | 文件操作检测 | `src/anti_rootkit.c` | fops Hook 检测 | 6.3.1 |
| 6.3.6 | 定时检测 | `src/anti_rootkit.c` | 15 分钟周期检测 | 6.3.1 |

---

#### 6.4 过滤模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 6.4.1 | 过滤框架 | `src/filter.c` | 白名单过滤框架 | 6.1.1 |
| 6.4.2 | 红黑树实现 | `src/filter.c` | 高效查询数据结构 | 6.4.1 |
| 6.4.3 | 字符设备接口 | `src/filter.c` | /dev/elkeid-filter | 6.4.1 |
| 6.4.4 | Execve 白名单 | `src/filter.c` | 可执行文件白名单 | 6.4.2 |
| 6.4.5 | Argv 白名单 | `src/filter.c` | 参数白名单 | 6.4.2 |
| 6.4.6 | 过滤头文件 | `include/filter.h` | 过滤接口定义 | 6.4.1 |

---

#### 6.5 工具模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 6.5.1 | 工具函数 | `src/util.c` | 通用工具函数 | 无 |
| 6.5.2 | 内存缓存 | `src/memcache.c` | Per-CPU 内存池 | 无 |
| 6.5.3 | 内存缓存头文件 | `include/memcache.h` | 缓存接口定义 | 6.5.2 |
| 6.5.4 | Murmur Hash | `src/filter.c` | OAAT64 哈希算法 | 无 |

---

### 模块 7: Plugin SDK - 公共库

**代码位置：** `/home/work/openSource/Elkeid/plugins/lib/`

#### 7.1 Go SDK

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 7.1.1 | 客户端基础 | `go/client.go` | 基础客户端 (bufio) | 无 |
| 7.1.2 | Linux 实现 | `go/client_linux.go` | stdin/stdout + FD3/FD4 | 7.1.1 |
| 7.1.3 | Windows 实现 | `go/client_windows.go` | Windows 管道实现 | 7.1.1 |
| 7.1.4 | ProtoBuf 定义 | `go/bridge.pb.go` | Record/Task 消息 | 无 |
| 7.1.5 | 日志记录器 | `go/log/logger.go` | 日志记录 | 无 |
| 7.1.6 | 日志写入器 | `go/log/writer.go` | 日志输出 | 7.1.5 |

---

#### 7.2 Rust SDK

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 7.2.1 | 核心库 | `rust/src/lib.rs` | 核心库导出 | 无 |
| 7.2.2 | ProtoBuf 定义 | `rust/src/bridge.rs` | Record/Task 消息 | 无 |
| 7.2.3 | 日志记录器 | `rust/src/logger.rs` | 日志记录 | 无 |
| 7.2.4 | 系统抽象 | `rust/src/sys/mod.rs` | 平台抽象层 | 无 |
| 7.2.5 | Unix 实现 | `rust/src/sys/unix.rs` | Unix 管道实现 | 7.2.4 |
| 7.2.6 | Windows 实现 | `rust/src/sys/windows.rs` | Windows 实现 | 7.2.4 |
| 7.2.7 | 构建脚本 | `rust/build.rs` | 构建配置 | 无 |

---

#### 7.3 协议定义

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 7.3.1 | ProtoBuf 源文件 | `bridge.proto` | Record/Task/Payload 定义 | 无 |
| 7.3.2 | 代码生成 | - | protoc 生成 Go/Rust 代码 | 7.3.1 |

---

## 三、开发阶段规划

### 阶段 1: SDK 和基础设施
```
优先级: P0 (最高)
任务: 7.x (Plugin SDK)
目标: 完成 Go/Rust SDK，建立插件通信基础
```

### 阶段 2: 内核驱动模块
```
优先级: P0
任务: 6.1.x (追踪系统), 6.2.x (系统调用 Hook)
目标: 完成内核级数据采集能力
```

### 阶段 3: Driver 插件
```
优先级: P0
任务: 5.x (Driver 插件)
目标: 实现内核驱动与用户态的数据桥接
```

### 阶段 4: Collector 插件
```
优先级: P1
任务: 1.x (Collector 插件全部)
目标: 完成资产采集能力
```

### 阶段 5: Baseline 插件
```
优先级: P1
任务: 2.x (Baseline 插件全部)
目标: 完成基线检查能力
```

### 阶段 6: Scanner 插件
```
优先级: P1
任务: 3.x (Scanner 插件全部)
目标: 完成恶意文件扫描能力
```

### 阶段 7: Journal Watcher 插件
```
优先级: P2
任务: 4.x (Journal Watcher 全部)
目标: 完成 SSH 日志监控能力
```

### 阶段 8: 反 Rootkit 和过滤
```
优先级: P2
任务: 6.3.x (反 Rootkit), 6.4.x (过滤模块)
目标: 完成高级安全检测和过滤能力
```

---

## 四、任务统计

| 模块 | 任务数量 | 语言 | 代码文件数 |
|------|---------|------|-----------|
| Collector-进程采集 | 5 | Go | 2 |
| Collector-端口采集 | 7 | Go | 4 |
| Collector-用户采集 | 6 | Go | 3 |
| Collector-软件采集 | 6 | Go | 3 |
| Collector-容器采集 | 5 | Go | 3 |
| Collector-应用采集 | 5 | Go | 1 |
| Collector-其他采集 | 7 | Go | 7 |
| Collector-引擎 | 4 | Go | 1 |
| Baseline-核心 | 5 | Go | 5 |
| Baseline-检查类型 | 8 | Go | 1 |
| Baseline-基础设施 | 3 | Go | 3 |
| Scanner-引擎 | 4 | Rust | 4 |
| Scanner-功能模块 | 7 | Rust | 6 |
| Scanner-勒索防护 | 3 | Rust | 1 |
| Scanner-核心 | 4 | Rust | 4 |
| Journal Watcher-日志源 | 4 | Rust | 1 |
| Journal Watcher-解析 | 7 | Rust | 2 |
| Driver-核心 | 5 | Rust | 5 |
| Driver-事件转换 | 4 | Rust | 3 |
| LKM-追踪系统 | 6 | C | 4 |
| LKM-系统调用 Hook | 13 | C | 1 |
| LKM-反 Rootkit | 6 | C | 1 |
| LKM-过滤模块 | 6 | C | 2 |
| LKM-工具模块 | 4 | C | 3 |
| SDK-Go | 6 | Go | 6 |
| SDK-Rust | 7 | Rust | 6 |
| SDK-协议 | 2 | Proto | 1 |
| **总计** | **~148** | - | **~85** |

---

## 五、关键依赖关系图

```
                    ┌─────────────────┐
                    │      Agent      │
                    │  (plugin.go)    │
                    └────────┬────────┘
         ┌──────────┬────────┼────────┬──────────┐
         ▼          ▼        ▼        ▼          ▼
    ┌─────────┐ ┌─────────┐ ┌───────┐ ┌────────┐ ┌──────┐
    │Collector│ │Baseline │ │Scanner│ │J.Watcher│ │Driver│
    │  (Go)   │ │  (Go)   │ │(Rust) │ │ (Rust) │ │(Rust)│
    └────┬────┘ └────┬────┘ └───┬───┘ └────┬───┘ └──┬───┘
         │          │          │          │        │
         └──────────┴──────────┴──────────┴────────┘
                              │
                              ▼
    ┌─────────────────────────────────────────────────┐
    │                  Plugin SDK                      │
    │              (Go/Rust 通用库)                   │
    └─────────────────────────────────────────────────┘
                              │
                              ▼
    ┌─────────────────────────────────────────────────┐
    │                   LKM 内核驱动                   │
    │  (kprobe Hook / 环形缓冲区 / 反 Rootkit)        │
    └─────────────────────────────────────────────────┘
                              │
                              ▼
    ┌─────────────────────────────────────────────────┐
    │                  Linux Kernel                    │
    └─────────────────────────────────────────────────┘
```

---

## 六、插件与 Agent 通信机制

### 6.1 通信协议 (ProtoBuf)

```protobuf
// 数据记录 (Plugin → Agent)
message Record {
  int32 data_type = 1;      // 数据类型
  int64 timestamp = 2;      // 时间戳
  Payload data = 3;         // 数据负载
}

message Payload {
  map<string, string> fields = 1;  // 键值对数据
}

// 任务 (Agent → Plugin)
message Task {
  int32 data_type = 1;      // 任务数据类型
  string object_name = 2;   // 对象名称
  string data = 3;          // 任务数据
  string token = 4;         // 任务令牌
}
```

### 6.2 IPC 管道通信

```
Agent                              Plugin
  │                                  │
  │ ─── FD3 (tx_r) 任务管道 ───────→ │
  │                                  │
  │ ←── FD4 (rx_w) 数据管道 ──────── │
  │                                  │
  └── 4字节长度 + ProtoBuf 格式 ────→ │
```

### 6.3 通信数据流

```
内核驱动
  │ (读取 /proc/elkeid-endpoint)
  ▼
Driver 插件
  │ (事件转换)
  ▼
Agent (plugin.go)
  │ (聚合到 buffer)
  ▼
gRPC 传输
  │
  ▼
Server (AgentCenter)
```

---

## 七、DataType 编码表

### 7.1 Collector 插件

| DataType | 描述 | 来源模块 |
|----------|------|---------|
| 3001 | 端口信息 | port.go |
| 3002 | 进程信息 | process.go |
| 3003 | 用户信息 | user.go |
| 3004 | 软件包信息 | software.go |
| 3005 | 容器信息 | container.go |
| 3006 | 应用信息 | app.go |
| 3007 | 服务信息 | service.go |
| 3008 | 定时任务 | cron.go |
| 3009 | 网卡信息 | net_interface.go |
| 3010 | 磁盘信息 | volume.go |
| 3011 | 完整性信息 | integrity.go |
| 3012 | 内核模块 | kmod.go |

### 7.2 Journal Watcher 插件

| DataType | 描述 | 来源模块 |
|----------|------|---------|
| 4000 | SSH 登录事件 | main.rs |
| 4001 | SSH 认证事件 | main.rs |

### 7.3 Scanner 插件

| DataType | 描述 | 来源模块 |
|----------|------|---------|
| 6000 | 扫描任务完成 | scanner_plugin.rs |
| 6001 | 静态恶意软件 | detector.rs |
| 6002 | 进程恶意软件 | detector.rs |
| 6003 | 路径扫描结果 | fulldiskscan.rs |

---

## 八、关键文件清单

### Collector 插件
- `/home/work/openSource/Elkeid/plugins/collector/main.go`
- `/home/work/openSource/Elkeid/plugins/collector/engine/engine.go`
- `/home/work/openSource/Elkeid/plugins/collector/process.go`
- `/home/work/openSource/Elkeid/plugins/collector/port.go`

### Baseline 插件
- `/home/work/openSource/Elkeid/plugins/baseline/main.go`
- `/home/work/openSource/Elkeid/plugins/baseline/src/check/analysis.go`
- `/home/work/openSource/Elkeid/plugins/baseline/src/check/rule_engine.go`

### Scanner 插件
- `/home/work/openSource/Elkeid/plugins/scanner/src/bin/scanner_plugin.rs`
- `/home/work/openSource/Elkeid/plugins/scanner/src/detector.rs`
- `/home/work/openSource/Elkeid/plugins/scanner/src/model/engine/clamav/clamav.rs`

### Journal Watcher 插件
- `/home/work/openSource/Elkeid/plugins/journal_watcher/src/main.rs`
- `/home/work/openSource/Elkeid/plugins/journal_watcher/sshd.pest`

### Driver 插件
- `/home/work/openSource/Elkeid/plugins/driver/src/main.rs`
- `/home/work/openSource/Elkeid/plugins/driver/src/kmod.rs`
- `/home/work/openSource/Elkeid/plugins/driver/src/transformer.rs`

### LKM 内核驱动
- `/home/work/openSource/Elkeid/driver/LKM/src/init.c`
- `/home/work/openSource/Elkeid/driver/LKM/src/trace.c`
- `/home/work/openSource/Elkeid/driver/LKM/src/smith_hook.c`
- `/home/work/openSource/Elkeid/driver/LKM/src/anti_rootkit.c`
- `/home/work/openSource/Elkeid/driver/LKM/src/filter.c`

### Plugin SDK
- `/home/work/openSource/Elkeid/plugins/lib/go/client.go`
- `/home/work/openSource/Elkeid/plugins/lib/rust/src/lib.rs`
- `/home/work/openSource/Elkeid/plugins/lib/bridge.proto`

### Agent 插件管理
- `/home/work/openSource/Elkeid/agent/plugin/plugin.go`
- `/home/work/openSource/Elkeid/agent/plugin/plugin_linux.go`
- `/home/work/openSource/Elkeid/agent/plugin/protocol.go`
