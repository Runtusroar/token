package adapter

import (
	"encoding/json"
	"net/http/httptest"
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
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse output: %v", err)
	}

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

func TestOpenAIToClaudeResponse_TextOnly(t *testing.T) {
	in := `{
		"id":"chatcmpl-x","object":"chat.completion","created":1,"model":"gpt-5.4-nano",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hello there"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16}
	}`
	out, err := OpenAIToClaudeResponse([]byte(in), "gpt-5.4-nano")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got["type"] != "message" || got["role"] != "assistant" {
		t.Fatalf("envelope wrong: %v", got)
	}
	content := got["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content blocks: %d", len(content))
	}
	tb := content[0].(map[string]any)
	if tb["type"] != "text" || tb["text"] != "hello there" {
		t.Fatalf("text block wrong: %v", tb)
	}
	if got["stop_reason"] != "end_turn" {
		t.Fatalf("stop_reason=%v", got["stop_reason"])
	}
	usage := got["usage"].(map[string]any)
	if usage["input_tokens"].(float64) != 12 || usage["output_tokens"].(float64) != 4 {
		t.Fatalf("usage: %v", usage)
	}
}

func TestOpenAIToClaudeResponse_ToolCalls(t *testing.T) {
	in := `{
		"id":"chatcmpl-y","object":"chat.completion","created":1,"model":"gpt-5.4-nano",
		"choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[
			{"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}
		]},"finish_reason":"tool_calls"}],
		"usage":{"prompt_tokens":20,"completion_tokens":15,"total_tokens":35}
	}`
	out, err := OpenAIToClaudeResponse([]byte(in), "gpt-5.4-nano")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	content := got["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 tool_use block, got %d", len(content))
	}
	tu := content[0].(map[string]any)
	if tu["type"] != "tool_use" || tu["id"] != "call_abc" || tu["name"] != "get_weather" {
		t.Fatalf("tool_use wrong: %v", tu)
	}
	if got["stop_reason"] != "tool_use" {
		t.Fatalf("stop_reason=%v, want \"tool_use\"", got["stop_reason"])
	}
}

func TestClaudeToOpenAIRequest_ToolChoiceVariants(t *testing.T) {
	cases := []struct {
		claudeChoice string
		want         any // either a string or a map
		wantName     string
	}{
		{`{"type":"none"}`, "none", ""},
		{`{"type":"any"}`, "required", ""},
		{`{"type":"tool","name":"my_func"}`, nil, "my_func"},
	}

	for _, tc := range cases {
		body := []byte(`{
			"model":"x","max_tokens":1,
			"messages":[{"role":"user","content":"hi"}],
			"tool_choice":` + tc.claudeChoice + `
		}`)
		out, _, err := ClaudeToOpenAIRequest(body)
		if err != nil {
			t.Fatalf("[%s] err: %v", tc.claudeChoice, err)
		}
		var got map[string]any
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("[%s] parse output: %v", tc.claudeChoice, err)
		}
		actual := got["tool_choice"]

		if tc.wantName != "" {
			obj, ok := actual.(map[string]any)
			if !ok {
				t.Fatalf("[%s] tool_choice not object: %v", tc.claudeChoice, actual)
			}
			if obj["type"] != "function" {
				t.Errorf("[%s] tool_choice.type = %v", tc.claudeChoice, obj["type"])
			}
			fn, _ := obj["function"].(map[string]any)
			if fn["name"] != tc.wantName {
				t.Errorf("[%s] tool_choice.function.name = %v, want %s", tc.claudeChoice, fn["name"], tc.wantName)
			}
		} else if actual != tc.want {
			t.Errorf("[%s] tool_choice = %v, want %v", tc.claudeChoice, actual, tc.want)
		}
	}
}

// helper: feeds a sequence of OpenAI SSE lines (each "data: {...}" or "data: [DONE]")
// to a ClaudeStreamWriter and returns the captured client-side bytes.
func runStream(t *testing.T, lines []string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	sw := NewClaudeStreamWriter(rec, "gpt-5.4-nano")
	for _, l := range lines {
		if _, err := sw.Write([]byte(l + "\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
		// blank line between SSE events
		if _, err := sw.Write([]byte("\n")); err != nil {
			t.Fatalf("write blank: %v", err)
		}
	}
	return rec.Body.String()
}

func TestClaudeStreamWriter_SimpleText(t *testing.T) {
	out := runStream(t, []string{
		`data: {"id":"chatcmpl-1","model":"gpt-5.4-nano","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"hel"}}]}`,
		`data: {"choices":[{"index":0,"delta":{"content":"lo"}}]}`,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":2}}`,
		`data: [DONE]`,
	})

	checks := []string{
		`event: message_start`,
		`"type":"message_start"`,
		`event: content_block_start`,
		`"type":"text"`,
		`event: content_block_delta`,
		`"text":"hel"`,
		`"text":"lo"`,
		`event: content_block_stop`,
		`event: message_delta`,
		`"stop_reason":"end_turn"`,
		`"output_tokens":2`,
		`event: message_stop`,
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in stream output:\n%s", c, out)
		}
	}
}

func TestClaudeStreamWriter_ToolCall(t *testing.T) {
	out := runStream(t, []string{
		`data: {"id":"chatcmpl-1","model":"gpt-5.4-nano","choices":[{"index":0,"delta":{"role":"assistant"}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_x","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"SF\"}"}}]}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
		`data: [DONE]`,
	})

	checks := []string{
		`event: content_block_start`,
		`"type":"tool_use"`,
		`"id":"call_x"`,
		`"name":"get_weather"`,
		`event: content_block_delta`,
		`"type":"input_json_delta"`,
		`"partial_json":"{\"city\":"`,
		`"partial_json":"\"SF\"}"`,
		`event: content_block_stop`,
		`"stop_reason":"tool_use"`,
		`event: message_stop`,
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q:\n%s", c, out)
		}
	}
}
