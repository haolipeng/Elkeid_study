# Elkeid AgentCenter 模块代码逻辑分析

## 1. 目录结构

```
/home/work/openSource/Elkeid/server/agent_center/
├── main.go                           # 入口 (53行)
├── svr_registry/                     # SD 注册
│   └── svr_registry.go               # 服务注册客户端 (120行)
├── common/                           # 公共工具
│   ├── init.go                       # 配置 & Kafka 初始化 (106行)
│   ├── kafka/                        # Kafka 集成
│   │   ├── producer.go               # 异步生产者 (162行) ★
│   │   └── consumer.go               # 消费者
│   ├── snappy/                       # Snappy 压缩
│   ├── zstd/                         # Zstd 压缩
│   └── ylog/                         # 日志模块
├── grpctrans/                        # gRPC 服务
│   ├── grpc_server.go                # 服务配置 (128行)
│   ├── proto/grpc.pb.go              # 协议定义
│   ├── pool/                         # 连接池
│   │   └── pool.go                   # GRPCPool 实现 (356行) ★
│   └── grpc_handler/                 # 请求处理器
│       ├── transfer_handler.go       # 数据传输 (139行) ★
│       ├── rawdata_worker.go         # 数据处理 (227行) ★
│       ├── file_handler.go           # 文件上传 (221行)
│       └── metrics.go                # Prometheus 指标 (163行)
└── httptrans/                        # HTTP 服务
    ├── scsvr.go                      # 服务配置 (108行)
    ├── midware/                      # 中间件
    │   └── akskAuth.go               # AK/SK 认证
    ├── http_handler/                 # API 处理器
    │   ├── command.go                # 命令下发 (86行) ★
    │   ├── conn.go                   # 连接管理 (104行)
    │   └── ...
    └── client/                       # Manager 客户端
        ├── config.go                 # 配置获取 (111行)
        ├── task.go                   # 任务上报 (44行)
        └── file.go                   # 文件上传
```

**总代码量：约 8,188 行**

---

## 2. 模块概述

AgentCenter (AC) 模块是 Elkeid 服务端的**数据中转核心**，负责：
- 与 Agent 建立 gRPC 双向流连接
- 接收 Agent 上报的采集数据
- 将数据写入 Kafka 供后续处理
- 接收 Manager 下发的命令并转发给 Agent
- 管理 Agent 连接池和状态
- 向 ServiceDiscovery 注册服务

---

## 3. 核心组件详解

### 3.1 启动入口 (`main.go`)

```go
func main() {
    ylog.Infof("[MAIN]", "START_SERVER")

    go httptrans.Run()    // HTTP 服务 (API + RawData)
    go grpctrans.Run()    // gRPC 服务 (Transfer + FileExt)
    go debug()            // pprof 调试

    // 向 SD 注册 gRPC 和 HTTP 服务
    regGrpc := svr_registry.NewGRPCServerRegistry()
    regHttp := svr_registry.NewHttpServerRegistry()

    defer regGrpc.Stop()
    defer regHttp.Stop()

    <-common.Sig
}
```

启动流程：
1. **common.init()** - 加载配置，初始化 Kafka 生产者
2. **httptrans.Run()** - 启动 HTTP API 服务 + RawData 服务
3. **grpctrans.Run()** - 启动 gRPC 服务
4. **svr_registry** - 向 SD 注册 gRPC/HTTP 服务

---

### 3.2 gRPC 服务 (`grpctrans/grpc_server.go`)

#### TLS 双向认证配置

```go
func credential(crtFile, keyFile, caFile string) credentials.TransportCredentials {
    return credentials.NewTLS(&tls.Config{
        Certificates: []tls.Certificate{cert},
        ClientAuth:   tls.RequireAndVerifyClientCert,  // 强制客户端证书验证
        ClientCAs:    certPool,
    })
}
```

#### gRPC 服务参数

```go
const (
    defaultMinPingTime    = 5 * time.Second   // 客户端最小 ping 间隔
    defaultMaxConnIdle    = 20 * time.Minute  // 最大空闲时间
    defaultPingTime       = 10 * time.Minute  // 服务端主动 ping 间隔
    defaultPingAckTimeout = 5 * time.Second   // ping ACK 超时
    maxMsgSize            = 10 * 1024 * 1024  // 消息最大 10MB
)
```

#### 服务注册

