# Go WaitGroup 级联使用模式学习文档

本文档以 Elkeid Agent 的 transport 模块为例，详细讲解 WaitGroup 和 Sub-WaitGroup 的级联使用模式。

## 1. 背景

在 Go 并发编程中，`sync.WaitGroup` 用于等待一组 goroutine 完成。当存在多层嵌套的 goroutine 时，使用**级联 WaitGroup 模式**可以实现优雅的生命周期管理。

## 3. 源码详解

### 3.1 Startup 函数 (transport.go:11-24)

```go
func Startup(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()                    // ① 函数退出时通知调用方

    subCtx, cancel := context.WithCancel(ctx)
    defer cancel()                     // ② 确保取消子 context

    subWg := &sync.WaitGroup{}         // ③ 创建子 WaitGroup
    defer subWg.Wait()                 // ④ 等待所有子 goroutine 完成

    subWg.Add(2)                       // ⑤ 预声明将启动 2 个 goroutine
    go startFileExt(subCtx, subWg)
    go func() {
        startTransfer(subCtx, subWg)
        cancel()                       // ⑥ transfer 退出时取消整个 context
    }()
}
```



| 层级 | 函数 | Add() 位置 | Wait() 位置 | Done() 位置 |
|-----|------|-----------|-------------|-------------|
| 层级1 | Startup | 启动子 goroutine 前 | defer (函数开头) | defer (函数开头) |
| 层级2 | startTransfer | 启动子 goroutine 前 | 循环内 + defer | defer (函数开头) |
| 层级3 | handleSend/Receive | - | - | defer (函数开头) |

## 5. subWg.Wait() 作用详解

代码中有多处 `subWg.Wait()` 调用，它们的作用各不相同。

### 5.1 Startup 中的 `defer subWg.Wait()`

```go
func Startup(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    subCtx, cancel := context.WithCancel(ctx)
    defer cancel()
    subWg := &sync.WaitGroup{}
    defer subWg.Wait()          // ← 这里
    subWg.Add(2)
    go startFileExt(subCtx, subWg)
    go func() {
        startTransfer(subCtx, subWg)
        cancel()
    }()
}
```

**作用**：确保 `startFileExt` 和 `startTransfer` 都退出后，Startup 才能退出。

```
没有 defer subWg.Wait() 的情况：
────────────────────────────────
Startup 启动两个 goroutine 后立即返回
    │
    ▼
wg.Done() 被调用
    │
    ▼
main 的 wg.Wait() 返回，程序退出
    │
    ▼
startTransfer 和 startFileExt 被强制终止（资源泄漏）
```

### 5.2 startTransfer 中的两处 Wait

```go
func startTransfer(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    subWg := &sync.WaitGroup{}
    defer subWg.Wait()          // ← 位置 A：函数退出时的保障

    for {
        // 获取连接...
        if err == nil {
            subWg.Add(2)
            go handleSend(subCtx, subWg, client)
            go func() {
                handleReceive(subCtx, subWg, client)
                cancel()
            }()
            subWg.Wait()        // ← 位置 B：循环内等待
        }
        // 等待后重试...
    }
}
```

#### 位置 B（循环内 `subWg.Wait()`）的作用

**确保当前连接的 send/receive 都完成后，才能开始新的连接尝试**：

```
正常流程：
─────────────────────────────────────────────────
连接1建立 → subWg.Add(2)
         → 启动 handleSend, handleReceive
         → subWg.Wait() 阻塞等待
         → (连接断开，两个 handler 都退出)
         → subWg.Wait() 返回
         → 进入下一轮循环，尝试新连接

如果没有这个 Wait：
─────────────────────────────────────────────────
连接1断开后立即尝试连接2
    │
    ▼
连接1的 handleSend 可能还在运行
    │
    ▼
同时存在多对 send/receive，状态混乱
```

#### 位置 A（`defer subWg.Wait()`）的作用

**兜底保障**：当 `startTransfer` 因 `return` 退出时（如重试超过 5 次），确保最后一组 send/receive 完成。

```go
if retries > 5 {
    return  // 直接返回，但 defer subWg.Wait() 会等待
}
```

### 5.3 两处 Wait 的协作关系

