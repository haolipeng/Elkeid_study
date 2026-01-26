# Elkeid Server 端功能拆分与开发计划

> 本文档详细拆分了 Elkeid Server 端的功能模块，并列出了细粒度的开发任务清单。

## 一、Server 端整体架构

```
Elkeid Server
├── 1. AgentCenter (AC) - Agent 通信与数据采集层
│   ├── 1.1 gRPC 传输层
│   ├── 1.2 HTTP 传输层
│   ├── 1.3 Kafka 数据输出
│   └── 1.4 服务注册
│
├── 2. Manager - 管理后端与 API 层
│   ├── 2.1 基础设施层 (infra)
│   ├── 2.2 业务逻辑层 (internal)
│   ├── 2.3 API 接口层 (biz)
│   └── 2.4 定时任务层 (cronjob)
│
├── 3. ServiceDiscovery (SD) - 服务发现中心
│   ├── 3.1 服务注册
│   ├── 3.2 负载均衡
│   └── 3.3 集群管理
│
├── 4. Web Console - 前端界面
│
└── 5. 基础设施服务
    ├── 5.1 MongoDB
    ├── 5.2 Redis
    ├── 5.3 Kafka
    ├── 5.4 Elasticsearch
    └── 5.5 Prometheus/Grafana
```

---

## 二、详细任务拆分

### 模块 1: AgentCenter (AC) - Agent 通信与数据采集层

**代码位置：** `/home/work/openSource/Elkeid/server/agent_center/`

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 1.1.1 | gRPC Server 框架搭建 | `grpctrans/grpc_server.go` | gRPC 服务器初始化和配置 | 无 |
| 1.1.2 | TLS/SSL 双向认证实现 | `grpctrans/grpc_server.go` | mTLS 证书配置和验证 | 1.1.1 |
| 1.1.3 | Transfer 双向流处理 | `grpctrans/grpc_handler/transfer_handler.go` | Agent 数据上报和命令下发 | 1.1.1 |
| 1.1.4 | FileExt 文件上传服务 | `grpctrans/grpc_handler/file_handler.go` | 文件流式上传处理 | 1.1.1 |
| 1.1.5 | 连接池管理 | `grpctrans/pool/` | Agent 连接池和限制 | 1.1.1 |
| 1.1.6 | RawData Worker 实现 | `grpctrans/grpc_handler/rawdata_worker.go` | 原始数据处理工作流 | 1.1.3 |
| 1.1.7 | Protocol Buffer 定义 | `grpctrans/proto/grpc.proto` | RawData/Command 消息定义 | 无 |
| 1.2.1 | HTTP Server 框架搭建 | `httptrans/scsvr.go` | Gin HTTP 服务器初始化 | 无 |
| 1.2.2 | AK/SK 认证中间件 | `httptrans/midware/` | HTTP 请求认证 | 1.2.1 |
| 1.2.3 | Command API 实现 | `httptrans/http_handler/command.go` | 命令下发接口 | 1.2.1, 1.2.2 |
| 1.2.4 | Audit API 实现 | `httptrans/http_handler/audit.go` | 审计日志处理 | 1.2.1 |
| 1.2.5 | Connection API 实现 | `httptrans/http_handler/conn.go` | 连接管理接口 | 1.2.1 |
| 1.2.6 | HTTP Client 封装 | `httptrans/client/` | 配置/任务/文件操作客户端 | 无 |
| 1.3.1 | Kafka Producer 初始化 | `common/kafka/` | Kafka 生产者配置 | 无 |
| 1.3.2 | SASL 认证支持 | `common/kafka/` | Kafka SASL 认证 | 1.3.1 |
| 1.3.3 | 数据序列化和发送 | `common/kafka/` | MQData 序列化到 Kafka | 1.3.1 |
| 1.3.4 | 分区策略实现 | `common/kafka/` | Agent ID 哈希分区 | 1.3.1 |
| 1.4.1 | SD 客户端实现 | `svr_registry/` | 向 ServiceDiscovery 注册 | 无 |
| 1.4.2 | 心跳保活机制 | `svr_registry/` | 定期发送心跳 | 1.4.1 |
| 1.5.1 | Snappy 压缩支持 | `common/snappy/` | 数据压缩解压 | 无 |
| 1.5.2 | Zstd 压缩支持 | `common/zstd/` | 高效压缩算法 | 无 |
| 1.5.3 | 日志系统 (ylog) | `common/ylog/` | 结构化日志 | 无 |
| 1.5.4 | 用户配置管理 | `common/userconfig/` | 运行时配置 | 无 |

