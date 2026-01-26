# Elkeid Server-Agent 命令下发机制

本文档详细描述 Elkeid Server 给 Agent 下发配置命令的机制。

## 1. 概述

### 1.1 通信架构

Elkeid 采用双向 gRPC 流实现 Server 与 Agent 之间的通信：

```
┌─────────────┐         gRPC (TLS + Snappy)        ┌─────────────┐
│   Server    │ ←──────────────────────────────────→│    Agent    │
│ AgentCenter │                                     │             │
└─────────────┘                                     └─────────────┘
      ↑                                                    │
      │ HTTP API                                           │
      ↓                                                    ↓
┌─────────────┐                                     ┌─────────────┐
│   Manager   │                                     │   Plugins   │
└─────────────┘                                     └─────────────┘
```

- Agent 主动连接 Server，建立长连接
- 使用 `Transfer RPC` 双向流进行数据传输
- 支持 TLS 加密和 Snappy 压缩

### 1.2 命令类型总览

Server 可以向 Agent 下发三类命令：

| 命令类型 | 用途 | 说明 |
|---------|------|------|
| **Config** | 配置下发 | 插件管理、Agent 升级 |
| **Task** | 任务下发 | 向 Agent 或插件发送任务 |
| **AgentCtrl** | 控制命令 | 直接控制 Agent 行为 |

---

## 2. 通信协议定义

### 2.1 Server 端 Protobuf 定义

**文件位置**: `server/agent_center/grpctrans/proto/grpc.proto`

```protobuf
// Server -> Agent 的命令消息
message Command {
  int32 AgentCtrl = 1;           // Agent 控制命令
  PluginTask Task = 2;           // Agent/插件任务
  repeated ConfigItem Config = 3; // 插件/Agent 配置
}

// 插件任务信息
message PluginTask {
  int32 DataType = 1;    // 数据类型标识
  string Name = 2;       // 插件名称或 Agent 产品名
  string Data = 3;       // 任务数据 (JSON 格式)
  string Token = 4;      // 用于对账的 token
}

// 配置项 (插件或 Agent 升级)
message ConfigItem {
  string Name = 1;              // 配置名 (插件名或 agent)
  string Type = 2;              // 配置类型
  string Version = 3;           // 版本号
  string SHA256 = 4;            // SHA256 校验值
  string Signature = 5;         // 数字签名
  repeated string DownloadURL = 6; // 下载地址列表
  string Detail = 7;            // 配置详情/启动参数
}

// gRPC Transfer 服务
service Transfer {
  rpc Transfer (stream RawData) returns (stream Command){}
}
```

### 2.2 Agent 端 Protobuf 定义

**文件位置**: `agent/proto/grpc.proto`

```protobuf
message Command {
  Task task = 2;
  repeated Config configs = 3;
}

message Task {
  int32 data_type = 1;
  string object_name = 2;  // 目标对象名 (agent product 或 plugin name)
  string data = 3;
  string token = 4;
}

message Config {
  string name = 1;
  string type = 2;
  string version = 3;
  string sha256 = 4;
  string signature = 5;
  repeated string download_urls = 6;
  string detail = 7;
}
```

---

## 3. 配置命令 (Config)

配置命令用于管理插件和升级 Agent。

### 3.1 插件管理机制

通过 Config 列表的增删来实现插件的开启/关闭：

| 操作 | 实现方式 |
|------|---------|
| **开启插件** | 在 Config 列表中添加插件配置，Agent 自动下载并启动 |
| **关闭插件** | 从 Config 列表中移除该插件，Agent 自动停止并删除 |
| **更新插件** | 发送新版本配置，Agent 下载新版本并重启插件 |
| **升级 Agent** | 发送新版本 Agent 配置，Agent 自动更新并重启 |

### 3.2 Config配置字段详解

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | 插件名称或 "agent" |
| `Type` | string | 插件类型标识 |
| `Version` | string | 版本号 |
| `SHA256` | string | 文件校验值 |
| `Signature` | string | 数字签名，用于验证完整性 |
| `DownloadURL` | []string | 下载地址列表 (支持多地址容错) |
| `Detail` | string | 启动参数 (JSON 格式) |

### 3.3 Detail 参数示例

```json
{
  "enable_mode": "kernel",
  "log_level": "info",
  "buffer_size": 4096,
  "custom_param": "value"
}
```

### 3.4 HTTP API 示例

```bash
# 下发插件配置
POST /api/command
{
  "agent_id": "agent_123",
  "command": {
    "config": [
      {
        "name": "collector",
        "type": "plugin",
        "version": "1.0.0",
        "sha256": "abc123...",
        "signature": "sig...",
        "download_url": ["http://server/plugins/collector-1.0.0"],
        "detail": "{\"log_level\": \"info\"}"
      }
    ]
  }
}
```