```go
func Run() {
    grpc_handler.InitGlobalGRPCPool()

    server := grpc.NewServer(opts...)
    pb.RegisterTransferServer(server, &grpc_handler.TransferHandler{})
    pb.RegisterFileExtServer(server, &grpc_handler.FileExtHandler{})

    server.Serve(lis)
}
```

---

### 3.3 连接池管理 (`grpctrans/pool/pool.go`)

#### GRPCPool 结构

```go
type GRPCPool struct {
    connPool       *cache.Cache       // Agent 连接缓存 Map[agentID] -> *Connection
    tokenChan      chan bool          // 连接令牌池 (限流)
    confChan       chan string        // 配置推送通道
    taskChan       chan map[string]string  // 任务上报通道
    extraInfoChan  chan string        // 扩展信息加载通道
    extraInfoCache map[string]AgentExtraInfo  // Agent 扩展信息缓存
    conf           *Config
}
```

#### Connection 结构

```go
type Connection struct {
    Ctx          context.Context      // 连接上下文
    CancelFuc    context.CancelFunc   // 取消函数
    CommandChan  chan *Command        // 命令发送通道
    AgentID      string               // Agent 唯一标识
    SourceAddr   string               // 客户端地址
    CreateAt     int64                // 创建时间
    agentDetail  map[string]interface{}           // Agent 心跳详情
    pluginDetail map[string]map[string]interface{} // 插件心跳详情
}
```

#### 令牌限流机制

```
┌─────────────────────────────────────────────────────────────┐
│                    Token 限流机制                            │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  初始化：tokenChan 预填充 PoolLength 个 token                 │
│                                                               │
│  新连接进入:                                                  │
│    LoadToken() ──► 尝试获取 token                            │
│        │                                                      │
│        ├─ 成功: 继续处理连接                                  │
│        └─ 失败: 返回 "out of max connection limit"           │
│                                                               │
│  连接关闭:                                                    │
│    ReleaseToken() ──► 归还 token                             │
│                                                               │
│  效果: 控制最大并发连接数 = PoolLength                        │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

#### 关键方法

| 方法 | 说明 |
|------|------|
| `LoadToken()` | 获取连接令牌，限制最大连接数 |
| `ReleaseToken()` | 释放连接令牌 |
| `Add(agentID, conn)` | 添加连接到池，触发 extraInfo 加载 |
| `Delete(agentID)` | 从池中删除连接 |
| `GetByID(agentID)` | 根据 AgentID 获取连接 |
| `GetList()` | 获取所有连接列表 |
| `PostCommand(agentID, cmd)` | 向指定 Agent 发送命令 |
| `PostLatestConfig(agentID)` | 触发配置推送 |
| `Close(agentID)` | 关闭指定 Agent 连接 |

---

### 3.4 数据传输核心 (`grpctrans/grpc_handler/transfer_handler.go`)

#### Transfer 主流程

```
Agent gRPC 连接
  ↓
Transfer(stream) 方法入口
  ↓
LoadToken() 检查连接数限制
  ↓ (失败返回错误)
stream.Recv() 接收第一个数据包
  ↓
提取 AgentID，创建 Connection
  ↓
GlobalGRPCPool.Add(agentID, conn)
  ↓
┌─────────────────────────────────────────┐
│  启动双向流处理                          │
│                                          │
│  go recvData(stream, conn)  ─── 接收    │
│  go sendData(stream, conn)  ─── 发送    │
│                                          │
│  <-conn.Ctx.Done() 等待任一方向结束      │
│                                          │
└─────────────────────────────────────────┘
  ↓ (连接关闭)
GlobalGRPCPool.Delete(agentID)
ReleaseToken()
```

#### recvData 接收协程

```go
func recvData(stream pb.Transfer_TransferServer, conn *pool.Connection) {
    defer conn.CancelFuc()
    for {
        select {
        case <-conn.Ctx.Done():
            return
        default:
            data, err := stream.Recv()
            if err != nil {
                return
            }
            recvCounter.Inc()  // Prometheus 计数
            handleRawData(data, conn)  // 处理数据
        }
    }
}
```

#### sendData 发送协程

```go
func sendData(stream pb.Transfer_TransferServer, conn *pool.Connection) {
    defer conn.CancelFuc()
    for {
        select {
        case <-conn.Ctx.Done():
            return
        case cmd := <-conn.CommandChan:
            if cmd == nil {  // nil 表示关闭信号
                return
            }
            err := stream.Send(cmd.Command)
            cmd.Error = err
            close(cmd.Ready)  // 通知调用方完成
        }
    }
}
```

---

### 3.5 数据处理 (`grpctrans/grpc_handler/rawdata_worker.go`)

#### handleRawData 处理流程

```
收到 RawData
  ↓