---

### 模块 2: Manager - 管理后端与 API 层

**代码位置：** `/home/work/openSource/Elkeid/server/manager/`

#### 2.1 基础设施层 (infra)

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.1.1 | MongoDB 驱动封装 | `infra/mongodb/` | MongoDB 连接和操作 | 无 |
| 2.1.2 | MongoDB 索引管理 | `conf/index.json` | 集合索引配置 | 2.1.1 |
| 2.1.3 | Redis 客户端封装 | `infra/redis/` | Redis 缓存操作 | 无 |
| 2.1.4 | Kafka Producer 封装 | `infra/kafka/` | 事件流输出 | 无 |
| 2.1.5 | Elasticsearch 集成 | `infra/es/` | 日志检索支持 | 无 |
| 2.1.6 | SD 客户端集成 | `infra/discovery/` | 服务发现客户端 | 无 |
| 2.1.7 | 服务注册实现 | `infra/discovery/svr_register.go` | Manager 服务注册 | 2.1.6 |
| 2.1.8 | TOS 对象存储集成 | `infra/tos/` | 文件存储服务 | 无 |
| 2.1.9 | 日志系统 (ylog) | `infra/ylog/` | 统一日志处理 | 无 |
| 2.1.10 | 工具函数库 | `infra/utils/` | MongoDB/随机/类型转换 | 无 |

---

#### 2.2 业务逻辑层 (internal)

##### 2.2.1 告警管理模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.1.1 | 告警常量定义 | `internal/alarm/alarm_const.go` | 告警类型/级别枚举 | 无 |
| 2.2.1.2 | 告警查询逻辑 | `internal/alarm/alarm_query.go` | 条件查询/分页 | 2.1.1 |
| 2.2.1.3 | 告警状态更新 | `internal/alarm/alarm_update.go` | 状态变更处理 | 2.1.1 |
| 2.2.1.4 | 告警统计分析 | `internal/alarm/alarm_statistics.go` | 聚合统计 | 2.1.1 |
| 2.2.1.5 | 告警初始化 | `internal/alarm/alarm_init.go` | 周期统计/异步更新 | 2.2.1.1-4 |
| 2.2.1.6 | 告警白名单管理 | `internal/alarm_whitelist/` | 白名单规则 CRUD | 2.1.1 |

---

##### 2.2.2 资产中心模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.2.1 | 主机资产管理 | `internal/asset_center/` | 主机信息 CRUD | 2.1.1 |
| 2.2.2.2 | 资产指纹采集 | `internal/asset_center/fingerprint/` | 进程/端口/用户等 | 2.2.2.1 |
| 2.2.2.3 | 资产标签管理 | `internal/asset_center/` | 标签分组 | 2.2.2.1 |
| 2.2.2.4 | 资产统计分析 | `internal/asset_center/` | 资产数量统计 | 2.2.2.1 |

---

##### 2.2.3 Agent 任务管理模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.3.1 | 任务模型定义 | `internal/atask/task.go` | 任务数据结构 | 无 |
| 2.2.3.2 | 子任务管理 | `internal/atask/subtask.go` | 子任务拆分和跟踪 | 2.2.3.1 |
| 2.2.3.3 | Job 调度器 | `internal/atask/job.go` | 任务调度执行 | 2.2.3.1 |
| 2.2.3.4 | 快速任务接口 | `internal/atask/fast.go` | 即时任务执行 | 2.2.3.1 |

---

##### 2.2.4 基线检查模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.4.1 | 基线初始化 | `internal/baseline/baseline_init.go` | 基线规则加载 | 无 |
| 2.2.4.2 | 基线配置管理 | `internal/baseline/baseline_config.go` | 配置 CRUD | 2.2.4.1 |
| 2.2.4.3 | 基线任务触发 | `internal/baseline/task.go` | 检测任务创建 | 2.2.3.x |
| 2.2.4.4 | 基线定时任务 | `internal/baseline/cronjob.go` | 定期检测 | 2.2.4.3 |
| 2.2.4.5 | 弱口令检测 | `internal/baseline/` | 弱口令规则 | 2.2.4.1 |

---

