# Elkeid Agent-Server 交互代码分析 (主要流程)

## 1. 整体架构

```
┌────────────────────────────────────────────────────────────────┐
│                         Agent (Go)                              │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐     │
│  │  Heartbeat   │    │   Plugins    │    │  Agent Tasks │     │
│  │   守护进程   │    │  (子进程)    │    │   (命令处理) │     │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘     │
│         │                   │                    │              │
│         └───────────────────┼────────────────────┘              │
│                             ↓                                   │
│               ┌─────────────────────────┐                      │
│               │    Buffer (缓冲区)      │                      │
│               │    2048 条记录槽位      │                      │
│               └───────────┬─────────────┘                      │
│                           ↓                                     │
│               ┌─────────────────────────┐                      │
│               │   Transport (传输层)    │                      │
│               │   gRPC 双向流通信       │                      │
│               └───────────┬─────────────┘                      │
│                           │                                     │
└───────────────────────────┼─────────────────────────────────────┘
                            │
                     gRPC Stream
                            │
┌───────────────────────────┼─────────────────────────────────────┐
│                           ↓                                     │
│               ┌─────────────────────────┐                      │
│               │    gRPC Server          │                      │
│               │    (Transfer 服务)      │                      │
│               └───────────┬─────────────┘                      │
│                           ↓                                     │
│               ┌─────────────────────────┐                      │
│               │   Connection Pool       │                      │
│               │   (连接池管理)          │                      │
│               └───────────┬─────────────┘                      │
│                           │                                     │
│         ┌─────────────────┼─────────────────┐                  │
│         ↓                 ↓                 ↓                  │
│  ┌────────────┐   ┌────────────┐   ┌────────────┐             │
│  │ HTTP API   │   │   Kafka    │   │  Manager   │             │
│  │ (命令下发) │   │ (数据分发) │   │ (业务逻辑) │             │
│  └────────────┘   └────────────┘   └────────────┘             │
│                                                                 │
│                    Server (Agent Center)                        │
└─────────────────────────────────────────────────────────────────┘
```

## 2. 核心代码文件

### Agent 端
| 文件 | 说明 |
|-----|------|
| `agent/main.go` | 入口，启动各守护进程 |
| `agent/proto/grpc.proto` | gRPC 协议定义 |
| `agent/transport/transfer.go` | 数据发送/接收核心逻辑 |
| `agent/transport/transport.go` | Transport 守护进程入口 |
| `agent/buffer/buffer.go` | 数据缓冲区 (2048 槽位) |
| `agent/heartbeat/heartbeat.go` | 心跳数据采集 |
| `agent/plugin/plugin.go` | 插件管理 |
| `agent/agent/id.go` | Agent ID 管理 |
| `agent/host/host.go` | 主机信息采集 |

### Server 端
| 文件 | 说明 |
|-----|------|
| `server/agent_center/main.go` | Agent Center 入口 |
| `server/agent_center/grpctrans/grpc_handler/transfer_handler.go` | 处理 Agent 数据流 |
| `server/agent_center/grpctrans/grpc_handler/rawdata_worker.go` | 解析原始数据 |
| `server/agent_center/grpctrans/pool/pool.go` | 连接池管理 |
| `server/agent_center/httptrans/http_handler/command.go` | 命令下发 HTTP API |
| `server/agent_center/httptrans/client/config.go` | 配置获取客户端 |

---

## 3. 主要流程

### 3.1 Agent 启动流程

**入口文件**: `agent/main.go:38-151`

