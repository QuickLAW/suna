# 06 — Suna 记忆系统

Suna 的记忆目标不是保存所有历史，也不是构建长期知识库，而是让用户在一个当前会话里持续对话时，Suna 能保持连续性、理解用户偏好，并在上下文变长后仍能可靠继续。

记忆只服务四个结果：

- 当前任务或当前对话不中断，自动 compact 后仍能继续执行或讨论。
- 较早完成的任务、讨论过的话题、用户要求和关键决策不会轻易消失。
- 退出 TUI 后恢复会话，用户能看到真实对话，模型能拿到精简但高价值的上下文。
- 跨会话保留少量长期用户偏好、习惯、约束和纠错。

## 设计原则

```text
原则 1: 单用户单当前会话。Suna 当前不做多会话管理；用户要么继续上一条，要么 /new。
原则 2: 不使用 embedding，不做向量检索或完整历史搜索。
原则 3: user_memory 只保存长期用户画像/偏好/约束，不保存具体任务日志。
原则 4: Session State 保存当前会话的可恢复状态，承接 compact 和 restore。
原则 5: TUI 展示真实可见对话；模型不必加载完整 transcript。
原则 6: compact 是低频高质量 LLM 请求，只在自动 80% 阈值或手动 /compact 时触发。
原则 7: compact 失败不 fallback、不伪压缩、不继续模型请求。
原则 8: 上下文结构必须缓存友好：稳定前缀在前，动态 active memory 靠近 latest user。
```

## 记忆模型

```text
┌─────────────────────────────────────────────────────────────┐
│ user_memory                                                  │
│ 长期 active memory。只记录用户画像、偏好、习惯、纠错和约束。     │
│ 数量很小，默认最多 30 条。                                    │
├─────────────────────────────────────────────────────────────┤
│ conversation_state                                           │
│ 当前会话恢复状态。保存 session_state、可见对话快照和工具摘要。   │
├─────────────────────────────────────────────────────────────┤
│ session_state                                                │
│ 当前会话的内部状态账本，由 compact 生成/更新并持久化。           │
├─────────────────────────────────────────────────────────────┤
│ working_memory                                               │
│ 当前进程内模型工作上下文。compact 后只保留 recent window。        │
├─────────────────────────────────────────────────────────────┤
│ memory_queue                                                 │
│ active memory 的临时提取队列。主链路写入，daemon 批量处理后删除。 │
└─────────────────────────────────────────────────────────────┘
```

## user_memory

`user_memory` 是长期 active memory。它不是知识库，也不是会话历史，而是用户理解层。

### 应该记住

- 用户沟通偏好：喜欢简洁、直接、详细、先给结论等。
- 用户做事习惯：偏好简单方案、重视稳定性、讨厌过度设计等。
- 用户长期约束：不要硬编码、不要兼容兜底、先讨论再实现等。
- 用户纠错记录：Suna 上次哪里做错了，下次应该避免什么。
- 用户性格/风格：效率优先、谨慎、产品直觉强、工程可控性优先等。
- 用户明确要求长期记住的信息。

### 不应该记住

- 所有会话内容。
- 某次任务的临时细节。
- 工具调用日志或测试输出。
- 本会话中“第一个任务做了什么”这类 session history。
- 大段历史摘要。
- 低置信推测。
- 已经过期或不再活跃的状态。

这些内容如果对当前会话后续有价值，应进入 `Session State`，而不是进入长期 `user_memory`。

### 数量限制

```text
active user_memory: 30 条以内
core memory:        5 条以内
每轮注入:            5 条以内
单条长度:            120-160 字符以内
memory brief:        400 tokens 以内
```

`core memory` 是几乎每轮都应该注入的高优先级记忆，例如用户明确表达的长期偏好或反复纠正。

## conversation_state

Suna 当前只有一个当前会话。`conversation_state` 是这个会话的持久化恢复状态，不是完整历史库。

### 保存内容

