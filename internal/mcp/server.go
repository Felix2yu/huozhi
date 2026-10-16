package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"huozhi/internal/handlers"
	"sync"
	"time"
)

// ToolHandler 工具实现。uid 由鉴权中间件注入，工具内部无需再关心身份来源。
// 返回 (结果数据, error)：error 非空时转成 isError=true 的工具结果，
// 让错误文本进入模型上下文而非直接中断会话。
type ToolHandler func(ctx context.Context, uid uint, args map[string]any) (any, error)

// Server MCP 服务端：持有工具注册表与会话表。
type Server struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	handlers map[string]ToolHandler
	order    []string

	sessMu   sync.Mutex
	sessions map[string]*Session
}

// New 创建一个尚未注册任何工具的 MCP 服务端
func New() *Server {
	return &Server{
		tools:    make(map[string]Tool),
		handlers: make(map[string]ToolHandler),
		sessions: make(map[string]*Session),
	}
}

// Register 注册工具。同名重复注册直接报错，避免部署期静默覆盖。
func (s *Server) Register(t Tool, h ToolHandler) error {
	if t.Name == "" || h == nil {
		return errors.New("mcp: 工具名与处理函数均不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tools[t.Name]; ok {
		return fmt.Errorf("mcp: 工具 %q 重复注册", t.Name)
	}
	s.tools[t.Name] = t
	s.handlers[t.Name] = h
	s.order = append(s.order, t.Name)
	return nil
}

// Tools 按注册顺序返回工具清单
func (s *Server) Tools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tool, 0, len(s.order))
	for _, n := range s.order {
		out = append(out, s.tools[n])
	}
	return out
}

// ==================== 会话 ====================

// Session 一次 MCP 连接。SSE 流按需创建，没有客户端监听 GET 流时为空。
type Session struct {
	ID        string
	UID       uint
	CreatedAt time.Time
	LastSeen  time.Time

	mu      sync.Mutex
	nextID  uint64
	streams map[uint64]chan []byte
	closed  bool
}

func (s *Server) createSession(uid uint) *Session {
	id := randHex(16)
	sess := &Session{
		ID:        id,
		UID:       uid,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
		streams:   make(map[uint64]chan []byte),
	}
	s.sessMu.Lock()
	s.sessions[id] = sess
	s.sessMu.Unlock()
	return sess
}

func (s *Server) session(id string) *Session {
	if id == "" {
		return nil
	}
	s.sessMu.Lock()
	defer s.sessMu.Unlock()
	return s.sessions[id]
}

// TouchSession 刷新会话活跃时间；会话不存在则创建。
func (s *Server) TouchSession(id string, uid uint) *Session {
	s.sessMu.Lock()
	sess := s.sessions[id]
	s.sessMu.Unlock()
	if sess != nil {
		sess.LastSeen = time.Now()
		return sess
	}
	return s.createSession(uid)
}

// DropSession 结束会话（DELETE /mcp），关闭所有 SSE 流。
func (s *Server) DropSession(id string) bool {
	s.sessMu.Lock()
	sess := s.sessions[id]
	if sess != nil {
		delete(s.sessions, id)
	}
	s.sessMu.Unlock()
	if sess == nil {
		return false
	}
	sess.close()
	return true
}

// GCIdleSessions 回收空闲超过 maxIdle 的会话，避免长期运行下内存只增不减。
func (s *Server) GCIdleSessions(maxIdle time.Duration) int {
	cut := time.Now().Add(-maxIdle)
	s.sessMu.Lock()
	stale := make([]*Session, 0)
	for id, sess := range s.sessions {
		if sess.LastSeen.Before(cut) {
			delete(s.sessions, id)
			stale = append(stale, sess)
		}
	}
	s.sessMu.Unlock()
	for _, sess := range stale {
		sess.close()
	}
	return len(stale)
}

func (sess *Session) close() {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.closed {
		return
	}
	sess.closed = true
	for id, ch := range sess.streams {
		close(ch)
		delete(sess.streams, id)
	}
}

