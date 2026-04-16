package adapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAIToClaude converts an OpenAI chat completions request body into the
// equivalent Anthropic Claude /v1/messages body.
//
// Conversion rules:
//   - Messages with role "system" are extracted and joined into the top-level
//     Claude "system" field (Claude does not accept system role inside messages).
//   - max_tokens defaults to 4096 when absent.
//   - The model name is taken from the OpenAI request and returned separately
//     so the caller can choose which upstream channel to use.
func OpenAIToClaude(openaiBody []byte) (claudeBody []byte, model string, err error) {
	var oaiReq OpenAIChatRequest
	if err = json.Unmarshal(openaiBody, &oaiReq); err != nil {
		return nil, "", fmt.Errorf("converter: parse OpenAI request: %w", err)
	}

	if oaiReq.Model == "" {
		return nil, "", fmt.Errorf("converter: model field is required")
	}
	if len(oaiReq.Messages) == 0 {
		return nil, "", fmt.Errorf("converter: messages must not be empty")
	}
	if len(oaiReq.Messages) > 500 {
		return nil, "", fmt.Errorf("converter: too many messages (max 500)")
	}

	validRoles := map[string]bool{"system": true, "user": true, "assistant": true}
	for _, msg := range oaiReq.Messages {
		if !validRoles[msg.Role] {
			return nil, "", fmt.Errorf("converter: invalid message role %q", msg.Role)
		}
	}

	model = oaiReq.Model

	var systemParts []string
	var claudeMessages []ClaudeMessage

	for _, msg := range oaiReq.Messages {
		if msg.Role == "system" {
			systemParts = append(systemParts, msg.Content)
		} else {
			claudeMessages = append(claudeMessages, ClaudeMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	maxTokens := 4096
	if oaiReq.MaxTokens != nil {
		maxTokens = int(*oaiReq.MaxTokens)
	}

	system := ""
	for i, part := range systemParts {
		if i > 0 {
			system += "\n"
		}
		system += part
	}

	claudeReq := ClaudeRequest{
		Model:     oaiReq.Model,
		Messages:  claudeMessages,
		MaxTokens: maxTokens,
		Stream:    oaiReq.Stream,
		System:    system,
	}

	claudeBody, err = json.Marshal(claudeReq)
	if err != nil {
		return nil, "", fmt.Errorf("converter: marshal Claude request: %w", err)
	}
	return claudeBody, model, nil
}

// openAIChoice is one choice inside an OpenAI chat completion response.
type openAIChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// openAIUsage mirrors the token usage object in an OpenAI response.
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openAIChatResponse is the full OpenAI chat.completion envelope.
type openAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

// ClaudeToOpenAIResponse wraps a raw Claude /v1/messages response body in the
// standard OpenAI chat.completion JSON envelope.
func ClaudeToOpenAIResponse(claudeBody []byte, model string) ([]byte, error) {
	var cr ClaudeResponse
	if err := json.Unmarshal(claudeBody, &cr); err != nil {
		return nil, fmt.Errorf("converter: parse Claude response: %w", err)
	}

	// Collect all text blocks into a single content string.
	content := ""
	for _, block := range cr.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	usedModel := model
	if cr.Model != "" {
		usedModel = cr.Model
	}

	oaiResp := openAIChatResponse{
		ID:      cr.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   usedModel,
		Choices: []openAIChoice{
			{
				Index: 0,
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: openAIUsage{
			PromptTokens:     cr.Usage.InputTokens,
			CompletionTokens: cr.Usage.OutputTokens,
			TotalTokens:      cr.Usage.InputTokens + cr.Usage.OutputTokens,
		},
	}

	out, err := json.Marshal(oaiResp)
	if err != nil {
		return nil, fmt.Errorf("converter: marshal OpenAI response: %w", err)
	}
	return out, nil
}

// ── OpenAI Streaming Converter ────────────────────────────────────────────

// openAIChunk is the JSON structure for an OpenAI streaming chunk.
type openAIChunk struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []openAIChunkChoice `json:"choices"`
}

type openAIChunkChoice struct {
	Index        int               `json:"index"`
	Delta        map[string]string `json:"delta"`
	FinishReason *string           `json:"finish_reason"`
}

// OpenAIStreamWriter wraps an http.ResponseWriter to convert Claude SSE
// events to OpenAI-compatible streaming chunk format on the fly.
type OpenAIStreamWriter struct {
	w          http.ResponseWriter
	model      string
	id         string
	created    int64
	statusCode int
	wroteData  bool
}

// NewOpenAIStreamWriter creates a streaming converter that transforms Claude
// SSE events into OpenAI chat.completion.chunk format.
func NewOpenAIStreamWriter(w http.ResponseWriter, model string) *OpenAIStreamWriter {
	return &OpenAIStreamWriter{
		w:       w,
		model:   model,
		id:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		created: time.Now().Unix(),
	}
}

func (o *OpenAIStreamWriter) Header() http.Header { return o.w.Header() }

func (o *OpenAIStreamWriter) WriteHeader(statusCode int) {
	o.statusCode = statusCode
	o.w.WriteHeader(statusCode)
}

func (o *OpenAIStreamWriter) Write(p []byte) (int, error) {
	// For error responses, pass through raw bytes.
	if o.statusCode != 0 && o.statusCode != http.StatusOK {
		return o.w.Write(p)
	}

	line := strings.TrimRight(string(p), "\r\n")

	// Skip event type lines (OpenAI streaming doesn't use them).
	if strings.HasPrefix(line, "event:") {
		return len(p), nil
	}

	// Blank lines: only emit if we wrote a data line (event boundary).
	if line == "" {
		if o.wroteData {
			o.wroteData = false
			return o.w.Write(p)
		}
		return len(p), nil
	}

	if !strings.HasPrefix(line, "data:") {
		return o.w.Write(p)
	}

	payload := strings.TrimSpace(line[5:])
	converted := o.convertEvent(payload)
	if converted == nil {
		return len(p), nil
	}

	o.wroteData = true
	_, err := fmt.Fprintf(o.w, "data: %s\n", converted)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Flush implements http.Flusher so the adapter can detect and use it.
func (o *OpenAIStreamWriter) Flush() {
	if f, ok := o.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (o *OpenAIStreamWriter) convertEvent(payload string) []byte {
	var event struct {
		Type    string `json:"type"`
		Message *struct {
			ID    string `json:"id"`
			Model string `json:"model"`
		} `json:"message"`
		Delta *struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return nil
	}

	switch event.Type {
	case "message_start":
		if event.Message != nil {
			if event.Message.ID != "" {
				o.id = event.Message.ID
			}
			if event.Message.Model != "" {
				o.model = event.Message.Model
			}
		}
		return o.buildChunk(map[string]string{"role": "assistant"}, nil)

	case "content_block_delta":
		if event.Delta != nil && event.Delta.Type == "text_delta" {
			return o.buildChunk(map[string]string{"content": event.Delta.Text}, nil)
		}

	case "message_delta":
		if event.Delta != nil && event.Delta.StopReason != "" {
			stop := "stop"
			return o.buildChunk(map[string]string{}, &stop)
		}

	case "message_stop":
		return []byte("[DONE]")
	}

	return nil
}

func (o *OpenAIStreamWriter) buildChunk(delta map[string]string, finishReason *string) []byte {
	chunk := openAIChunk{
		ID:      o.id,
		Object:  "chat.completion.chunk",
		Created: o.created,
		Model:   o.model,
		Choices: []openAIChunkChoice{{
			Index:        0,
			Delta:        delta,
			FinishReason: finishReason,
		}},
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return nil
	}
	return data
}
