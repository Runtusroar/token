package adapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ClaudeToOpenAIRequest converts a Claude /v1/messages request body into the
// equivalent OpenAI chat/completions body. Tool-call IDs are passed through
// unchanged (Claude `toolu_*` IDs become OpenAI `tool_call_id`s, and Claude
// `tool_use_id` on a tool_result becomes OpenAI `tool_call_id` on a role=tool
// message).
func ClaudeToOpenAIRequest(claudeBody []byte) (openaiBody []byte, model string, err error) {
	var cr struct {
		Model         string            `json:"model"`
		System        string            `json:"system,omitempty"`
		Messages      []ClaudeMessage   `json:"messages"`
		MaxTokens     int               `json:"max_tokens"`
		Stream        bool              `json:"stream,omitempty"`
		Temperature   *float64          `json:"temperature,omitempty"`
		TopP          *float64          `json:"top_p,omitempty"`
		StopSequences []string          `json:"stop_sequences,omitempty"`
		Tools         []ClaudeTool      `json:"tools,omitempty"`
		ToolChoice    *ClaudeToolChoice `json:"tool_choice,omitempty"`
	}
	if err = json.Unmarshal(claudeBody, &cr); err != nil {
		return nil, "", fmt.Errorf("converter_reverse: parse Claude request: %w", err)
	}
	if cr.Model == "" {
		return nil, "", fmt.Errorf("converter_reverse: model field is required")
	}
	if len(cr.Messages) == 0 {
		return nil, "", fmt.Errorf("converter_reverse: messages must not be empty")
	}

	model = cr.Model

	var msgs []OpenAIMessage

	if cr.System != "" {
		raw, _ := json.Marshal(cr.System)
		msgs = append(msgs, OpenAIMessage{Role: "system", Content: raw})
	}

	for i, m := range cr.Messages {
		converted, perr := claudeMessageToOpenAI(m)
		if perr != nil {
			return nil, "", fmt.Errorf("converter_reverse: message %d: %w", i, perr)
		}
		msgs = append(msgs, converted...)
	}

	out := map[string]any{
		"model":    cr.Model,
		"messages": msgs,
	}
	if cr.MaxTokens > 0 {
		out["max_tokens"] = cr.MaxTokens
	}
	if cr.Stream {
		out["stream"] = true
	}
	if cr.Temperature != nil {
		out["temperature"] = *cr.Temperature
	}
	if cr.TopP != nil {
		out["top_p"] = *cr.TopP
	}
	if len(cr.StopSequences) > 0 {
		out["stop"] = cr.StopSequences
	}
	if len(cr.Tools) > 0 {
		oaiTools := make([]map[string]any, 0, len(cr.Tools))
		for _, t := range cr.Tools {
			params := t.InputSchema
			if len(params) == 0 {
				params = json.RawMessage(`{}`)
			}
			oaiTools = append(oaiTools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  params,
				},
			})
		}
		out["tools"] = oaiTools
	}
	if cr.ToolChoice != nil {
		out["tool_choice"] = claudeToolChoiceToOpenAI(cr.ToolChoice)
	}

	openaiBody, err = json.Marshal(out)
	if err != nil {
		return nil, "", fmt.Errorf("converter_reverse: marshal: %w", err)
	}
	return openaiBody, model, nil
}

// claudeMessageToOpenAI converts a single Claude message into one or more
// OpenAI messages. role=assistant with tool_use blocks → assistant message
// with tool_calls; role=user with tool_result blocks → one role=tool message
// per tool_result (OpenAI requires each tool result as its own message).
func claudeMessageToOpenAI(m ClaudeMessage) ([]OpenAIMessage, error) {
	// Try parsing content as plain string first.
	var asString string
	if err := json.Unmarshal(m.Content, &asString); err == nil {
		raw, _ := json.Marshal(asString)
		return []OpenAIMessage{{Role: m.Role, Content: raw}}, nil
	}

	// Otherwise parse as content blocks.
	var blocks []ClaudeContentBlock
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return nil, fmt.Errorf("content not string or block array: %w", err)
	}

	switch m.Role {
	case "assistant":
		var text string
		var calls []OpenAIToolCall
		for _, b := range blocks {
			switch b.Type {
			case "text":
				text += b.Text
			case "tool_use":
				args := string(b.Input)
				if args == "" {
					args = "{}"
				}
				calls = append(calls, OpenAIToolCall{
					ID:   b.ID,
					Type: "function",
					Function: OpenAIFunctionCall{
						Name:      b.Name,
						Arguments: args,
					},
				})
			}
		}
		var content json.RawMessage
		if text == "" && len(calls) > 0 {
			content = json.RawMessage("null")
		} else {
			content, _ = json.Marshal(text)
		}
		return []OpenAIMessage{{Role: "assistant", Content: content, ToolCalls: calls}}, nil

	case "user":
		var out []OpenAIMessage
		var leftoverText string
		for _, b := range blocks {
			switch b.Type {
			case "tool_result":
				raw, _ := json.Marshal(b.ToolResultContent)
				out = append(out, OpenAIMessage{
					Role:       "tool",
					ToolCallID: b.ToolUseID,
					Content:    raw,
				})
			case "text":
				leftoverText += b.Text
			}
		}
		if leftoverText != "" {
			raw, _ := json.Marshal(leftoverText)
			out = append([]OpenAIMessage{{Role: "user", Content: raw}}, out...)
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unexpected role %q with block content", m.Role)
	}
}