```text
user_id              当前固定为默认用户
session_state        当前会话的内部状态账本，compact 后生成/更新
last_messages        TUI 恢复展示用的真实可见 user/assistant transcript
tool_summary         工具操作摘要，仅用于 TUI 恢复展示
memory_processed_at  最近一次 active memory 队列处理时间
updated_at           更新时间
```

`last_messages` 保存真实可见对话，用于 TUI 恢复时展示。它只保存 user/assistant 纯文本消息，会剥离 assistant tool_calls/raw 结构，不保存 system Session State，不保存原始 tool result。

`session_state` 给模型恢复和 compact 使用。它保存当前会话的高价值状态：当前任务、已完成任务/话题账本、用户要求、关键决策、工具事实和未完成事项。

`tool_summary` 只保存轻量工具摘要，例如“exec [success]: go test ./... 通过”。恢复时可以通过 TUI 展示给用户，但不作为原始 tool 上下文放回模型。

## Session State

Session State 是新记忆系统的核心。它不是用户可见摘要，而是给未来模型接力用的内部会话状态。

固定结构：

```markdown
# Session State

## Active context
当前正在做什么/聊什么，任务阶段，下一步。

## Completed work / topic ledger
本会话已完成任务、讨论过的话题、较早内容的可回忆索引。

## User requirements and decisions
用户明确要求、纠正、偏好、赞同/拒绝过的方案和已定决策。

## Tool facts
工具事实：读过什么、改过什么、跑过什么、失败过什么、验证结果是什么。

## Open threads
未完成、暂停、用户可能后续会继续的问题。

## Recovery note
未来恢复会话或 compact 后，agent 应如何接上。
```

设计要求：

- 当前任务不中断：coding/tool 任务要保留执行 checkpoint；纯对话要保留当前讨论焦点。
- 较早内容不消失：完成任务和旧话题至少保留模糊 ledger。
- 不 append-only：每次 compact 是 `旧 Session State + 新历史 -> 新 Session State`，不是不断追加摘要。
- bounded rewrite：Session State 有 token 预算，旧条目会合并为更短 ledger。
- 不进入 user_memory：Session State 是当前会话状态，不是长期用户画像。

## 恢复行为

### 没有 compact 过

```text
TUI:
  展示 last_messages 中的真实可见对话

模型:
  last_messages + active memory + latest user
```

此时没有 Session State，恢复行为接近原来的纯可见对话恢复。因为上下文还没触发 compact，通常说明会话规模仍可直接恢复。

### compact 过

```text
TUI:
  仍展示 last_messages 中的真实可见对话

模型:
  Session State + dynamic recent messages + active memory + latest user
```

用户视觉上看到历史还在；模型侧只加载设计好的高价值记忆，避免把完整 transcript 和大 tool 输出重新塞进上下文。

### 新建会话

`/new` 会清空 working memory、session_state、last_messages、tool_summary，并生成新的 session id。长期 `user_memory` 保留。

## Working Memory Compact

Working memory 是当前 agent 会话内发送给模型的短期上下文。完整 LLM 请求还包括 system prompt、tool schemas、active memory 注入和 max output reserve。

Compact 的目标：

- 降低上下文 token 占用。
- 保障当前任务或当前对话不中断。
- 通过 Session State 记录较早任务/话题/决策。
- 避免大 tool result 持续占用模型上下文。

### 自动 compact

自动 compact 在每次 LLM 请求发出前执行 preflight 检测：

```text
estimated_request_tokens =
  system prompt
  + Session State
  + working messages
  + active memory / internal context 注入
  + tool schemas
  + max output reserve

触发条件:
  estimated_request_tokens > context_window * 0.8
```

触发后流程：

