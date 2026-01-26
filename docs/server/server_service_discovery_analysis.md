# Elkeid ServiceDiscovery 模块代码逻辑分析

## 1. 目录结构

```
/home/work/openSource/Elkeid/server/service_discovery/
├── main.go                           # 入口 (17行)
├── cluster/                          # 集群管理
│   ├── cluster.go                    # 集群接口 + ping 机制 (92行)
│   └── use_config.go                 # 配置初始化 (49行)
├── common/                           # 公共工具
│   ├── init.go                       # 配置初始化 (77行)
│   ├── safemap/sm.go                 # 线程安全 Map (128行)
│   └── ylog/                         # 日志模块
├── server/                           # HTTP 服务
│   ├── server.go                     # 服务启动 (29行)
│   ├── router.go                     # 路由注册 (40行)
│   ├── midware/                      # 中间件
│   │   ├── akskAuth.go               # AK/SK 认证 (158行)
│   │   └── metrics.go                # Prometheus 指标 (29行)
│   └── handler/                      # API 处理器
│       ├── init.go                   # 初始化 (21行)
│       ├── registry.go               # Register/Evict/Sync (126行)
│       ├── endpoint.go               # 端点状态 (20行)
│       └── metrics.go                # 注册列表指标 (30行)
└── endpoint/                         # 核心注册逻辑
    ├── endpoint.go                   # 注册表管理 (340行) ★
    └── utils.go                      # 负载均衡算法 (95行)
```

**总代码量：约 1,251 行**

---

## 2. 模块概述

ServiceDiscovery (SD) 模块是 Elkeid 服务端的**服务注册发现中心**，负责：
- 管理 AgentCenter 等服务的注册和注销
- 提供服务实例列表查询（负载均衡）
- 集群节点间同步注册信息
- 定时健康检查和自动剔除

---

## 3. 核心组件详解

### 3.1 启动入口 (`main.go`)

```go
func main() {
    go server.ServerStart(common.SrvIp, common.SrvPort)
    <-common.Quit
    fmt.Printf("game over ...\n")
}
```

启动流程：
1. **common.init()** - 加载配置、初始化日志
2. **handler.init()** - 创建 Cluster 和 Endpoint 实例
3. **ServerStart()** - 启动 HTTP 服务

---

### 3.2 注册表核心 (`endpoint/endpoint.go`)

#### Registry 数据结构

```go
type Registry struct {
    Name     string                 `json:"name"`      // 服务名称 (如 "ac")
    Ip       string                 `json:"ip"`        // 服务 IP
    Port     int                    `json:"port"`      // 服务端口
    Status   int                    `json:"status"`    // 健康状态 (0-4)
    CreateAt int64                  `json:"create_at"` // 创建时间戳
    UpdateAt int64                  `json:"update_at"` // 最后更新时间戳
    Weight   int                    `json:"weight"`    // 负载权重
    Extra    map[string]interface{} `json:"extra"`     // 扩展数据
}
```

#### Endpoint 实例

```go
type Endpoint struct {
    cluster     cluster.Cluster       // 集群管理器
    registryMap *safemap.SafeMap      // 注册表 Map[name][ip:port] -> Registry
    sendChan    chan SyncInfo         // 发送同步通道 (8192 缓冲)
    recvChan    chan TransInfo        // 接收同步通道 (8192 缓冲)
    stop        chan bool             // 停止信号
}

func NewEndpoint(cluster cluster.Cluster) *Endpoint {
    e := &Endpoint{...}
    go e.registryRefresh()  // 健康检查协程
    go e.syncSend()         // 同步发送协程
    go e.syncRecv()         // 同步接收协程
    return e
}
```

#### 健康状态机

