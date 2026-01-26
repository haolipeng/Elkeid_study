# Elkeid Manager 模块代码逻辑分析

## 1. 目录结构

```
/home/work/openSource/Elkeid/server/manager/
├── main.go                           # 入口 (55行)
├── init/                             # 初始化
│   └── init.go                       # 组件初始化 (243行) ★
├── static/                           # 前端静态资源
│   └── frontend/                     # 嵌入的前端文件
├── infra/                            # 基础设施层
│   ├── defs.go                       # 集合定义 (193行) ★
│   ├── mongodb/mongodb.go            # MongoDB 客户端 (37行)
│   ├── redis/redis.go                # Redis 客户端
│   ├── discovery/                    # SD 服务发现
│   ├── tos/                          # 文件存储 (Nginx)
│   └── ylog/                         # 日志模块
├── biz/                              # 业务层
│   ├── router.go                     # 路由定义 (608行) ★
│   ├── midware/                      # 中间件
│   │   ├── tokenAuth.go              # JWT/Session 认证 (190行) ★
│   │   ├── rbacAuth.go               # RBAC 授权 (169行) ★
│   │   ├── akskAuth.go               # AK/SK 认证
│   │   └── metrics.go                # Prometheus 指标
│   └── handler/                      # API 处理器
│       ├── v0/                       # 内部 API (集群同步/任务)
│       ├── v1/                       # 遗留 API (Agent/用户管理)
│       └── v6/                       # Console API (30+ 文件) ★
└── internal/                         # 内部模块 (19+ 个)
    ├── alarm/                        # 告警处理
    ├── alarm_whitelist/              # 告警白名单
    ├── asset_center/                 # 资产管理
    ├── atask/                        # Agent 任务
    ├── baseline/                     # 基线扫描
    ├── container/                    # 容器安全
    ├── cronjob/                      # 定时任务
    ├── dbtask/                       # 数据库任务
    ├── distribute/job/               # 分布式任务 ★
    ├── kube/                         # K8s 安全
    ├── login/                        # 用户登录
    ├── metrics/                      # 监控指标
    ├── monitor/                      # 服务监控
    ├── outputer/                     # 数据输出
    ├── rasp/                         # RASP 集成
    ├── virus_detection/              # 病毒检测
    └── vuln/                         # 漏洞检测
```

**总代码量：约 25,000+ 行**

---

## 2. 模块概述

Manager 模块是 Elkeid 服务端的**后台管理中心**，负责：
- 提供 Web Console API 接口
- 管理 Agent 配置和任务下发
- 处理告警、漏洞、基线等安全数据
- 协调多 Manager 实例的分布式任务
- 用户认证和权限管理

---

## 3. 核心组件详解

### 3.1 启动入口 (`main.go`)

```go
func main() {
    err := initialize.Initialize()   // 初始化所有组件
    if err != nil {
        os.Exit(-1)
    }

    reg := discovery.NewServerRegistry()  // 向 SD 注册
    defer reg.Stop()

    go ServerStart()  // 启动 HTTP 服务

    <-infra.Sig
    close(infra.Quit)
}
```

---

### 3.2 初始化流程 (`init/init.go`)

```
Initialize()
  │
  ├─ initLog()                    # 配置日志
  ├─ initDefault()                # 加载配置
  ├─ initComponents()             # 连接 Redis/MongoDB
  │
  ├─ initDistribute()             # 分布式任务初始化 ★
  │   ├─ job.InitApiMap()
  │   ├─ job.AJF.Register("Server_AgentStat", ...)
  │   ├─ job.AJF.Register("Server_AgentList", ...)
  │   ├─ job.AJF.Register("Agent_Config", ...)
  │   ├─ job.AJF.Register("Agent_Ctrl", ...)
  │   ├─ job.AJF.Register("Agent_Task", ...)
  │   ├─ job.JM = job.NewJobManager()
  │   └─ job.CM = job.NewCronJobManager()
  │       ├─ CM.Add("Server_AgentStat", 180s, 120, 300)
  │       └─ CM.Add("Server_AgentList", 120s, 120, 180)
  │
  ├─ atask.Init()                 # Agent 任务初始化
  ├─ initAlarmWhitelist()         # 告警白名单
  ├─ initKube()                   # K8s 安全
  ├─ initMonitor()                # 服务监控
  ├─ metrics.Init()               # Prometheus 指标
  ├─ login.Init()                 # 用户登录
  ├─ baseline.InitBaseline()      # 基线扫描
  ├─ vuln.InitVuln()              # 漏洞检测
  ├─ rasp.RaspInit()              # RASP 集成
  ├─ container.ContainerInit()    # 容器安全
  ├─ outputer.InitOutput()        # 数据输出
  ├─ cronjob.InitCronjob()        # 定时任务
  ├─ initV6()                     # v6 API 初始化
  ├─ initIndexes()                # MongoDB 索引
  └─ initTos()                    # 文件存储
```