遍历 RawData.Data[] 记录列表
  ↓
封装 MQData (从对象池获取)
  │
  ├─ AgentID, Hostname, Version, Product
  ├─ IntranetIPv4/IPv6, ExtranetIPv4/IPv6
  ├─ DataType, Body, AgentTime, SvrTime
  └─ Tag (从 extraInfoCache 获取)
  ↓
按 DataType 分类处理:
  │
  ├─ 1000: Agent 心跳
  │   └─ parseAgentHeartBeat() → conn.SetAgentDetail()
  │   └─ 首次连接触发 PostLatestConfig()
  │
  ├─ 1001: 插件心跳
  │   └─ parsePluginHeartBeat() → conn.SetPluginDetail()
  │
  ├─ 2001/2003/5100/5101/6000/8010: 任务数据
  │   └─ PushTask2Manager() → 批量上报 Manager
  │
  └─ 1010/1011: Agent/插件错误日志
      └─ 写入日志文件
  ↓
KafkaProducer.SendPBWithKey(agentID, mqMsg)
  ↓
归还 MQData 到对象池
```

#### 数据类型定义

| DataType | 说明 | 处理方式 |
|----------|------|----------|
| 1000 | Agent 心跳 | 解析存储到 Connection |
| 1001 | 插件心跳 | 解析存储到 Connection |
| 1010 | Agent 错误日志 | 写日志 |
| 1011 | 插件错误日志 | 写日志 |
| 2001 | 告警任务 | 推送 Manager |
| 2003 | 告警任务 | 推送 Manager |
| 5100 | 资产扫描任务 | 推送 Manager |
| 5101 | 组件版本验证 | 推送 Manager |
| 6000 | 基线扫描 | 推送 Manager |
| 8010 | 基线扫描任务 | 推送 Manager |

---

### 3.6 Kafka 生产者 (`common/kafka/producer.go`)

#### Producer 配置

```go
config := sarama.NewConfig()
config.Producer.Return.Successes = true
config.Producer.MaxMessageBytes = 4 * 1024 * 1024  // 4MB
config.Producer.Timeout = 6 * time.Second
config.Producer.Flush.Bytes = 4 * 1024 * 1024
config.Producer.Flush.Frequency = 10 * time.Second
```

#### 异步发送流程

```
SendPBWithKey(agentID, mqMsg)
  ↓
PBSerialize(msg) ──► Protobuf 序列化
  ↓
从 mqProducerMessagePool 获取 ProducerMessage
  ↓
设置 Topic, Key(agentID), Value(序列化数据)
  ↓
producer.Input() <- proMsg  (异步发送)
  ↓
归还 mqMsg 到 MQMsgPool
```

#### 对象池优化

```go
var MQMsgPool = &sync.Pool{
    New: func() interface{} {
        return &pb.MQData{}
    },
}

var mqProducerMessagePool = &sync.Pool{
    New: func() interface{} {
        return &sarama.ProducerMessage{}
    },
}
```

---

### 3.7 文件上传 (`grpctrans/grpc_handler/file_handler.go`)

```
Agent Upload 流
  ↓
FileExtHandler.Upload(stream)
  ↓
循环 stream.Recv() 接收文件块
  │
  ├─ 首块: 创建文件 (路径: FileBaseDir/token)
  │
  └─ 后续: 追加写入
  ↓
写入完成 → Flush
  ↓
handlerFile(token, filePath):
  ├─ zipFile() 压缩
  ├─ fileMD5() 计算 MD5
  ├─ client.UploadFile() 上传到 Manager
  └─ PushTask2Manager() 通知结果
  ↓
删除临时文件
```

---

### 3.8 HTTP 服务 (`httptrans/scsvr.go`)

#### 双 HTTP 服务

```go
func Run() {
    go runAPIServer(port, enableSSL, enableAuth, certFile, keyFile)  // API 服务
    runRawDataServer(port, caFile, certFile, keyFile)                // RawData 服务 (mTLS)
}
```

#### API 路由

| 路径 | 方法 | 说明 |
|------|------|------|
| `/metrics` | GET | Prometheus 指标 |
| `/ping` | GET | 健康检查 |
| `/conn/stat` | GET | 连接详情 (Agent + 插件心跳) |
| `/conn/list` | GET | AgentID 列表 |
| `/conn/count` | GET | 连接总数 |
| `/conn/reset` | POST | 断开 Agent 连接 |
| `/command/` | POST | 下发命令给 Agent |
| `/kube/cluster/list` | GET | K8s 集群列表 |
| `/rawdata/audit` | POST | K8s 审计日志 (mTLS) |

---

### 3.9 命令下发 (`httptrans/http_handler/command.go`)

#### PostCommand 流程

```
Manager POST /command/
  ↓