```
┌─────────────────────────────────────────────────────────────────┐
│                     startTransfer 函数                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  defer subWg.Wait()  ◄─── 位置 A：函数退出时的最后防线           │
│                                                                 │
│  for {                                                          │
│      连接建立...                                                 │
│      subWg.Add(2)                                               │
│      go handleSend()                                            │
│      go handleReceive()                                         │
│      subWg.Wait()    ◄─── 位置 B：每轮循环的同步点               │
│                           确保旧连接清理后再建新连接              │
│  }                                                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 5.4 总结对比

| 位置 | 代码位置 | 触发时机 | 作用 |
|-----|---------|---------|------|
| Startup 的 `defer subWg.Wait()` | transport.go:17 | Startup 函数退出时 | 等待 startTransfer 和 startFileExt 完成 |
| startTransfer 循环内 `subWg.Wait()` | transfer.go:74 | 每次连接断开后 | 等待当前 send/receive 完成再重连 |
| startTransfer 的 `defer subWg.Wait()` | transfer.go:43 | startTransfer 函数退出时 | 兜底，确保最后一组 handler 完成 |

**核心目的**：保证资源按正确顺序释放，避免 goroutine 泄漏。

## 6. 核心设计原则

### 6.1 defer wg.Done() 在函数开头

```go
func worker(wg *sync.WaitGroup) {
    defer wg.Done()  // 推荐：保证任何退出路径都会调用

    // 业务逻辑...
    if err != nil {
        return       // Done() 仍会被调用
    }
    // 更多逻辑...
}
```

**好处**：无论函数如何退出（正常返回、panic、return），`Done()` 都会被调用。

### 6.2 defer subWg.Wait() 确保子任务完成

```go
func parent(wg *sync.WaitGroup) {
    defer wg.Done()

    subWg := &sync.WaitGroup{}
    defer subWg.Wait()  // 关键：等待子 goroutine

    subWg.Add(n)
    for i := 0; i < n; i++ {
        go child(subWg)
    }
    // 函数结束时，先等子任务完成，再通知上层
}
```

### 6.3 Add() 在启动 goroutine 之前

```go
// 正确 ✓
subWg.Add(2)
go worker1(subWg)
go worker2(subWg)

// 错误 ✗ - 可能存在竞态
go func() {
    subWg.Add(1)  // goroutine 可能还没执行到这里
    worker1(subWg)
}()
subWg.Wait()  // Wait 可能在 Add 之前执行
```

## 7. 时序图

```
时间线 ─────────────────────────────────────────────────────────────────►

main:       wg.Add(1) ──────────────────────────────────────► wg.Wait()
                │
Startup:        └─► subWg.Add(2) ─────────────────► subWg.Wait() ─► wg.Done()
                         │
startTransfer:           └─► subWg2.Add(2) ─► subWg2.Wait() ─► subWg.Done()
                                  │
handleSend:                       └─► 业务逻辑 ─────────► subWg2.Done()
                                  │
handleReceive:                    └─► 业务逻辑 ─────────► subWg2.Done()
```

## 8. 为什么需要级联 WaitGroup？

### 场景：优雅关闭

```go
// 不使用级联 WaitGroup 的问题：
func main() {
    wg.Add(3)  // 扁平管理所有 goroutine
    go startup(wg)
    go transfer(wg)
    go handler(wg)
    wg.Wait()
}
// 问题：无法控制关闭顺序，可能 handler 先退出导致 transfer 失败
```

```go
// 使用级联 WaitGroup：
func main() {
    wg.Add(1)
    go startup(wg)  // startup 内部管理 transfer 和 handler
    wg.Wait()
}
// 好处：
// 1. 层级清晰，每层只管理直接子任务
// 2. 关闭顺序可控：子任务先结束，父任务后结束
// 3. 资源释放有序：连接、文件句柄按正确顺序关闭
```

## 9. 模式对比

| 模式 | 适用场景 | 复杂度 |
|-----|---------|-------|
| 单层 WaitGroup | 简单并发任务 | 低 |
| 级联 WaitGroup | 多层嵌套的 goroutine | 中 |
| errgroup | 需要错误传播 | 中 |
| 级联 WaitGroup + Context | 需要取消和优雅关闭 | 高 |

## 10. 最佳实践清单

- [ ] `defer wg.Done()` 放在函数第一行
- [ ] `defer subWg.Wait()` 在创建子 WaitGroup 后立即声明
- [ ] `Add(n)` 在启动 goroutine 之前调用
- [ ] 配合 `context.Context` 实现取消信号传递
- [ ] 每个函数只管理直接子任务的 WaitGroup
- [ ] 循环中复用 WaitGroup 时，确保每轮 Wait 后再 Add

## 11. 完整示例代码

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    wg := &sync.WaitGroup{}

    wg.Add(1)
    go layer1(ctx, wg)

    // 模拟 5 秒后关闭
    time.Sleep(5 * time.Second)
    cancel()

    wg.Wait()
    fmt.Println("main: all done")
}

func layer1(ctx context.Context, wg *sync.WaitGroup) {
    defer wg.Done()
    defer fmt.Println("layer1: done")

    subWg := &sync.WaitGroup{}
    defer subWg.Wait()

    subWg.Add(2)
    go layer2(ctx, subWg, "worker-A")
    go layer2(ctx, subWg, "worker-B")

    <-ctx.Done()
    fmt.Println("layer1: received cancel signal")
}

func layer2(ctx context.Context, wg *sync.WaitGroup, name string) {
    defer wg.Done()
    defer fmt.Printf("layer2 %s: done\n", name)

    for {
        select {
        case <-ctx.Done():
            fmt.Printf("layer2 %s: shutting down\n", name)
            return
        case <-time.After(time.Second):
            fmt.Printf("layer2 %s: working...\n", name)
        }
    }
}
```

输出：
```
layer2 worker-A: working...
layer2 worker-B: working...
layer2 worker-A: working...
layer2 worker-B: working...
...
layer1: received cancel signal
layer2 worker-A: shutting down
layer2 worker-A: done
layer2 worker-B: shutting down
layer2 worker-B: done
layer1: done
main: all done
```

---

> 本文档基于 Elkeid Agent transport 模块源码整理
>
> 相关文件：
> - `agent/transport/transport.go`
> - `agent/transport/transfer.go`
