# Elkeid Agent 端功能拆分与开发计划

> 本文档详细拆分了 Elkeid Agent 端的功能模块，并列出了细粒度的开发任务清单。

## 一、核心功能模块拆分

```
Elkeid Agent
├── 1. 核心基础设施层
│   ├── 1.1 Agent 身份管理
│   ├── 1.2 状态管理
│   └── 1.3 主程序框架
│
├── 2. 主机信息采集层
│   ├── 2.1 静态平台信息
│   └── 2.2 动态主机信息
│
├── 3. 资源监控层
│   ├── 3.1 进程资源采集
│   ├── 3.2 系统资源采集
│   └── 3.3 硬件信息采集
│
├── 4. 心跳服务层
│   ├── 4.1 系统状态心跳
│   └── 4.2 插件状态心跳
│
├── 5. 数据缓冲层
│   ├── 5.1 环形缓冲区
│   └── 5.2 内存对象池
│
├── 6. 网络通信层
│   ├── 6.1 gRPC 连接管理
│   ├── 6.2 数据压缩
│   ├── 6.3 数据传输服务
│   └── 6.4 文件上传服务
│
├── 7. 插件管理层
│   ├── 7.1 插件生命周期
│   ├── 7.2 插件进程管理
│   └── 7.3 IPC 通信
│
├── 8. 日志系统层
│   ├── 8.1 本地日志
│   └── 8.2 远程日志上报
│
├── 9. 自更新机制
│
└── 10. 部署运维工具 (elkeidctl)
```

---

## 二、详细任务拆分

### 模块 1: 核心基础设施层

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 1.1.1 | Agent ID 生成策略实现 | `agent/id.go` | 实现多源 ID 生成: 环境变量 > SHA1-UUID > machine-id > 随机生成 | 无 |
| 1.1.2 | Agent ID 持久化存储 | `agent/id.go` | 将生成的 ID 写入本地文件并管理 | 1.1.1 |
| 1.1.3 | 工作目录初始化 | `agent/id.go` | 创建 `/etc/elkeid/` 目录结构 | 无 |
| 1.2.1 | 状态枚举定义 | `agent/state.go` | 定义 Running/Abnormal 状态类型 | 无 |
| 1.2.2 | 状态追踪器实现 | `agent/state.go` | 实现线程安全的状态查询和更新 | 1.2.1 |
| 1.3.1 | 主程序启动入口 | `main.go` | main() 函数框架搭建 | 无 |
| 1.3.2 | 信号处理机制 | `main.go` | 处理 SIGTERM/SIGUSR1/SIGUSR2 | 1.3.1 |
| 1.3.3 | Goroutine 守护进程启动 | `main.go` | 并行启动 heartbeat/plugin/transport | 1.3.1 |
| 1.3.4 | pprof 性能分析集成 | `main.go` | DEBUG 模式下启动 pprof 服务 | 1.3.1 |

---

### 模块 2: 主机信息采集层

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.1.1 | Platform 信息获取 | `host/platform.go` | 获取 OS 名称、版本、架构 | 无 |
| 2.1.2 | 内核版本采集 | `host/platform.go` | 获取 KernelVersion | 无 |
| 2.2.1 | Hostname 动态刷新 | `host/host.go` | 定期刷新 hostname (原子变量) | 无 |
| 2.2.2 | 私有 IP 地址采集 | `host/host.go` | 采集 IPv4/IPv6，过滤 docker/lo | 无 |
| 2.2.3 | 公网 IP 地址采集 | `host/host.go` | 采集外网 IP 地址 | 2.2.2 |
| 2.2.4 | 私网网段判定逻辑 | `host/host.go` | 判定 10.x/192.168.x/172.16-31.x | 2.2.2 |

---

