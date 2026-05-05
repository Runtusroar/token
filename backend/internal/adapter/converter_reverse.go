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