```go
func main() {
    // 1. 初始化 Logger
    // ... logger 配置 ...

    // 2. 日志输出启动信息
    zap.S().Info("product:", agent.Product)
    zap.S().Info("version:", agent.Version)
    zap.S().Info("id:", agent.ID)
    zap.S().Info("hostname:", host.Name.Load())
    // ...

    // 3. 启动各守护进程
    wg := &sync.WaitGroup{}
    wg.Add(3)
    go heartbeat.Startup(agent.Context, wg)  // 心跳守护进程
    go plugin.Startup(agent.Context, wg)      // 插件守护进程
    go func() {
        transport.Startup(agent.Context, wg)  // 传输守护进程 (核心)
        agent.Cancel()
    }()

    // 4. 信号处理
    // SIGTERM: 优雅退出
    // SIGUSR1: 开启/关闭 pprof
    // SIGUSR2: 释放内存

    wg.Wait()
}
```

### 3.2 gRPC 连接建立流程

**文件**: `agent/transport/transfer.go:39-86`

```go
func startTransfer(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    retries := 0

    for {
        // 1. 获取 gRPC 连接
        conn, err := connection.GetConnection(ctx)
        if err != nil {
            if retries > 5 {
                // 启动自保护退出
                return
            }
            // 等待重试
            retries++
            continue
        }
        retries = 0

        // 2. 创建 Transfer 客户端，使用 snappy 压缩
        subCtx, cancel := context.WithCancel(ctx)
        client, err := proto.NewTransferClient(conn).Transfer(subCtx, grpc.UseCompressor("snappy"))

        if err == nil {
            // 3. 并行启动收发处理
            subWg.Add(2)
            go handleSend(subCtx, subWg, client)   // 发送协程
            go func() {
                handleReceive(subCtx, subWg, client) // 接收协程
                cancel()
            }()
            subWg.Wait()
        }

        // 4. 等待 5 秒后重连
        select {
        case <-ctx.Done():
            return
        case <-time.After(time.Second * 5):
        }
    }
}
```

### 3.3 数据上报流程 (Agent → Server)

**Agent 发送端** (`agent/transport/transfer.go:88-127`):

```go
func handleSend(ctx context.Context, wg *sync.WaitGroup, client proto.Transfer_TransferClient) {
    defer wg.Done()
    defer client.CloseSend()

    ticker := time.NewTicker(time.Millisecond * 100)  // 每 100ms
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // 1. 从缓冲区读取待发送数据
            recs := buffer.ReadEncodedRecords()
            if len(recs) != 0 {
                // 2. 构造 PackagedData 消息
                err := client.Send(&proto.PackagedData{
                    Records:      recs,
                    AgentId:      agent.ID,
                    IntranetIpv4: host.PrivateIPv4.Load().([]string),
                    IntranetIpv6: host.PrivateIPv6.Load().([]string),
                    ExtranetIpv4: host.PublicIPv4.Load().([]string),
                    ExtranetIpv6: host.PublicIPv6.Load().([]string),
                    Hostname:     host.Name.Load().(string),
                    Version:      agent.Version,
                    Product:      agent.Product,
                })
                // 3. 回收记录到对象池
                for _, rec := range recs {
                    buffer.PutEncodedRecord(rec)
                }
            }
        }
    }
}
```

**Server 接收端** (`server/agent_center/grpctrans/grpc_handler/transfer_handler.go:27-89`):

```go
func (h *TransferHandler) Transfer(stream pb.Transfer_TransferServer) error {
    // 1. 检查连接数限制
    if !GlobalGRPCPool.LoadToken() {
        return errors.New("out of max connection limit")
    }
    defer GlobalGRPCPool.ReleaseToken()

    // 2. 接收第一个数据包获取 AgentID
    data, err := stream.Recv()
    agentID := data.AgentID

    // 3. 获取客户端地址
    p, _ := peer.FromContext(stream.Context())
    addr := p.Addr.String()

    // 4. 创建连接对象并加入连接池
    ctx, cancelButton := context.WithCancel(context.Background())
    connection := pool.Connection{
        AgentID:     agentID,
        SourceAddr:  addr,
        CreateAt:    time.Now().Unix(),
        CommandChan: make(chan *pool.Command),
        Ctx:         ctx,
        CancelFuc:   cancelButton,
    }
    GlobalGRPCPool.Add(agentID, &connection)
    defer GlobalGRPCPool.Delete(agentID)

    // 5. 处理第一个数据包
    handleRawData(data, &connection)

    // 6. 启动接收和发送协程
    go recvData(stream, &connection)  // 持续接收数据
    go sendData(stream, &connection)   // 发送命令

    <-connection.Ctx.Done()
    return nil
}
```

