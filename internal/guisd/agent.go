package guisd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/alanchenchen/suna/internal/logging"
	"github.com/alanchenchen/suna/internal/transport/local"
)

// agentSubsBufSize 是每个 WebSocket 客户端的 notification 缓冲容量。
// 流式 agent.stream 场景下峰值可能短时间累积数百条；512 在普通家用网络下可缓冲 5-10s。
const agentSubsBufSize = 512

// agentBatchInterval 控制 goroutine A 批量 flush notification 的最大间隔。
// 16ms (≈60fps) 对用户来说是无感的，同时显著降低 writeJSON 调用次数。
const agentBatchInterval = 16 * time.Millisecond

// agentBatchMax 控制单次 flush 最多合并的消息数。超过则分多次 flush，防止 goroutine A 长时间独占。
const agentBatchMax = 64

// AgentProxy 是 guisd 与 daemon 之间的桥接层。
// 它持有一条到 daemon 的 local transport 连接，将浏览器的 WebSocket 请求转发给 daemon，
// 并将 daemon 的 notification 广播给所有已连接的 WebSocket 客户端。
type AgentProxy struct {
	client *local.Client
	mu     sync.RWMutex
	subs   map[chan notifyMsg]struct{}
}

type notifyMsg struct {
	method string
	params json.RawMessage
}

// NewAgentProxy 连接 daemon 并创建代理。如果 daemon 不可达返回错误。
func NewAgentProxy() (*AgentProxy, error) {
	client, err := local.DialDefault(3 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect daemon: %w", err)
	}
	p := &AgentProxy{client: client, subs: make(map[chan notifyMsg]struct{})}
	client.OnNotify(p.broadcast)
	logging.Info("guisd", "agent_proxy_connected", nil)
	return p, nil
}

// Connected 返回 daemon 连接是否正常。
func (p *AgentProxy) Connected() bool {
	if p == nil || p.client == nil {
		return false
	}
	return p.client.Connected()
}

// broadcast 将 daemon notification 非阻塞地分发给所有订阅者。
// 慢消费者（缓冲已满）会被跳过：单客户端卡顿不应阻塞其他客户端或 daemon 回调。
func (p *AgentProxy) broadcast(method string, params json.RawMessage) {
	msg := notifyMsg{method: method, params: params}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for ch := range p.subs {
		select {
		case ch <- msg:
		default:
			// 慢消费者跳过：单客户端断连 / 卡顿不应阻塞 daemon 通知链。
		}
	}
}

func (p *AgentProxy) subscribe() chan notifyMsg {
	ch := make(chan notifyMsg, agentSubsBufSize)
	p.mu.Lock()
	p.subs[ch] = struct{}{}
	p.mu.Unlock()
	return ch
}

func (p *AgentProxy) unsubscribe(ch chan notifyMsg) {
	p.mu.Lock()
	if _, ok := p.subs[ch]; ok {
		delete(p.subs, ch)
		close(ch)
	}
	p.mu.Unlock()
}

// HandleWebSocket 处理一个 WebSocket 连接：双向转发浏览器与 daemon 之间的 JSON-RPC 消息。
func (p *AgentProxy) HandleWebSocket(conn *websocket.Conn) {
	ch := p.subscribe()
	defer p.unsubscribe(ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// goroutine A：daemon notification → WebSocket，使用 16ms 批量 flush。
	// 相比每条 notification 一次 WriteJSON，批量 flush 在流式响应时把 N 次系统调用合并为 1 次。
	go func() {
		defer cancel()
		batch := make([]notifyMsg, 0, agentBatchMax)
		ticker := time.NewTicker(agentBatchInterval)
		defer ticker.Stop()

		flush := func() {
			if len(batch) == 0 {
				return
			}
			for _, m := range batch {
				payload := map[string]any{
					"jsonrpc": "2.0",
					"method":  m.method,
				}
				if len(m.params) > 0 && string(m.params) != "null" {
					payload["params"] = json.RawMessage(m.params)
				}
				if err := conn.WriteJSON(payload); err != nil {
					return
				}
			}
			batch = batch[:0]
		}

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					flush()
					return
				}
				batch = append(batch, msg)
				if len(batch) >= agentBatchMax {
					flush()
				}
			case <-ticker.C:
				flush()
			}
		}
	}()

	// 主循环：WebSocket → daemon
	for {
		var req struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := conn.ReadJSON(&req); err != nil {
			return
		}

		var params any
		if len(req.Params) > 0 {
			params = req.Params
		}

		result, err := p.client.InvokeRaw(ctx, req.Method, params)
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
		}
		if err != nil {
			resp["error"] = map[string]any{
				"code":    -32603,
				"message": err.Error(),
			}
		} else if len(result) > 0 {
			resp["result"] = json.RawMessage(result)
		} else {
			resp["result"] = nil
		}
		if err := conn.WriteJSON(resp); err != nil {
			return
		}
	}
}

// Close 关闭与 daemon 的连接。
func (p *AgentProxy) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	return p.client.Close()
}