##### 2.2.5 漏洞管理模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.5.1 | 漏洞初始化 | `internal/vuln/vuln_init.go` | 漏洞库加载 | 无 |
| 2.2.5.2 | 漏洞任务管理 | `internal/vuln/task.go` | 检测任务 | 2.2.3.x |
| 2.2.5.3 | 漏洞定时扫描 | `internal/vuln/cronjob.go` | 定期扫描 | 2.2.5.2 |
| 2.2.5.4 | 漏洞结果处理 | `internal/vuln/` | 结果入库 | 2.1.1 |

---

##### 2.2.6 病毒检测模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.6.1 | 病毒检测初始化 | `internal/virus_detection/` | 病毒库加载 | 无 |
| 2.2.6.2 | 病毒告警处理 | `internal/virus_detection/` | 告警入库 | 2.1.1 |
| 2.2.6.3 | 病毒扫描任务 | `internal/virus_detection/` | 扫描任务 | 2.2.3.x |

---

##### 2.2.7 Kubernetes 安全模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.7.1 | K8s 集群管理 | `internal/kube/` | 集群配置 | 2.1.1 |
| 2.2.7.2 | K8s 节点管理 | `internal/kube/` | 节点信息 | 2.2.7.1 |
| 2.2.7.3 | K8s Pod 管理 | `internal/kube/` | Pod 信息 | 2.2.7.1 |
| 2.2.7.4 | K8s 告警处理 | `internal/kube/` | K8s 安全告警 | 2.1.1 |
| 2.2.7.5 | 审计日志处理 | `internal/kube/` | K8s 审计日志 | 2.2.7.1 |

---

##### 2.2.8 容器安全模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.8.1 | 容器资产管理 | `internal/container/` | 容器信息采集 | 2.1.1 |
| 2.2.8.2 | 镜像安全扫描 | `internal/container/` | 镜像漏洞扫描 | 2.2.8.1 |

---

##### 2.2.9 数据写入模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.9.1 | Hub 告警写入 | `internal/dbtask/hub_alarm.go` | HIDS 告警入库 | 2.1.1 |
| 2.2.9.2 | Hub 资产写入 | `internal/dbtask/hub_asset.go` | 资产数据入库 | 2.1.1 |
| 2.2.9.3 | K8s 告警写入 | `internal/dbtask/kube_alarm.go` | K8s 告警入库 | 2.1.1 |
| 2.2.9.4 | Agent 心跳写入 | `internal/dbtask/agent_hb.go` | 心跳数据入库 | 2.1.1 |
| 2.2.9.5 | 子任务状态更新 | `internal/dbtask/agent_subtask.go` | 任务状态同步 | 2.1.1 |
| 2.2.9.6 | 批量写入引擎 | `internal/dbtask/db_writer.go` | 异步批量写入 | 2.1.1 |

---

##### 2.2.10 数据输出模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.10.1 | ES 输出插件 | `internal/outputer/es.go` | Elasticsearch 输出 | 2.1.5 |
| 2.2.10.2 | Kafka 输出插件 | `internal/outputer/kafka.go` | Kafka 输出 | 2.1.4 |
| 2.2.10.3 | Syslog 输出插件 | `internal/outputer/syslog.go` | Syslog 输出 | 无 |
| 2.2.10.4 | Hub 输出插件 | `internal/outputer/hub_plugin.go` | Hub 集成输出 | 无 |

---

##### 2.2.11 定时任务模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.11.1 | 定时任务框架 | `internal/cronjob/cronjob.go` | Cron 调度器 | 无 |
| 2.2.11.2 | 资产中心定时任务 | `internal/cronjob/asset-center.go` | 资产统计更新 | 2.2.2.x |
| 2.2.11.3 | 概览定时任务 | `internal/cronjob/overview.go` | 仪表盘数据 | 2.1.1 |
| 2.2.11.4 | 指纹定时任务 | `internal/cronjob/fingerprint.go` | 指纹数据刷新 | 2.2.2.2 |

---

##### 2.2.12 监控模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.12.1 | AC 状态监控 | `internal/monitor/config/ac.go` | AgentCenter 监控 | 无 |
| 2.2.12.2 | MongoDB 状态监控 | `internal/monitor/config/mongodb.go` | 数据库监控 | 2.1.1 |
| 2.2.12.3 | Kafka 状态监控 | `internal/monitor/config/kafka.go` | 消息队列监控 | 2.1.4 |
| 2.2.12.4 | Redis 状态监控 | `internal/monitor/config/redis.go` | 缓存监控 | 2.1.3 |
| 2.2.12.5 | Hub 状态监控 | `internal/monitor/config/hub.go` | HUB 监控 | 无 |

