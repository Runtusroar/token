package adapter

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOpenAIToClaude_Tools exercises the core tool-calling translation paths:
// tools[] → tools[], tool_choice variants, assistant.tool_calls → tool_use
// blocks, and role=tool → user message with tool_result block.
func TestOpenAIToClaude_Tools(t *testing.T) {
	req := `{
		"model": "claude-opus-4-7",
		"messages": [
			{"role": "user", "content": "what's the weather in SF?"},
			{"role": "assistant", "content": null, "tool_calls": [
				{"id": "call_1", "type": "function",
				 "function": {"name": "get_weather", "arguments": "{\"city\":\"SF\"}"}}
			]},
			{"role": "tool", "tool_call_id": "call_1", "content": "72F sunny"}
		],
		"tools": [{
			"type": "function",
			"function": {
				"name": "get_weather",
				"description": "Get the weather",
				"parameters": {"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}
			}
		}],
		"tool_choice": "auto"
	}`

	out, model, err := OpenAIToClaude([]byte(req))
	if err != nil {
		t.Fatalf("OpenAIToClaude failed: %v", err)
	}
	if model != "claude-opus-4-7" {
		t.Fatalf("model = %q, want claude-opus-4-7", model)
	}

	// Parse the Claude-format output and verify the key fields.
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse output: %v — output=%s", err, out)
	}

	// tools[] survived
	tools, ok := got["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools missing/wrong: %v", got["tools"])
	}
	tool := tools[0].(map[string]any)
	if tool["name"] != "get_weather" {
		t.Errorf("tool name = %v, want get_weather", tool["name"])
	}
	if _, ok := tool["input_schema"]; !ok {
		t.Errorf("tool missing input_schema")
	}

	// tool_choice
	tc := got["tool_choice"].(map[string]any)
	if tc["type"] != "auto" {
		t.Errorf("tool_choice.type = %v, want auto", tc["type"])
	}

	// messages: user text, assistant tool_use, user tool_result
	msgs := got["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages len = %d, want 3 — %s", len(msgs), out)
	}

	// msg[0]: user text
	m0 := msgs[0].(map[string]any)
	if m0["role"] != "user" {
		t.Errorf("msg0 role = %v", m0["role"])
	}

	// msg[1]: assistant with tool_use block
	m1 := msgs[1].(map[string]any)
	if m1["role"] != "assistant" {
		t.Errorf("msg1 role = %v", m1["role"])
	}
	m1c, ok := m1["content"].([]any)
	if !ok || len(m1c) != 1 {
		t.Fatalf("msg1 content not an array of 1: %v", m1["content"])
	}
	block := m1c[0].(map[string]any)
	if block["type"] != "tool_use" {
		t.Errorf("msg1 block type = %v, want tool_use", block["type"])
	}
	if block["id"] != "call_1" {
		t.Errorf("msg1 tool_use id = %v", block["id"])
	}
	if block["name"] != "get_weather" {
		t.Errorf("msg1 tool_use name = %v", block["name"])
	}
	// input must be a parsed object (the arguments JSON string → object)
	input, ok := block["input"].(map[string]any)
	if !ok {
		t.Fatalf("msg1 tool_use input not object: %v", block["input"])
	}
	if input["city"] != "SF" {
		t.Errorf("msg1 tool_use input.city = %v, want SF", input["city"])
	}

	// msg[2]: user (converted from role=tool) with tool_result block
	m2 := msgs[2].(map[string]any)
	if m2["role"] != "user" {
		t.Errorf("msg2 role = %v, want user (converted from role=tool)", m2["role"])
	}
	m2c := m2["content"].([]any)
	if len(m2c) != 1 {
		t.Fatalf("msg2 content len = %d", len(m2c))
	}
	tr := m2c[0].(map[string]any)
	if tr["type"] != "tool_result" {
		t.Errorf("msg2 block type = %v, want tool_result", tr["type"])
	}
	if tr["tool_use_id"] != "call_1" {
		t.Errorf("msg2 tool_use_id = %v", tr["tool_use_id"])
	}
	if tr["content"] != "72F sunny" {
		t.Errorf("msg2 content = %v, want 72F sunny", tr["content"])
	}
}

func TestConvertOpenAIToolChoice(t *testing.T) {
	tests := []struct {
		in   string
		want ClaudeToolChoice
	}{
		{`"auto"`, ClaudeToolChoice{Type: "auto"}},
		{`"required"`, ClaudeToolChoice{Type: "any"}},
		{`"none"`, ClaudeToolChoice{Type: "none"}},
		{`{"type":"function","function":{"name":"myfn"}}`, ClaudeToolChoice{Type: "tool", Name: "myfn"}},
	}
	for _, tc := range tests {
		got := convertOpenAIToolChoice(json.RawMessage(tc.in))
		if got == nil {
			t.Errorf("in=%s: got nil, want %+v", tc.in, tc.want)
			continue
		}
		if *got != tc.want {
			t.Errorf("in=%s: got %+v, want %+v", tc.in, *got, tc.want)
		}
	}

	if got := convertOpenAIToolChoice(nil); got != nil {
		t.Errorf("nil input: got %+v, want nil", got)
	}
	if got := convertOpenAIToolChoice(json.RawMessage(`"gibberish"`)); got != nil {
		t.Errorf("unknown string: got %+v, want nil", got)
	}
}

