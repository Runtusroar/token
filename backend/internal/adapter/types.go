package adapter

import "net/http"

// ProxyResult holds token usage and model info returned by a proxied request.
type ProxyResult struct {
	StatusCode       int
	PromptTokens     int
	CompletionTokens int
	Model            string
}

// Adapter is the interface that every upstream provider adapter must implement.
type Adapter interface {
	ProxyRequest(w http.ResponseWriter, body []byte, model, apiKey, baseURL string, stream bool) (*ProxyResult, error)
}

// ---------------------------------------------------------------------------
// OpenAI types
// ---------------------------------------------------------------------------

// OpenAIMessage is a single message in an OpenAI chat request.
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatRequest is the body accepted by POST /v1/chat/completions.
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   *float64        `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream"`
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
	Model     string          `json:"model"`
	Messages  []ClaudeMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
	Stream    bool            `json:"stream"`
	System    string          `json:"system,omitempty"`
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
	Model string `json:"model"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}