**数据处理** (`server/agent_center/grpctrans/grpc_handler/rawdata_worker.go:19-101`):

```go
func handleRawData(req *pb.RawData, conn *pool.Connection) (agentID string) {
    // 准备公共字段
    var inIpv4 = strings.Join(req.IntranetIPv4, ",")
    var exIpv4 = strings.Join(req.ExtranetIPv4, ",")
    var SvrTime = time.Now().Unix()

    for k, v := range req.GetData() {
        // 从对象池获取消息对象
        mqMsg := kafka.MQMsgPool.Get().(*pb.MQData)
        mqMsg.DataType = v.DataType
        mqMsg.AgentTime = v.Timestamp
        mqMsg.Body = v.Body
        mqMsg.AgentID = req.AgentID
        // ... 填充其他字段 ...

        switch mqMsg.DataType {
        case 1000:  // Agent 心跳
            detail := parseAgentHeartBeat(v, req, conn)
            metricsAgentHeartBeat(req.AgentID, "agent", detail)

        case 1001:  // 插件心跳
            detail := parsePluginHeartBeat(v, req, conn)
            // ...

        case 2001, 2003, 6000, 5100, 5101, 8010:
            // 任务数据，推送到 Manager 进行对账
            item, _ := parseRecord(v)
            GlobalGRPCPool.PushTask2Manager(item)

        case 1010, 1011:
            // Agent 或插件错误日志
            item, _ := parseRecord(v)
            ylog.Infof("AgentErrorLog", "...")
        }

        // 推送到 Kafka
        common.KafkaProducer.SendPBWithKey(req.AgentID, mqMsg)
    }
    return req.AgentID
}
```

### 3.4 命令下发流程 (Server → Agent)

**HTTP API 入口** (`server/agent_center/httptrans/http_handler/command.go:38-85`):

```go
// POST /command/
func PostCommand(c *gin.Context) {
    var taskModel CommandRequest
    c.BindJSON(&taskModel)

    // 1. 构造 Command 消息
    mgCommand := &pb.Command{
        AgentCtrl: taskModel.Command.AgentCtrl,
    }

    // 2. 填充配置列表
    if taskModel.Command.Config != nil {
        mgCommand.Config = make([]*pb.ConfigItem, 0)
        for _, v := range taskModel.Command.Config {
            tmp := &pb.ConfigItem{
                Name:        v.Name,
                Version:     v.Version,
                DownloadURL: v.DownloadURL,
                SHA256:      v.SHA256,
                Detail:      v.Detail,
            }
            mgCommand.Config = append(mgCommand.Config, tmp)
        }
    }

    // 3. 填充任务
    if taskModel.Command.Task != nil {
        task := pb.PluginTask{
            Name:     taskModel.Command.Task.Name,
            DataType: taskModel.Command.Task.DataType,
            Data:     taskModel.Command.Task.Data,
            Token:    taskModel.Command.Task.Token,
        }
        mgCommand.Task = &task
    }

    // 4. 通过连接池发送到 Agent
    err = grpc_handler.GlobalGRPCPool.PostCommand(taskModel.AgentID, mgCommand)
}
```

**连接池发送** (`server/agent_center/grpctrans/pool/pool.go:262-286`):

