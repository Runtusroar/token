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

	for i, msg := range oaiReq.Messages {
		text, cErr := extractContentText(msg.Content)
		if cErr != nil {
			return nil, "", fmt.Errorf("converter: message %d: %w", i, cErr)
		}
		if msg.Role == "system" {
			systemParts = append(systemParts, text)
		} else {
			claudeMessages = append(claudeMessages, ClaudeMessage{
				Role:    msg.Role,
				Content: text,
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
		Model:         oaiReq.Model,
		Messages:      claudeMessages,
		MaxTokens:     maxTokens,
		Stream:        oaiReq.Stream,
		System:        system,
		Temperature:   oaiReq.Temperature,
		TopP:          oaiReq.TopP,
		StopSequences: parseStopSequences(oaiReq.Stop),
	}

	claudeBody, err = json.Marshal(claudeReq)
	if err != nil {
		return nil, "", fmt.Errorf("converter: marshal Claude request: %w", err)
	}
	return claudeBody, model, nil
}

// parseStopSequences normalizes OpenAI's "stop" field (which accepts either
// a single string or an array of strings) into Anthropic's stop_sequences
// ([]string). Returns nil when absent/null so the field is omitted in the
// outgoing JSON.
func parseStopSequences(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if single == "" {
			return nil
		}
		return []string{single}
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

// claudeStopReasonToOpenAI maps Anthropic's stop_reason values to OpenAI's
// finish_reason vocabulary. Unknown values default to "stop" since most
// clients only branch on "length" (hit max_tokens) and "tool_calls".
func claudeStopReasonToOpenAI(reason string) string {
	switch reason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		// "end_turn", "stop_sequence", "" → "stop"
		return "stop"
	}
}

// extractContentText normalizes an OpenAI message "content" field (which may
// be a string or an array of content blocks like
// [{"type":"text","text":"..."}]) into a single plain text string.
//
// Non-text blocks (e.g. image_url) are rejected with a clear error — this
// relay currently targets text-only upstreams.
func extractContentText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	// Fast path: plain string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}

	// Slow path: OpenAI multi-part array. Each block has a "type" discriminator.
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "", fmt.Errorf("content must be string or array of content blocks")
	}

	var sb strings.Builder
	for i, b := range blocks {
		switch b.Type {
		case "text", "input_text":
			sb.WriteString(b.Text)
		case "":
			return "", fmt.Errorf("content block %d missing 'type'", i)
		default:
			// image_url / input_image / audio / etc. — not yet supported.
			return "", fmt.Errorf("content block %d has unsupported type %q", i, b.Type)
		}
	}
	return sb.String(), nil
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
// PromptTokensDetails exposes cached-token counts (OpenAI added this in 2024
// to match what providers like Anthropic already report).
type openAIUsage struct {
	PromptTokens        int                      `json:"prompt_tokens"`
	CompletionTokens    int                      `json:"completion_tokens"`
	TotalTokens         int                      `json:"total_tokens"`
	PromptTokensDetails *openAIPromptTokenDetail `json:"prompt_tokens_details,omitempty"`
}

type openAIPromptTokenDetail struct {
	CachedTokens int `json:"cached_tokens"`
}

// claudeUsageToOpenAI converts Anthropic's usage object to OpenAI's shape.
// For prompt_tokens we follow OpenAI semantics: count every billed input
// token once — that is, fresh input + cache reads + cache creation.
// cached_tokens surfaces the portion that hit the cache so clients can
// reconcile billing.
func claudeUsageToOpenAI(u ClaudeUsage) openAIUsage {
	prompt := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
	ou := openAIUsage{
		PromptTokens:     prompt,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      prompt + u.OutputTokens,
	}
	if u.CacheReadInputTokens > 0 {
		ou.PromptTokensDetails = &openAIPromptTokenDetail{CachedTokens: u.CacheReadInputTokens}
	}
	return ou
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
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
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
				FinishReason: claudeStopReasonToOpenAI(cr.StopReason),
			},
		},
		Usage: claudeUsageToOpenAI(cr.Usage),
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

// openAIUsageChunk is the final streaming chunk emitted when the client opts
// in via stream_options.include_usage. Choices is always empty per OpenAI's
// protocol.
type openAIUsageChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []openAIChunkChoice `json:"choices"`
	Usage   openAIUsage         `json:"usage"`
}

// IncludeUsageRequested reports whether the OpenAI request asked for usage
// to be emitted in a trailing stream chunk via stream_options.include_usage.
func IncludeUsageRequested(openaiBody []byte) bool {
	var probe struct {
		StreamOptions *OpenAIStreamOptions `json:"stream_options"`
	}
	if err := json.Unmarshal(openaiBody, &probe); err != nil {
		return false
	}
	return probe.StreamOptions != nil && probe.StreamOptions.IncludeUsage
}