---

## 4. 任务命令 (Task)

任务命令用于向 Agent 或特定插件发送指令。

### 4.1 Agent 任务类型

| DataType | 说明 | 处理方式 |
|----------|------|---------|
| **1050** | 文件上传请求 | JSON 解析后调用 `UploadFile()` |
| **1051** | 元数据设置 (IDC/Region) | 执行 `agent-control set` 命令 |
| **1060** | 关闭 Agent | 调用 `agent.Cancel()` |
| **其他** | 插件任务 | 转发给对应插件处理 |

### 4.2 Task 数据结构

```go
type TaskMsg struct {
    DataType int32  `json:"data_type,omitempty"`  // 任务类型
    Name     string `json:"name,omitempty"`       // 目标名 (插件名或 "agent")
    Data     string `json:"data,omitempty"`       // 任务数据 (JSON)
    Token    string `json:"token,omitempty"`      // 对账 token
}
```

### 4.3 HTTP API 示例

**向 Agent 发送任务**:
```bash
POST /api/command
{
  "agent_id": "agent_123",
  "command": {
    "task": {
      "data_type": 1050,
      "name": "agent",
      "data": "{\"path\": \"/var/log/app.log\", \"buf_size\": 1048576}",
      "token": "token_abc123"
    }
  }
}
```

**向插件发送任务**:
```bash
POST /api/command
{
  "agent_id": "agent_123",
  "command": {
    "task": {
      "data_type": 2001,
      "name": "scanner",
      "data": "{\"action\": \"scan\", \"target\": \"/etc\"}",
      "token": "token_abc123"
    }
  }
}
```

---

## 5. Agent 命令处理流程

### 5.1 命令接收入口

**文件位置**: `agent/transport/transfer.go:128-244`

```go
func handleReceive(ctx context.Context, wg *sync.WaitGroup, client proto.Transfer_TransferClient) {
    for {
        cmd, err := client.Recv()
        if err != nil {
            return
        }

        // 处理 Task 类型命令
        if cmd.Task != nil {
            if cmd.Task.ObjectName == agent.Product {
                // Agent 任务
                switch cmd.Task.DataType {
                case 1050:  // 文件上传
                case 1051:  // 元数据设置
                case 1060:  // 关闭 Agent
                }
            } else {
                // 插件任务 - 转发给对应插件
                plg, ok := plugin.Get(cmd.Task.ObjectName)
                if ok {
                    plg.SendTask(*cmd.Task)
                }
            }
            continue
        }

        // 处理 Config 类型配置
        cfgs := map[string]*proto.Config{}
        for _, config := range cmd.Configs {
            cfgs[config.Name] = config
        }

        // Agent 升级检查
        if cfg, ok := cfgs[agent.Product]; ok && cfg.Version != agent.Version {
            agent.Update(*cfg)
        }

        // 插件同步
        plugin.Sync(cfgs)
    }
}
```

### 5.2 插件同步流程

**文件位置**: `agent/plugin/plugin.go:163-230`

```go
func Sync(cfgs map[string]*proto.Config) (err error) {
    select {
    case syncCh <- cfgs:
    default:
        err = errors.New("plugins are syncing")
    }
}

// 在 Startup() goroutine 中处理
func Startup(ctx context.Context, wg *sync.WaitGroup) {
    for {
        select {
        case cfgs := <-syncCh:
            // 1. 加载新插件或更新现有插件
            for _, cfg := range cfgs {
                if cfg.Name != agent.Product {
                    plg, err := Load(ctx, *cfg)
                    // ...
                }
            }

            // 2. 移除不在配置中的插件
            for _, plg := range GetAll() {
                if _, ok := cfgs[plg.Config.Name]; !ok {
                    plg.Shutdown()
                    m.Delete(plg.Config.Name)
                }
            }
        }
    }
}
```

### 5.3 插件加载流程

```
Load() 入口
  ↓
验证插件名称合法性
  ↓
检查是否已加载相同版本 → 若是，跳过
  ↓
验证本地签名 → 若不符，从服务器下载
  ↓
启动插件进程 (设置父进程组)
  ↓
建立通信管道 (stdin/stdout/stderr)
  ↓
启动 3 个 goroutine:
  - 监听插件进程退出
  - 接收插件数据
  - 转发任务给插件
```

---

## 6. Server 端发送命令流程

### 6.1 HTTP API 入口

**文件位置**: `server/agent_center/httptrans/http_handler/command.go:38-85`