```go
func (g *GRPCPool) PostCommand(agentID string, command *pb.Command) (err error) {
    // 1. 获取连接
    conn, err := g.GetByID(agentID)
    if err != nil {
        return err
    }

    // 2. 构造带回调的命令
    cmdToSend := &Command{
        Command: command,
        Error:   nil,
        Ready:   make(chan bool, 1),
    }

    // 3. 发送到 Channel (带超时)
    select {
    case conn.CommandChan <- cmdToSend:
    case <-time.After(g.conf.CommSendTimeOut):
        return errors.New("the sendPool of the agent is full")
    }

    // 4. 等待发送结果
    select {
    case <-cmdToSend.Ready:
        return cmdToSend.Error
    case <-time.After(g.conf.CommResultTimeOut):
        return errors.New("timeout")
    }
}
```

**gRPC 发送协程** (`server/agent_center/grpctrans/grpc_handler/transfer_handler.go:111-138`):

```go
func sendData(stream pb.Transfer_TransferServer, conn *pool.Connection) {
    defer conn.CancelFuc()

    for {
        select {
        case <-conn.Ctx.Done():
            return
        case cmd := <-conn.CommandChan:
            // nil 命令表示关闭连接
            if cmd == nil {
                return
            }
            // 发送命令给 Agent
            err := stream.Send(cmd.Command)
            if err != nil {
                cmd.Error = err
                close(cmd.Ready)
                return
            }
            cmd.Error = nil
            close(cmd.Ready)  // 通知发送成功
        }
    }
}
```

**Agent 接收端** (`agent/transport/transfer.go:129-245`):

```go
func handleReceive(ctx context.Context, wg *sync.WaitGroup, client proto.Transfer_TransferClient) {
    defer wg.Done()

    for {
        // 1. 接收命令
        cmd, err := client.Recv()
        if err != nil {
            return
        }

        // 2. 处理 Task
        if cmd.Task != nil {
            // 2.1 给 Agent 的任务
            if cmd.Task.ObjectName == agent.Product {
                switch cmd.Task.DataType {
                case 1050:  // 文件上传
                    req := UploadRequest{}
                    json.Unmarshal([]byte(cmd.Task.Data), &req)
                    UploadFile(req)

                case 1051:  // 设置元数据 (IDC/Region)
                    req := map[string]any{}
                    json.Unmarshal([]byte(cmd.Task.Data), &req)
                    if idc, ok := req["idc"]; ok {
                        connection.IDC.Store(idc)
                    }
                    if region, ok := req["region"]; ok {
                        connection.Region.Store(region)
                    }

                case 1060:  // 优雅关闭
                    agent.Cancel()
                    return
                }
            } else {
                // 2.2 给插件的任务
                plg, ok := plugin.Get(cmd.Task.ObjectName)
                if ok {
                    plg.SendTask(*cmd.Task)
                }
            }
            continue
        }

        // 3. 处理配置同步
        agent.SetRunning()
        cfgs := map[string]*proto.Config{}
        for _, config := range cmd.Configs {
            cfgs[config.Name] = config
        }

        // 3.1 升级 Agent
        if cfg, ok := cfgs[agent.Product]; ok && cfg.Version != agent.Version {
            agent.Update(*cfg)
        }
        delete(cfgs, agent.Product)

        // 3.2 同步插件
        plugin.Sync(cfgs)
    }
}
```

### 3.5 心跳流程

**Agent 心跳采集** (`agent/heartbeat/heartbeat.go:26-151`):

