package adapter

import (
	"encoding/json"
	"fmt"
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