解析 CommandRequest JSON
  │
  ├─ agent_id: 目标 Agent
  │
  └─ command:
      ├─ agent_ctrl: Agent 控制指令
      ├─ task: 任务 (DataType, Name, Data, Token)
      └─ config: 配置列表
  ↓
构建 pb.Command 对象
  ↓
GlobalGRPCPool.PostCommand(agentID, mgCommand)
  │
  ├─ 获取 Connection
  ├─ 构建 Command{Command, Ready chan}
  ├─ 发送到 conn.CommandChan
  └─ 等待 Ready 信号或超时
  ↓
返回结果
```

---

### 3.10 服务注册 (`svr_registry/svr_registry.go`)

#### 注册流程

```
NewGRPCServerRegistry() / NewHttpServerRegistry()
  ↓
POST http://{sd}/registry/register
  │
  ├─ name: "ac_grpc" / "ac_http"
  ├─ ip: 本机 IP
  ├─ port: 服务端口
  └─ weight: 当前连接数
  ↓
启动 renewRegistry() 协程
  │
  └─ 每 30 秒:
      ├─ 更新 Weight = GlobalGRPCPool.GetCount()
      └─ POST /registry/register (刷新心跳)
```

---

### 3.11 Manager 客户端 (`httptrans/client/`)

#### 配置获取

```
GetConfigFromRemote(agentID, detail)
  ↓
POST http://{manager}/api/v6/component/GetComponentInstances
  ↓
返回 Agent 应安装的组件配置
```

#### 任务上报

```
PostTask(taskList)
  ↓
POST http://{manager}/api/v1/agent/updateSubTask
  ↓
批量上报任务执行结果
```

#### 扩展信息获取

```
GetExtraInfoFromRemote(idList)
  ↓
POST http://{manager}/api/v1/agent/queryInfo
  ↓
返回 Agent 标签信息
```

---

## 4. 数据流向图

```
                     ┌─────────────────────────────────────┐
                     │            Manager                   │
                     │  ┌─────────────────────────────────┐ │
                     │  │  /api/v6/component/...          │ │
                     │  │  /api/v1/agent/updateSubTask    │ │
                     │  │  /api/v1/agent/queryInfo        │ │
                     │  └───────────────┬─────────────────┘ │
                     └───────────────────│─────────────────┘
                                         │ HTTP
                     ┌───────────────────▼─────────────────┐
                     │            AgentCenter               │
                     │  ┌─────────────────────────────────┐ │
                     │  │         GRPCPool                │ │
                     │  │    Map[AgentID]*Connection      │ │
                     │  └───────────────┬─────────────────┘ │
                     │          ┌───────┴───────┐           │
                     │          │               │           │
                     │  ┌───────▼───────┐ ┌─────▼─────────┐ │
                     │  │ recvData      │ │   sendData    │ │
                     │  │ (接收数据)    │ │   (下发命令)  │ │
                     │  └───────┬───────┘ └───────────────┘ │
                     │          │                           │
                     │  ┌───────▼───────┐                   │
                     │  │ handleRawData │                   │
                     │  │ (数据处理)    │                   │
                     │  └───────┬───────┘                   │
                     │          │                           │
                     │  ┌───────▼───────┐                   │
                     │  │ KafkaProducer │                   │
                     │  │ (写入Kafka)   │                   │
                     │  └───────────────┘                   │
                     └───────────────────│─────────────────┘
                                         │ gRPC (mTLS)
              ┌──────────────────────────┼──────────────────────────┐
              ▼                          ▼                          ▼
     ┌────────────────┐        ┌────────────────┐        ┌────────────────┐
     │    Agent-1     │        │    Agent-2     │        │    Agent-N     │
     │    (Linux)     │        │    (Linux)     │        │    (Linux)     │
     └────────────────┘        └────────────────┘        └────────────────┘