```go
func Startup(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()

    // 立即采集一次
    getAgentStat(time.Now())

    ticker := time.NewTicker(time.Minute)  // 每 60 秒
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case t := <-ticker.C:
            host.RefreshHost()
            getAgentStat(t)   // Agent 状态
            getPlgStat(t)     // 插件状态
        }
    }
}

func getAgentStat(now time.Time) {
    rec := &proto.Record{
        DataType:  1000,  // Agent 心跳类型
        Timestamp: now.Unix(),
        Data: &proto.Payload{
            Fields: map[string]string{},
        },
    }

    // 系统信息
    rec.Data.Fields["kernel_version"] = host.KernelVersion
    rec.Data.Fields["arch"] = host.Arch
    rec.Data.Fields["platform"] = host.Platform
    rec.Data.Fields["platform_family"] = host.PlatformFamily
    rec.Data.Fields["platform_version"] = host.PlatformVersion

    // Agent 状态
    rec.Data.Fields["state"], rec.Data.Fields["state_detail"] = agent.State()
    rec.Data.Fields["idc"] = connection.IDC.Load().(string)
    rec.Data.Fields["region"] = connection.Region.Load().(string)

    // 资源使用
    cpuPercent, rss, readSpeed, writeSpeed, fds, startAt, _ := resource.GetProcResouce(os.Getpid())
    rec.Data.Fields["cpu"] = strconv.FormatFloat(cpuPercent, 'f', 8, 64)
    rec.Data.Fields["rss"] = strconv.FormatUint(rss, 10)
    rec.Data.Fields["nfd"] = strconv.FormatInt(int64(fds), 10)

    // 传输统计
    rec.Data.Fields["rx_speed"] = ...
    rec.Data.Fields["tx_speed"] = ...
    rec.Data.Fields["ngr"] = strconv.Itoa(runtime.NumGoroutine())

    // 系统负载 (Linux)
    rec.Data.Fields["load_1"] = ...
    rec.Data.Fields["load_5"] = ...
    rec.Data.Fields["load_15"] = ...
    rec.Data.Fields["cpu_usage"] = ...
    rec.Data.Fields["mem_usage"] = ...

    // 写入缓冲区
    buffer.WriteRecord(rec)
}

func getPlgStat(now time.Time) {
    plgs := plugin.GetAll()
    for _, plg := range plgs {
        if !plg.IsExited() {
            rec := &proto.Record{
                DataType:  1001,  // 插件心跳类型
                Timestamp: now.Unix(),
                Data: &proto.Payload{
                    Fields: map[string]string{
                        "name":     plg.Name(),
                        "pversion": plg.Version(),
                    },
                },
            }
            // 采集插件资源使用
            cpuPercent, rss, _, _, fds, startAt, _ := resource.GetProcResouce(plg.Pid())
            rec.Data.Fields["cpu"] = strconv.FormatFloat(cpuPercent, 'f', 8, 64)
            rec.Data.Fields["rss"] = strconv.FormatUint(rss, 10)
            rec.Data.Fields["pid"] = strconv.Itoa(plg.Pid())
            // ...

            buffer.WriteRecord(rec)
        }
    }
}
```

### 3.6 插件通信流程

**Agent 与插件通信** (`agent/plugin/plugin.go`):

```
Agent (父进程)                    Plugin (子进程)
      │                                │
      │←───── stdout (数据上报) ───────│
      │                                │
      │────── stdin (任务下发) ───────→│
      │                                │
```

**插件数据结构**:

```go
type Plugin struct {
    Config proto.Config
    cmd    *exec.Cmd
    rx     io.ReadCloser   // 从插件读取 (stdout)
    tx     io.WriteCloser  // 写入插件 (stdin)
    reader *bufio.Reader
    taskCh chan proto.Task
    // 统计
    rxBytes uint64
    txBytes uint64
    rxCnt   uint64
    txCnt   uint64
}
```

**接收插件数据** (`agent/plugin/plugin.go:74-131`):

```go
func (p *Plugin) ReceiveData() (rec *proto.EncodedRecord, err error) {
    // 1. 读取 4 字节长度头 (Little Endian)
    var l uint32
    err = binary.Read(p.reader, binary.LittleEndian, &l)

    // 2. 跳过分隔符，读取 DataType
    p.reader.Discard(1)
    dt, _, err := readVarint(p.reader)

    // 3. 读取 Timestamp
    p.reader.Discard(1)
    ts, _, err := readVarint(p.reader)

    // 4. 读取数据体
    p.reader.Discard(1)
    _, e, err := readVarint(p.reader)

    ne := int(l) - totalHeaderSize
    rec = buffer.GetEncodedRecord(ne)
    rec.DataType = int32(dt)
    rec.Timestamp = int64(ts)
    rec.Data = make([]byte, ne)
    io.ReadFull(p.reader, rec.Data)

    // 更新统计
    atomic.AddUint64(&p.txCnt, 1)
    atomic.AddUint64(&p.txBytes, uint64(l))
    return
}
```