---

### 3.3 API 路由层级 (`biz/router.go`)

#### 路由分组

| 分组 | 前缀 | 认证方式 | 说明 |
|------|------|----------|------|
| 公开 | `/` | 无 | `/metrics`, `/ping`, 前端静态资源 |
| v0 内部 | `/api/v0/inner` | AK/SK | 集群同步 |
| v0 任务 | `/api/v0/job` | Token | 分布式任务管理 |
| v1 遗留 | `/api/v1` | Token + RBAC | 用户/Agent 管理 |
| v6 Console | `/api/v6` | Token + RBAC | 前端 API |

#### API 模块分布

```
/api/v6/
├── user/           # 用户管理 (4个接口)
├── agent/          # Agent 任务 (6个接口)
├── asset-center/   # 资产中心 (20+ 个接口)
│   └── fingerprint/  # 资产指纹 (15+ 个接口)
├── shared/         # 文件上传下载 (2个接口)
├── component/      # 组件管理 (10个接口)
├── alarm/          # 告警管理 (7个接口)
├── whitelist/      # 告警白名单 (4个接口)
├── vuln/           # 漏洞检测 (15个接口)
├── baseline/       # 基线扫描 (15个接口)
├── systemRouter/   # 系统告警 (2个接口)
├── monitor/        # 服务监控 (15个接口)
├── rasp/           # RASP 管理 (20+ 个接口)
├── kube/           # K8s 安全 (25+ 个接口)
├── overview/       # 首页概览 (6个接口)
├── virus/          # 病毒检测 (15个接口)
├── notice/         # 通知管理 (6个接口)
└── license/        # 授权管理 (2个接口)
```

---

### 3.4 认证授权机制

#### 3.4.1 Token 认证 (`midware/tokenAuth.go`)

```
请求头: token: {JWT或Session}
  ↓
TokenAuth() 中间件
  │
  ├─ ApiAuth 禁用 → 直接放行
  │
  ├─ 白名单 URL → 直接放行
  │   ├─ /api/v1/user/login
  │   ├─ /api/v1/agent/updateSubTask
  │   ├─ /api/v6/component/GetComponentInstances
  │   └─ ...
  │
  └─ 验证 Token
      │
      ├─ Session Token (以 "seesion-" 开头)
      │   └─ Redis 获取用户名，刷新过期时间
      │
      └─ JWT Token
          └─ VerifyToken(token, secret)
              └─ 提取 username
  ↓
设置 c.Set("user", userName)
c.Next()
```

#### 3.4.2 RBAC 授权 (`midware/rbacAuth.go`)

```
rbac.json 配置文件
  │
  └─ MetaRule 规则
      ├─ PathEqual: 精确匹配
      ├─ PathPre: 前缀匹配
      ├─ PathRegex: 正则匹配
      ├─ AuthorizedRoles: 授权角色列表
      └─ AllowAnyone: 允许任何人
  ↓
RBACAuth() 中间件
  ↓
ACWorker.IsRequestGranted(path, username)
  ↓
遍历规则列表:
  ├─ MatchPath(path) → 匹配路径
  └─ MatchRole(username) → 检查用户角色
      └─ login.GetUser(username).Level in rule.AuthorizedRoles
  ↓
通过 → c.Next()
失败 → 403 Forbidden
```

#### 3.4.3 JWT 配置

```go
const (
    JWTExpireMinute = 720  // 12 小时过期
)

type AuthClaims struct {
    Username string
    jwt.StandardClaims
}
```

---

### 3.5 分布式任务系统 (`internal/distribute/job/`)