```
┌─────────────────────────────────────────────────────────────────┐
│                    健康状态自动转换 (每15秒检查)                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ≤45s          46-60s        61-75s        76-90s       91-105s  │
│  ┌──────┐     ┌──────┐     ┌──────┐     ┌──────┐     ┌──────┐   │
│  │Green │ ──► │ Blue │ ──► │Yellow│ ──► │Orange│ ──► │ Red  │   │
│  │  0   │     │  1   │     │  2   │     │  3   │     │  4   │   │
│  └──────┘     └──────┘     └──────┘     └──────┘     └──────┘   │
│                                                           │       │
│                                                      >105s│       │
│                                                           ▼       │
│                                                      ┌──────┐    │
│                                                      │ EVICT│    │
│                                                      │ 剔除  │    │
│                                                      └──────┘    │
└─────────────────────────────────────────────────────────────────┘
```

#### registryRefresh 健康检查逻辑

```go
func (e *Endpoint) registryRefresh() {
    t := time.NewTicker(15 * time.Second)  // 每15秒检查一次
    for {
        select {
        case <-t.C:
            nowAt := time.Now().Unix()
            for _, name := range e.registryMap.Keys() {
                for _, reg := range e.registryMap.Get(name) {
                    d := nowAt - reg.UpdateAt
                    if d <= 45 {
                        reg.Status = StatusGreen
                    } else if d <= 60 {
                        reg.Status = StatusBlue
                    } else if d <= 75 {
                        reg.Status = StatusYellow
                    } else if d <= 90 {
                        reg.Status = StatusOrange
                    } else if d <= 105 {
                        reg.Status = StatusRed
                    } else {
                        e.Evict(name, reg.Ip, reg.Port)  // 超过105秒自动剔除
                    }
                }
            }
        case <-e.stop:
            return
        }
    }
}
```

---

### 3.3 服务注册与注销

#### Register 注册流程

```
客户端 POST /registry/register
  ↓
handler.Register() 解析 JSON
  ↓
EI.Register(name, ip, port, weight, extra)
  │
  ├─ 新注册: 创建 Registry {Status: Green, CreateAt: now, UpdateAt: now}
  │
  └─ 已存在: 更新 Weight 和 UpdateAt (刷新心跳)
  ↓
写入 registryMap[name]["ip:port"]
  ↓
推送 SyncInfo{Action: "REGISTER", Registry} 到 sendChan
  ↓
响应 {"msg": "ok"}
```

#### Evict 注销流程

```
客户端 POST /registry/evict
  ↓
handler.Evict() 解析 JSON
  ↓
EI.Evict(name, ip, port)
  │
  ├─ 从 registryMap 删除
  │
  └─ 推送 SyncInfo{Action: "EVICT"} 到 sendChan
  ↓
响应 {"msg": "ok"}
```

---

### 3.4 集群同步机制

#### 同步发送 (syncSend)

```
┌─────────────────────────────────────────────────────────────┐
│                    syncSend 协程                             │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  sendChan ──► 收集 SyncInfo 到 syncInfoList                  │
│                    │                                          │
│       ┌────────────┴────────────┐                            │
│       ▼                         ▼                             │
│  达到100条               每2秒定时触发                         │
│       │                         │                             │
│       └────────────┬────────────┘                            │
│                    ▼                                          │
│       封装 TransInfo{Source: 本机, Data: syncInfoList}        │
│                    │                                          │
│                    ▼                                          │
│       并发 POST 到所有其他节点 /registry/sync                   │
│       (超时 2秒)                                               │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

#### 同步接收 (syncRecv)

```
其他节点 POST /registry/sync
  ↓
handler.Sync() 解析 TransInfo
  ↓
EI.Recv(transInfo) 写入 recvChan
  ↓
syncRecv 协程处理:
  for _, syncInfo := range transInfo.Data {
      switch syncInfo.Action {
      case "REGISTER":
          registryMap.HSet(name, "ip:port", registry)
      case "EVICT":
          registryMap.HDel(name, "ip:port")
      }
  }
```

---

### 3.5 负载均衡算法 (`endpoint/utils.go`)

```go
type FetchAlgorithm func(items ItemList, n int) []Item

