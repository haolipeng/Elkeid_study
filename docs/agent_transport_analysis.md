# Elkeid Agent Transport 模块代码逻辑分析

## 1. 目录结构

```
/home/work/openSource/Elkeid/agent/transport/
├── transport.go                 # 主启动入口 (24行)
├── transfer.go                  # 数据传输核心 (245行)
├── file_ext.go                  # 文件上传处理 (133行)
├── connection/                  # 连接管理子模块
│   ├── connection.go            # gRPC连接管理 (185行)
│   ├── product.go               # 产品配置初始化 (34行)
│   ├── stats_handler.go         # 性能统计收集 (54行)
│   ├── ca.crt                   # CA证书
│   ├── client.crt               # 客户端证书
│   └── client.key               # 客户端密钥
└── compressor/                  # 压缩子模块
    └── snappy.go                # Snappy压缩实现 (70行)
```

**总代码量：约 745 行**

---

## 2. 模块概述

Transport 模块是 Elkeid Agent 的**通信核心**，负责：
- 与服务器建立安全的 gRPC 双向流连接
- 将 Agent 和插件采集的数据发送到服务器
- 接收服务器下发的命令并分发处理
- 支持文件上传功能

---

## 3. 核心组件详解

### 3.1 启动入口 (`transport.go`)

```go
func Startup(ctx context.Context, wg *sync.WaitGroup) {
    go startFileExt(ctx, wg)   // 启动文件上传处理
    go startTransfer(ctx, wg)  // 启动数据收发处理
}
```

启动两个并发 goroutine：
1. **startFileExt** - 处理文件上传请求
2. **startTransfer** - 处理数据收发

---

### 3.2 数据传输核心 (`transfer.go`)

#### 主循环 `startTransfer`

```
┌─────────────────────────────────────────────────┐
│              startTransfer 主循环                │
├─────────────────────────────────────────────────┤
│  1. 获取 gRPC 连接 (最多重试6次)                  │
│  2. 创建 Transfer 双向流                         │
│  3. 启动 handleSend (发送协程)                   │
│  4. 启动 handleReceive (接收协程)                │
│  5. 等待任一协程退出后重连                        │
└─────────────────────────────────────────────────┘
```

#### 发送流程 `handleSend`

```
每 100ms 执行一次：
  ↓
buffer.ReadEncodedRecords() 从环形缓冲区读取数据
  ↓
打包成 PackagedData：
  - records: 编码记录列表
  - agent_id: Agent 唯一标识
  - intranet_ipv4/ipv6: 内网IP
  - extranet_ipv4/ipv6: 外网IP
  - hostname: 主机名
  - version: Agent版本
  - product: 产品名
  ↓
client.Send() 通过 gRPC 发送 (Snappy压缩)
  ↓
回收 EncodedRecord 对象到对象池
```

#### 接收流程 `handleReceive`

```
阻塞等待 client.Recv()
  ↓
解析 Command 命令
  ├─ Task (任务类型)
  │   ├─ ObjectName == agent.Product (给Agent的任务)
  │   │   ├─ DataType=1050: 文件上传请求 → UploadFile()
  │   │   ├─ DataType=1051: 元数据设置 (IDC/Region)
  │   │   └─ DataType=1060: 关闭Agent → agent.Cancel()
  │   │
  │   └─ ObjectName != agent.Product (给插件的任务)
  │       └─ plugin.SendTask() 转发给对应插件
  │
  └─ Config (配置更新)
      ├─ Agent版本比对 → 触发升级
      └─ 插件配置同步 → plugin.Sync()
```

---

### 3.3 文件上传模块 (`file_ext.go`)

```
UploadFile(req) 入口
  ↓ (非阻塞写入 uploadCh)
startFileExt() 监听 uploadCh
  ↓
handleUpload() 处理上传：
  1. 打开文件
  2. 校验大小 (不超过 BufSize * 600s)
  3. 获取 gRPC 连接
  4. 创建 Upload 流 (超时600s)
  5. 分块读取发送 (每块最大500KB)
  6. CloseAndRecv() 完成上传
```

**特点**：
- 同一时刻只处理一个上传任务
- 流式传输，支持大文件

---

### 3.4 连接管理 (`connection/connection.go`)

#### 连接优先级