#### 核心架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    分布式任务系统架构                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐         │
│  │  Manager-1  │     │  Manager-2  │     │  Manager-3  │         │
│  │             │     │             │     │             │         │
│  │ JobManager  │────►│ JobManager  │────►│ JobManager  │         │
│  │             │ sync│             │ sync│             │         │
│  └──────┬──────┘     └──────┬──────┘     └──────┬──────┘         │
│         │                   │                   │                 │
│         └───────────────────┼───────────────────┘                 │
│                             │                                     │
│                    ┌────────▼────────┐                           │
│                    │      Redis      │                           │
│                    │                 │                           │
│                    │ ┌─────────────┐ │                           │
│                    │ │ Pub/Sub     │ │ ◄── 任务分发              │
│                    │ │ 通道        │ │                           │
│                    │ └─────────────┘ │                           │
│                    │ ┌─────────────┐ │                           │
│                    │ │ Job 状态    │ │ ◄── 状态存储              │
│                    │ └─────────────┘ │                           │
│                    └─────────────────┘                           │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

#### CronJob 定时任务

```go
// 定时采集 Agent 状态
job.CM.Add("Server_AgentStat", 180, 120, 300)
// interval=180s, conNum=120个并发, timeout=300s

// 定时采集 Agent 列表
job.CM.Add("Server_AgentList", 120, 120, 180)
```

#### 任务执行流程

```
CronJobManager.Manage()
  ↓
每 interval 秒:
  ├─ Redis SetNX 获取分布式锁
  │   └─ 锁 Key: LOCK:{name}:{timestamp/interval}
  │
  ├─ 获取锁成功 → NewCronJob()
  │   ├─ 创建 SimpleJob
  │   ├─ 同步到其他 Manager (HTTP POST /api/v0/inner/sync)
  │   └─ 启动 Run() goroutine
  │
  ├─ DistributeJob() 分发任务
  │   ├─ 调用 Dis 回调生成任务列表
  │   ├─ 轮询分发到所有 Manager
  │   └─ Redis Publish 到对应 channel
  │
  └─ Finish() 发送结束信号
      └─ Publish FINISH 到所有 channel
```

#### Job 执行

```go
func (sj *SimpleJob) Run(over chan bool) {
    // 订阅本机任务 channel
    jobChannel := fmt.Sprintf("chan-%s-%s", LocalHost, sj.Id)
    ps := sj.Rds.Subscribe(ctx, jobChannel)

    // 启动 ConNum 个 goroutine
    for i := 0; i < sj.ConNum; i++ {
        go func() {
            for {
                select {
                case jobMsg := <-ch:
                    if jobMsg.Payload == "FINISH" {
                        return
                    }
                    // 执行任务
                    rlt, err := sj.Do(sj.Id, jobMsg.Payload)
                    sj.Rlt(sj.Id, rlt)
                case <-timeout:
                    return
                case <-sj.Done:
                    return
                }
            }
        }()
    }
    wg.Wait()
}
```

---

### 3.6 数据存储层

#### MongoDB 集合定义 (`infra/defs.go`)

| 类别 | 集合名 | 说明 |
|------|--------|------|
| **用户** | `user` | 用户信息 |
| **Agent** | `agent_heartbeat` | Agent 心跳数据 |
| | `agent_task` | Agent 任务 |
| | `agent_subtask` | Agent 子任务 |
| | `agent_config_template` | 配置模板 |
| **告警** | `hub_alarm_v1` | HIDS 告警 |
| | `hub_whitelist_v1` | 告警白名单 |
| | `hub_alarm_event_v1` | 告警事件 |
| | `rasp_alarm_v1` | RASP 告警 |
| | `kube_alarm_v1` | K8s 告警 |
| | `virus_detection_alarm_v1` | 病毒告警 |
| **漏洞** | `vuln_info` | 漏洞信息 |
| | `agent_vuln_info` | Agent 漏洞 |
| | `vuln_task_status` | 漏洞任务状态 |
| **基线** | `baseline_info` | 基线信息 |
| | `agent_baseline` | Agent 基线 |
| | `baseline_check_info` | 检查结果 |
| **资产** | `agent_asset_5050` | 进程指纹 |
| | `agent_asset_5051` | 端口指纹 |
| | `agent_asset_5052` | 用户指纹 |
| | `agent_asset_5053` | 定时任务 |
| | `agent_asset_5054` | 服务指纹 |
| | `agent_asset_5055` | 软件指纹 |
| | `agent_asset_5056` | 容器信息 |
| **K8s** | `kube_cluster_config` | 集群配置 |
| | `kube_cluster_info` | 集群信息 |
| | `kube_node_info` | 节点信息 |
| | `kube_pod_info` | Pod 信息 |
| **组件** | `component` | 组件信息 |
| | `component_version` | 组件版本 |
| | `component_policy` | 组件策略 |

---

### 3.7 内部模块详解

#### 告警模块 (`internal/alarm/`)

