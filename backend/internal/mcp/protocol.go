package mcp

import "encoding/json"

// 本文件定义 MCP（Model Context Protocol）使用的 JSON-RPC 2.0 消息结构。
//
// 参考规范：MCP Revision 2025-06-18（Streamable HTTP 传输）。
// 之所以不引入第三方 SDK：本项目是单进程自托管服务，MCP 端点与既有 HTTP API
// 共享同一个 gin 引擎、同一个鉴权中间件和同一份数据库连接；自建协议层可以
// 直接把 uid 透传给业务核心层，同时避免为一个几百行的协议引入外部依赖树。

// ==================== 协议版本 ====================

const (
	// ProtocolVersionLatest 服务端首选版本。客户端请求的版本若服务端不支持，
	// 服务端应回落到自己支持的最新版本（规范允许，且主流客户端都能接受）。
	ProtocolVersionLatest = "2025-06-18"
	protocolVersion0326   = "2025-03-26"
	protocolVersion1105   = "2024-11-05"
)

// SupportedProtocolVersions 服务端支持的协议版本（按优先级降序）
var SupportedProtocolVersions = []string{ProtocolVersionLatest, protocolVersion0326, protocolVersion1105}

// NegotiateProtocolVersion 版本协商：客户端版本在支持列表内则原样采纳，
// 否则回落到服务端最新版本（而不是报错，避免旧客户端直接连不上）。
func NegotiateProtocolVersion(client string) string {
	for _, v := range SupportedProtocolVersions {
		if v == client {
			return v
		}
	}
	return ProtocolVersionLatest
}

const (
	ServerName    = "huozhi"
	ServerTitle   = "货殖记账"
	ServerVersion = "1.0.0"
)

// ==================== JSON-RPC 2.0 ====================

const jsonrpcVersion = "2.0"

// Request JSON-RPC 请求
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response JSON-RPC 响应
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error JSON-RPC 错误对象
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// 标准 JSON-RPC 错误码
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// 业务错误码：沿用 HTTP 语义，便于调用方一眼看出性质。
// 取值与 JSON-RPC 保留区间（-32768..-32000）错开。
const (
	CodeBusinessBadRequest = 1400
	CodeBusinessForbidden  = 1403
	CodeBusinessNotFound   = 1404
	CodeBusinessInternal   = 1500
)

// ==================== 能力协商 ====================

// Implementation 实现方描述
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Title   string `json:"title,omitempty"`
}

// ServerCapabilities 服务端能力
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

type PromptsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type LoggingCapability struct{}

// InitializeResult initialize 的响应
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      Implementation     `json:"serverInfo"`
	// Instructions 会被客户端注入到模型上下文里，是让模型「会用工具」的关键。
	// 这里写清中文金额单位、日期表达习惯和写操作的确认要求。
	Instructions string `json:"instructions,omitempty"`
}

// ==================== 工具 ====================

// Tool 工具定义
type Tool struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema ToolInputSchema  `json:"inputSchema"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// ToolInputSchema JSON Schema（MCP 要求 type 固定为 object）
type ToolInputSchema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties,omitempty"`
	Required   []string       `json:"required,omitempty"`
	// AdditionalProperties 显式声明，避免某些客户端把未声明字段视为非法
	AdditionalProperties bool `json:"additionalProperties"`
}

// ToolAnnotations 行为提示（MCP 2025-03-26 引入）
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    bool   `json:"readOnlyHint,omitempty"`
	DestructiveHint bool   `json:"destructiveHint,omitempty"`
	IdempotentHint  bool   `json:"idempotentHint,omitempty"`
	OpenWorldHint   bool   `json:"openWorldHint,omitempty"`
}

// ListToolsResult tools/list 的响应
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams tools/call 的参数
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Content 工具返回内容块
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallToolResult tools/call 的响应。
//
// 注意：工具执行失败**不返回 JSON-RPC error**，而是返回 IsError=true 的结果，
// 这样错误文本会进入模型上下文，模型可以据此自我纠正参数重试（规范推荐做法）。
type CallToolResult struct {
	Content           []Content `json:"content"`
	IsError           bool      `json:"isError,omitempty"`
	StructuredContent any       `json:"structuredContent,omitempty"`
}

// EmptyResult 无内容结果（如 resources/list 未实现任何资源时）
type EmptyResult struct{}