// OpenAIStreamWriter wraps an http.ResponseWriter to convert Claude SSE
// events to OpenAI-compatible streaming chunk format on the fly.
type OpenAIStreamWriter struct {
	w                http.ResponseWriter
	model            string
	id               string
	created          int64
	statusCode       int
	wroteData        bool
	includeUsage     bool
	promptTokens     int
	completionTokens int
	cacheReadTokens  int
}

// NewOpenAIStreamWriter creates a streaming converter that transforms Claude
// SSE events into OpenAI chat.completion.chunk format. When includeUsage is
// true, a final chunk carrying token counts is emitted before [DONE], matching
// OpenAI's stream_options.include_usage behavior.
func NewOpenAIStreamWriter(w http.ResponseWriter, model string, includeUsage bool) *OpenAIStreamWriter {
	return &OpenAIStreamWriter{
		w:            w,
		model:        model,
		id:           fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		created:      time.Now().Unix(),
		includeUsage: includeUsage,
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
	chunks := o.convertEvent(payload)
	if len(chunks) == 0 {
		return len(p), nil
	}

	// When a single upstream event produces multiple OpenAI chunks (e.g. the
	// optional usage chunk before [DONE]), emit the earlier ones as complete
	// events ("data: X\n\n") and leave the trailing newline of the last chunk
	// to the upstream's own blank line — matching the existing single-chunk
	// behavior so event framing stays consistent.
	for i, c := range chunks {
		if i < len(chunks)-1 {
			if _, err := fmt.Fprintf(o.w, "data: %s\n\n", c); err != nil {
				return 0, err
			}
		} else {
			if _, err := fmt.Fprintf(o.w, "data: %s\n", c); err != nil {
				return 0, err
			}
			o.wroteData = true
		}
	}
	return len(p), nil
}

// Flush implements http.Flusher so the adapter can detect and use it.
func (o *OpenAIStreamWriter) Flush() {
	if f, ok := o.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (o *OpenAIStreamWriter) convertEvent(payload string) [][]byte {
	// Usage appears inside message.usage on message_start and at the top level
	// on message_delta; cache-token counts only surface on message_start.
	// NOTE: we intentionally do NOT overwrite Claude's own message id onto
	// o.id — we want the id to stay "chatcmpl-*" for OpenAI-client parity.
	var event struct {
		Type    string `json:"type"`
		Message *struct {
			ID    string       `json:"id"`
			Model string       `json:"model"`
			Usage *ClaudeUsage `json:"usage"`
		} `json:"message"`
		Delta *struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		Usage *ClaudeUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return nil
	}

	switch event.Type {
	case "message_start":
		if event.Message != nil {
			if event.Message.Model != "" {
				o.model = event.Message.Model
			}
			if event.Message.Usage != nil {
				u := event.Message.Usage
				// prompt_tokens = fresh input + cache read + cache create, to
				// match OpenAI semantics (every billed input token counted once).
				o.promptTokens = u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
				o.cacheReadTokens = u.CacheReadInputTokens
				if u.OutputTokens > 0 {
					o.completionTokens = u.OutputTokens
				}
			}
		}
		return [][]byte{o.buildChunk(map[string]string{"role": "assistant"}, nil)}

	case "content_block_delta":
		if event.Delta != nil && event.Delta.Type == "text_delta" {
			return [][]byte{o.buildChunk(map[string]string{"content": event.Delta.Text}, nil)}
		}

	case "message_delta":
		if event.Usage != nil && event.Usage.OutputTokens > 0 {
			o.completionTokens = event.Usage.OutputTokens
		}
		if event.Delta != nil && event.Delta.StopReason != "" {
			reason := claudeStopReasonToOpenAI(event.Delta.StopReason)
			return [][]byte{o.buildChunk(map[string]string{}, &reason)}
		}

	case "message_stop":
		if o.includeUsage {
			return [][]byte{o.buildUsageChunk(), []byte("[DONE]")}
		}
		return [][]byte{[]byte("[DONE]")}
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

func (o *OpenAIStreamWriter) buildUsageChunk() []byte {
	usage := openAIUsage{
		PromptTokens:     o.promptTokens,
		CompletionTokens: o.completionTokens,
		TotalTokens:      o.promptTokens + o.completionTokens,
	}
	if o.cacheReadTokens > 0 {
		usage.PromptTokensDetails = &openAIPromptTokenDetail{CachedTokens: o.cacheReadTokens}
	}
	chunk := openAIUsageChunk{
		ID:      o.id,
		Object:  "chat.completion.chunk",
		Created: o.created,
		Model:   o.model,
		Choices: []openAIChunkChoice{},
		Usage:   usage,
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return nil
	}
	return data
}