// TestClaudeToOpenAIResponse_Tools verifies that tool_use content blocks in a
// Claude response are lifted into OpenAI-style tool_calls with content=null.
func TestClaudeToOpenAIResponse_Tools(t *testing.T) {
	claudeResp := `{
		"id": "msg_x",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "tool_use", "id": "toolu_y", "name": "get_weather", "input": {"city": "SF"}}
		],
		"model": "claude-opus-4-7",
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 42, "output_tokens": 7}
	}`

	out, err := ClaudeToOpenAIResponse([]byte(claudeResp), "claude-opus-4-7")
	if err != nil {
		t.Fatalf("ClaudeToOpenAIResponse failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("parse output: %v — %s", err, out)
	}

	choices := got["choices"].([]any)
	choice := choices[0].(map[string]any)
	if choice["finish_reason"] != "tool_calls" {
		t.Errorf("finish_reason = %v, want tool_calls", choice["finish_reason"])
	}
	msg := choice["message"].(map[string]any)
	if msg["content"] != nil {
		t.Errorf("content = %v, want null", msg["content"])
	}
	tcs := msg["tool_calls"].([]any)
	if len(tcs) != 1 {
		t.Fatalf("tool_calls len = %d", len(tcs))
	}
	tc := tcs[0].(map[string]any)
	if tc["id"] != "toolu_y" {
		t.Errorf("tool_call id = %v", tc["id"])
	}
	fn := tc["function"].(map[string]any)
	if fn["name"] != "get_weather" {
		t.Errorf("tool_call function name = %v", fn["name"])
	}
	// arguments must be the JSON-encoded object string, not the object itself.
	argStr, ok := fn["arguments"].(string)
	if !ok {
		t.Fatalf("arguments not a string: %v", fn["arguments"])
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argStr), &args); err != nil {
		t.Fatalf("arguments not valid JSON: %q — %v", argStr, err)
	}
	if args["city"] != "SF" {
		t.Errorf("arguments.city = %v", args["city"])
	}
}

// TestOpenAIStreamWriter_ToolUse verifies that a tool_use content block
// streamed from upstream results in an OpenAI-shaped tool_calls delta
// sequence on the client side.
func TestOpenAIStreamWriter_ToolUse(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewOpenAIStreamWriter(rec, "claude-opus-4-7", false)

	// Simulate the sequence of Anthropic SSE data: lines.
	sseLines := []string{
		`data: {"type":"message_start","message":{"id":"msg_x","model":"claude-opus-4-7","usage":{"input_tokens":3,"output_tokens":1}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_z","name":"get_weather"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"SF\"}"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}
	for _, line := range sseLines {
		if _, err := w.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("write line %q: %v", line, err)
		}
	}

	body := rec.Body.String()

	// Collect every "data: {...}" chunk (excluding the final "data: [DONE]").
	var chunks []map[string]any
	for _, raw := range strings.Split(body, "\n") {
		raw = strings.TrimSpace(raw)
		if !strings.HasPrefix(raw, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(raw, "data: ")
		if payload == "[DONE]" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(payload), &m); err != nil {
			continue // event: lines, etc.
		}
		chunks = append(chunks, m)
	}

	if len(chunks) < 4 {
		t.Fatalf("expected >=4 chunks (role, tool_call start, 2x args, finish_reason); got %d\nbody=%s", len(chunks), body)
	}

	// First chunk: role marker.
	if d := delta(chunks[0]); d["role"] != "assistant" {
		t.Errorf("chunk0 delta = %v, want role=assistant", d)
	}

	// Second chunk: tool_call start with id + name + empty arguments.
	d1 := delta(chunks[1])
	tcs1, ok := d1["tool_calls"].([]any)
	if !ok || len(tcs1) != 1 {
		t.Fatalf("chunk1 missing tool_calls: %v", d1)
	}
	tc1 := tcs1[0].(map[string]any)
	if tc1["id"] != "toolu_z" {
		t.Errorf("chunk1 tool_call id = %v", tc1["id"])
	}
	if tc1["type"] != "function" {
		t.Errorf("chunk1 tool_call type = %v", tc1["type"])
	}

	// Subsequent chunks that carry only arguments deltas (no id).
	argsConcat := ""
	finishSeen := false
	for _, c := range chunks[2:] {
		d := delta(c)
		if tcs, ok := d["tool_calls"].([]any); ok && len(tcs) > 0 {
			tc := tcs[0].(map[string]any)
			if fn, ok := tc["function"].(map[string]any); ok {
				if a, ok := fn["arguments"].(string); ok {
					argsConcat += a
				}
			}
		}
		if choices, ok := c["choices"].([]any); ok {
			ch := choices[0].(map[string]any)
			if fr, ok := ch["finish_reason"].(string); ok && fr == "tool_calls" {
				finishSeen = true
			}
		}
	}

	var parsedArgs map[string]any
	if err := json.Unmarshal([]byte(argsConcat), &parsedArgs); err != nil {
		t.Fatalf("streamed arguments not valid JSON: %q — %v", argsConcat, err)
	}
	if parsedArgs["city"] != "SF" {
		t.Errorf("streamed args.city = %v, want SF", parsedArgs["city"])
	}
	if !finishSeen {
		t.Errorf("no chunk carried finish_reason=tool_calls")
	}

	// Also confirm [DONE] is present.
	if !strings.Contains(body, "data: [DONE]") {
		t.Errorf("body missing [DONE]:\n%s", body)
	}

	// And no "msg_x" should have leaked into the OpenAI id (id stays chatcmpl-*).
	if bytes.Contains([]byte(body), []byte(`"id":"msg_x"`)) {
		t.Errorf("OpenAI stream leaked upstream msg id:\n%s", body)
	}
}

// delta extracts choices[0].delta from a chunk.
func delta(chunk map[string]any) map[string]any {
	choices, ok := chunk["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil
	}
	ch, ok := choices[0].(map[string]any)
	if !ok {
		return nil
	}
	d, _ := ch["delta"].(map[string]any)
	return d
}