```

---

## 5. 关键时间参数

| 参数 | 值 | 说明 |
|------|-----|------|
| 服务注册心跳 | 30s | 向 SD 更新注册信息 |
| gRPC 客户端最小 ping | 5s | 防止客户端频繁 ping |
| gRPC 最大空闲时间 | 20min | 空闲连接自动关闭 |
| gRPC 服务端 ping | 10min | 服务端主动检测 |
| gRPC ping ACK 超时 | 5s | ping 响应超时 |
| gRPC 消息最大尺寸 | 10MB | 单条消息限制 |
| Kafka 消息最大 | 4MB | Kafka 消息限制 |
| Kafka 刷新间隔 | 10s | 批量刷新周期 |
| Kafka 发送超时 | 6s | 单条发送超时 |
| 命令发送超时 | 配置 | CommSendTimeOut |
| 命令结果超时 | 配置 | CommResultTimeOut |
| 任务上报超时 | 2s | PostTask HTTP 超时 |

---

## 6. 高可用设计

### 6.1 连接限流
- Token 令牌池控制最大连接数
- 超出限制返回错误，触发 Agent 重连其他 AC

### 6.2 双向流容错
- recvData 和 sendData 独立协程
- 任一方向错误触发连接关闭
- 自动清理连接池和释放令牌

### 6.3 异步处理
- Kafka 异步生产者
- 任务批量上报 (按数量+时间)
- extraInfo 异步加载

### 6.4 对象池复用
- MQMsgPool 复用 MQData 对象
- mqProducerMessagePool 复用 ProducerMessage
- 减少 GC 压力

### 6.5 服务注册
- 自动向 SD 注册
- 定时更新连接数权重
- 优雅关闭时注销服务

---

## 7. 外部依赖关系

```
AgentCenter 模块依赖：
├── ServiceDiscovery  - 服务注册发现
├── Manager           - 配置获取、任务上报
├── Kafka             - 数据写入
├── gRPC              - Agent 通信
├── gin               - HTTP 框架
├── grequests         - HTTP 客户端
├── sarama            - Kafka 客户端
├── prometheus        - 指标收集
└── go-cache          - 连接缓存

被依赖：
├── Agent             - gRPC 连接
├── Manager           - 命令下发
└── K8s Cluster       - 审计日志上报
```

---

## 8. Prometheus 指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `elkeid_ac_grpc_conn_count` | Gauge | - | 当前 gRPC 连接数 |
| `elkeid_ac_grpc_recv_qps` | Counter | - | gRPC 接收 QPS |
| `elkeid_ac_grpc_send_qps` | Counter | - | gRPC 发送 QPS |
| `elkeid_ac_output_data_type_count` | Counter | data_type | 各 DataType 输出计数 |
| `elkeid_ac_output_count` | Counter | agent_id | 各 Agent 输出计数 |
| `elkeid_ac_agent_cpu` | Gauge | agent_id, name | Agent/插件 CPU 使用率 |
| `elkeid_ac_agent_rss` | Gauge | agent_id, name | Agent/插件内存使用 |
| `elkeid_ac_agent_du` | Gauge | agent_id, name | Agent/插件磁盘使用 |
| `elkeid_ac_agent_read_speed` | Gauge | agent_id, name | 磁盘读速度 |
| `elkeid_ac_agent_write_speed` | Gauge | agent_id, name | 磁盘写速度 |
| `elkeid_ac_agent_tx_speed` | Gauge | agent_id, name | 网络发送速度 |
| `elkeid_ac_agent_rx_speed` | Gauge | agent_id, name | 网络接收速度 |

---

## 9. 安全机制

### 9.1 gRPC mTLS
- 双向 TLS 认证
- 客户端必须提供有效证书
- CA 证书验证

### 9.2 HTTP AK/SK
- API 接口支持 AK/SK 认证
- HMAC-SHA256 签名
- 时间戳防重放

### 9.3 RawData mTLS
- K8s 审计日志接口
- 强制客户端证书验证

---

## 总结

AgentCenter 模块用约 **8,188 行代码**实现了：

1. **gRPC 双向流** - 与 Agent 建立长连接，支持数据收发
2. **连接池管理** - Token 限流，Agent 状态追踪
3. **数据处理** - 心跳解析、任务路由、Kafka 写入
4. **命令下发** - 接收 Manager 命令，转发给目标 Agent
5. **文件上传** - 流式接收、压缩、转存
6. **服务注册** - 自动注册到 SD，权重动态更新
7. **高性能** - 异步 Kafka、对象池复用、批量上报
8. **可观测性** - 丰富的 Prometheus 指标
9. **安全认证** - mTLS + AK/SK 双重保护