// claudeToolChoiceToOpenAI maps Claude's tool_choice object to OpenAI's
// equivalent. Claude: {type:"auto"|"any"|"none"|"tool", name?:"X"}.
// OpenAI accepts either a string ("auto"|"none"|"required") or an object
// {"type":"function","function":{"name":"X"}}.
func claudeToolChoiceToOpenAI(tc *ClaudeToolChoice) any {
	switch tc.Type {
	case "auto":
		return "auto"
	case "none":
		return "none"
	case "any":
		return "required"
	case "tool":
		return map[string]any{
			"type":     "function",
			"function": map[string]any{"name": tc.Name},
		}
	}
	return "auto"
}

// OpenAIToClaudeResponse converts an OpenAI chat.completion non-streaming
// response into a Claude /v1/messages response shape. The model param is the
// fallback when the OpenAI body's model field is empty.
func OpenAIToClaudeResponse(openaiBody []byte, model string) ([]byte, error) {
	var oai struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role      string           `json:"role"`
				Content   json.RawMessage  `json:"content"`
				ToolCalls []OpenAIToolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens        int `json:"prompt_tokens"`
			CompletionTokens    int `json:"completion_tokens"`
			PromptTokensDetails *struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(openaiBody, &oai); err != nil {
		return nil, fmt.Errorf("converter_reverse: parse OpenAI response: %w", err)
	}
	if len(oai.Choices) == 0 {
		return nil, fmt.Errorf("converter_reverse: response has no choices")
	}
	choice := oai.Choices[0]

	var blocks []map[string]any
	var asText string
	if err := json.Unmarshal(choice.Message.Content, &asText); err == nil && asText != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": asText})
	}
	for _, tc := range choice.Message.ToolCalls {
		var input json.RawMessage
		if tc.Function.Arguments != "" {
			input = json.RawMessage(tc.Function.Arguments)
		} else {
			input = json.RawMessage("{}")
		}
		blocks = append(blocks, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Function.Name,
			"input": input,
		})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, map[string]any{"type": "text", "text": ""})
	}

	resolvedModel := oai.Model
	if resolvedModel == "" {
		resolvedModel = model
	}

	cached := 0
	if oai.Usage.PromptTokensDetails != nil {
		cached = oai.Usage.PromptTokensDetails.CachedTokens
	}
	usage := map[string]any{
		"input_tokens":                oai.Usage.PromptTokens - cached,
		"output_tokens":               oai.Usage.CompletionTokens,
		"cache_read_input_tokens":     cached,
		"cache_creation_input_tokens": 0,
	}

	id := oai.ID
	if id == "" {
		id = "msg_unknown"
	}
	resp := map[string]any{
		"id":            id,
		"type":          "message",
		"role":          "assistant",
		"content":       blocks,
		"model":         resolvedModel,
		"stop_reason":   openaiFinishReasonToClaude(choice.FinishReason),
		"stop_sequence": nil,
		"usage":         usage,
	}
	return json.Marshal(resp)
}

// openaiFinishReasonToClaude is the inverse of claudeStopReasonToOpenAI.
func openaiFinishReasonToClaude(r string) string {
	switch r {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "stop_sequence"
	default:
		return "end_turn"
	}
}