// 按权重升序，取最小的 N 个 (负载最轻)
func FetchMinN(items ItemList, n int) []Item

// 按权重降序，取最大的 N 个 (负载最重)
func FetchMaxN(items ItemList, n int) []Item

// 随机选取 N 个
func FetchRandomN(items ItemList, n int) []Item
```

#### Fetch API 调用

```
GET /registry/detail?name=ac&mode=min&count=5
  ↓
handler.Fetch() 解析参数
  │
  ├─ mode=min  → FetchMinN  (默认)
  ├─ mode=max  → FetchMaxN
  └─ mode=rand → FetchRandomN
  ↓
EI.Fetch(name, count, algorithm)
  ↓
返回 Registry 列表 (最多100个)
```

---

### 3.6 集群管理 (`cluster/`)

#### Cluster 接口

```go
type Cluster interface {
    refresh()            // 刷新集群成员列表
    ping()               // 定时 ping 其他节点
    Stop()               // 停止集群
    GetHost() string     // 获取本机地址
    GetHosts() []string  // 获取所有成员
    GetOtherHosts() []string  // 获取其他成员
}
```

#### ConfigCluster 实现

```
┌─────────────────────────────────────────────────────────┐
│                 ConfigCluster                             │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  refresh() 协程:                                          │
│    监听 ConfigChangeNotify                                │
│    从配置文件读取 Cluster.Members                          │
│    更新 Members SafeMap                                   │
│                                                           │
│  ping() 协程:                                             │
│    每 5 秒 GET http://{host}/endpoint/ping                │
│    检测其他节点存活                                        │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

---

### 3.7 HTTP API 路由 (`server/router.go`)

| 路径 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `/metrics` | GET | 否 | Prometheus 指标 |
| `/ping` | GET | 否 | 健康检查 |
| `/registry/register` | POST | AK/SK | 服务注册 |
| `/registry/evict` | POST | AK/SK | 服务注销 |
| `/registry/sync` | POST | AK/SK | 集群同步 |
| `/endpoint/ping` | GET | 否 | 集群节点检测 |
| `/endpoint/stat` | GET | 否 | 集群成员列表 |
| `/registry/summary` | GET | 否 | 注册摘要 |
| `/registry/detail` | GET | 否 | 服务详情 |
| `/registry/list` | GET | 否 | 服务名称列表 |

---

### 3.8 AK/SK 认证 (`server/midware/akskAuth.go`)

#### 签名生成算法

```
Sign = HMAC-SHA256(
    "{Method}\n{Path}\n{Query}\n{AK}\n{Timestamp}\n{SHA256(Body)}",
    SK
)
```

#### 认证流程

```
请求头:
  AccessKey: {AK}
  Signature: {Sign}
  TimeStamp: {Unix时间戳}
  ↓
AKSKAuth 中间件:
  1. 检查时间戳是否在 ±60秒内
  2. 根据 AK 查找 SK
  3. 计算 serverSign = generateSign(...)
  4. 比对 serverSign == 请求Sign
  ↓
  通过 → c.Next()
  失败 → 403 Forbidden
```

---

### 3.9 SafeMap 线程安全存储 (`common/safemap/sm.go`)

```go
type SafeMap struct {
    name    string
    dataMap map[string]map[string]interface{}  // 二级 Map
    mu      sync.RWMutex
}

// 主要方法
HSet(key, subKey, value)   // 设置 dataMap[key][subKey] = value
HGet(key, subKey)          // 获取 dataMap[key][subKey]
HDel(key, subKey)          // 删除 dataMap[key][subKey]
Get(key)                   // 获取 dataMap[key] 的副本
Keys()                     // 获取所有一级 key
HKeys(key)                 // 获取指定 key 的所有 subKey
```

---

## 4. 数据流向图

