# Elkeid Collector 插件实现分析

## 目录
- [插件概述](#插件概述)
- [核心架构](#核心架构)
- [资产类型详解](#资产类型详解)
- [调度引擎实现](#调度引擎实现)
- [数据采集流程](#数据采集流程)
- [容器支持](#容器支持)
- [性能优化](#性能优化)
- [扩展机制](#扩展机制)

---

## 插件概述

### 功能定位

Collector 是 Elkeid 的核心资产采集插件，负责**周期性**采集主机上的各类资产信息，并进行**关联分析**。

### 支持的资产类型

| 资产类型 | DataType | 采集频率 | 跨容器支持 | 主要用途 |
|---------|---------|---------|-----------|---------|
| 进程 (Process) | 5050 | 1小时 | ✅ | 威胁情报关联、数据溯源 |
| 端口 (Port) | 5051 | 1小时 | ✅ | 暴露面分析、服务发现 |
| 账户 (User) | 5052 | 6小时 | ❌ | 弱口令检测、权限审计 |
| 软件 (Software) | 5055 | 凌晨随机 | ⚠️部分 | 漏洞扫描、依赖分析 |
| 容器 (Container) | 5056 | 5分钟 | - | 容器运行时监控 |
| 应用 (Application) | 5060 | 凌晨随机 | ✅ | 应用发现、配置审计 |
| 网卡 (Network Interface) | - | 6小时 | ❌ | 硬件资产清点 |
| 磁盘 (Volume) | - | 6小时 | ❌ | 存储资产管理 |
| 内核模块 (Kernel Module) | - | 1小时 | ❌ | 内核态安全监控 |
| 系统服务 (Service) | - | 6小时 | ❌ | 服务管理、启动项审计 |
| 定时任务 (Cron) | - | 6小时 | ❌ | 计划任务审计 |
| 文件完整性 (Integrity) | 5057 | 凌晨随机 | ❌ | 篡改检测 |

**源码位置**: `/home/work/openSource/Elkeid/plugins/collector/main.go:38-49`

```go
e.AddHandler(time.Hour, &ProcessHandler{})
e.AddHandler(time.Hour, &PortHandler{})
e.AddHandler(time.Hour*6, &UserHandler{})
e.AddHandler(time.Hour*6, &CronHandler{})
e.AddHandler(time.Hour*6, &ServiceHandler{})
e.AddHandler(engine.BeforeDawn(), &SoftwareHandler{})
e.AddHandler(time.Minute*5, &ContainerHandler{})
e.AddHandler(engine.BeforeDawn(), &IntegrityHandler{})
e.AddHandler(time.Hour*6, &NetInterfaceHandler{})
e.AddHandler(time.Hour*6, &VolumeHandler{})
e.AddHandler(time.Hour, &KmodHandler{})
e.AddHandler(engine.BeforeDawn(), &AppHandler{})
```

### 支持平台

- **操作系统**: CentOS, RHEL, Debian, Ubuntu, RockyLinux, OpenSUSE
- **架构**: x86-64, aarch64
- **容器运行时**: Docker, CRI, containerd

---

## 核心架构

### 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    Elkeid Agent/Server                       │
│              (接收资产数据 & 下发采集任务)                    │
└────────────────────┬─────────────────────┬──────────────────┘
                     │                     │
              Task Dispatch          Data Report
              (on-demand)            (periodic)
                     │                     │
                     ▼                     ▲
┌──────────────────────────────────────────────────────────────┐
│                   Collector Plugin                            │
│                                                               │
│  ┌────────────────────────────────────────────────────┐     │
│  │              Collection Engine                      │     │
│  │  ┌──────────────────────────────────────────┐     │     │
│  │  │  Cron Scheduler (robfig/cron/v3)         │     │     │
│  │  │  - BeforeDawn: 0:00-6:00 随机时间        │     │     │
│  │  │  - @every Xm: 固定间隔                   │     │     │
│  │  └──────────────────────────────────────────┘     │     │
│  │  ┌──────────────────────────────────────────┐     │     │
│  │  │  Task Receiver (ReceiveTask)             │     │     │
│  │  │  - 监听 Server 下发的按需采集任务        │     │     │
│  │  └──────────────────────────────────────────┘     │     │
│  │  ┌──────────────────────────────────────────┐     │     │
│  │  │  Cache (多层缓存)                         │     │     │
│  │  │  - DataType → Key → Record              │     │     │
│  │  │  - 用于资产间关联 (进程-容器、端口-容器)  │     │     │
│  │  └──────────────────────────────────────────┘     │     │
│  └────────────────────────────────────────────────────┘     │
│                                                               │
│  ┌────────────────────────────────────────────────────┐     │
│  │              Asset Handlers (12个)                  │     │
│  │                                                     │     │
│  │  ┌────────────┐  ┌────────────┐  ┌─────────────┐  │     │
│  │  │  Process   │  │    Port    │  │    User     │  │     │
│  │  │  Handler   │  │  Handler   │  │   Handler   │  │     │
│  │  └────────────┘  └────────────┘  └─────────────┘  │     │
│  │                                                     │     │
│  │  ┌────────────┐  ┌────────────┐  ┌─────────────┐  │     │
│  │  │ Software   │  │ Container  │  │     App     │  │     │
│  │  │  Handler   │  │  Handler   │  │   Handler   │  │     │
│  │  └────────────┘  └────────────┘  └─────────────┘  │     │
│  │                                                     │     │
│  │  ┌────────────┐  ┌────────────┐  ┌─────────────┐  │     │
│  │  │   Cron     │  │  Service   │  │    Kmod     │  │     │
│  │  │  Handler   │  │  Handler   │  │   Handler   │  │     │
│  │  └────────────┘  └────────────┘  └─────────────┘  │     │
│  │                                                     │     │
│  │  ┌────────────┐  ┌────────────┐  ┌─────────────┐  │     │
│  │  │ Integrity  │  │  NetIface  │  │   Volume    │  │     │
│  │  │  Handler   │  │  Handler   │  │   Handler   │  │     │
│  │  └────────────┘  └────────────┘  └─────────────┘  │     │
│  └────────────────────────────────────────────────────┘     │
│                                                               │
│  ┌────────────────────────────────────────────────────┐     │
│  │           Supporting Libraries                      │     │
│  │  - process: 进程枚举与分析                          │     │
│  │  - port: 端口扫描 (netlink + procfs)                │     │
│  │  - container: 容器运行时适配                        │     │
│  │  - rpm: RPM 包解析                                  │     │
│  │  - zip: JAR 文件解析                                │     │
│  └────────────────────────────────────────────────────┘     │
└───────────────────────────────────────────────────────────────┘
```

### Handler 接口设计

**接口定义** (`engine/engine.go:59-63`):

```go
type Handler interface {
    Handle(c *plugins.Client, cache *Cache, seq string)
    Name() string
    DataType() int
}
```

**字段说明**:
- `Handle()`: 执行资产采集，参数包括：
  - `c`: Plugin SDK 客户端，用于发送数据
  - `cache`: 全局缓存，用于资产关联
  - `seq`: 批次序列号（同一批次采集的数据具有相同 seq）
- `Name()`: 返回 Handler 名称（用于日志）
- `DataType()`: 返回数据类型（用于上报和缓存键）

**示例实现** (`process.go:13-20`):

```go
type ProcessHandler struct{}

func (h ProcessHandler) Name() string {
    return "process"
}
func (h ProcessHandler) DataType() int {
    return 5050  // 进程数据类型
}
```

---

## 资产类型详解

### 1. 进程资产 (ProcessHandler)

**DataType**: 5050

**采集频率**: 每小时

**核心功能**:

1. **进程枚举**: 遍历 `/proc` 目录获取所有进程
2. **可执行文件哈希**: 计算 exe 的 MD5 和自定义 checksum
3. **容器关联**: 通过 PID Namespace 关联容器信息
4. **完整性校验**: 标记是否被篡改

**采集字段**:

| 字段 | 说明 | 来源 |
|------|------|------|
| pid | 进程ID | `/proc/[pid]` |
| cmdline | 命令行参数 | `/proc/[pid]/cmdline` |
| exe | 可执行文件路径 | `/proc/[pid]/exe` |
| exe_hash | 可执行文件哈希 (MD5) | 文件内容计算 |
| checksum | 自定义校验和 | 特定字节计算 |
| cwd | 当前工作目录 | `/proc/[pid]/cwd` |
| ppid | 父进程ID | `/proc/[pid]/stat` |
| state | 进程状态 | `/proc/[pid]/stat` |
| uid, gid | 用户/组ID | `/proc/[pid]/status` |
| pid_ns, mnt_ns... | 命名空间ID | `/proc/[pid]/ns/*` |
| container_id | 容器ID | 通过 cache 关联 |
| container_name | 容器名称 | 通过 cache 关联 |
| integrity | 完整性标记 | 与软件包哈希对比 |

**实现代码** (`process.go:22-70`):

```go
func (h *ProcessHandler) Handle(c *plugins.Client, cache *engine.Cache, seq string) {
    // 1. 枚举所有进程
    procs, err := process.Processes(false)
    if err != nil {
        zap.S().Error(err)
    } else {
        for _, p := range procs {
            time.Sleep(process.TraversalInterval)  // 限速，避免CPU占用过高

            // 2. 读取进程信息
            cmdline, _ := p.Cmdline()
            stat, _ := p.Stat()
            status, _ := p.Status()
            ns, _ := p.Namespaces()

            // 3. 构造 Record
            rec := &plugins.Record{
                DataType:  int32(h.DataType()),
                Timestamp: time.Now().Unix(),
                Data: &plugins.Payload{
                    Fields: make(map[string]string, 40),
                },
            }

            // 4. 填充基本字段
            rec.Data.Fields["cmdline"] = cmdline
            rec.Data.Fields["cwd"], _ = p.Cwd()
            rec.Data.Fields["checksum"], _ = p.ExeChecksum()
            rec.Data.Fields["exe_hash"], _ = p.ExeHash()
            rec.Data.Fields["exe"], _ = p.Exe()
            rec.Data.Fields["pid"] = p.Pid()

            // 5. 使用 mapstructure 批量映射
            mapstructure.Decode(stat, &rec.Data.Fields)
            mapstructure.Decode(status, &rec.Data.Fields)
            mapstructure.Decode(ns, &rec.Data.Fields)

            // 6. 容器关联 (从 cache 获取容器信息)
            m, _ := cache.Get(5056, ns.Pid)  // 通过 PID namespace 查询
            rec.Data.Fields["container_id"] = m["container_id"]
            rec.Data.Fields["container_name"] = m["container_name"]

            // 7. 完整性校验
            rec.Data.Fields["integrity"] = "true"
            if _, ok := cache.Get(5057, rec.Data.Fields["exe"]); ok && rec.Data.Fields["container_id"] == "" {
                rec.Data.Fields["integrity"] = "false"  // 文件哈希不匹配
            }

            // 8. 批次序列号
            rec.Data.Fields["package_seq"] = seq

            // 9. 发送数据
            c.SendRecord(rec)
        }
    }
}
```

**关键点**:
- 使用 `process.TraversalInterval` 限速，避免 CPU 飙升
- 通过 PID Namespace 实现进程与容器的关联
- 支持完整性校验，检测文件是否被篡改

### 2. 端口资产 (PortHandler)

**DataType**: 5051

**采集频率**: 每小时

**核心功能**:

1. **监听端口扫描**: 获取 TCP/UDP 监听端口
2. **进程关联**: 通过 socket inode 关联进程
3. **容器关联**: 通过进程关联容器
4. **暴露面分析**: 识别对外暴露的服务

**采集字段**:

| 字段 | 说明 |
|------|------|
| protocol | 协议 (tcp/udp) |
| saddr | 监听地址 |
| sport | 监听端口 |
| pid | 进程ID |
| container_id | 容器ID |
| container_name | 容器名称 |

**实现代码** (`port.go:21-42`):

```go
func (h *PortHandler) Handle(c *plugins.Client, cache *engine.Cache, seq string) {
    // 1. 获取所有监听端口
    ports, err := port.ListeningPorts()
    if err != nil {
        zap.S().Error(err)
    } else {
        for _, port := range ports {
            rec := &plugins.Record{
                DataType:  int32(h.DataType()),
                Timestamp: time.Now().Unix(),
                Data: &plugins.Payload{
                    Fields: make(map[string]string, 15),
                },
            }

            // 2. 映射端口字段
            mapstructure.Decode(port, &rec.Data.Fields)

            // 3. 容器关联 (通过进程PID查询容器)
            m, _ := cache.Get(5056, port.Sport)  // 实际应该是 port.Pid
            rec.Data.Fields["container_id"] = m["container_id"]
            rec.Data.Fields["container_name"] = m["container_name"]

            rec.Data.Fields["package_seq"] = seq
            c.SendRecord(rec)
        }
    }
}
```

**端口扫描实现**:

端口信息通过两种方式获取：
1. **Netlink**: 通过 `NETLINK_INET_DIAG` 套接字获取（高效）
2. **Procfs**: 解析 `/proc/net/tcp`, `/proc/net/udp`（兜底）

**源码位置**: `/home/work/openSource/Elkeid/plugins/collector/port/`

### 3. 软件资产 (SoftwareHandler)

**DataType**: 5055

**采集频率**: 凌晨随机时间（0:00-6:00）

**核心功能**:

1. **系统软件包**: dpkg (Debian/Ubuntu), rpm (CentOS/RHEL)
2. **PyPI 包**: Python 包管理
3. **JAR 包**: Java 应用依赖（支持递归扫描嵌套 JAR）

**采集字段**:

| 字段 | 说明 | 适用类型 |
|------|------|---------|
| name | 软件包名称 | All |
| sversion | 版本号 | All |
| type | 类型 (dpkg/rpm/pypi/jar) | All |
| source | 源包名 | dpkg |
| status | 安装状态 | dpkg |
| vendor | 供应商 | rpm |
| component_version | 组件版本 | rpm, pypi |
| pid | 关联进程ID | jar |
| pod_name | Pod名称 | jar |
| psm | 服务名 | jar |

**JAR 包扫描实现** (`software.go:91-100`):

```go
func findJar(c *plugins.Client, rec *plugins.Record, r *zip.Reader, n string) {
    // 解析文件名提取版本
    name, version := parseJarFilename(filepath.Base(n[:len(n)-4]))

    // 递归遍历 JAR 内的文件
    r.WalkFiles(func(f *zip.File) {
        if strings.HasSuffix(f.Name, ".jar") {
            // 发现嵌套 JAR
            rec.Data.Fields["name"], rec.Data.Fields["sversion"] = parseJarFilename(filepath.Base(f.Name[:len(f.Name)-4]))
            rec.Data.Fields["path"] = filepath.Join(r.Name(), f.Name)
            rec.Timestamp = time.Now().Unix()
            c.SendRecord(rec)
        }
        // 递归深度限制: MaxRecursionLevel = 3
    })
}
```

**版本号解析** (`software.go:80-90`):

```go
func parseJarFilename(fn string) (n, v string) {
    // 使用正则查找版本号: -[0-9]
    index := VersionReg.FindStringIndex(fn)  // -[0-9]
    if len(index) == 0 {
        n = fn
        v = ""
    } else {
        n = fn[:(index[0])]      // 名称部分
        v = fn[(index[0] + 1):]  // 版本部分
    }
    return
}
```

**示例**:
- `spring-boot-2.5.0.jar` → name: `spring-boot`, version: `2.5.0`
- `commons-lang3-3.12.0.jar` → name: `commons-lang3`, version: `3.12.0`

### 4. 容器资产 (ContainerHandler)

**DataType**: 5056

**采集频率**: 每 5 分钟（最高频）

**核心功能**:

1. **多运行时支持**: Docker, CRI, containerd
2. **容器状态监控**: 运行中、停止、异常
3. **镜像信息采集**: 镜像ID、镜像名称
4. **命名空间跟踪**: 用于进程/端口关联

**采集字段**:

| 字段 | 说明 |
|------|------|
| id | 容器ID |
| name | 容器名称 |
| state | 状态 (running/stopped/...) |
| image_id | 镜像ID |
| image_name | 镜像名称 |
| pid | 容器主进程PID |
| pns | PID Namespace |
| runtime | 运行时类型 |
| create_time | 创建时间 |

**实现代码** (`container.go:34-69`):

```go
func (h *ContainerHandler) Handle(c *plugins.Client, cache *engine.Cache, seq string) {
    // 1. 创建多个运行时客户端
    clients := container.NewClients()  // Docker, CRI, containerd

    for _, client := range clients {
        // 2. 列出所有容器
        containers, err := client.ListContainers(context.Background())
        client.Close()

        if err != nil {
            continue  // 该运行时不可用，尝试下一个
        }

        for _, ctr := range containers {
            // 3. 上报容器信息
            c.SendRecord(&plugins.Record{
                DataType:  int32(h.DataType()),
                Timestamp: time.Now().Unix(),
                Data: &plugins.Payload{
                    Fields: map[string]string{
                        "id":          ctr.ID,
                        "name":        ctr.Name,
                        "state":       ctr.State,
                        "image_id":    ctr.ImageID,
                        "image_name":  ctr.ImageName,
                        "pid":         ctr.Pid,
                        "pns":         ctr.Pns,
                        "runtime":     ctr.State,
                        "create_time": ctr.CreateTime,
                        "package_seq": seq,
                    },
                },
            })

            // 4. 缓存运行中容器的 Pns (用于进程/端口关联)
            if ctr.State == container.StateName[int32(container.RUNNING)] &&
               ctr.Pns != "" &&
               process.PnsDiffWithRpns(ctr.Pns) {
                cache.Put(h.DataType(), ctr.Pns, map[string]string{
                    "container_id":   ctr.ID,
                    "container_name": ctr.Name,
                })
            }
        }
    }
}
```

**容器运行时适配**:

支持的运行时接口定义在 `/home/work/openSource/Elkeid/plugins/collector/container/` 目录下，包括：
- Docker API
- CRI (Container Runtime Interface)
- containerd API

### 5. 应用资产 (AppHandler)

**DataType**: 5060

**采集频率**: 凌晨随机时间

**核心功能**:

1. **应用发现**: 基于进程命令行和可执行文件路径识别应用
2. **版本提取**: 执行 `--version` 或 `-v` 获取版本信息
3. **配置文件定位**: 自动查找应用配置文件
4. **跨容器支持**: 可以进入容器命名空间获取信息

**支持的应用类型**:

**Web 服务**:
- Apache, Nginx, Tengine, OpenResty

**数据库**:
- MySQL, PostgreSQL, MongoDB, Redis, Elasticsearch

**消息队列**:
- Kafka, RabbitMQ, RocketMQ

**容器编排**:
- Docker, Kubernetes (kubelet, kube-proxy, etc.)

**DevOps 工具**:
- Jenkins, Grafana, Prometheus

**总计**: 30+ 常见应用

**应用规则定义** (`app.go:23-47`):

```go
var (
    apacheRule = &AppRule{
        name:              "apache",
        _type:             "web_service",
        versionRegex:      regexp.MustCompile(`Apache\/(\d+\.)+\d+`),
        versionArgs:       []string{"-v"},
        versionTrimPrefix: "Apache/",
        confFunc: func(rc RuleContext) string {
            // 1. 从命令行提取配置路径
            res := regexp.MustCompile(`-f\s\S+`).Find([]byte(rc.cmdline))
            if res != nil {
                return strings.TrimPrefix(string(res), "-f ")
            }

            // 2. 尝试默认配置路径
            rootPath := "/"
            if rc.enterContainer {
                rootPath = filepath.Join("/proc", rc.proc.Pid(), "root")
            }
            for _, path := range []string{
                "/usr/local/apache2/conf/httpd.conf",
                "/etc/apache2/apache2.conf",
                "/etc/httpd/conf/httpd.conf",
                "/etc/apache2/httpd.conf"} {
                if _, err := os.Stat(filepath.Join(rootPath, path)); err == nil {
                    return path
                }
            }
            return ""
        },
    }
)
```

**RuleContext 结构**:

```go
type RuleContext struct {
    proc           process.Process  // 进程对象
    cmdline        string           // 命令行
    enterContainer bool             // 是否需要进入容器
}
```

**版本提取流程**:

1. 检查进程命令行是否匹配应用特征
2. 执行 `exe -v` 或 `exe --version` 获取版本信息
3. 使用正则表达式 `versionRegex` 提取版本号
4. 调用 `confFunc` 获取配置文件路径
5. 上报应用信息

**嵌套规则**:

某些应用支持子规则，例如 Nginx → Tengine → OpenResty：

```go
nginxRule = &AppRule{
    name: "nginx",
    // ...
    sub: &AppRule{
        name: "tengine",
        // ...
        sub: &AppRule{
            name: "openresty",
            // ...
        },
    },
}
```

执行时会依次尝试匹配，直到找到最具体的应用类型。

---

## 调度引擎实现

### Engine 架构

**核心组件** (`engine/engine.go:92-97`):

```go
type Engine struct {
    m     map[int]*handler       // DataType → Handler 映射
    s     *cron.Cron             // Cron 调度器
    c     *plugins.Client        // Plugin SDK 客户端
    cache *Cache                 // 全局缓存
}
```

### 调度策略

#### 1. 定时调度 (Cron)

使用 `robfig/cron/v3` 库实现，支持两种调度模式：

**模式1: BeforeDawn (凌晨随机)**

```go
func BeforeDawn() time.Duration {
    return -1  // 特殊标记
}
```

**调度逻辑** (`engine/engine.go:119-121`):

```go
if h.interval == BeforeDawn() {
    spec = fmt.Sprintf("%d %d * * *", rand.Intn(60), rand.Intn(6))  // 0-5点, 0-59分
    r = rand.Intn(14400) + 7200  // 初始延迟: 2-6小时
}
```

**示例**: 可能在每天 `03:27` 执行

**模式2: 固定间隔 (@every)**

```go
else if minutes > 0 {
    r = rand.Intn(minutes * 60)  // 初始随机延迟
    spec = fmt.Sprintf("@every %dm", int(minutes))
}
```

**示例**: `@every 60m` 每小时执行一次

#### 2. 按需调度 (Task)

**任务接收循环** (`engine/engine.go:142-176`):

```go
for {
    // 1. 阻塞等待任务
    t, err := e.c.ReceiveTask()
    if err != nil {
        break
    }

    // 2. 根据 DataType 查找 Handler
    zap.S().Infof("received task %+v", t)
    if h, ok := e.m[int(t.DataType)]; ok {
        // 3. 执行采集
        h.Handle(e.c, e.cache)

        // 4. 上报成功状态 (DataType: 5100)
        e.c.SendRecord(&plugins.Record{
            DataType:  5100,
            Timestamp: time.Now().Unix(),
            Data: &plugins.Payload{
                Fields: map[string]string{
                    "status": "succeed",
                    "msg":    "",
                    "token":  t.Token,
                },
            }})
    } else {
        // 5. Handler 不存在，上报失败
        e.c.SendRecord(&plugins.Record{
            DataType:  5100,
            // ...
            "status": "failed",
            "msg":    "the data_type hasn't been implemented",
        })
    }
}
```

### 初始化延迟

为了避免所有 Agent 同时采集导致服务器压力过大，每个 Handler 在首次执行前会随机延迟：

```go
r := rand.Intn(minutes * 60)  // 0 到 interval 之间的随机秒数
h.l.Infof("init call will after %d secs\n", r)
time.Sleep(time.Second * time.Duration(r))
```

**示例**:
- 1小时间隔的 Handler: 随机延迟 0-3600 秒
- 6小时间隔的 Handler: 随机延迟 0-21600 秒

### 批次序列号 (seq)

每次采集都会生成唯一的批次序列号，用于标识同一批次的数据：

```go
// engine/engine.go:77-79
f := fnv.New32()
binary.Write(f, binary.LittleEndian, time.Now().UnixNano())
seq := hex.EncodeToString(f.Sum(nil))
```

**作用**:
- 数据溯源：追踪数据来源
- 批量处理：Server 端可以批量处理同一批次的数据
- 增量同步：判断数据是否为最新

---

## 数据采集流程

### 完整流程图

```
1. Engine 初始化
   ├─ 创建 Cron 调度器
   ├─ 创建 Plugin Client
   ├─ 初始化 Cache
   └─ 注册所有 Handler

2. 启动调度
   ├─ 为每个 Handler 创建 Goroutine
   ├─ 随机延迟后首次执行
   └─ 添加到 Cron 调度器

3. 定时触发 / 任务触发
   ↓
4. Handler.Handle() 执行
   ├─ 生成批次 seq
   ├─ 清空该 DataType 的 cache
   └─ 执行具体采集逻辑
       ├─ 枚举资产（进程/端口/容器...）
       ├─ 提取字段
       ├─ 从 cache 关联其他资产
       └─ 构造 Record 并发送

5. 数据上报
   ├─ 通过 Plugin SDK 发送到 Agent
   └─ Agent 转发到 Server

6. Cache 更新
   └─ 存储关联信息供其他 Handler 使用
```

### 资产关联机制

**Cache 数据结构** (`engine/engine.go:28-32`):

```go
type Cache struct {
    // DataType → Key → Record
    m  map[int]Records
    mu *sync.RWMutex
}
```

**关联示例**:

1. **容器 → 进程/端口**

```go
// ContainerHandler 存储容器信息
cache.Put(5056, ctr.Pns, map[string]string{
    "container_id":   ctr.ID,
    "container_name": ctr.Name,
})

// ProcessHandler 查询容器信息
m, _ := cache.Get(5056, ns.Pid)  // 通过 PID namespace 查询
rec.Data.Fields["container_id"] = m["container_id"]
```

2. **软件包 → 进程完整性**

```go
// IntegrityHandler 存储软件包文件哈希
cache.Put(5057, filePath, fileHash)

// ProcessHandler 检查文件完整性
if _, ok := cache.Get(5057, rec.Data.Fields["exe"]); ok {
    rec.Data.Fields["integrity"] = "false"  // 哈希不匹配
}
```

### 并发安全

Cache 使用读写锁保证并发安全：

```go
func (c *Cache) Get(dt int, key string) (map[string]string, bool) {
    c.mu.RLock()         // 读锁
    res, ok := c.m[dt][key]
    c.mu.RUnlock()
    return res, ok
}

func (c *Cache) Put(dt int, key string, value map[string]string) {
    c.mu.Lock()          // 写锁
    c.m[dt][key] = value
    c.mu.Unlock()
}
```

---

## 容器支持

### 跨容器采集

**实现原理**:

通过 Linux Namespace 机制，Collector 可以"进入"容器的文件系统和进程空间：

```go
rootPath := "/"
if rc.enterContainer {
    rootPath = filepath.Join("/proc", rc.proc.Pid(), "root")
}
```

**访问容器内文件**:

```
Host:      /etc/nginx/nginx.conf
Container: /proc/12345/root/etc/nginx/nginx.conf
```

其中 `12345` 是容器主进程的 PID。

### 容器运行时适配

**接口定义** (示意):

```go
type Client interface {
    ListContainers(ctx context.Context) ([]Container, error)
    Close()
}

type Container struct {
    ID        string
    Name      string
    State     string
    ImageID   string
    ImageName string
    Pid       string
    Pns       string  // PID Namespace
    CreateTime string
}
```

**多运行时探测**:

```go
func NewClients() []Client {
    clients := []Client{}

    // 尝试 Docker
    if dockerClient, err := NewDockerClient(); err == nil {
        clients = append(clients, dockerClient)
    }

    // 尝试 CRI
    if criClient, err := NewCRIClient(); err == nil {
        clients = append(clients, criClient)
    }

    // 尝试 containerd
    if ctrdClient, err := NewContainerdClient(); err == nil {
        clients = append(clients, ctrdClient)
    }

    return clients
}
```

---

## 性能优化

### 1. CPU 限制

```go
func init() {
    runtime.GOMAXPROCS(8)  // 最多使用 8 个 CPU 核心
}
```

### 2. 进程遍历限速

```go
for _, p := range procs {
    time.Sleep(process.TraversalInterval)  // 每个进程之间暂停
    // ...
}
```

避免短时间内大量系统调用导致 CPU 飙升。

### 3. JAR 递归深度限制

```go
const (
    MaxRecursionLevel = 3  // 最多递归 3 层
)
```

防止嵌套 JAR 导致无限递归。

### 4. 调度时间分散

- **初始延迟**: 随机延迟 0 到 interval 时间
- **凌晨调度**: 随机分布在 0:00-6:00
- **避免同时执行**: 不同 Handler 不会在同一时刻启动

### 5. Cron 任务跳过

使用 `cron.SkipIfStillRunning` 策略：

```go
cron.New(cron.WithChain(cron.SkipIfStillRunning(l)))
```

如果上一次任务还在执行，跳过本次触发。

---

## 扩展机制

### 添加新的资产类型

**步骤**:

1. **定义 Handler**

```go
type MyAssetHandler struct{}

func (h *MyAssetHandler) Name() string {
    return "my_asset"
}

func (h *MyAssetHandler) DataType() int {
    return 5099  // 自定义 DataType
}

func (h *MyAssetHandler) Handle(c *plugins.Client, cache *engine.Cache, seq string) {
    // 采集逻辑
    // ...
    c.SendRecord(rec)
}
```

2. **注册 Handler**

```go
// main.go
e.AddHandler(time.Hour, &MyAssetHandler{})
```

3. **编译部署**

```bash
BUILD_VERSION=1.7.0.141 bash build.sh
```

### 扩展应用识别

**添加新应用**:

```go
var (
    myAppRule = &AppRule{
        name:              "myapp",
        _type:             "custom_service",
        versionRegex:      regexp.MustCompile(`MyApp\/(\d+\.)+\d+`),
        versionArgs:       []string{"--version"},
        versionTrimPrefix: "MyApp/",
        confFunc: func(rc RuleContext) string {
            // 定位配置文件
            return "/etc/myapp/config.yaml"
        },
    }
)
```

然后在 AppHandler 中注册该规则。

---

## 总结

### 架构特点

1. **模块化设计**: Handler 接口 + Engine 框架
2. **灵活调度**: 支持定时 + 按需两种模式
3. **资产关联**: 通过 Cache 实现跨资产关联
4. **容器支持**: 原生支持容器环境采集
5. **性能优化**: 多层次的性能保护机制

### 数据流

```
资产采集 → Handler → Record → Plugin SDK → Agent → Server → 资产中心
```

### 关键技术

- **进程枚举**: `/proc` 文件系统
- **端口扫描**: Netlink + Procfs
- **软件包**: dpkg/rpm 数据库 + 文件系统遍历
- **容器**: 多运行时 API 适配
- **应用识别**: 进程匹配 + 版本提取
- **调度**: Cron 表达式 + 随机延迟

### 适用场景

- 资产清点与管理
- 漏洞扫描（基于软件版本）
- 暴露面分析（基于端口信息）
- 弱口令检测（基于账户哈希）
- 配置审计（基于应用配置）
- 威胁情报关联（基于进程哈希）
- 容器安全监控

---

**参考文档**:
- [Collector README](../README-zh_CN.md)
- [Elkeid 项目文档](https://github.com/bytedance/Elkeid)