// ClaudeStreamWriter wraps an http.ResponseWriter and converts OpenAI SSE
// streaming chunks into Anthropic's Claude SSE event protocol on the fly.
//
// It is the reverse of OpenAIStreamWriter. The state machine maintains the
// concept of a "currently open content block" (text or tool_use) so that
// content_block_start / content_block_stop frames can be synthesized from
// the flat OpenAI delta stream.
type ClaudeStreamWriter struct {
	w         http.ResponseWriter
	model     string
	messageID string

	headerSent bool
	statusCode int
	startSent  bool
	finished   bool

	currentIdx  int    // -1 means no block is currently open
	currentKind string // "text" or "tool"

	toolIdxMap   map[int]int // OpenAI tool_calls.index → Claude content_block index
	nextBlockIdx int

	stopReason  string // captured from finish_reason
	stopPending bool   // finish_reason received but message_delta not yet emitted
	stopEmitted bool   // emitted message_delta with stop_reason yet?

	inputTokens  int
	outputTokens int
	cachedTokens int

	// Buffer holds bytes between newlines so partial writes are concatenated
	// before parsing. Each "data: ..." chunk is processed when its line ends.
	buf strings.Builder
}

func NewClaudeStreamWriter(w http.ResponseWriter, model string) *ClaudeStreamWriter {
	return &ClaudeStreamWriter{
		w:          w,
		model:      model,
		messageID:  fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		currentIdx: -1,
	}
}

func (s *ClaudeStreamWriter) Header() http.Header { return s.w.Header() }

func (s *ClaudeStreamWriter) WriteHeader(statusCode int) {
	s.headerSent = true
	s.statusCode = statusCode
	if statusCode != http.StatusOK {
		// Non-OK passthrough: do NOT apply SSE headers; let upstream's
		// content-type and body flow to the client unchanged.
		s.w.WriteHeader(statusCode)
		return
	}
	s.w.Header().Set("Content-Type", "text/event-stream")
	s.w.Header().Set("Cache-Control", "no-cache")
	s.w.Header().Set("X-Accel-Buffering", "no")
	s.w.WriteHeader(statusCode)
}

func (s *ClaudeStreamWriter) Flush() {
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
}

// Write consumes bytes from the upstream OpenAI SSE stream and emits Claude
// SSE events to the wrapped writer. It is line-oriented; lines arrive as
// "data: {...}\n", blank "\n" between events, or "data: [DONE]\n".
func (s *ClaudeStreamWriter) Write(p []byte) (int, error) {
	if !s.headerSent {
		s.WriteHeader(http.StatusOK)
	}
	if s.statusCode != 0 && s.statusCode != http.StatusOK {
		// Error passthrough: forward bytes verbatim, no SSE conversion.
		return s.w.Write(p)
	}
	s.buf.Write(p)
	all := s.buf.String()
	for {
		nl := strings.IndexByte(all, '\n')
		if nl < 0 {
			break
		}
		line := strings.TrimRight(all[:nl], "\r")
		all = all[nl+1:]
		s.handleLine(line)
	}
	s.buf.Reset()
	s.buf.WriteString(all)
	return len(p), nil
}

func (s *ClaudeStreamWriter) handleLine(line string) {
	if line == "" || !strings.HasPrefix(line, "data:") {
		return
	}
	payload := strings.TrimSpace(line[5:])
	if payload == "[DONE]" {
		s.finalize()
		return
	}
	s.handleChunk([]byte(payload))
}

type openaiStreamDelta struct {
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	ToolCalls []struct {
		Index    int    `json:"index"`
		ID       string `json:"id,omitempty"`
		Type     string `json:"type,omitempty"`
		Function struct {
			Name      string `json:"name,omitempty"`
			Arguments string `json:"arguments,omitempty"`
		} `json:"function,omitempty"`
	} `json:"tool_calls,omitempty"`
}

type openaiStreamChunk struct {
	ID      string `json:"id,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Delta        openaiStreamDelta `json:"delta"`
		FinishReason *string           `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage,omitempty"`
}