### 模块 3: 资源监控层

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 3.1.1 | 进程 CPU 使用率采集 | `resource/resource.go` | 获取进程 CPU 占用百分比 | 无 |
| 3.1.2 | 进程内存 RSS 采集 | `resource/resource.go` | 获取进程常驻内存大小 | 无 |
| 3.1.3 | 进程 I/O 速度采集 | `resource/resource.go` | 获取进程读写速度 | 无 |
| 3.1.4 | 进程 FD 数量采集 | `resource/resource.go` | 获取文件描述符数量 | 无 |
| 3.1.5 | 进程启动时间采集 | `resource/resource.go` | 获取进程启动时间戳 | 无 |
| 3.2.1 | 系统内存总量获取 | `resource/resource.go` | 获取系统物理内存大小 | 无 |
| 3.2.2 | 系统启动时间获取 | `resource/resource.go` | 获取系统开机时间 | 无 |
| 3.3.1 | CPU 型号采集 | `resource/resource.go` | 获取 CPU 品牌和型号 | 无 |
| 3.3.2 | DMI 信息采集 | `resource/resource_linux.go` | 从 /sys/class/dmi 读取硬件信息 | 无 |
| 3.3.3 | 产品序列号/UUID 获取 | `resource/resource_linux.go` | 获取主机唯一标识 | 3.3.2 |
| 3.4.1 | DNS 服务器解析 | `resource/resource_linux.go` | 解析 /etc/resolv.conf | 无 |
| 3.4.2 | 网关地址获取 | `resource/resource_linux.go` | 解析 /proc/net/route | 无 |
| 3.5.1 | LRU 缓存实现 | `resource/resource.go` | 实现资源查询缓存（100条） | 无 |

---

### 模块 4: 心跳服务层

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 4.1.1 | 心跳定时器设计 | `heartbeat/heartbeat.go` | 1分钟周期心跳触发 | 无 |
| 4.1.2 | Agent 状态数据组装 | `heartbeat/heartbeat.go` | 组装 DataType=1000 心跳数据 | 2.x, 3.x |
| 4.1.3 | 系统资源指标采集 | `heartbeat/heartbeat.go` | CPU/内存/磁盘/网络指标 | 3.x |
| 4.1.4 | Goroutine 数量统计 | `heartbeat/heartbeat.go` | 获取运行时 goroutine 数 | 无 |
| 4.1.5 | 网络流量统计 | `heartbeat/heartbeat.go` | Rx/Tx 速度、TPS 计算 | 无 |
| 4.2.1 | 插件状态心跳数据组装 | `heartbeat/heartbeat.go` | 组装 DataType=1001 插件心跳 | 7.x |
| 4.2.2 | 插件性能指标采集 | `heartbeat/heartbeat.go` | 采集每个插件的资源占用 | 7.x |
| 4.3.1 | Systemd Watchdog 集成 | `heartbeat/heartbeat.go` | 支持 sd_notify 看门狗 | 4.1.1 |

---

### 模块 5: 数据缓冲层

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 5.1.1 | 环形缓冲区结构设计 | `buffer/buffer.go` | 2048 槽位的环形队列 | 无 |
| 5.1.2 | WriteRecord 实现 | `buffer/buffer.go` | Protobuf Record 写入 | 5.1.1 |
| 5.1.3 | WriteEncodedRecord 实现 | `buffer/buffer.go` | 编码后数据直接写入 | 5.1.1 |
| 5.1.4 | ReadEncodedRecords 实现 | `buffer/buffer.go` | 批量读取数据 | 5.1.1 |
| 5.1.5 | Transmission Hook 机制 | `buffer/buffer.go` | 数据处理钩子支持 | 5.1.1 |
| 5.2.1 | 多级对象池设计 | `buffer/pool.go` | 4 个不同大小的缓冲池 | 无 |
| 5.2.2 | GetEncodedRecord 实现 | `buffer/pool.go` | 从池获取缓冲区 | 5.2.1 |
| 5.2.3 | PutEncodedRecord 实现 | `buffer/pool.go` | 归还缓冲区到池 | 5.2.1 |

---