**发送任务给插件** (`agent/plugin/plugin.go:133-140`):

```go
func (p *Plugin) SendTask(task proto.Task) (err error) {
    select {
    case p.taskCh <- task:
    default:
        err = errors.New("plugin is processing task or context has been cancled")
    }
    return
}
```

**插件同步** (`agent/plugin/plugin.go:163-230`):

```go
func Startup(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            // 关闭所有插件
            m.Range(func(key, value any) bool {
                plg := value.(*Plugin)
                plg.Shutdown()
                return true
            })
            return

        case cfgs := <-syncCh:
            // 加载新插件
            for _, cfg := range cfgs {
                if cfg.Name != agent.Product {
                    plg, err := Load(ctx, *cfg)
                    if err == ErrDuplicatePlugin {
                        continue  // 同版本已运行
                    }
                }
            }
            // 移除不在配置中的插件
            for _, plg := range GetAll() {
                if _, ok := cfgs[plg.Config.Name]; !ok {
                    plg.Shutdown()
                    m.Delete(plg.Config.Name)
                    os.RemoveAll(plg.GetWorkingDirectory())
                }
            }
        }
    }
}
```

---

## 4. 数据结构

### 4.1 上行消息 (PackagedData)

**文件**: `agent/proto/grpc.proto:5-15`

```protobuf
message PackagedData {
  repeated EncodedRecord records = 1;  // 数据记录数组
  string agent_id = 2;                  // Agent 唯一标识
  repeated string intranet_ipv4 = 3;    // 内网 IPv4
  repeated string extranet_ipv4 = 4;    // 外网 IPv4
  repeated string intranet_ipv6 = 5;    // 内网 IPv6
  repeated string extranet_ipv6 = 6;    // 外网 IPv6
  string hostname = 7;                  // 主机名
  string version = 8;                   // Agent 版本
  string product = 9;                   // 产品名
}

message EncodedRecord {
  int32 data_type = 1;   // 数据类型
  int64 timestamp = 2;   // 时间戳
  bytes data = 3;        // 具体数据 (protobuf 序列化的 Payload)
}

message Payload {
  map<string, string> fields = 1;  // 键值对数据
}
```

### 4.2 下行消息 (Command)

**文件**: `agent/proto/grpc.proto:30-50`

```protobuf
message Command {
  Task task = 2;                   // 插件任务
  repeated Config configs = 3;     // 配置列表
}

message Task {
  int32 data_type = 1;      // 任务类型
  string object_name = 2;   // 目标对象 (agent 或插件名)
  string data = 3;          // 任务数据 (JSON)
  string token = 4;         // 任务追踪 ID
}

message Config {
  string name = 1;               // 插件名
  string type = 2;               // 类型
  string version = 3;            // 版本
  string sha256 = 4;             // 校验和
  string signature = 5;          // 签名
  repeated string download_urls = 6;  // 下载地址
  string detail = 7;             // 详细配置 JSON
}
```

### 4.3 文件上传消息

**文件**: `agent/proto/grpc.proto:56-71`

```protobuf
message FileUploadRequest {
  string token = 1;   // 上传令牌
  bytes data = 2;     // 文件数据块
}

message FileUploadResponse {
  enum StatusCode {
    SUCCESS = 0;
    FAILED = 1;
  }
  StatusCode status = 1;
}

service FileExt {
  rpc Upload(stream FileUploadRequest) returns (FileUploadResponse);
}
```

### 4.4 常用 DataType