```text
1. 通知 TUI: session.compact_result {running:true}。
2. 按真实请求预算计算 recent window 可用 token。
3. 用代码确定性选择 dynamic recent messages：
   - 普通对话目标保留最近多个 user turn。
   - tool-heavy 场景保留更少 user turn 和当前工具链附近上下文。
   - 至少保留最新 1 条。
   - 最多保留固定消息上限，且不能超过 token budget。
4. 调用压缩 LLM：旧 Session State + 待折叠历史 -> 新 Session State。
5. 更新独立 Session State，WorkingMemory = dynamic recent messages。
6. 重新构造完整请求并再次估算。
7. 成功则通知 TUI compact_done，模型自动继续。
8. 如果 compact 失败或 compact 后仍超限，通知 TUI compact_error 并停止本轮模型请求。
```

自动 compact 最多额外发起一次压缩 LLM 请求。失败时不使用 deterministic fallback，不硬裁剪继续。

### 手动 compact

手动 `/compact` 是用户显式整理当前上下文：

```text
旧 Session State + 当前 working messages
  -> 更新独立 Session State
  -> WorkingMemory = dynamic recent messages
```

手动 compact 不看 80% 自动阈值；只要用户执行就会尝试整理。若没有可折叠内容且没有已有 Session State，返回 noop，并在 TUI 说明暂无可压缩内容。若已有 Session State，即使新增消息较少，也会尝试把新上下文合并进 Session State。

### compact 失败策略

compact 是 Session State 正确性的边界。失败时必须显式失败：

- prompt loader 未配置：失败。
- compress prompt 渲染失败或为空：失败。
- 压缩 LLM 请求失败：失败。
- 压缩 LLM 返回空 Session State：失败。
- compact 后完整请求仍超过安全阈值：失败。

失败时不修改 working memory，不写坏 Session State，不继续主模型请求。TUI 清理 loading 状态、解锁输入并显示错误。

### 大 tool result 策略

TUI tool 事件仍展示原始结果；模型上下文只保存截断后的 tool result。这样能避免单个工具输出直接拉爆上下文。compact 时工具结果会进一步被压成 `Tool facts`，而不是保留原始日志。

## 缓存友好上下文结构

每次主模型请求尽量保持稳定前缀：

```text
System prompt / project instructions / skills / tool schemas  稳定
Session State                                                 compact 后稳定
Recent messages                                                普通轮次 append-only
Active Memory internal block                                   靠近 latest user
Latest user                                                    每轮变化
```

约束：

- Session State 不拼进 system prompt，避免 compact 后污染稳定 system 前缀。
- 不做每轮 rolling Session State update；只在自动 compact、手动 compact 或 restore 加载时变化。
- Active Memory 按 latest user query 召回，插在最新 user message 之前。
- Recent messages 普通轮次只追加，不每轮重排；compact 时才按预算裁剪。

## memory_queue 与 Active Memory 整理

`memory_queue` 是 active memory 的临时队列。主链路只负责按显著性写入，daemon 负责批量消费。

主链路：

```text
用户消息
  -> 写入 working memory
  -> 按显著性决定是否写 memory_queue
  -> 从 user_memory 召回 active memory brief
  -> 构建短上下文
  -> 调用主 LLM
  -> 保存 conversation_state
```

daemon 链路：

```text
读取未处理 memory_queue
读取当前 user_memory
调用 LLM 做 full compaction
得到新的 user_memory 列表
代码 diff 后更新数据库
删除已处理 queue
```

判断 significance 不依赖召回，也不调用 LLM。召回只影响主 LLM prompt。

## Full Compaction

Suna 不做复杂局部 patch，也不做 append-only。每次 daemon 处理队列时，把当前全部 active memory 和新事件一起交给 LLM：

```text
old_user_memory + new_queue_events -> new_user_memory
```

LLM 输出新的 active memory 列表，最多 30 条。系统代码再做 diff：

- 内容相同或语义延续：保留原 id，刷新字段。
- 内容变化：更新原 id。
- 新的重要信息：新增。
- 未返回的旧记忆：删除或标记 inactive。

## 召回策略

主链路召回不使用 LLM，也不使用 embedding。因为 active memory 数量很小，规则足够可靠。

召回发生在用户消息进入 working memory 后、主 LLM 请求前：