func (s *ClaudeStreamWriter) handleChunk(payload []byte) {
	var chunk openaiStreamChunk
	if err := json.Unmarshal(payload, &chunk); err != nil {
		return
	}

	if chunk.Model != "" {
		s.model = chunk.Model
	}

	if !s.startSent {
		s.emitMessageStart()
	}

	if chunk.Usage != nil {
		s.recordUsage(chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens, func() int {
			if chunk.Usage.PromptTokensDetails != nil {
				return chunk.Usage.PromptTokensDetails.CachedTokens
			}
			return 0
		}())
	}

	for _, ch := range chunk.Choices {
		if ch.Delta.Content != "" {
			s.handleTextDelta(ch.Delta.Content)
		}
		for _, tc := range ch.Delta.ToolCalls {
			s.handleToolCallDelta(tc.Index, tc.ID, tc.Function.Name, tc.Function.Arguments)
		}
		if ch.FinishReason != nil && *ch.FinishReason != "" {
			s.stopReason = openaiFinishReasonToClaude(*ch.FinishReason)
			s.stopPending = true
			s.closeCurrentBlock()
			// Don't emit message_delta yet — wait for trailing usage chunk
			// or [DONE]. This avoids emitting an off-spec dual event.
		}
	}
}

func (s *ClaudeStreamWriter) recordUsage(prompt, completion, cached int) {
	if prompt > 0 {
		s.inputTokens = prompt - cached
		s.cachedTokens = cached
	}
	if completion > 0 {
		s.outputTokens = completion
	}
	// If the stop_reason was already captured in a prior chunk, emit the
	// deferred message_delta now that usage is available.
	if s.stopPending && !s.stopEmitted {
		s.emitMessageDelta()
	}
}

func (s *ClaudeStreamWriter) emitMessageStart() {
	s.startSent = true
	s.writeEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            s.messageID,
			"type":          "message",
			"role":          "assistant",
			"model":         s.model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":                0,
				"output_tokens":               0,
				"cache_read_input_tokens":     0,
				"cache_creation_input_tokens": 0,
			},
		},
	})
}

func (s *ClaudeStreamWriter) handleTextDelta(text string) {
	if s.currentKind != "text" {
		s.closeCurrentBlock()
		s.openTextBlock()
	}
	s.writeEvent("content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": s.currentIdx,
		"delta": map[string]any{"type": "text_delta", "text": text},
	})
}

func (s *ClaudeStreamWriter) handleToolCallDelta(oaiIdx int, id, name, args string) {
	claudeIdx, opened := s.toolIdxMap[oaiIdx]
	if !opened {
		// First sighting of this tool index: close any open block and open
		// a new tool_use block. The first chunk from OpenAI carries id+name.
		s.closeCurrentBlock()
		if s.toolIdxMap == nil {
			s.toolIdxMap = map[int]int{}
		}
		claudeIdx = s.nextBlockIdx
		s.nextBlockIdx++
		s.toolIdxMap[oaiIdx] = claudeIdx
		s.currentIdx = claudeIdx
		s.currentKind = "tool"
		s.writeEvent("content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": claudeIdx,
			"content_block": map[string]any{
				"type":  "tool_use",
				"id":    id,
				"name":  name,
				"input": map[string]any{},
			},
		})
	}
	if args != "" {
		s.writeEvent("content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": claudeIdx,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": args},
		})
	}
}

func (s *ClaudeStreamWriter) openTextBlock() {
	s.currentIdx = s.nextBlockIdx
	s.nextBlockIdx++
	s.currentKind = "text"
	s.writeEvent("content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         s.currentIdx,
		"content_block": map[string]any{"type": "text", "text": ""},
	})
}

func (s *ClaudeStreamWriter) closeCurrentBlock() {
	if s.currentIdx < 0 {
		return
	}
	s.writeEvent("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": s.currentIdx,
	})
	s.currentIdx = -1
	s.currentKind = ""
}

func (s *ClaudeStreamWriter) emitMessageDelta() {
	if s.stopEmitted {
		return
	}
	s.stopEmitted = true
	stop := s.stopReason
	if stop == "" {
		stop = "end_turn"
	}
	usage := map[string]any{"output_tokens": s.outputTokens}
	s.writeEvent("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stop, "stop_sequence": nil},
		"usage": usage,
	})
}

func (s *ClaudeStreamWriter) finalize() {
	if !s.startSent {
		s.emitMessageStart()
	}
	s.closeCurrentBlock()
	if !s.stopEmitted {
		s.emitMessageDelta()
	}
	if !s.finished {
		s.finished = true
		s.writeEvent("message_stop", map[string]any{"type": "message_stop"})
	}
}

func (s *ClaudeStreamWriter) writeEvent(name string, data map[string]any) {
	body, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(s.w, "event: %s\n", name)
	fmt.Fprintf(s.w, "data: %s\n\n", body)
	s.Flush()
}