| DataType | 说明 | 方向 |
|----------|------|------|
| 1000 | Agent 心跳 | Agent → Server |
| 1001 | 插件心跳 | Agent → Server |
| 1010 | Agent 错误日志 | Agent → Server |
| 1011 | 插件错误日志 | Agent → Server |
| 1050 | 文件上传请求 | Server → Agent |
| 1051 | 设置元数据 (IDC/Region) | Server → Agent |
| 1060 | 优雅关闭 | Server → Agent |
| 2001 | 任务结果 | Agent → Server |
| 2003 | 任务结果 | Agent → Server |
| 5100 | 主动触发资产数据扫描 | Agent → Server |
| 5101 | 组件版本验证 | Agent → Server |
| 6000 | 任务数据 | Agent → Server |
| 8010 | 基线扫描 | Agent → Server |

---

## 5. 缓冲区实现

**文件**: `agent/buffer/buffer.go`

```go
var (
    mu     = &sync.Mutex{}
    buf    = [2048]*proto.EncodedRecord{}  // 2048 槽位环形缓冲区
    offset = 0
    hook   func(any) any
)

// 写入记录
func WriteEncodedRecord(rec *proto.EncodedRecord) {
    if hook != nil {
        rec = hook(rec).(*proto.EncodedRecord)
    }
    mu.Lock()
    if offset < len(buf) {
        buf[offset] = rec
        offset++
    } else {
        // 缓冲区满，丢弃或回收
        PutEncodedRecord(rec)
    }
    mu.Unlock()
}

// 读取所有记录 (清空缓冲区)
func ReadEncodedRecords() (ret []*proto.EncodedRecord) {
    mu.Lock()
    ret = make([]*proto.EncodedRecord, offset)
    copy(ret, buf[:offset])
    offset = 0
    mu.Unlock()
    return
}
```

---

## 6. 连接池实现

**文件**: `server/agent_center/grpctrans/pool/pool.go`

```go
type GRPCPool struct {
    connPool  *cache.Cache        // AgentID -> *Connection 映射
    tokenChan chan bool           // 连接数限制令牌
    confChan  chan string         // 配置推送通道
    taskChan  chan map[string]string  // 任务推送通道
    conf      *Config
}

type Connection struct {
    Ctx         context.Context
    CancelFuc   context.CancelFunc
    CommandChan chan *Command      // 命令发送通道

    AgentID    string
    SourceAddr string
    CreateAt   int64

    agentDetail  map[string]interface{}   // Agent 心跳详情
    pluginDetail map[string]map[string]interface{}  // 插件心跳详情
}

type Command struct {
    Command *pb.Command
    Error   error
    Ready   chan bool  // 发送完成信号
}
```

**连接限制**:

```go
func (g *GRPCPool) LoadToken() bool {
    select {
    case _, ok := <-g.tokenChan:
        if ok {
            return true
        }
    default:
    }
    return false
}

func (g *GRPCPool) ReleaseToken() {
    g.tokenChan <- true
}
```

---

## 7. 时序图

### 完整通信时序

