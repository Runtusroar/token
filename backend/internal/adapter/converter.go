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
//   - Messages with role "tool"/"function" (tool results) become Claude "user"
//     messages containing a tool_result content block.
//   - Assistant messages with tool_calls become Claude "assistant" messages
//     with text + tool_use content blocks.
//   - OpenAI tools[] → Claude tools[] (function.parameters → input_schema).
//   - tool_choice "auto"/"none"/"required"/{name:X} → Claude's equivalent.
//   - max_tokens defaults to 4096 when absent.
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

	validRoles := map[string]bool{
		"system": true, "user": true, "assistant": true,
		"tool": true, "function": true,
	}
	for _, msg := range oaiReq.Messages {
		if !validRoles[msg.Role] {
			return nil, "", fmt.Errorf("converter: invalid message role %q", msg.Role)
		}
	}

	model = oaiReq.Model

	var systemParts []string
	var claudeMessages []ClaudeMessage

	for i, msg := range oaiReq.Messages {
		if msg.Role == "system" {
			text, cErr := extractContentText(msg.Content)
			if cErr != nil {
				return nil, "", fmt.Errorf("converter: message %d: %w", i, cErr)
			}
			systemParts = append(systemParts, text)
			continue
		}
		cm, cErr := convertOpenAIMessage(msg)
		if cErr != nil {
			return nil, "", fmt.Errorf("converter: message %d: %w", i, cErr)
		}
		claudeMessages = append(claudeMessages, cm)
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
		Tools:         convertOpenAITools(oaiReq.Tools),
		ToolChoice:    convertOpenAIToolChoice(oaiReq.ToolChoice),
	}

	claudeBody, err = json.Marshal(claudeReq)
	if err != nil {
		return nil, "", fmt.Errorf("converter: marshal Claude request: %w", err)
	}
	return claudeBody, model, nil
}

