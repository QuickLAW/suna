# 单 Agent 多 UI 协作与 daemon 生命周期设计

本文记录 Suna 未来多入口接入的设计方向。它不是当前已完整实现的事实文档，而是后续扩展 Web UI、GUI、WeChat bot 等入口时的约束和目标。

## 背景

Suna 当前是本地个人 Agent：一个 daemon 管理一个 Agent、一个 active working context。CLI/TUI 只是连接 daemon 的交互入口。未来可能增加更多入口：

- Terminal TUI
- Desktop GUI
- Web UI
- WeChat bot / IM bot
- HTTP/WebSocket adapter

目标不是把 Suna 做成复杂的多用户、多会话 Agent 平台，而是在保持单 Agent 心智模型的前提下，让多个 UI 可以协作观察和操作同一个本地 Agent。

## 非目标

明确不做：

- 不引入多用户权限系统。
- 不把 daemon 做成 SaaS 式多租户服务。
- 不为每个 UI 创建独立 Agent。
- 不把当前 single active session 扩展成复杂 session tree。
- 不让多个主 Agent run 同时修改同一份 working memory。

也就是说，Suna 的核心产品形态仍然是：

```text
一个本地用户
  一个 daemon
    一个 Agent
      一个 active working context
```

## 目标模型

未来推荐模型：

```text
daemon
  ├── single Agent / single active session
  ├── transports
  │   ├── local TUI / CLI
  │   ├── WebSocket UI
  │   ├── Desktop GUI
  │   └── WeChat bot transport
  ├── event subscribers
  └── lifecycle manager
```

关键原则：

1. **单 Agent、单 active session**：所有 UI 操作的是同一份当前上下文。
2. **单飞主运行**：同一时间只允许一个 main Agent run；其他入口发送消息时应得到 busy 状态，或由 UI 引导等待/取消。
3. **事件订阅协作**：多个 UI 可以订阅当前 Agent 事件，共同观察 stream、tool、AskUser、GuardConfirm、status 等状态。
4. **长期 transport 显式声明持久性**：bot/webhook 等长期入口通过 transport/lifecycle 层声明自己需要 keep daemon alive，而不是把普通 UI 连接永久化。

## 当前实现状态

当前代码已经具备部分基础：

- daemon 可以接受多个 local transport connection。
- `Daemon.sinks` 以 connection ID 保存 event sink，并用 mutex 保护。
- `Agent.Run()` 使用 `runMu.TryLock()`，避免两个主 Agent run 同时执行。
- AskUser / GuardConfirm 使用 pending map 等待回复。
- daemon lifecycle 当前按连接数做 idle shutdown：无连接一段时间后自动退出。

但当前仍不是完整多 UI 协作模型：

- 运行事件主要发送给发起该 run 的连接，不是广播给所有 UI。
- 没有显式 subscribe/unsubscribe 协议。
- AskUser / GuardConfirm 没有跨 UI resolved 广播语义。
- session/restore/compact/config 等部分操作还不是按“多 UI 同时操作”严格设计。
- lifecycle 只看连接数，还没有 persistent transport / lease 概念。

## Event subscription 设计

### 连接流程

未来 UI 连接 daemon 后应主动订阅事件：

```text
client connect
client -> events.subscribe { interactive: true }
daemon -> subscribed
client -> daemon.status / config.get / attachment.status
```

订阅是全局 active session 级别，不引入 session_id。

### 订阅参数

建议：

```json
{
  "interactive": true,
  "client_type": "tui|web|gui|wechat|cli",
  "client_name": "optional display name"
}
```

含义：

- `interactive=true`：该客户端可以显示并回复 AskUser / GuardConfirm。
- `interactive=false`：只观察事件，不参与回复。
- CLI 短命令通常不订阅，只做 request-response。

### 广播事件

订阅后，daemon 将当前 run 的关键事件广播给所有 subscriber：

```text
run.started
stream.delta
reasoning.delta
tool.started
tool.ended
tool.guard
ask_user.created
ask_user.resolved
guard_confirm.created
guard_confirm.resolved
usage
run.done
run.cancelled
status.changed
config.changed
memory.changed
skill.review
```

事件建议带上：

```json
{
  "run_id": "...",
  "seq": 123,
  "origin_conn_id": "...",
  "event": "..."
}
```

目的：

- 多 UI 可以知道事件属于哪个 run。
- UI 可以区分自己是否是发起端。
- 未来如需补简单 replay，可按 `run_id + seq` 扩展。

## AskUser / GuardConfirm 多 UI 语义

跨 UI 协作后，AskUser 和 GuardConfirm 不应只属于发起端 UI。推荐语义：