```
                    ┌──────────────────────────────────────────┐
                    │          ServiceDiscovery 集群             │
                    │  ┌─────────┐  ┌─────────┐  ┌─────────┐   │
                    │  │  SD-1   │◄─┤  SD-2   │◄─┤  SD-3   │   │
                    │  │ Master  │─►│ Replica │─►│ Replica │   │
                    │  └────┬────┘  └────┬────┘  └────┬────┘   │
                    │       │ sync       │ sync       │        │
                    │       └────────────┴────────────┘        │
                    └──────────────────┬───────────────────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              │                        │                        │
              ▼                        ▼                        ▼
     ┌────────────────┐      ┌────────────────┐      ┌────────────────┐
     │  AgentCenter-1 │      │  AgentCenter-2 │      │    Manager     │
     │                │      │                │      │                │
     │ POST /register │      │ POST /register │      │ GET /detail    │
     │ (每30s心跳)    │      │ (每30s心跳)    │      │ (获取AC列表)   │
     └────────────────┘      └────────────────┘      └────────────────┘
              │                        │
              ▼                        ▼
     ┌────────────────────────────────────────────────────────┐
     │                        Agents                           │
     │  通过 SD 获取 AC 列表，选择负载最低的 AC 建立 gRPC 连接  │
     └────────────────────────────────────────────────────────┘
```

---

## 5. 关键时间参数

| 参数 | 值 | 说明 |
|------|-----|------|
| 健康检查间隔 | 15s | registryRefresh 定时器周期 |
| 集群同步间隔 | 2s | syncSend 批量发送周期 |
| 同步批量阈值 | 100 | 达到100条立即发送 |
| 集群 ping 间隔 | 5s | ping 其他节点周期 |
| 同步超时 | 2s | HTTP 请求超时 |
| ping 超时 | 1s | ping 请求超时 |
| 认证时间窗口 | ±60s | 时间戳有效范围 |
| Green 阈值 | ≤45s | 健康状态 |
| Blue 阈值 | 46-60s | 轻微延迟 |
| Yellow 阈值 | 61-75s | 警告状态 |
| Orange 阈值 | 76-90s | 危险状态 |
| Red 阈值 | 91-105s | 临界状态 |
| 自动剔除 | >105s | 超时自动删除 |

---

## 6. 高可用设计

### 6.1 集群同步
- 多节点部署，每个节点独立提供服务
- 注册/注销操作实时同步到所有节点
- 批量+定时双重机制保证同步效率

### 6.2 健康检查
- 5 级健康状态，渐进式降级
- 超过 105 秒无心跳自动剔除
- ping 机制检测集群节点存活

### 6.3 负载均衡
- 支持 min/max/random 三种算法
- 默认返回权重最小的实例（负载最轻）
- 最多返回 100 个实例

### 6.4 配置热加载
- 使用 fsnotify 监控配置文件变化
- 自动更新集群成员列表

---

## 7. 外部依赖关系

```
ServiceDiscovery 模块依赖：
├── viper          - 配置管理
├── fsnotify       - 文件变化监控
├── gin            - HTTP 框架
├── grequests      - HTTP 客户端
├── prometheus     - 指标收集
└── ylog           - 日志记录

被依赖：
├── AgentCenter    - 调用 /registry/register 注册服务
├── Agent          - 调用 /registry/detail 获取 AC 列表
└── Manager        - 查询服务状态
```

---

## 8. Prometheus 指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `elkeid_sd_http_api` | Counter | path, code | HTTP API 调用计数 |
| `elkeid_sd_registry_list` | Gauge | name | 各服务注册实例数量 |

---

## 总结

ServiceDiscovery 模块用约 **1,251 行代码**实现了：

1. **服务注册发现** - Register/Evict/Fetch 完整生命周期管理
2. **集群同步** - 多节点实时同步，保证数据一致性
3. **健康检查** - 5 级状态渐进降级 + 自动剔除
4. **负载均衡** - 支持最小权重/最大权重/随机三种算法
5. **安全认证** - HMAC-SHA256 签名的 AK/SK 认证
6. **可观测性** - Prometheus 指标 + 结构化日志
7. **高可用** - 集群部署 + 配置热加载
