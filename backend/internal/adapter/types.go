package adapter

import (
	"context"
	"encoding/json"
	"net/http"
)

// ProxyResult holds token usage and model info returned by a proxied request.
// UpstreamError carries a sampled upstream response body when the upstream
// returned 4xx/5xx (or a transport-level error string). Truncated to a few KB
// by the adapter; truncate further before persisting.
type ProxyResult struct {
	StatusCode       int
	PromptTokens     int
	CompletionTokens int
	Model            string
	UpstreamError    string
}

// Adapter is the interface that every upstream provider adapter must implement.
type Adapter interface {
	ProxyRequest(ctx context.Context, w http.ResponseWriter, body []byte, model, apiKey, baseURL string, stream bool, clientHeaders http.Header) (*ProxyResult, error)
}

// ---------------------------------------------------------------------------
// OpenAI types
// ---------------------------------------------------------------------------

// OpenAIMessage is a single message in an OpenAI chat request.
//
// Content is kept as RawMessage because OpenAI accepts both a string and an
// array of content blocks (e.g. [{"type":"text","text":"..."}]); callers use
// extractContentText to normalize it. For assistant messages that only carry
// tool calls, Content may be null and ToolCalls populated instead. For tool
// result messages, Role is "tool" (modern) or "function" (legacy) and
// ToolCallID points at the prior assistant tool_call being answered.
type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// OpenAIToolCall is a single function-call emitted by the assistant.
// Arguments is a JSON-encoded string (per OpenAI spec, not a nested object).
type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // always "function" today
	Function OpenAIFunctionCall `json:"function"`
}

type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OpenAITool describes a function the model is allowed to call.
type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIFunctionSpec `json:"function"`
}

// OpenAIFunctionSpec mirrors the "function" object in tools[]. Parameters is
// the raw JSON Schema; forwarded to Anthropic unchanged as input_schema.
type OpenAIFunctionSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// OpenAIStreamOptions mirrors OpenAI's stream_options object. Currently only
// include_usage is honored.
type OpenAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// OpenAIChatRequest is the body accepted by POST /v1/chat/completions.
// Stop and ToolChoice are RawMessage because both accept multiple shapes
// (string/array for Stop; "auto"/"none"/"required" string or object for
// ToolChoice).
type OpenAIChatRequest struct {
	Model         string               `json:"model"`
	Messages      []OpenAIMessage      `json:"messages"`
	MaxTokens     *float64             `json:"max_tokens,omitempty"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	Stop          json.RawMessage      `json:"stop,omitempty"`
	Stream        bool                 `json:"stream"`
	StreamOptions *OpenAIStreamOptions `json:"stream_options,omitempty"`
	Tools         []OpenAITool         `json:"tools,omitempty"`
	ToolChoice    json.RawMessage      `json:"tool_choice,omitempty"`
}

// ---------------------------------------------------------------------------
// Claude (Anthropic) types
// ---------------------------------------------------------------------------

// ClaudeMessage is a single message in a Claude messages request.
// Content is RawMessage so it can carry either a string (simple text) or an
// array of content blocks (tool_use, tool_result, multi-part text, images).
type ClaudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ClaudeContentBlock is a single block inside a Claude message content array
// (assistant response) or the payload we build for tool_use / tool_result.
// Only the fields relevant to Type are populated.
type ClaudeContentBlock struct {
	Type string `json:"type"`

	// type=text
	Text string `json:"text,omitempty"`

	// type=tool_use (as emitted by Claude)
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// type=tool_result (as sent back to Claude for tool responses).
	// Content here is a plain string; Claude also accepts an array of sub-blocks
	// but our upstream flow always produces a string.
	ToolUseID string `json:"tool_use_id,omitempty"`
	// Content is the tool result text (Go field shares name with the outer
	// struct's Text path via JSON; we use a separate tag to avoid clash).
	ToolResultContent string `json:"content,omitempty"`
}

// ClaudeTool describes a function the model may call. Name is unique within
// the request; InputSchema is a JSON Schema object.
type ClaudeTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ClaudeToolChoice controls tool selection. Type is "auto" | "any" | "tool" | "none".
// Name is required when Type=="tool".
type ClaudeToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// ClaudeRequest is the body sent to Anthropic's /v1/messages endpoint.
type ClaudeRequest struct {
	Model         string            `json:"model"`
	Messages      []ClaudeMessage   `json:"messages"`
	MaxTokens     int               `json:"max_tokens"`
	Stream        bool              `json:"stream"`
	System        string            `json:"system,omitempty"`
	Temperature   *float64          `json:"temperature,omitempty"`
	TopP          *float64          `json:"top_p,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
	Tools         []ClaudeTool      `json:"tools,omitempty"`
	ToolChoice    *ClaudeToolChoice `json:"tool_choice,omitempty"`
}

// ClaudeUsage mirrors Anthropic's usage object, including prompt-caching fields.
type ClaudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// ClaudeResponse is the non-streaming response from Anthropic's /v1/messages.
type ClaudeResponse struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	Role       string               `json:"role"`
	Content    []ClaudeContentBlock `json:"content"`
	Model      string               `json:"model"`
	StopReason string               `json:"stop_reason"`
	Usage      ClaudeUsage          `json:"usage"`
}