---

##### 2.2.13 用户认证模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.13.1 | 用户登录 | `internal/login/` | 登录认证 | 2.1.3 |
| 2.2.13.2 | 会话管理 | `internal/login/` | Session 管理 | 2.1.3 |
| 2.2.13.3 | RBAC 权限 | `conf/rbac.json` | 角色权限配置 | 2.2.13.1 |

---

##### 2.2.14 系统告警模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.14.1 | 系统告警管理 | `internal/system_alert/` | 系统级告警 | 2.1.1 |
| 2.2.14.2 | 通知推送 | `internal/system_alert/` | 告警通知 | 2.2.14.1 |

---

##### 2.2.15 Agent 配置模块

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.2.15.1 | Agent 配置管理 | `internal/aconfig/` | Agent 配置 CRUD | 2.1.1 |
| 2.2.15.2 | 插件配置管理 | `internal/aconfig/` | 插件配置 | 2.2.15.1 |
| 2.2.15.3 | 驱动配置管理 | `internal/aconfig/` | 驱动配置 | 2.2.15.1 |

---

#### 2.3 API 接口层 (biz)

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 2.3.1 | AK/SK 认证中间件 | `biz/midware/akskAuth.go` | API 认证 | 无 |
| 2.3.2 | RBAC 认证中间件 | `biz/midware/rbacAuth.go` | 权限验证 | 2.2.13.3 |
| 2.3.3 | Metrics 中间件 | `biz/midware/metrics.go` | 性能指标 | 无 |
| 2.3.4 | Agent API (v1) | `biz/handler/v1/` | Agent 管理接口 | 2.2.3.x |
| 2.3.5 | Alarm API (v6) | `biz/handler/v6/alarm.go` | 告警管理接口 | 2.2.1.x |
| 2.3.6 | Asset API (v6) | `biz/handler/v6/asset_center.go` | 资产管理接口 | 2.2.2.x |
| 2.3.7 | Baseline API (v6) | `biz/handler/v6/baseline.go` | 基线检查接口 | 2.2.4.x |
| 2.3.8 | Vuln API (v6) | `biz/handler/v6/vuln.go` | 漏洞管理接口 | 2.2.5.x |
| 2.3.9 | Kube API (v6) | `biz/handler/v6/kube_sec.go` | K8s 安全接口 | 2.2.7.x |
| 2.3.10 | Monitor API (v6) | `biz/handler/v6/monitor_*.go` | 监控接口 | 2.2.12.x |
| 2.3.11 | User API (v6) | `biz/handler/v6/user.go` | 用户管理接口 | 2.2.13.x |
| 2.3.12 | Task API (v6) | `biz/handler/v6/task.go` | 任务管理接口 | 2.2.3.x |
| 2.3.13 | Virus API (v6) | `biz/handler/v6/virus_detection.go` | 病毒检测接口 | 2.2.6.x |

---

### 模块 3: ServiceDiscovery (SD) - 服务发现中心

**代码位置：** `/home/work/openSource/Elkeid/server/service_discovery/`

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 3.1.1 | HTTP Server 搭建 | `server/server.go` | SD HTTP 服务 | 无 |
| 3.1.2 | 路由配置 | `server/router.go` | API 路由 | 3.1.1 |
| 3.1.3 | 服务注册处理 | `server/handler/registry.go` | 注册/下线 | 3.1.1 |
| 3.1.4 | 端点发现处理 | `server/handler/endpoint.go` | 服务发现 | 3.1.1 |
| 3.1.5 | 指标采集处理 | `server/handler/metrics.go` | Prometheus 指标 | 3.1.1 |
| 3.2.1 | AK/SK 认证 | `server/midware/akskAuth.go` | 请求认证 | 无 |
| 3.2.2 | Metrics 中间件 | `server/midware/metrics.go` | 指标收集 | 无 |
| 3.3.1 | 集群管理 | `cluster/cluster.go` | 多节点同步 | 无 |
| 3.3.2 | 配置管理 | `cluster/use_config.go` | 集群配置 | 3.3.1 |
| 3.4.1 | 端点数据结构 | `endpoint/endpoint.go` | 端点模型 | 无 |
| 3.4.2 | 端点工具函数 | `endpoint/utils.go` | 辅助函数 | 3.4.1 |
| 3.5.1 | 线程安全 Map | `common/safemap/` | 并发安全存储 | 无 |
| 3.5.2 | 日志系统 | `common/ylog/` | 日志处理 | 无 |
| 3.5.3 | 初始化模块 | `common/init.go` | 系统初始化 | 无 |