1. daemon 向所有 `interactive=true` subscriber 广播 `ask_user.created` 或 `guard_confirm.created`。
2. 任意一个 interactive UI 可以回复。
3. 第一个有效回复生效。
4. daemon 删除 pending item，并广播 `ask_user.resolved` / `guard_confirm.resolved`。
5. 其他 UI 收到 resolved 后关闭对应面板。

需要注意：

- 回复必须携带 pending ID。
- 如果 pending ID 已被消费，后续回复返回 `not found or already resolved`。
- resolved 事件可包含 `resolved_by`，用于 UI 展示。

## Busy / Cancel 语义

Suna 继续保持单 main run：

```text
if agent is running:
    send_message -> busy
```

后续可以把当前错误文案升级成结构化 busy response：

```json
{
  "status": "busy",
  "run_id": "...",
  "message": "agent is already running"
}
```

UI 可以提供：

- 等待当前 run 完成；
- 取消当前 run；
- 丢弃本次输入；
- 把输入保存在草稿中。

`cancel` 可以允许任意 interactive subscriber 发起，或者只允许 origin UI；建议先允许任意 interactive subscriber，并广播 `run.cancelled`。

## Persistent transport 与 daemon 生命周期

当前 daemon 的 idle shutdown 是合理的性能设计：没有客户端连接时，不必让本地 daemon 永久常驻。

未来增加 bot/webhook 等长期入口后，应引入 persistent source 概念。

### Transport 声明持久性

推荐在 transport/lifecycle 层扩展：

```go
type Transport interface {
    Name() string
    Mount(ctx context.Context, svc protocol.Service) error
    Close(ctx context.Context) error
    Persistent() bool
}
```

或者更抽象：

```go
type LifecycleSource interface {
    KeepsDaemonAlive() bool
}
```

生命周期判断：

```text
if ConnectionCount() > 0:
    keep alive
else if any mounted transport is persistent:
    keep alive
else:
    idle shutdown after delay
```

适合声明 persistent 的入口：

- WeChat bot transport
- webhook server
- HTTP/WebSocket server configured as always-on
- file watcher / timer / trigger runtime

不建议普通 TUI/GUI 一连接就永久 persist。

### Lease 作为可选后续能力

如果未来外部 GUI 希望临时保持 daemon 常驻，建议设计 TTL lease，而不是永久 persist flag：

```text
daemon.lease_acquire { ttl_seconds, reason }
daemon.lease_renew { lease_id }
daemon.lease_release { lease_id }
```

原因：普通 client 可能崩溃；TTL lease 到期后 daemon 能恢复 idle shutdown，不会被坏客户端永久保活。

第一阶段可以不做 lease，仅支持 persistent transport。

## 生命周期最终规则

推荐最终规则：

```text
if active client connections > 0:
    do not idle shutdown
else if persistent transport/source exists:
    do not idle shutdown
else if active lifecycle lease exists:
    do not idle shutdown
else:
    run current idle timer and shutdown after idle delay
```

其中 lease 是可选后续项。

## 数据一致性与锁边界

保持单 Agent 后，仍需把多 UI 并发操作纳入设计：

- main run 继续由 Agent run lock 单飞。
- `new session`、`restore`、`compact`、`clear attachments` 等会修改 shared working state 的操作应串行化。
- config 更新需要继续通过 config lock，并广播 `config.changed`。
- AskUser / GuardConfirm pending map 需要支持 first reply wins。
- 事件广播不应阻塞 Agent 主循环；慢 subscriber 应有超时、缓冲或断开策略。

第一阶段可先实现简单广播；如果后续出现慢 UI 阻塞问题，再引入 per-connection writer queue。

## 推荐实施顺序

1. **明确 subscriber map**
   - connection 结构从 `sink` 扩展为 `{sink, subscribed, interactive, clientType}`。
   - 新增 `events.subscribe` / `events.unsubscribe`。

2. **广播 run events**
   - 将 stream/tool/status/skill review 等事件从 origin-only 改为 broadcast to subscribers。
   - 保留 origin sink fallback，兼容未订阅客户端。

3. **AskUser / GuardConfirm resolved 事件**
   - created 广播给 interactive subscribers。
   - first reply wins。
   - resolved 广播关闭其他 UI 面板。

4. **busy response 结构化**
   - 当前已有 run lock，先把 busy/error 语义协议化。

5. **persistent transport**
   - transport 增加 persistent/lifecycle source 能力。
   - lifecycle 在 idle 判断中纳入 persistent source。

6. **可选 lease**
   - 如外部 GUI/第三方 adapter 有临时保活需求，再加 TTL lease。

## 结论

Suna 后续多入口扩展不需要走多 Agent、多 session 的复杂路线。推荐方向是：

```text
single Agent + single active session + event subscription + persistent transport lifecycle
```

这能保持当前本地个人 Agent 的简单心智模型，同时支持 TUI、Web UI、GUI、WeChat bot 等多个入口协作使用同一个 daemon。