### 模块 6: 网络通信层

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 6.1.1 | gRPC 证书管理 | `transport/connection/product.go` | 嵌入 ca.crt/client.crt/key | 无 |
| 6.1.2 | TLS 双向认证实现 | `transport/connection/connection.go` | mTLS 配置和验证 | 6.1.1 |
| 6.1.3 | Service Discovery 客户端 | `transport/connection/connection.go` | HTTP 服务发现接口 | 无 |
| 6.1.4 | 多端点容错连接 | `transport/connection/connection.go` | SD→私网→公网 容错逻辑 | 6.1.3 |
| 6.1.5 | 连接状态追踪 | `transport/connection/connection.go` | 连接池状态管理 | 6.1.4 |
| 6.1.6 | 环境变量配置覆盖 | `transport/connection/product.go` | 支持 IDC/Region 环境变量 | 无 |
| 6.2.1 | Snappy 压缩器实现 | `transport/compressor/snappy.go` | gRPC 消息压缩 | 无 |
| 6.2.2 | Snappy 解压器实现 | `transport/compressor/snappy.go` | gRPC 消息解压 | 无 |
| 6.2.3 | 压缩器对象池 | `transport/compressor/snappy.go` | sync.Pool 复用压缩器 | 6.2.1, 6.2.2 |
| 6.3.1 | Transfer 服务启动 | `transport/transfer.go` | 双向流 gRPC 服务 | 6.1.x |
| 6.3.2 | handleSend 数据发送 | `transport/transfer.go` | 批量发送 PackagedData | 5.x, 6.3.1 |
| 6.3.3 | handleReceive 命令接收 | `transport/transfer.go` | 处理 Server 下发命令 | 6.3.1 |
| 6.3.4 | Task 命令分发 | `transport/transfer.go` | 分发给 Agent/Plugin | 6.3.3 |
| 6.3.5 | Config 同步处理 | `transport/transfer.go` | 调用 plugin.Sync() | 6.3.3, 7.x |
| 6.3.6 | 传输统计 Handler | `transport/connection/stats_handler.go` | Rx/Tx 速度统计 | 6.3.1 |
| 6.3.7 | 自动重连机制 | `transport/transfer.go` | 连接失败重试（5次） | 6.3.1 |
| 6.4.1 | FileExt 服务启动 | `transport/file_ext.go` | 文件上传 gRPC 服务 | 6.1.x |
| 6.4.2 | 文件分块上传 | `transport/file_ext.go` | 500KB 分块流式上传 | 6.4.1 |
| 6.4.3 | 上传进度回调 | `transport/file_ext.go` | 上传状态反馈 | 6.4.2 |
| 6.4.4 | 上传大小限制 | `transport/file_ext.go` | 512MB 限制、10分钟超时 | 6.4.2 |

---

### 模块 7: 插件管理层

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 7.1.1 | 插件配置解析 | `plugin/plugin.go` | 解析 Config 配置结构 | 无 |
| 7.1.2 | 插件版本校验 | `plugin/plugin.go` | SHA256 签名校验 | 7.1.1 |
| 7.1.3 | 插件下载逻辑 | `plugin/plugin.go` | 多源下载、解压 tar.gz | 7.1.2 |
| 7.1.4 | 插件目录管理 | `plugin/plugin_linux.go` | /etc/elkeid/plugin/{name}/ | 7.1.3 |
| 7.1.5 | 插件 Sync 主流程 | `plugin/plugin.go` | 同步配置列表，增删改 | 7.1.1-4 |
| 7.2.1 | 插件进程启动 | `plugin/plugin_linux.go` | exec.Command 创建子进程 | 7.1.x |
| 7.2.2 | 进程组设置 | `plugin/plugin_linux.go` | setpgid 进程组隔离 | 7.2.1 |
| 7.2.3 | 错误输出重定向 | `plugin/plugin_linux.go` | stderr → {name}.stderr | 7.2.1 |
| 7.2.4 | 进程退出监听 | `plugin/plugin.go` | Wait goroutine | 7.2.1 |
| 7.2.5 | 插件优雅关闭 | `plugin/plugin.go` | Shutdown 流程实现 | 7.2.4 |
| 7.3.1 | Unix 管道创建 | `plugin/plugin_linux.go` | rx/tx 双向管道 | 7.2.1 |
| 7.3.2 | IPC 消息格式实现 | `plugin/protocol.go` | [4字节长度][Protobuf] | 7.3.1 |
| 7.3.3 | 插件数据接收 | `plugin/plugin.go` | rx goroutine | 7.3.1, 7.3.2 |
| 7.3.4 | 任务下发到插件 | `plugin/plugin.go` | tx goroutine | 7.3.1, 7.3.2 |
| 7.3.5 | 零拷贝数据转发 | `plugin/plugin.go` | 不解码直接发送到 buffer | 7.3.3 |
| 7.4.1 | 插件列表管理 | `plugin/plugin.go` | sync.Map 存储 | 无 |
| 7.4.2 | 插件性能统计 | `plugin/plugin.go` | RxSpeed/TxSpeed/TPS | 7.3.x |

---

