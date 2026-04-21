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
// Content is kept as RawMessage because OpenAI accepts both a string and an
// array of content blocks (e.g. [{"type":"text","text":"..."}]); callers use
// ExtractContentText to normalize it.
type OpenAIMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// OpenAIStreamOptions mirrors OpenAI's stream_options object. Currently only
// include_usage is honored.
type OpenAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// OpenAIChatRequest is the body accepted by POST /v1/chat/completions.
// Stop is kept as RawMessage because OpenAI accepts both a string and a []string.
type OpenAIChatRequest struct {
	Model         string               `json:"model"`
	Messages      []OpenAIMessage      `json:"messages"`
	MaxTokens     *float64             `json:"max_tokens,omitempty"`
	Temperature   *float64             `json:"temperature,omitempty"`
	TopP          *float64             `json:"top_p,omitempty"`
	Stop          json.RawMessage      `json:"stop,omitempty"`
	Stream        bool                 `json:"stream"`
	StreamOptions *OpenAIStreamOptions `json:"stream_options,omitempty"`
}

// ---------------------------------------------------------------------------
// Claude (Anthropic) types
// ---------------------------------------------------------------------------

// ClaudeMessage is a single message in a Claude messages request.
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeRequest is the body sent to Anthropic's /v1/messages endpoint.
type ClaudeRequest struct {
	Model         string          `json:"model"`
	Messages      []ClaudeMessage `json:"messages"`
	MaxTokens     int             `json:"max_tokens"`
	Stream        bool            `json:"stream"`
	System        string          `json:"system,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
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
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model      string      `json:"model"`
	StopReason string      `json:"stop_reason"`
	Usage      ClaudeUsage `json:"usage"`
}