---

### 模块 4: Web Console - 前端界面

**代码位置：** `/home/work/openSource/Elkeid/server/web_console/`

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 4.1.1 | 前端构建集成 | `build.sh` | 前端打包到 Manager | 无 |
| 4.1.2 | 静态资源服务 | `manager/static/frontend/` | 静态文件服务 | 4.1.1 |
| 4.2.1 | 登录页面 | - | 用户登录界面 | 无 |
| 4.2.2 | 仪表盘页面 | - | 概览仪表盘 | 无 |
| 4.2.3 | 资产管理页面 | - | 主机资产列表 | 无 |
| 4.2.4 | 告警管理页面 | - | 告警列表和详情 | 无 |
| 4.2.5 | 基线检查页面 | - | 基线检测结果 | 无 |
| 4.2.6 | 漏洞管理页面 | - | 漏洞扫描结果 | 无 |
| 4.2.7 | K8s 安全页面 | - | K8s 安全监控 | 无 |
| 4.2.8 | 系统设置页面 | - | 系统配置 | 无 |

---

### 模块 5: 部署与运维

**代码位置：** `/home/work/openSource/Elkeid/elkeidup/`

| 任务编号 | 任务名称 | 代码位置 | 详细描述 | 依赖 |
|---------|---------|---------|---------|------|
| 5.1.1 | ElkeidUp 部署工具 | `elkeidup/` | 分布式部署工具 | 无 |
| 5.1.2 | Docker 镜像构建 | `elkeidup/` | All-in-one 镜像 | 无 |
| 5.1.3 | 部署文档 | `elkeidup/deploy.md` | 部署指南 | 无 |
| 5.2.1 | AC 构建脚本 | `agent_center/build.sh` | AC 编译打包 | 无 |
| 5.2.2 | Manager 构建脚本 | `manager/build.sh` | Manager 编译打包 | 无 |
| 5.2.3 | SD 构建脚本 | `service_discovery/build.sh` | SD 编译打包 | 无 |
| 5.3.1 | MongoDB 初始化 | `manager/conf/index.json` | 索引初始化 | 无 |
| 5.3.2 | RBAC 初始化 | `manager/conf/rbac.json` | 权限初始化 | 无 |
| 5.3.3 | 基线配置初始化 | `manager/conf/baseline_config/` | 基线规则 | 无 |
| 5.4.1 | HTTPS 配置 | `elkeidup/https_config/` | SSL/TLS 配置 | 无 |
| 5.4.2 | Nginx 配置 | - | 反向代理配置 | 无 |

---

## 三、开发阶段规划

### 阶段 1: 基础设施层
```
优先级: P0 (最高)
任务: 2.1.x (MongoDB/Redis/Kafka/ES), 3.x (ServiceDiscovery)
目标: 完成基础设施层，服务可启动并互联
```

### 阶段 2: Agent 通信层
```
优先级: P0
任务: 1.x (AgentCenter 全部)
目标: 实现 Agent 与 Server 的双向通信
```

### 阶段 3: 核心业务层
```
优先级: P0
任务: 2.2.1.x (告警), 2.2.2.x (资产), 2.2.3.x (任务)
目标: 完成告警、资产、任务的核心功能
```

### 阶段 4: 安全检测层
```
优先级: P1
任务: 2.2.4.x (基线), 2.2.5.x (漏洞), 2.2.6.x (病毒)
目标: 完成安全检测功能
```

### 阶段 5: 云原生安全功能
```
优先级: P1
任务: 2.2.7.x (K8s), 2.2.8.x (容器)
目标: 完成 Kubernetes 和容器安全功能
```

### 阶段 6: 数据处理层
```
优先级: P1
任务: 2.2.9.x (数据写入), 2.2.10.x (数据输出), 2.2.11.x (定时任务)
目标: 完成数据处理和输出功能
```