### 模块 8: 日志系统层

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 8.1.1 | Zap 日志器初始化 | `main.go` | 配置 zap 日志格式 | 无 |
| 8.1.2 | Lumberjack 日志轮转 | `main.go` | 配置日志文件滚动 | 8.1.1 |
| 8.1.3 | 双 Core 日志系统 | `main.go` | gRPC + 文件双写 | 8.1.1, 8.2.x |
| 8.2.1 | GrpcWriter 实现 | `log/writer.go` | zapcore.WriteSyncer 实现 | 5.x |
| 8.2.2 | JSON 转 Protobuf | `log/writer.go` | 日志格式转换 | 8.2.1 |
| 8.2.3 | DataType=1010 日志上报 | `log/writer.go` | Agent 内部日志上报 | 8.2.2 |
| 8.3.1 | ErrorWithToken 实现 | `log/custom.go` | 带 token 的错误记录 | 5.x |
| 8.3.2 | DataType=5100 错误上报 | `log/custom.go` | 操作结果反馈 | 8.3.1 |

---

### 模块 9: 自更新机制

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 9.1.1 | 版本比对逻辑 | `transport/transfer.go` | 检测服务端版本号 | 6.3.x |
| 9.1.2 | 更新包下载 | `agent/update.go` | 下载 deb/rpm 包 | 9.1.1 |
| 9.1.3 | 包管理器检测 | `agent/update.go` | 检测 dpkg/rpm 可用性 | 无 |
| 9.1.4 | dpkg 安装逻辑 | `agent/update.go` | Debian 系统更新 | 9.1.2, 9.1.3 |
| 9.1.5 | rpm 安装逻辑 | `agent/update.go` | RHEL 系统更新 | 9.1.2, 9.1.3 |
| 9.1.6 | 更新后服务重启 | `agent/update.go` | systemctl restart | 9.1.4, 9.1.5 |

---

### 模块 10: 部署运维工具 (elkeidctl)

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 10.1.1 | Cobra CLI 框架搭建 | `deploy/control/` | 基础命令行框架 | 无 |
| 10.1.2 | start 命令实现 | `deploy/control/cmd/` | 启动 Agent 服务 | 10.1.1 |
| 10.1.3 | stop 命令实现 | `deploy/control/cmd/` | 停止 Agent 服务 | 10.1.1 |
| 10.1.4 | restart 命令实现 | `deploy/control/cmd/` | 重启 Agent 服务 | 10.1.2, 10.1.3 |
| 10.1.5 | status 命令实现 | `deploy/control/cmd/` | 查询运行状态 | 10.1.1 |
| 10.1.6 | enable/disable 命令 | `deploy/control/cmd/` | 开机自启管理 | 10.1.1 |
| 10.1.7 | check 命令实现 | `deploy/control/cmd/` | 健康检查（cron 用） | 10.1.1 |
| 10.1.8 | set/unset 命令实现 | `deploy/control/cmd/` | 参数配置管理 | 10.1.1 |
| 10.1.9 | cgroup 命令实现 | `deploy/control/cmd/` | 资源限制管理 | 10.1.1 |
| 10.1.10 | cleanup 命令实现 | `deploy/control/cmd/` | 清理数据 | 10.1.1 |
| 10.2.1 | preinstall 脚本 | `deploy/scripts/` | 安装前准备 | 无 |
| 10.2.2 | postinstall 脚本 | `deploy/scripts/` | 安装后配置 | 无 |
| 10.2.3 | preremove 脚本 | `deploy/scripts/` | 卸载前清理 | 无 |
| 10.2.4 | sysvinit 服务脚本 | `deploy/scripts/` | 传统 init.d 支持 | 无 |
| 10.3.1 | nfpm 打包配置 | `deploy/nfpm.yaml` | deb/rpm 打包配置 | 无 |
| 10.3.2 | build.sh 构建脚本 | `build.sh` | 多架构编译脚本 | 无 |

---

### 模块 11: 通用工具库

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 11.1.1 | HTTP 文件下载 | `utils/download.go` | 支持多源容错 | 无 |
| 11.1.2 | SHA256 校验 | `utils/download.go` | 文件完整性校验 | 11.1.1 |
| 11.1.3 | tar.gz 自动解压 | `utils/download.go` | 下载后自动解压 | 11.1.1 |
| 11.2.1 | tar.gz 解压实现 | `utils/decompress.go` | 安全解压逻辑 | 无 |
| 11.2.2 | 路径遍历防护 | `utils/decompress.go` | 检查 `..` 目录 | 11.2.1 |