```go
// POST /api/command
func PostCommand(c *gin.Context) {
    var taskModel CommandRequest
    c.BindJSON(&taskModel)

    // 构建 gRPC Command
    mgCommand := &pb.Command{
        AgentCtrl: taskModel.Command.AgentCtrl,
    }

    // 转换配置和任务...

    // 发送给 Agent
    grpc_handler.GlobalGRPCPool.PostCommand(taskModel.AgentID, mgCommand)
}
```

### 6.2 gRPC 连接池管理

**文件位置**: `server/agent_center/grpctrans/pool/pool.go:99-286`

```go
type Connection struct {
    CommandChan chan *Command  // 命令发送 channel
    AgentID     string
    SourceAddr  string
    CreateAt    int64
}

func (g *GRPCPool) PostCommand(agentID string, command *pb.Command) error {
    conn, err := g.GetByID(agentID)
    if err != nil {
        return err
    }

    cmdToSend := &Command{
        Command: command,
        Ready:   make(chan bool, 1),
    }

    // 发送到命令 channel
    select {
    case conn.CommandChan <- cmdToSend:
    case <-time.After(g.conf.CommSendTimeOut):
        return errors.New("send timeout")
    }

    // 等待发送结果
    select {
    case <-cmdToSend.Ready:
        return cmdToSend.Error
    case <-time.After(g.conf.CommResultTimeOut):
        return errors.New("result timeout")
    }
}
```

### 6.3 配置下发时机

- **Agent 首次连接**: 自动触发配置同步
- **手动下发**: 通过 Manager 调用 HTTP API
- **配置变更**: 调用 `PostLatestConfig()` 推送更新

---

## 7. 完整流程图

```
┌─────────────────────────────────────────────────────────────┐
│                    Manager 服务                              │
│  (创建任务、编辑 Agent 配置)                                  │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
         ┌──────────────────────────┐
         │  AgentCenter HTTP API    │
         │  POST /api/command       │
         └────────────┬─────────────┘
                      │
                      ▼
        ┌─────────────────────────────┐
        │  GRPCPool.PostCommand()     │
        │  将命令放入 CommandChan      │
        └────────────┬────────────────┘
                     │
                     ▼
        ┌─────────────────────────────────────┐
        │   Transfer Handler (gRPC)           │
        │   sendData() 发送给 Agent           │
        └────────────┬────────────────────────┘
                     │
          gRPC (TLS + Snappy 压缩)
                     │
                     ▼
        ┌─────────────────────────────┐
        │   Agent Transport Module    │
        │   handleReceive()           │
        └────────────┬────────────────┘
                     │
         ┌───────────┼───────────┐
         │           │           │
         ▼           ▼           ▼
      Task       Config     AgentCtrl
       │           │            │
       ├→[1050]    │         [1060]
       ├→[1051]    │         关闭 Agent
       └→[插件]    │
         转发任务   │
                   ├──→ Agent 升级
                   │    agent.Update()
                   │
                   └──→ 插件同步
                        plugin.Sync()
                        ├─→ Load(config)
                        │   启动插件进程
                        └─→ Shutdown()
                            关闭插件
```

---

## 8. 关键代码位置索引

| 功能模块 | 文件路径 |
|---------|---------|
| Agent 命令接收 | `agent/transport/transfer.go:128-244` |
| Agent Protobuf 定义 | `agent/proto/grpc.proto` |
| Server Protobuf 定义 | `server/agent_center/grpctrans/proto/grpc.proto` |
| HTTP 命令 API | `server/agent_center/httptrans/http_handler/command.go` |
| gRPC 连接池 | `server/agent_center/grpctrans/pool/pool.go` |
| 配置获取 | `server/agent_center/httptrans/client/config.go` |
| 插件管理 | `agent/plugin/plugin.go` |
| 任务分发 | `server/manager/internal/atask/job.go` |
| Agent 配置 | `server/manager/internal/aconfig/config.go` |
| Transfer Handler | `server/agent_center/grpctrans/grpc_handler/transfer_handler.go` |
| RawData 处理 | `server/agent_center/grpctrans/grpc_handler/rawdata_worker.go` |

---

## 9. 核心要点总结

1. **双向 gRPC 流**: 采用 `Transfer RPC` 双向流，Agent 主动连接 Server 建立长连接
2. **命令分类**: 三类命令 - Task (任务)、Config (配置)、AgentCtrl (控制)
3. **插件管理**: 通过配置列表的增删来实现插件的开启/关闭，而非单独的开关命令
4. **对账机制**: 每个任务带 Token，用于 Manager 的异步对账
5. **安全认证**: 双向 TLS + 签名验证
6. **配置获取**: Agent 首次连接时触发配置同步，之后通过 `PostLatestConfig` 更新
7. **可靠性**: 命令通过 Channel 传输，有超时机制和错误回报