```text
用户输入
  -> working memory.Add(user)
  -> buildSystemPrompt()
  -> user_memory.BuildBrief(last_user_text)
  -> 作为 internal-context user message 注入到最新 user message 之前
  -> 调用主 LLM
```

每次最多从 30 条 active memory 中选 5 条。

排序必须稳定，避免 memory brief 无意义变化并影响 prompt cache。

## SQLite 设计

数据库使用默认数据目录下的 `memory.db`。

### user_memory

```text
id TEXT PRIMARY KEY
user_id TEXT NOT NULL
kind TEXT NOT NULL
content TEXT NOT NULL
tags TEXT NOT NULL DEFAULT '[]'
priority INTEGER NOT NULL DEFAULT 50
is_core INTEGER NOT NULL DEFAULT 0
use_count INTEGER NOT NULL DEFAULT 0
last_used_at DATETIME
refreshed_at DATETIME NOT NULL
expires_at DATETIME
created_at DATETIME NOT NULL
updated_at DATETIME NOT NULL
```

### conversation_state

```text
user_id TEXT PRIMARY KEY
session_state TEXT NOT NULL DEFAULT ''
last_messages TEXT NOT NULL DEFAULT '[]'
tool_summary TEXT NOT NULL DEFAULT '[]'
memory_processed_at DATETIME
updated_at DATETIME
```

`last_messages` 为 JSON，保存恢复展示用的可见 user/assistant 纯文本 transcript。

`session_state` 为文本，保存 compact 生成的当前会话状态。

`tool_summary` 为 JSON，只保存工具操作摘要，用于 TUI 恢复展示，不注入 LLM 上下文。

### memory_queue

```text
id TEXT PRIMARY KEY
user_id TEXT NOT NULL
role TEXT NOT NULL
content TEXT NOT NULL
significance TEXT
created_at DATETIME NOT NULL
processed_at DATETIME
attempts INTEGER NOT NULL DEFAULT 0
next_attempt_at DATETIME
last_error TEXT
```

当前实现成功处理后直接删除队列行；失败时通过 `attempts`、`next_attempt_at`、`last_error` 做退避重试。

## 日志

### llm.log

记录底层 LLM 请求：

```text
purpose=chat
purpose=compress
purpose=memory_compact
```

用于排查模型调用、耗时、token 和 cached tokens。

### memory.log

记录记忆系统业务事件：

```text
session_compact_start
session_compact_success
session_compact_failed
session_compact_noop
session_compact_still_oversized
compaction_start
compaction_success
compaction_failed
```

`session_compact_*` 用于排查 compact 为什么触发、压缩前后 token、folded messages、recent messages、truncated tool outputs 和失败原因。

## Subtask 关系

Subtask 保持独立上下文：

- 不继承 main conversation。
- 不继承 main working memory。
- 不继承 main Session State。
- 不继承 active memory，除非 main 显式放进 task/context。
- 子任务内部可 auto compact，但只影响子任务临时 working memory，不持久化、不污染主会话。

## 不做的事情

当前明确不做：

- embedding。
- 向量检索。
- 完整历史搜索。
- append-only 事实库。
- 多会话管理。
- workspace/project 记忆层。
- `/memory search` 精准历史回溯。
- subtask 独立长期记忆。
- 退出 TUI 时额外发起 LLM compact。
- 50%-60% 阈值的频繁 rolling Session State update。

## 成功标准

- 用户一直对话时，自动 compact 不打断当前任务。
- 用户问起较早完成的任务或讨论过的话题，Suna 能通过 Session State 模糊回忆。
- 退出后恢复上一轮，TUI 展示真实对话，模型能用 Session State + recent 接上。
- 新建会话后，临时上下文清空，但长期 user_memory 保留。
- compact 后 token 占用明显下降，且失败时明确报错、不伪压缩。
- prompt cache 命中率不会因为 active memory 或 Session State 设计大幅下降。

最终目标：用户只管和 Suna 对话；Suna 在后台用 active memory、Session State、dynamic recent window 和低频 compact 管理连续性与 token。