---

### 模块 12: Protocol Buffer 定义

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 12.1.1 | 消息结构定义 | `proto/grpc.proto` | PackagedData/Command 等 | 无 |
| 12.1.2 | gRPC 服务定义 | `proto/grpc.proto` | Transfer/FileExt 服务 | 12.1.1 |
| 12.1.3 | Protobuf 代码生成 | `proto/grpc.pb.go` | protoc 生成 Go 代码 | 12.1.1, 12.1.2 |

---

## 三、开发阶段规划

### 阶段 1: 基础设施
```
优先级: P0 (最高)
任务: 1.x, 11.x, 12.x
目标: Agent 可启动，具备基础 ID 和状态管理
```

### 阶段 2: 主机信息采集
```
优先级: P0
任务: 2.x, 3.x
目标: 能够采集完整的主机和系统信息
```

### 阶段 3: 网络通信
```
优先级: P0
任务: 6.1.x, 6.2.x, 6.3.x
目标: 建立与服务端的双向通信
```

### 阶段 4: 数据流水线
```
优先级: P1
任务: 5.x, 8.x
目标: 完成数据缓冲和日志系统
```

### 阶段 5: 心跳服务
```
优先级: P1
任务: 4.x
目标: 定期上报系统状态
```

### 阶段 6: 插件系统
```
优先级: P1
任务: 7.x
目标: 支持动态加载和管理插件
```

### 阶段 7: 高级功能
```
优先级: P2
任务: 6.4.x (文件上传), 9.x (自更新)
目标: 支持文件上传和自动升级
```

### 阶段 8: 运维工具
```
优先级: P2
任务: 10.x
目标: 完成 elkeidctl 和部署脚本
```

---

## 四、任务统计

| 模块 | 任务数量 | 代码行数 |
|------|---------|---------|
| 核心基础设施 | 9 | ~300 |
| 主机信息采集 | 6 | ~90 |
| 资源监控 | 13 | ~200 |
| 心跳服务 | 8 | ~150 |
| 数据缓冲 | 8 | ~130 |
| 网络通信 | 21 | ~600 |
| 插件管理 | 17 | ~430 |
| 日志系统 | 8 | ~100 |
| 自更新 | 6 | ~50 |
| 运维工具 | 14 | ~400 |
| 通用工具 | 5 | ~200 |
| Protocol Buffer | 3 | ~70 |
| **总计** | **118** | **~5141** |

---

## 五、关键依赖关系图

```
                    ┌─────────────────┐
                    │   main.go (1.3) │
                    └────────┬────────┘
           ┌─────────────────┼─────────────────┐
           ▼                 ▼                 ▼
    ┌──────────┐      ┌──────────┐      ┌───────────┐
    │heartbeat │      │ plugin   │      │ transport │
    │   (4)    │      │   (7)    │      │    (6)    │
    └────┬─────┘      └────┬─────┘      └─────┬─────┘
         │                 │                  │
         ▼                 ▼                  ▼
    ┌──────────┐      ┌──────────┐      ┌───────────┐
    │ resource │      │  buffer  │◄─────│connection │
    │   (3)    │      │   (5)    │      │   (6.1)   │
    └────┬─────┘      └──────────┘      └───────────┘
         │                 ▲
         ▼                 │
    ┌──────────┐      ┌──────────┐
    │   host   │      │   log    │
    │   (2)    │      │   (8)    │
    └──────────┘      └──────────┘
```

---

## 六、关键数据流

### 6.1 启动流程

```
main()
  ├─ 日志初始化
  ├─ 并行启动 3 个守护进程:
  │   ├─ heartbeat.Startup() → 1分钟收集一次系统信息
  │   ├─ plugin.Startup() → 监听插件配置变化，动态加载/卸载
  │   └─ transport.Startup() → 连接服务器，双向通信
  └─ 信号处理循环 (SIGTERM/SIGUSR1/SIGUSR2)
```

### 6.2 数据汇聚链路

```
各数据源
  ├─ 系统资源 (heartbeat)
  ├─ Agent 日志 (log writer)
  ├─ 插件数据 (plugin rx)
  └─ 系统事件

  ↓ buffer.WriteRecord/WriteEncodedRecord()

缓冲区 (2048 条记录环形队列)

  ↓ transfer.handleSend()

gRPC 双向流 Transfer.Transfer()
(使用 snappy 压缩)

  ↓ 网络传输

服务器 (AgentCenter)
```