```
告警处理流程:
  │
  ├─ alarm_init.go        # 初始化告警类型映射
  ├─ alarm_query.go       # 告警查询 (分页/过滤)
  ├─ alarm_update.go      # 状态更新 (已处理/忽略/误报)
  ├─ alarm_statistics.go  # 统计分析
  └─ alarm_data_type.go   # 告警数据类型定义
```

#### 漏洞模块 (`internal/vuln/`)

```
漏洞检测流程:
  │
  ├─ vuln_init.go    # 初始化漏洞库
  ├─ task.go         # 检测任务管理
  ├─ cronjob.go      # 定时扫描
  └─ base_type.go    # 数据类型定义
```

#### 基线模块 (`internal/baseline/`)

```
基线扫描流程:
  │
  ├─ baseline_init.go    # 初始化基线规则
  ├─ baseline_config.go  # 基线配置
  ├─ cronjob.go          # 定时检查
  └─ base_type.go        # 数据类型定义
```

#### 分布式数据库任务 (`internal/dbtask/`)

```
数据写入流程:
  │
  ├─ db_writer.go             # 通用写入器
  ├─ agent_hb.go              # Agent 心跳写入
  ├─ agent_subtask.go         # 子任务更新
  ├─ hub_alarm.go             # HIDS 告警写入
  ├─ hub_asset.go             # 资产数据写入
  ├─ kube_alarm.go            # K8s 告警写入
  ├─ rasp_alarm.go            # RASP 告警写入
  ├─ virus_detection.go       # 病毒检测写入
  ├─ leader_vuln.go           # 漏洞数据处理
  └─ leader_baseline.go       # 基线数据处理
```

---

## 4. 数据流向图

```
                     ┌─────────────────────────────────────┐
                     │           Web Console               │
                     │         (前端界面)                   │
                     └───────────────┬─────────────────────┘
                                     │ HTTP
                     ┌───────────────▼─────────────────────┐
                     │             Manager                  │
                     │  ┌─────────────────────────────────┐ │
                     │  │         Gin Router              │ │
                     │  │    Token + RBAC 认证            │ │
                     │  └───────────────┬─────────────────┘ │
                     │          ┌───────┴───────┐           │
                     │          │               │           │
                     │  ┌───────▼───────┐ ┌─────▼─────────┐ │
                     │  │  Handler v6   │ │ Handler v1/v0 │ │
                     │  │  (Console)    │ │  (内部/遗留)  │ │
                     │  └───────┬───────┘ └───────┬───────┘ │
                     │          └───────┬─────────┘         │
                     │          ┌───────▼───────┐           │
                     │          │   Internal    │           │
                     │          │   Modules     │           │
                     │          │  (19+ 模块)   │           │
                     │          └───────┬───────┘           │
                     │  ┌───────────────┼───────────────┐   │
                     │  │               │               │   │
                     │  ▼               ▼               ▼   │
                     │ MongoDB       Redis        AgentCenter│
                     │ (数据存储)   (缓存/任务)   (命令下发)  │
                     └─────────────────────────────────────┘
                                     │
              ┌──────────────────────┼──────────────────────┐
              ▼                      ▼                      ▼
     ┌────────────────┐    ┌────────────────┐    ┌────────────────┐
     │   Manager-2    │    │   Manager-3    │    │      ...       │
     │  (分布式节点)  │    │  (分布式节点)  │    │                │
     └────────────────┘    └────────────────┘    └────────────────┘
```

---

## 5. API 白名单机制

```go
var whiteUrlList = []string{
    "/api/v1/user/login",                      // 登录
    "/api/v1/agent/updateSubTask",             // AC 数据对账
    "/api/v1/agent/queryInfo",                 // AC 查询 Agent 信息
    "/api/v6/component/GetComponentInstances", // AC 获取组件配置
    "/api/v6/shared/Upload",                   // 文件上传
    "/api/v6/kube/inner/cluster/list",         // AC 获取 K8s 集群
    "/api/v6/systemRouter/InsertAlert",        // 系统告警
    // ...
}
```

---

## 6. 关键时间参数

| 参数 | 值 | 说明 |
|------|-----|------|
| JWT 过期时间 | 720min (12h) | Token 有效期 |
| Session 过期 | 配置 | Redis Session 有效期 |
| AgentStat 采集 | 180s | 定时采集 Agent 状态 |
| AgentList 采集 | 120s | 定时采集 Agent 列表 |
| Job 默认超时 | 300s | 任务执行超时 |
| Job 状态过期 | 12h | Redis 任务状态保留时间 |
| 分布式锁有效期 | interval | 与任务间隔相同 |
| 任务分发重试 | 10次 | Publish 失败重试次数 |
| 任务分发间隔 | 500ms | 重试等待间隔 |
| MongoDB 连接池 | 10 | 最小/最大连接数 |

