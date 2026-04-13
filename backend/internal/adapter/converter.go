package adapter

import (
	"encoding/json"
	"fmt"
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