// convertOpenAIMessage converts a non-system OpenAI message to its Claude
// equivalent. Role "tool"/"function" becomes a user message with a tool_result
// block; assistant messages with tool_calls become an assistant message with
// text + tool_use blocks.
func convertOpenAIMessage(msg OpenAIMessage) (ClaudeMessage, error) {
	switch msg.Role {
	case "user":
		text, err := extractContentText(msg.Content)
		if err != nil {
			return ClaudeMessage{}, err
		}
		payload, _ := json.Marshal(text)
		return ClaudeMessage{Role: "user", Content: payload}, nil

	case "assistant":
		text, _ := extractContentText(msg.Content) // ignore error; assistant may have null content
		var blocks []ClaudeContentBlock
		if text != "" {
			blocks = append(blocks, ClaudeContentBlock{Type: "text", Text: text})
		}
		for _, tc := range msg.ToolCalls {
			if tc.Type != "" && tc.Type != "function" {
				continue
			}
			// Claude expects input as an object, OpenAI gives a JSON string.
			input := json.RawMessage(tc.Function.Arguments)
			if len(input) == 0 || string(input) == "" {
				input = json.RawMessage("{}")
			}
			blocks = append(blocks, ClaudeContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
		if len(blocks) == 0 {
			// Empty assistant message — fall back to empty string content so
			// Claude doesn't reject the conversation turn.
			payload, _ := json.Marshal("")
			return ClaudeMessage{Role: "assistant", Content: payload}, nil
		}
		payload, err := json.Marshal(blocks)
		if err != nil {
			return ClaudeMessage{}, fmt.Errorf("marshal assistant blocks: %w", err)
		}
		return ClaudeMessage{Role: "assistant", Content: payload}, nil

	case "tool", "function":
		// Tool result — Anthropic models this as a user message with a
		// tool_result block. tool_call_id is required on OpenAI's side;
		// "function" legacy role uses "name" and lacks tool_call_id, in
		// which case we fall back to the function name.
		text, err := extractContentText(msg.Content)
		if err != nil {
			return ClaudeMessage{}, err
		}
		id := msg.ToolCallID
		if id == "" {
			id = msg.Name
		}
		if id == "" {
			return ClaudeMessage{}, fmt.Errorf("tool message missing tool_call_id")
		}
		block := ClaudeContentBlock{
			Type:              "tool_result",
			ToolUseID:         id,
			ToolResultContent: text,
		}
		payload, err := json.Marshal([]ClaudeContentBlock{block})
		if err != nil {
			return ClaudeMessage{}, fmt.Errorf("marshal tool_result: %w", err)
		}
		return ClaudeMessage{Role: "user", Content: payload}, nil
	}

	return ClaudeMessage{}, fmt.Errorf("unsupported role %q", msg.Role)
}

// convertOpenAITools maps the OpenAI tools[] array to Claude's tools[].
// Unknown tool types are skipped (Claude only understands function-style).
func convertOpenAITools(tools []OpenAITool) []ClaudeTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]ClaudeTool, 0, len(tools))
	for _, t := range tools {
		if t.Type != "" && t.Type != "function" {
			continue
		}
		if t.Function.Name == "" {
			continue
		}
		out = append(out, ClaudeTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// convertOpenAIToolChoice maps OpenAI's tool_choice (string or object) to
// Claude's shape. Returns nil to let Anthropic apply its default ("auto"
// when tools are present).
//
//	OpenAI                                   →  Claude
//	"auto"                                      {type:"auto"}
//	"required"                                  {type:"any"}
//	"none"                                      {type:"none"}
//	{"type":"function","function":{"name":X}}   {type:"tool","name":X}
func convertOpenAIToolChoice(raw json.RawMessage) *ClaudeToolChoice {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "auto":
			return &ClaudeToolChoice{Type: "auto"}
		case "required":
			return &ClaudeToolChoice{Type: "any"}
		case "none":
			return &ClaudeToolChoice{Type: "none"}
		}
		return nil
	}
	var obj struct {
		Type     string `json:"type"`
		Function *struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	if obj.Type == "function" && obj.Function != nil && obj.Function.Name != "" {
		return &ClaudeToolChoice{Type: "tool", Name: obj.Function.Name}
	}
	return nil
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

// openAIChoiceMessage is the "message" object inside a choice. Content is
// json.RawMessage because OpenAI wants `null` (not `""`) when the assistant
// only produced tool calls.
type openAIChoiceMessage struct {
	Role      string           `json:"role"`
	Content   json.RawMessage  `json:"content"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// openAIChoice is one choice inside an OpenAI chat completion response.
type openAIChoice struct {
	Index        int                 `json:"index"`
	Message      openAIChoiceMessage `json:"message"`
	FinishReason string              `json:"finish_reason"`
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

	// Separate text blocks (concatenated into content) from tool_use blocks
	// (lifted to tool_calls). "thinking" blocks are dropped silently for now.
	content := ""
	var toolCalls []OpenAIToolCall
	for _, block := range cr.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			// Claude's input is a JSON object; OpenAI wants a JSON string.
			args := string(block.Input)
			if args == "" {
				args = "{}"
			}
			toolCalls = append(toolCalls, OpenAIToolCall{
				ID:   block.ID,
				Type: "function",
				Function: OpenAIFunctionCall{
					Name:      block.Name,
					Arguments: args,
				},
			})
		}
	}

	// Per OpenAI spec: when the assistant only emits tool calls, content is
	// null; otherwise it's the concatenated text (possibly empty string).
	var contentRaw json.RawMessage
	if content == "" && len(toolCalls) > 0 {
		contentRaw = json.RawMessage("null")
	} else {
		b, _ := json.Marshal(content)
		contentRaw = b
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
				Message: openAIChoiceMessage{
					Role:      "assistant",
					Content:   contentRaw,
					ToolCalls: toolCalls,
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

// openAIChunkDelta is the delta object inside a streaming chunk. Only the
// populated fields are emitted (omitempty); an initial chunk carries Role,
// text chunks carry Content, tool-call chunks carry ToolCalls.
type openAIChunkDelta struct {
	Role      string                 `json:"role,omitempty"`
	Content   string                 `json:"content,omitempty"`
	ToolCalls []openAIToolCallDelta  `json:"tool_calls,omitempty"`
}

// openAIToolCallDelta is a single tool_call entry in a streaming delta.
// On the first chunk for a given index we set ID + Type + Function.Name
// (and Function.Arguments=""); subsequent chunks only update Function.Arguments.
type openAIToolCallDelta struct {
	Index    int                        `json:"index"`
	ID       string                     `json:"id,omitempty"`
	Type     string                     `json:"type,omitempty"`
	Function *openAIToolCallFunctionDelta `json:"function,omitempty"`
}

type openAIToolCallFunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openAIChunkChoice struct {
	Index        int              `json:"index"`
	Delta        openAIChunkDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
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
//
// toolCallIndex maps an Anthropic content_block index to an OpenAI tool_call
// array index so input_json_delta events emitted later can be attributed back
// to the right tool_call entry.
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
	toolCallIndex    map[int]int
	nextToolIdx      int
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
	// Anthropic event envelope. We look at `index` + `content_block` to
	// distinguish text vs tool_use blocks, and `delta.type` inside
	// content_block_delta to split text_delta from input_json_delta.
	// NOTE: we intentionally do NOT overwrite Claude's own message id onto
	// o.id — we want the id to stay "chatcmpl-*" for OpenAI-client parity.
	var event struct {
		Type         string `json:"type"`
		Index        *int   `json:"index,omitempty"`
		ContentBlock *struct {
			Type  string          `json:"type"`
			ID    string          `json:"id,omitempty"`
			Name  string          `json:"name,omitempty"`
			Input json.RawMessage `json:"input,omitempty"`
		} `json:"content_block,omitempty"`
		Message *struct {
			ID    string       `json:"id"`
			Model string       `json:"model"`
			Usage *ClaudeUsage `json:"usage"`
		} `json:"message"`
		Delta *struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
			StopReason  string `json:"stop_reason"`
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
		return [][]byte{o.buildChunk(openAIChunkDelta{Role: "assistant"}, nil)}

	case "content_block_start":
		// Only tool_use needs an OpenAI-side marker chunk. Text blocks don't
		// emit anything here — OpenAI clients expect text via delta.content.
		if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" && event.Index != nil {
			if o.toolCallIndex == nil {
				o.toolCallIndex = map[int]int{}
			}
			tcIdx := o.nextToolIdx
			o.nextToolIdx++
			o.toolCallIndex[*event.Index] = tcIdx
			return [][]byte{o.buildChunk(openAIChunkDelta{
				ToolCalls: []openAIToolCallDelta{{
					Index: tcIdx,
					ID:    event.ContentBlock.ID,
					Type:  "function",
					Function: &openAIToolCallFunctionDelta{
						Name:      event.ContentBlock.Name,
						Arguments: "",
					},
				}},
			}, nil)}
		}

	case "content_block_delta":
		if event.Delta == nil {
			return nil
		}
		switch event.Delta.Type {
		case "text_delta":
			return [][]byte{o.buildChunk(openAIChunkDelta{Content: event.Delta.Text}, nil)}
		case "input_json_delta":
			if event.Index == nil {
				return nil
			}
			tcIdx, ok := o.toolCallIndex[*event.Index]
			if !ok {
				return nil
			}
			return [][]byte{o.buildChunk(openAIChunkDelta{
				ToolCalls: []openAIToolCallDelta{{
					Index: tcIdx,
					Function: &openAIToolCallFunctionDelta{
						Arguments: event.Delta.PartialJSON,
					},
				}},
			}, nil)}
		}

	case "content_block_stop":
		// No OpenAI counterpart; framing is implicit in delta.finish_reason.
		return nil

	case "message_delta":
		if event.Usage != nil && event.Usage.OutputTokens > 0 {
			o.completionTokens = event.Usage.OutputTokens
		}
		if event.Delta != nil && event.Delta.StopReason != "" {
			reason := claudeStopReasonToOpenAI(event.Delta.StopReason)
			return [][]byte{o.buildChunk(openAIChunkDelta{}, &reason)}
		}

	case "message_stop":
		if o.includeUsage {
			return [][]byte{o.buildUsageChunk(), []byte("[DONE]")}
		}
		return [][]byte{[]byte("[DONE]")}
	}

	return nil
}

func (o *OpenAIStreamWriter) buildChunk(delta openAIChunkDelta, finishReason *string) []byte {
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
