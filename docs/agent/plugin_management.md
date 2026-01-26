# Elkeid 插件管理机制

本文档详细描述 Elkeid 插件的开启和关闭机制。

## 核心设计理念

插件的开启和关闭是通过**配置列表的增删**来实现的，而非单独的开关命令。Server 只需维护"期望状态"（哪些插件应该运行），Agent 自动对比并同步到该状态。

---

## 插件开启流程

当 Server 下发的 Config 列表中**包含**某个插件配置时，Agent 会自动加载并启动该插件。

### 流程图

```
Server 发送 Command.Configs = [{name: "collector", version: "1.0.0", ...}]
                ↓
Agent handleReceive() 接收
                ↓
调用 plugin.Sync(cfgs)
                ↓
plugin.Load(config)
    ├─ 验证插件名称合法性
    ├─ 检查是否已加载相同版本 → 若是，跳过
    ├─ 验证本地签名 → 若不符，从服务器下载
    ├─ 启动插件进程 (设置父进程组)
    └─ 建立 stdin/stdout/stderr 通信管道
                ↓
启动 3 个 goroutine:
    - 监听插件进程退出
    - 接收插件数据
    - 转发任务给插件
```

重要流程：

**调用链：transport.handleReceive() → plugin.Sync(cfgs) → syncCh → Startup 中的 case cfgs := <-syncCh 分支处理加载/卸载插件。**



### 关键代码

**文件位置**: `agent/plugin/plugin.go`

```go
for _, cfg := range cfgs {
    if cfg.Name != agent.Product {
        plg, err := Load(ctx, *cfg)  // 加载并启动插件
        if err == ErrDuplicatePlugin {
            continue  // 同版本已运行，跳过
        }
        if err != nil {
            agent.SetAbnormal(fmt.Sprintf("load plugin failed: %v", err))
        }
    }
}
```

---

## 插件关闭流程

当 Server 下发的 Config 列表中**不包含**某个已运行的插件时，Agent 会自动关闭并删除该插件。

### 流程图

```
Server 发送 Command.Configs = []  // 不包含 "collector"
                ↓
Agent handleReceive() 接收
                ↓
调用 plugin.Sync(cfgs)
                ↓
遍历所有已运行插件:
    if 插件不在 cfgs 中:
        plg.Shutdown()            // 关闭插件进程
        m.Delete(plg.Config.Name) // 从内存映射中移除
        os.RemoveAll(workDir)     // 删除插件工作目录
```

### 关键代码

**文件位置**: `agent/plugin/plugin.go:163-230`

```go
// 移除不在配置中的插件
for _, plg := range GetAll() {
    if _, ok := cfgs[plg.Config.Name]; !ok {
        plg.Shutdown()
        m.Delete(plg.Config.Name)
        os.RemoveAll(plg.GetWorkingDirectory())
    }
}
```

---

## 插件更新流程

当 Server 下发的 Config 列表中包含某个插件的**新版本**时：

```
Server 发送 Config: {name: "collector", version: "2.0.0", ...}
                ↓
Agent 检测到版本不同
                ↓
Load() 函数处理:
    ├─ 关闭旧版本插件
    ├─ 下载新版本二进制
    ├─ 验证签名
    └─ 启动新版本插件
```

---

## 操作总结

| 操作 | Server 做法 | Agent 行为 |
|------|------------|-----------|
| **开启插件** | Config 列表中添加该插件 | `Load()` 下载并启动 |
| **关闭插件** | Config 列表中移除该插件 | `Shutdown()` 停止并删除 |
| **更新插件** | Config 列表中发送新版本 | 关闭旧版本，启动新版本 |

---

## 关键文件位置

| 功能 | 文件路径 |
|------|---------|
| 插件同步入口 | `agent/plugin/plugin.go:163-230` |
| 插件加载逻辑 | `agent/plugin/plugin.go` - `Load()` 函数 |
| 命令接收处理 | `agent/transport/transfer.go:128-244` |
| 插件进程管理 | `agent/plugin/plugin.go` - `Shutdown()` 函数 |

---

## 设计优点

1. **声明式配置**: Server 只需声明期望状态，Agent 自动同步
2. **幂等性**: 多次下发相同配置不会重复操作
3. **原子性**: 插件更新时先关闭再启动，保证一致性
4. **自动清理**: 关闭插件时自动删除工作目录，避免残留