---

## 7. 高可用设计

### 7.1 多实例部署
- 多 Manager 实例向 SD 注册
- 负载均衡器分发请求
- Session 存储在 Redis，支持会话漂移

### 7.2 分布式任务
- Redis Pub/Sub 实现任务分发
- SetNX 分布式锁避免重复执行
- 任务状态和结果存储在 Redis

### 7.3 任务同步
- 新建任务时同步到所有 Manager
- 各 Manager 订阅自己的任务 Channel
- 任务执行结果汇总到 Redis

### 7.4 故障恢复
- 任务执行失败自动进入 Retry 队列
- Job 超时自动终止
- 分布式锁自动过期

---

## 8. 安全机制

### 8.1 多层认证
```
请求 → Token 认证 → RBAC 授权 → Handler
        │              │
        ├─ JWT         ├─ 路径匹配
        └─ Session     └─ 角色检查
```

### 8.2 密码安全
```go
// SHA1 + Salt 哈希
hash = SHA1(password + salt)
```

### 8.3 AK/SK 认证
- 用于内部服务间通信
- HMAC-SHA256 签名
- 时间戳防重放

---

## 9. 外部依赖关系

```
Manager 模块依赖：
├── ServiceDiscovery  - 服务注册发现
├── AgentCenter       - 命令下发
├── MongoDB           - 数据存储
├── Redis             - 缓存、Session、分布式任务
├── Kafka             - 数据消费 (可选)
├── Elasticsearch     - 日志存储 (可选)
├── Nginx             - 文件存储
├── gin               - HTTP 框架
├── grequests         - HTTP 客户端
├── go-redis          - Redis 客户端
├── mongo-driver      - MongoDB 客户端
├── jwt-go            - JWT 认证
├── prometheus        - 指标收集
└── viper             - 配置管理

被依赖：
├── Web Console       - 前端界面
├── AgentCenter       - 配置获取、任务上报
└── 外部系统          - API 集成
```

---

## 10. Prometheus 指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `elkeid_manager_http_api` | Counter | path, code | HTTP API 调用计数 |
| `elkeid_manager_agent_count` | Gauge | - | Agent 总数 |
| `elkeid_manager_alarm_count` | Gauge | type | 各类告警数量 |
| `elkeid_manager_job_count` | Gauge | name, status | 任务执行统计 |

---

## 11. 内部模块概览

| 模块 | 文件数 | 主要功能 |
|------|--------|----------|
| alarm | 6 | 告警查询、更新、统计 |
| alarm_whitelist | 4 | 白名单管理 |
| asset_center | 1 | 资产管理 |
| atask | 5 | Agent 任务调度 |
| baseline | 4 | 基线扫描配置和检查 |
| container | 4 | 容器安全、K8s 集群管理 |
| cronjob | 4 | 定时任务调度 |
| dbtask | 12 | 数据库批量写入 |
| distribute/job | 4 | 分布式任务框架 |
| kube | 7 | K8s 安全告警和统计 |
| login | 2 | 用户认证和管理 |
| metrics | 7 | 监控指标收集 |
| monitor | 6+ | 服务状态监控 |
| outputer | 6 | 数据输出 (Kafka/ES/Syslog) |
| rasp | 2+ | RASP 进程管理和告警 |
| virus_detection | - | 病毒检测告警 |
| vuln | 4 | 漏洞检测和管理 |

---

## 总结

Manager 模块用约 **25,000+ 行代码**实现了：

1. **Web API 服务** - 提供 100+ 个 REST API 接口
2. **多层认证授权** - JWT/Session + RBAC 权限控制
3. **分布式任务** - Redis Pub/Sub 实现跨节点任务调度
4. **多维度安全** - 告警/漏洞/基线/RASP/K8s/病毒检测
5. **资产管理** - 10+ 种资产指纹采集和查询
6. **组件管理** - Agent 组件版本和策略管理
7. **服务监控** - Agent 和服务状态监控
8. **数据输出** - Kafka/ES/Syslog 多通道输出
9. **高可用** - 多实例部署 + 分布式锁 + 任务同步
10. **前端集成** - 嵌入静态资源，一体化部署