```
GetConnection() 按优先级尝试连接：

优先级1: Service Discovery (服务发现)
  └─ HTTP请求: http://{sd_host}:8088/registry/detail
  └─ 获取多个服务地址，逐一尝试
  └─ NetMode = "sd"

优先级2: Private Network (内网直连)
  └─ 直接连接 {private_host}:{port}
  └─ NetMode = "private"

优先级3: Public Network (公网)
  └─ 直接连接 {public_host}:{port}
  └─ NetMode = "public"
```

#### TLS 安全认证

```go
// 双向 TLS 认证配置
- 客户端证书: client.crt + client.key
- CA证书: ca.crt
- 服务器名验证: elkeid.com
```

#### 全局状态变量

```go
var (
    IDC     atomic.Value  // 数据中心标识
    Region  atomic.Value  // 区域信息
    NetMode atomic.Value  // 网络模式 ("sd"/"private"/"public")
)
```

---

### 3.5 性能统计 (`connection/stats_handler.go`)

实现 `grpc.StatsHandler` 接口，追踪：
- **rxBytes**: 接收字节数
- **txBytes**: 发送字节数
- **RxSpeed/TxSpeed**: 收发速度 (字节/秒)

供心跳模块获取网络性能指标。

---

### 3.6 Snappy 压缩 (`compressor/snappy.go`)

- 所有 gRPC 通信使用 Snappy 压缩
- 使用 `sync.Pool` 对象池减少 GC 开销
- 自动注册到 gRPC 编码系统

---

## 4. 数据流向图

```
                    ┌──────────────────────────────────────┐
                    │           Elkeid Server              │
                    └──────────────┬───────────────────────┘
                                   │ gRPC (TLS + Snappy)
                                   ▼
┌──────────────────────────────────────────────────────────────┐
│                     Transport Module                          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────┐   │
│  │ connection  │◄───│  transfer   │───►│    file_ext     │   │
│  │   管理连接   │    │   收发数据   │    │    文件上传      │   │
│  └─────────────┘    └──────┬──────┘    └─────────────────┘   │
└──────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        ┌──────────┐    ┌──────────┐    ┌──────────┐
        │  buffer  │    │  plugin  │    │  agent   │
        │ 环形缓冲区 │    │  插件管理 │    │ Agent核心 │
        └──────────┘    └──────────┘    └──────────┘
```

---

## 5. 关键时间参数

| 参数 | 值 | 说明 |
|------|-----|------|
| 发送间隔 | 100ms | handleSend 定时器周期 |
| 连接超时 | 15s | gRPC DialContext 超时 |
| 文件上传超时 | 600s | 单个文件上传操作超时 |
| 重连等待 | 5s | 连接失败后等待间隔 |
| 最大重试次数 | 6次 | 超过后模块自动关闭 |

---

## 6. 高可用设计

### 6.1 连接容错
- 支持3种连接模式自动降级
- 最多6次重试，每次间隔5秒
- 任一方向错误触发重连

### 6.2 性能优化
- 批量发送 (100ms 聚合)
- Snappy 压缩传输
- 对象池复用减少 GC

### 6.3 安全机制
- 双向 TLS 认证
- 证书编译时嵌入
- Region 字段校验

---

## 7. 外部依赖关系

```
transport 模块依赖：
├── buffer      - 环形缓冲区读写
├── agent       - Agent ID/Version/Product, Cancel/Update
├── plugin      - 插件管理 (Get/SendTask/Sync)
├── host        - 主机信息 (IP/Hostname)
├── proto       - gRPC protobuf 定义
└── log         - 日志记录

被依赖：
├── main        - 调用 Startup() 启动
└── heartbeat   - 调用 GetState() 获取性能指标
```

---

## 总结

Transport 模块是 Elkeid Agent 的通信中枢，仅用约 **745 行代码**实现了：

1. **双向 gRPC 通信** - 发送采集数据 + 接收控制命令
2. **灵活连接策略** - 服务发现 → 内网 → 公网自动降级
3. **可靠传输** - 自动重连、错误恢复、自保护机制
4. **高效传输** - 批量发送、压缩传输、对象池复用
5. **安全认证** - 双向 TLS + 证书嵌入
6. **扩展功能** - 文件上传、插件分发、版本升级