### 阶段 7: API 接口层
```
优先级: P1
任务: 2.3.x (全部 API)
目标: 完成所有 REST API 接口
```

### 阶段 8: 前端界面
```
优先级: P2
任务: 4.x (Web Console)
目标: 完成前端管理界面
```

### 阶段 9: 部署运维
```
优先级: P2
任务: 5.x (部署工具和文档)
目标: 完成部署工具和文档
```

---

## 四、任务统计

| 模块 | 任务数量 | 代码文件数 |
|------|---------|-----------|
| AgentCenter | 24 | 35 |
| Manager-基础设施 | 10 | 22 |
| Manager-告警管理 | 6 | 6 |
| Manager-资产中心 | 4 | 10+ |
| Manager-Agent任务 | 4 | 5 |
| Manager-基线检查 | 5 | 8 |
| Manager-漏洞管理 | 4 | 6 |
| Manager-病毒检测 | 3 | 4 |
| Manager-K8s安全 | 5 | 10 |
| Manager-容器安全 | 2 | 4 |
| Manager-数据写入 | 6 | 7 |
| Manager-数据输出 | 4 | 5 |
| Manager-定时任务 | 4 | 5 |
| Manager-监控 | 5 | 6 |
| Manager-用户认证 | 3 | 4 |
| Manager-系统告警 | 2 | 3 |
| Manager-Agent配置 | 3 | 4 |
| Manager-API接口 | 13 | 40 |
| ServiceDiscovery | 14 | 17 |
| Web Console | 8 | - |
| 部署运维 | 11 | - |
| **总计** | **~135** | **~210+** |

---

## 五、关键依赖关系图

```
                         ┌─────────────────┐
                         │   Web Console   │
                         └────────┬────────┘
                                  │
                         ┌────────▼────────┐
                         │     Manager     │
                         │   (API + BIZ)   │
                         └────────┬────────┘
              ┌───────────────────┼───────────────────┐
              │                   │                   │
     ┌────────▼────────┐ ┌────────▼────────┐ ┌────────▼────────┐
     │  AgentCenter    │ │ServiceDiscovery │ │   Elkeid HUB    │
     │  (gRPC/HTTP)    │ │  (注册/发现)    │ │  (规则引擎)     │
     └────────┬────────┘ └─────────────────┘ └────────┬────────┘
              │                                       │
              └───────────────────┬───────────────────┘
                                  │
                         ┌────────▼────────┐
                         │      Kafka      │
                         │   (消息队列)    │
                         └────────┬────────┘
                                  │
         ┌────────────────────────┼────────────────────────┐
         │                        │                        │
┌────────▼────────┐      ┌────────▼────────┐      ┌────────▼────────┐
│     MongoDB     │      │      Redis      │      │  Elasticsearch  │
│   (主数据库)    │      │    (缓存)       │      │   (日志检索)    │
└─────────────────┘      └─────────────────┘      └─────────────────┘
```

---

## 六、关键数据流

### 6.1 Agent 数据上报流程

```
Agent
  │
  ├─→ gRPC 双向流 (Transfer)
  │     └─→ AgentCenter
  │           ├─→ Kafka Topic: hids_svr
  │           └─→ Kafka Topic: k8s (审计日志)
  │
  └─→ 文件上传 (FileExt)
        └─→ AgentCenter → TOS/本地存储
```

### 6.2 命令下发流程

```
Web Console / API
  │
  └─→ Manager
        └─→ AgentCenter (HTTP)
              └─→ Agent (gRPC Command)
```

### 6.3 告警处理流程

```
Kafka (hids_svr)
  │
  └─→ Elkeid HUB (规则引擎)
        │
        └─→ Manager (dbtask)
              ├─→ MongoDB (告警存储)
              └─→ 通知推送
```

---

## 七、核心设计模式

### 7.1 微服务架构
- **AgentCenter**: 负责 Agent 通信，无状态可水平扩展
- **Manager**: 负责业务逻辑，通过 SD 实现服务发现
- **ServiceDiscovery**: 服务注册与发现中心

### 7.2 异步处理
- **Kafka**: 解耦数据生产和消费
- **Channel + Goroutine**: Go 原生并发处理
- **批量写入**: 减少数据库压力

### 7.3 高可用设计
- **多实例部署**: 各服务可部署多个实例
- **负载均衡**: SD 提供服务端负载均衡
- **故障转移**: 自动检测并切换

