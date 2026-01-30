# journal_watcher_go 技术实现分析

## 1. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        main.go                               │
├────────────────���────────────────────────────────────────────┤
│  ┌─────────────────┐        ┌─────────────────────────┐     │
│  │ sendRecordLoop  │        │   receiveTaskLoop       │     │
│  │   (goroutine)   │        │   (main goroutine)      │     │
│  └────────┬────────┘        └───────────┬─────────────┘     │
│           │                             │                    │
│           ▼                             ▼                    │
│  ┌─────────────────┐        ┌─────────────────────────┐     │
│  │ 日志源子进程     │        │  plugins.Client         │     │
│  │ journalctl/tail │        │  ReceiveTask()          │     │
│  └─────────────────┘        └─────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

## 2. 双Goroutine并发模型

| Goroutine | 职责 | 阻塞点 |
|-----------|------|--------|
| `sendRecordLoop` | 读取日志、解析、发送Record | `reader.Scan()` 阻塞等待新日志 |
| `receiveTaskLoop` | 接收Agent下发的任务 | `client.ReceiveTask()` 阻塞等待任务 |

**协调机制：** 任一goroutine出错时，通过`killChild()`终止子进程，并使用`sync.Mutex`保护共享的`childCmd`变量。

## 3. 日志源优先级选择

```go
// main.go:89-102
if findCommand("journalctl") {           // 优先级1: systemd journal
    cmd = exec.Command("journalctl", "-f", "_COMM=sshd", "-o", "json")
} else if os.Stat("/var/log/auth.log") { // 优先级2: Debian系
    cmd = exec.Command("tail", "-F", "/var/log/auth.log")
} else if os.Stat("/var/log/secure") {   // 优先级3: RHEL系
    cmd = exec.Command("tail", "-F", "/var/log/secure")
}
```

## 4. 日志解析流水线

```
原始日志行 → parseLine() → Entry → processEntry() → Record → SendRecord()
                 │                      │
                 ▼                      ▼
         日志格式归一化          sshd_parser解析
```

**`parseLine` 双格式适配：**

| 格式 | 来源 | 时间戳处理 |
|------|------|-----------|
| JSON | journalctl -o json | 微秒→秒(截断后6位) |
| Log | auth.log/secure | syslog格式解析+补全年份 |

## 5. SSHD日志解析器 (sshd_parser.go)

**正则表达式设计：** 基于原Rust版本的PEG语法(sshd.pest)转换

```go
// Login事件正则 - 匹配SSH登录日志
^(Accepted|Failed)\s+           // 认证结果
([a-zA-Z0-9\-_.@]+)\s+          // 认证方法(publickey/password等)
for\s+(invalid user\s+)?        // 可选的"invalid user"标记
([a-zA-Z0-9\-_.@]*)\s+          // 用户名
from\s+([a-zA-Z0-9\-_.@]+)\s+   // 源IP
port\s+([a-zA-Z0-9\-_.@]+)\s+   // 源端口
ssh2:?\s*(.*)$                  // 额外信息(如密钥指纹)
```

**支持的事件类型：**

| DataType | 事件 | 解析函数 | 输出字段 |
|----------|------|----------|----------|
| 4000 | SSH登录 | `ParseLogin()` | status, types, invalid, user, sip, sport, extra |
| 4001 | Kerberos认证 | `ParseCertify()` | authorized, principal |

## 6. 与Elkeid Agent通信

```go
// 使用protobuf序列化的Record结构
type Record struct {
    DataType  int32    // 事件类型标识
    Timestamp int64    // Unix时间戳
    Data      *Payload // 字段键值对
}
```

**通信通道：** 通过文件描述符3(读)/4(写)与Agent通信，这是Elkeid插件的标准IPC机制。

## 7. 容错与重连机制

```go
// main.go:145-161 - 子进程退出后自动重启
childCmd.Process.Kill()
childCmd.Wait()
time.Sleep(10 * time.Second)  // 10秒后重试
// 外层for循环会重新启动日志监控
```

## 8. 关键设计决策

| 决策 | 原因 |
|------|------|
| 使用正则替代PEG解析器 | Go没有pest等价库，正则足够处理固定格式 |
| `tail -F` 而非 `tail -f` | `-F`支持文件轮转(logrotate)时自动重新打开 |
| 全局变量 + Mutex | 简化goroutine间状态共享 |
| 时间戳字符串截断而非数学除法 | 避免整数溢出，与原Rust实现保持一致 |

## 9. 文件结构

```
journal_watcher_go/
├── go.mod              # Go模块定义
├── go.sum              # 依赖校验
├── main.go             # 主程序入口和业务逻辑
├── sshd_parser.go      # SSHD日志解析器
├── sshd_parser_test.go # 解析器单元测试
└── docs/
    └── 技术实现分析.md  # 本文档
```

## 10. 依赖关系

| 依赖 | 用途 |
|------|------|
| `github.com/bytedance/plugins` | Elkeid插件SDK，提供与Agent的IPC通信 |
| `go.uber.org/zap` | 高性能结构化日志库 |
| `gopkg.in/natefinch/lumberjack.v2` | 日志文件轮转 |