// addStream 注册一条 SSE 流，返回接收通道与注销函数。
func (sess *Session) addStream() (chan []byte, func()) {
	ch := make(chan []byte, 32)
	sess.mu.Lock()
	if sess.closed {
		sess.mu.Unlock()
		return nil, func() {}
	}
	sess.nextID++
	id := sess.nextID
	sess.streams[id] = ch
	sess.mu.Unlock()
	return ch, func() {
		sess.mu.Lock()
		if c, ok := sess.streams[id]; ok {
			delete(sess.streams, id)
			// 只关不重复关：close 已由会话关闭流程处理过的通道会 panic
			select {
			case <-c:
			default:
				close(c)
			}
		}
		sess.mu.Unlock()
	}
}

// Broadcast 向该会话所有已建立的 SSE 流推送消息（服务端主动通知用）。
func (sess *Session) Broadcast(payload []byte) {
	sess.mu.Lock()
	chs := make([]chan []byte, 0, len(sess.streams))
	for _, ch := range sess.streams {
		chs = append(chs, ch)
	}
	sess.mu.Unlock()
	for _, ch := range chs {
		select {
		case ch <- payload:
		default:
			// 客户端消费不过来就丢弃，绝不阻塞请求处理
		}
	}
}

// GetSession 暴露给传输层查询会话
func (s *Server) GetSession(id string) *Session { return s.session(id) }

// ==================== 请求分发 ====================

// HandleBytes 处理一帧请求体（单条或批量），返回需要回写给客户端的载荷。
//
// 返回值 hasResponse=false 表示这是通知（无 id），按规范应回 202 且不带响应体。
func (s *Server) HandleBytes(ctx context.Context, uid uint, raw []byte) (payload []byte, hasResponse bool, err error) {
	trimmed := trimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false, errors.New("空请求体")
	}
	if trimmed[0] == '[' {
		var reqs []Request
		if err := json.Unmarshal(trimmed, &reqs); err != nil {
			return nil, false, err
		}
		if len(reqs) == 0 {
			return nil, false, errors.New("空批量请求")
		}
		resps := make([]Response, 0, len(reqs))
		for i := range reqs {
			if resp, ok := s.dispatch(ctx, uid, &reqs[i]); ok {
				resps = append(resps, *resp)
			}
		}
		if len(resps) == 0 {
			return nil, false, nil
		}
		out, err := json.Marshal(resps)
		return out, true, err
	}

	var req Request
	if err := json.Unmarshal(trimmed, &req); err != nil {
		return nil, false, err
	}
	resp, ok := s.dispatch(ctx, uid, &req)
	if !ok {
		return nil, false, nil
	}
	out, err := json.Marshal(resp)
	return out, true, err
}

// dispatch 处理单条请求；ok=false 表示无需响应（通知）。
func (s *Server) dispatch(ctx context.Context, uid uint, req *Request) (*Response, bool) {
	if req.JSONRPC != "" && req.JSONRPC != jsonrpcVersion {
		return &Response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error:   &Error{Code: CodeInvalidRequest, Message: "jsonrpc 版本必须为 2.0"},
		}, true
	}
	if req.Method == "" {
		return &Response{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error:   &Error{Code: CodeInvalidRequest, Message: "method 不能为空"},
		}, true
	}

	// 通知：方法名以 notifications/ 开头，且不带 id → 不回响应
	isNotification := len(req.ID) == 0 || (len(req.ID) == 4 && string(req.ID) == "null")
	if isNotification && isNotificationMethod(req.Method) {
		return nil, false
	}

	result, rpcErr := s.call(ctx, uid, req)
	if rpcErr != nil {
		return &Response{JSONRPC: jsonrpcVersion, ID: req.ID, Error: rpcErr}, true
	}
	// 通知类方法即使带了 id 也不应有结果体，这里统一返回空结果
	return &Response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: result}, true
}

func isNotificationMethod(m string) bool {
	return len(m) > 15 && m[:15] == "notifications/" || m == "notifications/cancelled" || m == "notifications/initialized"
}

