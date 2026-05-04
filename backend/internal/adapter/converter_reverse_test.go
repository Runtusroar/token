package adapter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClaudeToOpenAIRequest_SimpleText(t *testing.T) {
	in := `{
		"model":"gpt-5.4-nano",
		"max_tokens":100,
		"system":"You are helpful.",
		"messages":[{"role":"user","content":"hi"}]
	}`
	out, model, err := ClaudeToOpenAIRequest([]byte(in))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if model != "gpt-5.4-nano" {
		t.Fatalf("model=%q", model)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	msgs, ok := got["messages"].([]any)
	if !ok || len(msgs) != 2 {
		t.Fatalf("messages len=%d, want 2 (system + user)", len(msgs))
	}
	first := msgs[0].(map[string]any)
	if first["role"] != "system" || first["content"] != "You are helpful." {
		t.Fatalf("system msg wrong: %v", first)
	}
	if got["max_tokens"].(float64) != 100 {
		t.Fatalf("max_tokens missing: %v", got["max_tokens"])
	}
}

func TestClaudeToOpenAIRequest_ToolUseRoundTrip(t *testing.T) {
	in := `{
		"model":"gpt-5.4-nano",
		"max_tokens":50,
		"messages":[
			{"role":"user","content":"weather?"},
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"city":"SF"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":"72F"}
			]}
		],
		"tools":[{"name":"get_weather","description":"x","input_schema":{"type":"object"}}],
		"tool_choice":{"type":"auto"}
	}`
	out, _, err := ClaudeToOpenAIRequest([]byte(in))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(out, &got)

	msgs := got["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d: %s", len(msgs), out)
	}
	asst := msgs[1].(map[string]any)
	if asst["role"] != "assistant" {
		t.Fatalf("msg[1] role=%v", asst["role"])
	}
	tc := asst["tool_calls"].([]any)
	if len(tc) != 1 {
		t.Fatalf("tool_calls len=%d", len(tc))
	}
	call := tc[0].(map[string]any)
	if call["id"] != "toolu_1" {
		t.Fatalf("tool_call id should pass through, got %v", call["id"])
	}
	fn := call["function"].(map[string]any)
	if fn["name"] != "get_weather" {
		t.Fatalf("fn name=%v", fn["name"])
	}
	args := fn["arguments"].(string)
	if !strings.Contains(args, `"city"`) || !strings.Contains(args, `"SF"`) {
		t.Fatalf("arguments should be JSON-stringified input: %q", args)
	}

	tool := msgs[2].(map[string]any)
	if tool["role"] != "tool" || tool["tool_call_id"] != "toolu_1" || tool["content"] != "72F" {
		t.Fatalf("tool result msg wrong: %v", tool)
	}

	tools := got["tools"].([]any)
	tspec := tools[0].(map[string]any)
	if tspec["type"] != "function" {
		t.Fatalf("tool type=%v", tspec["type"])
	}
	tfn := tspec["function"].(map[string]any)
	if tfn["name"] != "get_weather" {
		t.Fatalf("tool fn name=%v", tfn["name"])
	}
	if _, has := tfn["parameters"]; !has {
		t.Fatalf("tools[].function.parameters missing (must come from input_schema)")
	}
	if got["tool_choice"] != "auto" {
		t.Fatalf("tool_choice=%v, want \"auto\"", got["tool_choice"])
	}
}