### 7.4 安全机制
- **mTLS**: gRPC 双向证书认证
- **AK/SK**: HTTP API 认证
- **RBAC**: 基于角色的权限控制

---

## 八、通信协议

### 8.1 Agent ↔ AgentCenter

**gRPC 协议定义：**

```protobuf
// Transfer - Agent 数据上报
service Transfer {
  rpc Transfer(stream RawData) returns (stream Command){}
}

// RawData - Agent 原始数据
message RawData {
  repeated Record Data = 1;
  string AgentID = 2;
  repeated string IntranetIPv4 = 3;
  repeated string ExtranetIPv4 = 4;
  string Hostname = 7;
  string Version = 8;
  string Product = 9;
}

// Command - 下发命令
message Command {
  int32 AgentCtrl = 1;
  PluginTask Task = 2;
  repeated ConfigItem Config = 3;
}

// FileExt - 文件上传
service FileExt {
  rpc Upload(stream UploadRequest) returns (UploadResponse);
}
```

### 8.2 AgentCenter ↔ Kafka

**MQData 消息格式：**

```protobuf
message MQData {
  int32 DataType = 1;
  int64 AgentTime = 2;
  bytes Body = 3;
  string AgentID = 4;
  string IntranetIPv4 = 5;
  string ExtranetIPv4 = 6;
  string Hostname = 9;
  string Version = 10;
  string Product = 11;
  int64 SvrTime = 12;
}
```

---

## 九、配置文件说明

### 9.1 AgentCenter 配置

**文件位置：** `/server/agent_center/conf/svr.yml`

```yaml
server:
  grpc:
    port: 6751
    connlimit: 1500
  http:
    port: 6752
    auth:
      enable: true
      aksk: {...}
  ssl:
    keyfile: ./conf/server.key
    certfile: ./conf/server.crt
    cafile: ./conf/ca.crt

kafka:
  addrs:
    - 127.0.0.1:9092
  topic: hids_svr

sd:
  name: hids_svr
  addrs:
    - 127.0.0.1:8088
```

### 9.2 Manager 配置

**文件位置：** `/server/manager/conf/svr.yml`

```yaml
http:
  port: 6701
  apiauth:
    enable: true
    secret: xxx

mongo:
  uri: mongodb://user:pass@host:27017/elkeid
  dbname: elkeid

redis:
  addrs:
    - 127.0.0.1:6379
  passwd: xxx

es:
  host: []
  gzip: false
```

### 9.3 ServiceDiscovery 配置

**文件位置：** `/server/service_discovery/conf/conf.yaml`

```yaml
Server:
  Ip: "0.0.0.0"
  Port: 8088

Cluster:
  Mode: "config"
  Members: ["127.0.0.1:8088"]

Auth:
  Enable: true
  Keys:
    ak1: sk1
    ak2: sk2
```

---

## 十、关键文件清单

### AgentCenter
- `/home/work/openSource/Elkeid/server/agent_center/main.go`
- `/home/work/openSource/Elkeid/server/agent_center/conf/svr.yml`
- `/home/work/openSource/Elkeid/server/agent_center/grpctrans/proto/grpc.proto`
- `/home/work/openSource/Elkeid/server/agent_center/grpctrans/grpc_server.go`
- `/home/work/openSource/Elkeid/server/agent_center/httptrans/scsvr.go`

### Manager
- `/home/work/openSource/Elkeid/server/manager/main.go`
- `/home/work/openSource/Elkeid/server/manager/conf/svr.yml`
- `/home/work/openSource/Elkeid/server/manager/conf/index.json`
- `/home/work/openSource/Elkeid/server/manager/conf/rbac.json`
- `/home/work/openSource/Elkeid/server/manager/biz/handler/v6/`

### ServiceDiscovery
- `/home/work/openSource/Elkeid/server/service_discovery/main.go`
- `/home/work/openSource/Elkeid/server/service_discovery/conf/conf.yaml`
- `/home/work/openSource/Elkeid/server/service_discovery/server/server.go`

### 部署
- `/home/work/openSource/Elkeid/elkeidup/deploy.md`
- `/home/work/openSource/Elkeid/server/agent_center/build.sh`
- `/home/work/openSource/Elkeid/server/manager/build.sh`
- `/home/work/openSource/Elkeid/server/service_discovery/build.sh`