```
Agent                              Server (Agent Center)              Manager
  │                                       │                              │
  │══════ 建立 gRPC Stream ══════════════→│                              │
  │   (Transfer.Transfer, snappy 压缩)    │                              │
  │                                       │                              │
  │──── PackagedData (首次连接) ─────────→│                              │
  │     {agent_id, hostname, version,     │                              │
  │      intranet_ipv4, ...}              │                              │
  │                                       │── LoadToken() 检查连接限制    │
  │                                       │── 创建 Connection 对象        │
  │                                       │   加入 connPool               │
  │                                       │                              │
  │                                       │── handleRawData()             │
  │                                       │   解析首个心跳数据            │
  │                                       │                              │
  │                                       │── PostLatestConfig() ───────→│
  │                                       │←─── 返回配置列表 ─────────────│
  │                                       │                              │
  │←───── Command (配置同步) ─────────────│                              │
  │       {configs: [...]}                │                              │
  │                                       │                              │
  ╔══════════════════════════════════════════════════════════════════════╗
  ║ 循环: 每 100ms                                                        ║
  ╠══════════════════════════════════════════════════════════════════════╣
  │                                       │                              │
  │   buffer.ReadEncodedRecords()         │                              │
  │──── PackagedData (数据上报) ─────────→│                              │
  │     {records: [...]}                  │                              │
  │                                       │──── handleRawData()           │
  │                                       │     解析各类数据              │
  │                                       │──── SendPBWithKey() → Kafka   │
  │                                       │                              │
  ╚══════════════════════════════════════════════════════════════════════╝
  │                                       │                              │
  ╔══════════════════════════════════════════════════════════════════════╗
  ║ 循环: 每 60s (1 分钟)                                                 ║
  ╠══════════════════════════════════════════════════════════════════════╣
  │   getAgentStat() / getPlgStat()       │                              │
  │──── PackagedData (心跳) ─────────────→│                              │
  │     {records: [{type:1000, ...},      │                              │
  │                {type:1001, ...}]}     │                              │
  │                                       │──── parseAgentHeartBeat()     │
  │                                       │──── parsePluginHeartBeat()    │
  │                                       │──── SetAgentDetail()          │
  │                                       │──── metricsAgentHeartBeat()   │
  ╚══════════════════════════════════════════════════════════════════════╝
  │                                       │                              │
  │                                       │←─── POST /api/v1/command/ ───│
  │                                       │     {agent_id, command}      │
  │                                       │                              │
  │                                       │── PostCommand()               │
  │                                       │   conn.CommandChan <- cmd     │
  │                                       │                              │
  │←───── Command (任务下发) ─────────────│                              │
  │       {task: {...}}                   │                              │
  │                                       │                              │
  │   handleReceive()                     │                              │
  │   plugin.Get().SendTask()             │                              │
  │                                       │                              │
  │──── PackagedData (任务结果) ─────────→│                              │
  │     {records: [{type:2001, ...}]}     │                              │
  │                                       │──── PushTask2Manager() ──────→│
  │                                       │                              │
```

---

## 8. 关键配置参数

### Agent 端

| 参数 | 值 | 说明 |
|------|-----|------|
| 数据发送间隔 | 100ms | `time.NewTicker(time.Millisecond * 100)` |
| 心跳间隔 | 60s | `time.NewTicker(time.Minute)` |
| 缓冲区大小 | 2048 | `buf = [2048]*proto.EncodedRecord{}` |
| 重连间隔 | 5s | `time.After(time.Second * 5)` |
| 最大重试次数 | 5 | 连接失败后的最大重试次数 |
| gRPC 压缩 | snappy | `grpc.UseCompressor("snappy")` |
| GOMAXPROCS | 8 | `runtime.GOMAXPROCS(8)` |

### Server 端

| 参数 | 说明 |
|------|------|
| ConnLimit | 最大连接数限制 |
| ChanLen | 各 Channel 缓冲区长度 |
| CommSendTimeOut | 命令发送超时 |
| CommResultTimeOut | 命令结果等待超时 |
| TaskTimeWeight | 任务批量推送间隔 |
| TaskCountWeight | 任务批量推送数量阈值 |

---

## 9. 总结

**核心要点:**

1. **通信方式**: gRPC 双向流 (`Transfer.Transfer`)，使用 snappy 压缩
2. **上报频率**: 数据每 100ms 批量发送，心跳每 60s
3. **命令下发**: HTTP API → 连接池 Channel → gRPC Stream → Agent
4. **插件通信**: 父子进程，stdin/stdout 管道，Little Endian 长度头
5. **数据缓冲**: 2048 槽位的固定数组缓冲区
6. **连接管理**: Token 限制并发连接数，AgentID 唯一标识
7. **配置同步**: 首次心跳后自动推送最新配置
8. **数据分发**: 解析后推送到 Kafka，任务数据推送到 Manager 对账