func (s *Server) call(ctx context.Context, uid uint, req *Request) (any, *Error) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string         `json:"protocolVersion"`
			Capabilities    map[string]any `json:"capabilities"`
			ClientInfo      Implementation `json:"clientInfo"`
		}
		if err := decodeParams(req.Params, &p); err != nil {
			return nil, &Error{Code: CodeInvalidParams, Message: err.Error()}
		}
		return InitializeResult{
			ProtocolVersion: NegotiateProtocolVersion(p.ProtocolVersion),
			Capabilities: ServerCapabilities{
				Tools:     &ToolsCapability{ListChanged: false},
				Resources: &ResourcesCapability{Subscribe: false, ListChanged: false},
				Prompts:   &PromptsCapability{ListChanged: false},
				Logging:   &LoggingCapability{},
			},
			ServerInfo:   Implementation{Name: ServerName, Version: ServerVersion, Title: ServerTitle},
			Instructions: serverInstructions,
		}, nil

	case "ping":
		return EmptyResult{}, nil

	case "tools/list":
		return ListToolsResult{Tools: s.Tools()}, nil

	case "tools/call":
		var p CallToolParams
		if err := decodeParams(req.Params, &p); err != nil {
			return nil, &Error{Code: CodeInvalidParams, Message: err.Error()}
		}
		return s.callTool(ctx, uid, p)

	case "resources/list":
		return struct {
			Resources []any `json:"resources"`
		}{Resources: []any{}}, nil

	case "resources/templates/list":
		return struct {
			Templates []any `json:"resourceTemplates"`
		}{Templates: []any{}}, nil

	case "prompts/list":
		return struct {
			Prompts []any `json:"prompts"`
		}{Prompts: []any{}}, nil

	case "logging/setLevel":
		return EmptyResult{}, nil

	default:
		return nil, &Error{Code: CodeMethodNotFound, Message: "不支持的方法: " + req.Method}
	}
}

func (s *Server) callTool(ctx context.Context, uid uint, p CallToolParams) (any, *Error) {
	s.mu.RLock()
	h, ok := s.handlers[p.Name]
	s.mu.RUnlock()
	if !ok {
		// 未知工具同样以工具错误返回（而不是 JSON-RPC 错误），
		// 这样模型能看到可用工具提示并自行纠正。
		return CallToolResult{
			Content: []Content{{Type: "text", Text: fmt.Sprintf("未知工具 %q。可用工具：%v", p.Name, s.toolNames())}},
			IsError: true,
		}, nil
	}
	data, err := h(ctx, uid, p.Arguments)
	if err != nil {
		return CallToolResult{
			Content: []Content{{Type: "text", Text: toolErrorText(err)}},
			IsError: true,
		}, nil
	}
	res := CallToolResult{
		Content:           []Content{{Type: "text", Text: mustJSON(data)}},
		StructuredContent: data,
	}
	return res, nil
}

func (s *Server) toolNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

func decodeParams(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func mustJSON(v any) string {
	if v == nil {
		return "{}"
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// toolErrorText 把工具执行错误渲染成给模型看的文本。
// 关键：错误信息要带「下一步怎么做」，而不是干巴巴一句失败——
// 模型只能依据这段文本决定重试策略，纯 "not found" 会让它原地打转。
func toolErrorText(err error) string {
	if err == nil {
		return ""
	}
	var ce *handlers.CoreError
	if errors.As(err, &ce) {
		switch ce.Status {
		case 404:
			return "未找到：" + ce.Message + "。请先用 search_transactions 定位账单并确认 id。"
		case 403:
			return "无权限：" + ce.Message
		default:
			return ce.Message
		}
	}
	return err.Error()
}

func trimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\n' || b[j-1] == '\r') {
		j--
	}
	return b[i:j]
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// 随机数不可用时的兜底：时间戳 + 计数器，保证在同一进程内不重复
		return fmt.Sprintf("%d%06d", time.Now().UnixNano(), nextFallbackID())
	}
	return hex.EncodeToString(b)
}

var fallbackMu sync.Mutex
var fallbackSeq int

func nextFallbackID() int {
	fallbackMu.Lock()
	defer fallbackMu.Unlock()
	fallbackSeq = (fallbackSeq + 1) % 1000000
	return fallbackSeq
}