### 6.3 下行命令处理

```
Server → Transfer.Transfer(Command)
  ├─ Task 任务
  │   ├─ object_name == "elkeid-agent" → Agent 自身任务
  │   │   ├─ 1050: 文件上传
  │   │   ├─ 1051: 设置元数据 (IDC/Region)
  │   │   └─ 1060: 关闭 Agent
  │   └─ object_name == plugin_name → 转发给插件
  │
  └─ Configs 配置列表
      ├─ 包含自身配置 → 检查版本，触发自更新
      └─ 包含插件配置 → 同步到 plugin.Sync()
```

### 6.4 插件生命周期

```
Server Config → transport.handleReceive()
  ↓
plugin.Sync(cfgs) → plugin.Startup()
  ├─ 新增配置: Load() → 启动新进程
  ├─ 更新配置: Shutdown() 旧版本 → Load() 新版本
  └─ 删除配置: Shutdown() → 清理目录
```

---

## 七、核心设计模式

### 7.1 并发设计
- **sync.Map**: 线程安全的插件存储
- **atomic.Value**: 原子变量用于 Host 信息更新
- **缓冲 Channel**: 用于 Sync/Task 通信

### 7.2 资源管理
- **对象池**: buffer.pool 复用 EncodedRecord
- **LRU 缓存**: resource 模块避免重复查询
- **连接复用**: gRPC 连接池

### 7.3 容错能力
- **自动重连**: Transfer 连接失败重试 (5 次后自保护退出)
- **版本校验**: 文件 SHA256 校验
- **多源容错**: Service Discovery 失败后尝试私网/公网端点

### 7.4 性能优化
- **零拷贝**: 插件数据不再解码，直接转发给服务器
- **批量发送**: 100ms 周期批量收集数据
- **数据压缩**: Snappy 压缩
- **异步非阻塞**: 大量使用 Channel 和 Goroutine

### 7.5 安全机制
- **TLS 双向认证**: 基于自签名证书
- **签名验证**: 文件下载后 SHA256 校验
- **沙盒隔离**: 每个插件独立进程 + IPC 通信

---

## 八、通信协议

### 8.1 Agent ↔ Server 通信
- **协议**: gRPC (HTTP/2)
- **认证**: mTLS (双向证书)
- **压缩**: Snappy
- **连接**: 持久化双向流

### 8.2 Agent ↔ Plugin 通信
- **通道**: Unix 管道 (两条)
- **格式**: `[4字节小端长度][Protobuf 二进制]`
- **编码**: 不进行额外编码 (直接转发到服务器)

### 8.3 数据包结构

```protobuf
PackagedData {
  records: [
    { data_type, timestamp, data (binary) },
    ...
  ],
  agent_id, hostname, version,
  intranet_ipv4/6, extranet_ipv4/6,
  product
}
```

---

## 九、DataType 编码表

| DataType | 描述 | 来源 |
|----------|------|------|
| 1000 | Agent 系统心跳 | heartbeat |
| 1001 | 插件状态心跳 | heartbeat |
| 1010 | Agent 内部日志 | log/writer |
| 1050 | 文件上传任务 | transport |
| 1051 | 元数据设置任务 | transport |
| 1060 | Agent 关闭命令 | transport |
| 5100 | 操作结果反馈 | log/custom |

---

## 十、运行时配置

### 10.1 环境变量

| 变量名 | 描述 | 默认值 |
|--------|------|--------|
| `RUNTIME_MODE` | 运行模式 (DEBUG) | 空 |
| `service_type` | 服务类型 (sysvinit) | systemd |
| `specified_idc` | 指定 IDC | default |
| `specified_region` | 指定区域 | default |
| `SPECIFIED_AGENT_ID` | 指定 Agent ID | 自动生成 |
| `DETAIL` | 插件详情参数 | 空 |

### 10.2 目录结构

```
/etc/elkeid/
├── elkeid-agent              # Agent 主程序
├── elkeidctl                 # 控制工具
├── machine-id                # Agent ID 持久化
├── specified_env             # 配置文件
├── log/
│   └── elkeid-agent.log      # 日志文件
└── plugin/
    └── {plugin_name}/        # 插件工作目录
        ├── {plugin_name}     # 插件可执行文件
        └── {plugin_name}.stderr  # 插件错误输出
```
