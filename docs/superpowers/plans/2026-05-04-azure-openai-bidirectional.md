# Azure OpenAI Bidirectional Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Azure OpenAI as an upstream channel type and route between Claude-format and OpenAI-format on a per-request basis so any client protocol can reach any upstream protocol.

**Architecture:** Each adapter declares its native protocol via `Protocol() string`. The handler tags inbound requests with `InboundProto` ("claude" or "openai") and passes the raw body to `ProxyService`. After channel selection, the service compares `inbound proto` with `adapter.Protocol()` — when they differ, it applies the matching request converter, wraps the writer with the matching stream writer (or buffers + post-converts for non-streaming), then calls the adapter. Three new conversion functions mirror the existing forward-direction trio.

**Tech Stack:** Go 1.x, Gin, GORM, PostgreSQL, Redis, React+Vite+AntD

---

## File Map

**New backend files:**
- `backend/internal/adapter/azure.go` — Azure OpenAI adapter (OpenAI protocol over Azure URL)
- `backend/internal/adapter/azure_test.go` — Azure adapter tests (httptest mock upstream)
- `backend/internal/adapter/converter_reverse.go` — three new converters (Claude→OpenAI request, OpenAI→Claude response, ClaudeStreamWriter)
- `backend/internal/adapter/converter_reverse_test.go` — reverse converter tests

**Modified backend files:**
- `backend/internal/adapter/types.go` — extend `Adapter` interface with `Protocol() string`
- `backend/internal/adapter/claude.go` — add `Protocol()` method
- `backend/internal/handler/proxy.go` — remove inline conversion, pass raw body + `InboundProto`
- `backend/internal/service/proxy.go` — accept `InboundProto`, dispatch conversion based on `(inbound, adapter.Protocol())`
- `backend/cmd/server/main.go` — register `AzureAdapter`
- `backend/migration/seed.sql` — add `azure` model_config row(s)

**Modified frontend files:**
- `frontend/src/pages/admin/Channels.tsx` — add `azure` to providers list, base URL hint, type colors

---

## Task 0: Worktree Setup and Baseline

**Files:** none (just env)

- [ ] **Step 1: Confirm clean main**

```bash
cd /home/colorful/Documents/claude/token
git status
git branch --show-current
```

Expected: clean tree on `main`.

- [ ] **Step 2: Create feature branch via worktree**

```bash
git worktree add ../token-azure -b feature/azure-bidirectional
cd ../token-azure
```

Expected: new directory `../token-azure` on branch `feature/azure-bidirectional`.

- [ ] **Step 3: Run existing tests as baseline**

```bash
cd backend && go test ./... 2>&1 | tail -20
```

Expected: all green. If anything fails on `main`, stop and report — plan assumes a green baseline.

- [ ] **Step 4: Bring up dev deps**

```bash
make deps
```

Expected: Postgres + Redis containers up.

---

## Task 1: Extend Adapter Interface with Protocol()

**Files:**
- Modify: `backend/internal/adapter/types.go:30-33`
- Modify: `backend/internal/adapter/claude.go:14-18`

- [ ] **Step 1: Write failing test in `backend/internal/adapter/claude_test.go` (new file)**

```go
package adapter

import "testing"

func TestClaudeAdapter_Protocol(t *testing.T) {
	a := &ClaudeAdapter{}
	if got := a.Protocol(); got != "claude" {
		t.Fatalf("Protocol() = %q, want %q", got, "claude")
	}
}
```

- [ ] **Step 2: Run test, confirm fail**

```bash
cd backend && go test ./internal/adapter/ -run TestClaudeAdapter_Protocol
```

Expected: build error — `a.Protocol undefined`.

- [ ] **Step 3: Update interface in `types.go`**

Replace lines 30-33:

```go
// Adapter is the interface that every upstream provider adapter must implement.
type Adapter interface {
	// Protocol returns the wire protocol this adapter speaks to its upstream:
	// "claude" for Anthropic Messages API, "openai" for OpenAI chat/completions.
	// The proxy service uses this to decide whether request/response conversion
	// is needed between the inbound client format and the upstream format.
	Protocol() string

	ProxyRequest(ctx context.Context, w http.ResponseWriter, body []byte, model, apiKey, baseURL string, stream bool, clientHeaders http.Header) (*ProxyResult, error)
}
```

- [ ] **Step 4: Add `Protocol()` to ClaudeAdapter**

In `claude.go` after the struct definition (after line 18), add:

```go
// Protocol identifies this adapter as speaking Anthropic Messages format.
func (a *ClaudeAdapter) Protocol() string { return "claude" }
```

- [ ] **Step 5: Run tests, confirm pass**

```bash
cd backend && go test ./internal/adapter/
```

Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/adapter/types.go backend/internal/adapter/claude.go backend/internal/adapter/claude_test.go
git commit -m "adapter: add Protocol() to interface, claude returns \"claude\""
```

---

## Task 2: AzureAdapter (OpenAI Protocol over Azure URL)

**Files:**
- Create: `backend/internal/adapter/azure.go`
- Create: `backend/internal/adapter/azure_test.go`

**Design notes:**
- The deployment name and `api-version` are encoded **into `channel.BaseURL`** by convention: `https://<resource>.openai.azure.com/openai/deployments/<deployment>?api-version=<ver>`. The adapter appends `/chat/completions` and preserves the query string.
- Header is `api-key:` (not `Authorization: Bearer`).
- Request body must have `max_tokens` rewritten to `max_completion_tokens` for GPT-5/o-series models. We rewrite unconditionally — Azure ignores the field name when neither is supplied; when both are present it errors. The adapter strips `max_tokens` after copying.
- Streaming: OpenAI SSE format, `data: {...}` and `data: [DONE]`. Usage is in the final chunk when `stream_options.include_usage=true`; otherwise we may not get usage in stream mode. The adapter parses what it sees.

- [ ] **Step 1: Write failing test for non-streaming happy path**

Create `backend/internal/adapter/azure_test.go`:

```go
package adapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAzureAdapter_NonStream_HappyPath(t *testing.T) {
	var gotURL, gotKey, gotBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		gotKey = r.Header.Get("api-key")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-1","object":"chat.completion","created":1,
			"model":"gpt-5.4-nano","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}
		}`))
	}))
	defer upstream.Close()

	baseURL := upstream.URL + "/openai/deployments/gpt-5.4-nano?api-version=2024-10-21"
	body := []byte(`{"model":"gpt-5.4-nano","messages":[{"role":"user","content":"hi"}],"max_tokens":50}`)

	a := &AzureAdapter{HTTPClient: http.DefaultClient}
	rec := httptest.NewRecorder()

	res, err := a.ProxyRequest(context.Background(), rec, body, "gpt-5.4-nano", "test-key", baseURL, false, http.Header{})
	if err != nil {
		t.Fatalf("ProxyRequest: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status=%d", res.StatusCode)
	}
	if !strings.HasSuffix(strings.SplitN(gotURL, "?", 2)[0], "/openai/deployments/gpt-5.4-nano/chat/completions") {
		t.Fatalf("url path wrong: %s", gotURL)
	}
	if !strings.Contains(gotURL, "api-version=2024-10-21") {
		t.Fatalf("api-version missing: %s", gotURL)
	}
	if gotKey != "test-key" {
		t.Fatalf("api-key header = %q", gotKey)
	}
	var parsed map[string]any
	_ = json.Unmarshal([]byte(gotBody), &parsed)
	if _, hasOld := parsed["max_tokens"]; hasOld {
		t.Fatalf("max_tokens should have been rewritten, body=%s", gotBody)
	}
	if v, ok := parsed["max_completion_tokens"].(float64); !ok || v != 50 {
		t.Fatalf("max_completion_tokens missing or wrong: %v", parsed["max_completion_tokens"])
	}
	if res.PromptTokens != 10 || res.CompletionTokens != 2 {
		t.Fatalf("usage parsed wrong: prompt=%d completion=%d", res.PromptTokens, res.CompletionTokens)
	}
}

func TestAzureAdapter_Protocol(t *testing.T) {
	a := &AzureAdapter{}
	if a.Protocol() != "openai" {
		t.Fatalf("Protocol() = %q, want %q", a.Protocol(), "openai")
	}
}
```

- [ ] **Step 2: Run, confirm build fail**

```bash
cd backend && go test ./internal/adapter/ -run TestAzureAdapter
```

Expected: undefined `AzureAdapter`.

- [ ] **Step 3: Create `azure.go` with adapter scaffold + Protocol + max_tokens rewrite**

`backend/internal/adapter/azure.go`:

```go
package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AzureAdapter proxies OpenAI-format requests to an Azure OpenAI deployment.
//
// The channel.BaseURL must encode the deployment and api-version, e.g.:
//   https://juezhou.openai.azure.com/openai/deployments/gpt-5.4-nano?api-version=2024-10-21
// The adapter appends /chat/completions to the path while preserving the query.
type AzureAdapter struct {
	HTTPClient *http.Client
}

// Protocol identifies this adapter as speaking OpenAI chat/completions format.
func (a *AzureAdapter) Protocol() string { return "openai" }

// ProxyRequest forwards body to {baseURL}/chat/completions?{api-version}.
// max_tokens is rewritten to max_completion_tokens (required by GPT-5 / o-series).
func (a *AzureAdapter) ProxyRequest(
	ctx context.Context,
	w http.ResponseWriter,
	body []byte,
	model, apiKey, baseURL string,
	stream bool,
	clientHeaders http.Header,
) (*ProxyResult, error) {
	upstreamURL, err := buildAzureURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("azure: build url: %w", err)
	}

	rewritten, err := rewriteMaxTokens(body)
	if err != nil {
		return nil, fmt.Errorf("azure: rewrite body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(rewritten))
	if err != nil {
		return nil, fmt.Errorf("azure: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", apiKey)

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: upstream request: %w", err)
	}
	defer resp.Body.Close()

	result := &ProxyResult{StatusCode: resp.StatusCode, Model: model}

	if resp.StatusCode >= 400 {
		const maxErrSample = 4096
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrSample))
		result.UpstreamError = string(errBody)
		for k, vs := range resp.Header {
			lk := strings.ToLower(k)
			if lk == "content-length" || lk == "content-encoding" {
				continue
			}
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(errBody)
		return result, nil
	}

	if !stream {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("azure: read response: %w", err)
		}
		if resp.StatusCode == http.StatusOK {
			result.applyOpenAIUsage(respBody)
		}
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return result, nil
	}

	// Streaming: forward each line; parse usage from final data: chunk if present.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)
	flusher, canFlush := w.(http.Flusher)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(line[5:])
			if payload != "[DONE]" {
				result.applyOpenAIUsage([]byte(payload))
			}
		}
		_, _ = fmt.Fprintln(w, line)
		if canFlush {
			flusher.Flush()
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return result, fmt.Errorf("azure: stream scan: %w", err)
	}
	return result, nil
}

// buildAzureURL appends /chat/completions to the channel base URL while
// preserving the api-version query string.
//
// baseURL forms accepted:
//   https://x.openai.azure.com/openai/deployments/<dep>?api-version=YYYY-MM-DD
//   https://x.openai.azure.com/openai/deployments/<dep>/?api-version=YYYY-MM-DD
//   https://x.openai.azure.com/openai/deployments/<dep>/chat/completions?api-version=YYYY-MM-DD
func buildAzureURL(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	path := strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(path, "/chat/completions") {
		path += "/chat/completions"
	}
	u.Path = path
	return u.String(), nil
}

// rewriteMaxTokens copies an OpenAI request body, replacing max_tokens with
// max_completion_tokens. If max_completion_tokens is already present, leaves
// it alone and just removes max_tokens. If neither is present, returns body
// unchanged.
func rewriteMaxTokens(body []byte) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body, nil // not JSON object — let upstream reject
	}
	mt, hasMT := m["max_tokens"]
	_, hasMCT := m["max_completion_tokens"]
	if !hasMT {
		return body, nil
	}
	delete(m, "max_tokens")
	if !hasMCT {
		m["max_completion_tokens"] = mt
	}
	return json.Marshal(m)
}

// applyOpenAIUsage extracts usage from an OpenAI JSON payload (full response
// body or one streaming data chunk) and merges it into the result. Cached
// tokens, when reported via prompt_tokens_details.cached_tokens, are recorded
// separately so billing applies the cache-hit multiplier.
func (r *ProxyResult) applyOpenAIUsage(body []byte) {
	var probe struct {
		Model string `json:"model"`
		Usage *struct {
			PromptTokens        int `json:"prompt_tokens"`
			CompletionTokens    int `json:"completion_tokens"`
			PromptTokensDetails *struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return
	}
	if probe.Model != "" {
		r.Model = probe.Model
	}
	if probe.Usage == nil {
		return
	}
	cached := 0
	if probe.Usage.PromptTokensDetails != nil {
		cached = probe.Usage.PromptTokensDetails.CachedTokens
	}
	if probe.Usage.PromptTokens > 0 {
		r.PromptTokens = probe.Usage.PromptTokens
		r.InputTokens = probe.Usage.PromptTokens - cached
		r.CacheReadTokens = cached
	}
	if probe.Usage.CompletionTokens > 0 {
		r.CompletionTokens = probe.Usage.CompletionTokens
	}
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
cd backend && go test ./internal/adapter/ -run TestAzureAdapter -v
```

Expected: both `TestAzureAdapter_NonStream_HappyPath` and `TestAzureAdapter_Protocol` pass.

- [ ] **Step 5: Add streaming test**

Append to `azure_test.go`:

```go
func TestAzureAdapter_Stream_ForwardsAndParsesUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		f, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		if f != nil { f.Flush() }
	}))
	defer upstream.Close()

	baseURL := upstream.URL + "/openai/deployments/gpt-5.4-nano?api-version=2024-10-21"
	body := []byte(`{"model":"gpt-5.4-nano","messages":[{"role":"user","content":"hi"}],"stream":true,"stream_options":{"include_usage":true}}`)

	a := &AzureAdapter{HTTPClient: http.DefaultClient}
	rec := httptest.NewRecorder()
	res, err := a.ProxyRequest(context.Background(), rec, body, "gpt-5.4-nano", "k", baseURL, true, http.Header{})
	if err != nil {
		t.Fatalf("ProxyRequest: %v", err)
	}
	if !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Fatalf("client SSE missing [DONE]:\n%s", rec.Body.String())
	}
	if res.PromptTokens != 7 || res.CompletionTokens != 3 {
		t.Fatalf("usage prompt=%d completion=%d", res.PromptTokens, res.CompletionTokens)
	}
}
```

Add the missing import: `"fmt"` to the test file's import block.

- [ ] **Step 6: Run, confirm pass**

```bash
cd backend && go test ./internal/adapter/ -run TestAzureAdapter -v
```

Expected: 3 tests pass.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/adapter/azure.go backend/internal/adapter/azure_test.go
git commit -m "adapter: add AzureAdapter with deployment-URL parsing and max_tokens rewrite"
```

---

## Task 3: Register AzureAdapter in main.go

**Files:**
- Modify: `backend/cmd/server/main.go:166`

- [ ] **Step 1: Edit adapter map**

Replace line 166 (the `Adapters: map[...]...` literal):

```go
		Adapters: map[string]adapter.Adapter{
			"claude": &adapter.ClaudeAdapter{HTTPClient: upstreamClient},
			"azure":  &adapter.AzureAdapter{HTTPClient: upstreamClient},
		},
```

- [ ] **Step 2: Build to confirm wiring**

```bash
cd backend && go build ./...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "main: register AzureAdapter as channel type \"azure\""
```

---

## Task 4: ClaudeToOpenAIRequest Converter

**Files:**
- Create: `backend/internal/adapter/converter_reverse.go`
- Create: `backend/internal/adapter/converter_reverse_test.go`

**Goal:** Pure function that takes a Claude `/v1/messages` body and returns an OpenAI chat/completions body.

**Conversion rules (mirror of `OpenAIToClaude`):**
- `system` (top-level string) → prepend a `{role:"system", content: <system>}` message
- For each `messages[i]`:
  - role=`user` with string content → `{role:"user", content:<string>}`
  - role=`user` with array content where any block is `tool_result` → emit one OpenAI `{role:"tool", tool_call_id:<id>, content:<text>}` message per `tool_result` block, plus optional `{role:"user", content:<text>}` for any plain text blocks in the same message
  - role=`assistant` with string content → `{role:"assistant", content:<string>}`
  - role=`assistant` with array content containing `tool_use` blocks → `{role:"assistant", content:<concatenated text or null>, tool_calls:[{id,type:"function",function:{name,arguments:JSON.stringify(input)}}]}`
- `tools[]` (Claude) → `tools[]` (OpenAI) with `function.parameters = input_schema`
- `tool_choice` → OpenAI form (`auto`/`none`/`required` or `{type:"function",function:{name}}`)
- `max_tokens` → carry over (Azure path will rewrite to `max_completion_tokens`)
- `stop_sequences` → `stop`
- `temperature`, `top_p`, `stream` → carry over

- [ ] **Step 1: Write failing test for simple text conversion**

Create `backend/internal/adapter/converter_reverse_test.go`:

```go
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
```

- [ ] **Step 2: Run, confirm fail (undefined)**

```bash
cd backend && go test ./internal/adapter/ -run TestClaudeToOpenAIRequest
```

- [ ] **Step 3: Implement converter in `converter_reverse.go`**

```go
package adapter

import (
	"encoding/json"
	"fmt"
)

// outboundOpenAIMessage is the message shape we emit when converting Claude
// requests to OpenAI requests. Declared at package level so the helper and
// caller share the same named type.
type outboundOpenAIMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
}

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

	var msgs []outboundOpenAIMessage

	if cr.System != "" {
		raw, _ := json.Marshal(cr.System)
		msgs = append(msgs, outboundOpenAIMessage{Role: "system", Content: raw})
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
func claudeMessageToOpenAI(m ClaudeMessage) ([]outboundOpenAIMessage, error) {
	// Try parsing content as plain string first.
	var asString string
	if err := json.Unmarshal(m.Content, &asString); err == nil {
		raw, _ := json.Marshal(asString)
		return []outboundOpenAIMessage{{Role: m.Role, Content: raw}}, nil
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
		return []outboundOpenAIMessage{{Role: "assistant", Content: content, ToolCalls: calls}}, nil

	case "user":
		var out []outboundOpenAIMessage
		var leftoverText string
		for _, b := range blocks {
			switch b.Type {
			case "tool_result":
				raw, _ := json.Marshal(b.ToolResultContent)
				out = append(out, outboundOpenAIMessage{
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
			out = append([]outboundOpenAIMessage{{Role: "user", Content: raw}}, out...)
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
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
cd backend && go test ./internal/adapter/ -run TestClaudeToOpenAIRequest -v
```

Expected: both tests pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/adapter/converter_reverse.go backend/internal/adapter/converter_reverse_test.go
git commit -m "adapter: add ClaudeToOpenAIRequest converter (system, messages, tools, tool_use round-trip)"
```

---

## Task 5: OpenAIToClaudeResponse Converter (Non-Streaming)

**Files:**
- Modify: `backend/internal/adapter/converter_reverse.go` (append)
- Modify: `backend/internal/adapter/converter_reverse_test.go` (append)

**Goal:** Take an OpenAI chat.completion JSON response body and return a Claude /v1/messages response JSON body.

- [ ] **Step 1: Write failing test for non-stream response**

Append to `converter_reverse_test.go`:

```go
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
	_ = json.Unmarshal(out, &got)
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
	_ = json.Unmarshal(out, &got)
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
```

- [ ] **Step 2: Run, confirm fail**

```bash
cd backend && go test ./internal/adapter/ -run TestOpenAIToClaudeResponse
```

- [ ] **Step 3: Implement in `converter_reverse.go` (append)**

```go
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
		"input_tokens":             oai.Usage.PromptTokens - cached,
		"output_tokens":            oai.Usage.CompletionTokens,
		"cache_read_input_tokens":  cached,
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
```

- [ ] **Step 4: Run, confirm pass**

```bash
cd backend && go test ./internal/adapter/ -run TestOpenAIToClaudeResponse -v
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/adapter/converter_reverse.go backend/internal/adapter/converter_reverse_test.go
git commit -m "adapter: add OpenAIToClaudeResponse converter (text + tool_use blocks)"
```

---

## Task 6: ClaudeStreamWriter (OpenAI SSE → Claude SSE)

**Files:**
- Modify: `backend/internal/adapter/converter_reverse.go` (append)
- Modify: `backend/internal/adapter/converter_reverse_test.go` (append)

**Design notes:**

This is the trickiest piece. OpenAI streaming chunks are flat deltas (`{choices:[{delta:{content:"hi"}}]}`). Claude streaming has a typed event machine:

```
event: message_start         {type:"message_start", message:{...,usage:{...}}}
event: content_block_start   {type:"content_block_start", index:N, content_block:{type:"text"|"tool_use", ...}}
event: content_block_delta   {type:"content_block_delta", index:N, delta:{type:"text_delta"|"input_json_delta", ...}}
event: content_block_stop    {type:"content_block_stop", index:N}
event: message_delta         {type:"message_delta", delta:{stop_reason:"end_turn"}, usage:{output_tokens:N}}
event: message_stop          {type:"message_stop"}
```

**State machine** held by ClaudeStreamWriter:
- `started` (bool) — emitted message_start?
- `currentIdx` (-1 = no block open; otherwise the active content_block index)
- `currentKind` ("text" | "tool" | "")
- `toolIdxMap` (map[int]int) — OpenAI tool_calls.index → Claude content_block index
- `nextBlockIdx` (int) — next free content_block index
- `model`, `id` — set from upstream chunks
- `inputTokens`, `outputTokens`, `cachedTokens` — accumulated from final usage chunk
- `finished` (bool) — sent message_stop?

**Transitions:**
- First chunk arrives → emit `message_start` with usage zeros (Claude permits zero on start; updated in message_delta).
- Chunk has `delta.content="..."` → if currentKind != "text": close any open block via `content_block_stop`, open a new text block via `content_block_start`. Then emit `content_block_delta` with `text_delta`.
- Chunk has `delta.tool_calls=[{index:K, id, type, function:{name}}]` → first sight of K: close any open block, open a new `tool_use` block via `content_block_start` with `id`/`name`/`input:{}`. Record `toolIdxMap[K] = nextBlockIdx`. Increment.
- Chunk has `delta.tool_calls=[{index:K, function:{arguments:"..."}}]` (no id, only args) → emit `content_block_delta` with `input_json_delta` for that block.
- Chunk has `finish_reason != null` → close current block (`content_block_stop`), emit `message_delta` with mapped stop_reason, emit `message_stop`. Set `finished=true`.
- A chunk that is purely a final usage chunk (`choices=[]`, `usage={...}`) → record into `outputTokens`/`inputTokens`/`cachedTokens`. If we already emitted `message_delta` without usage, emit a second `message_delta` with usage. (OpenAI usage often arrives AFTER `finish_reason`.)
- `data: [DONE]` line arrives → ensure `message_stop` emitted (idempotent).

- [ ] **Step 1: Write failing test for simple text stream**

Append to `converter_reverse_test.go`:

```go
import (
	"net/http/httptest"  // add to existing import block
)

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
```

- [ ] **Step 2: Run, confirm fail (undefined NewClaudeStreamWriter)**

```bash
cd backend && go test ./internal/adapter/ -run TestClaudeStreamWriter
```

- [ ] **Step 3: Implement skeleton + message_start**

First, ensure these imports are present at the top of `converter_reverse.go` (add to the existing `import (...)` block — do not create a new one):

```go
"net/http"
"strings"
"time"
```

Then append to the bottom of `converter_reverse.go`:

```go
// ClaudeStreamWriter wraps an http.ResponseWriter and converts OpenAI SSE
// streaming chunks into Anthropic's Claude SSE event protocol on the fly.
//
// It is the reverse of OpenAIStreamWriter. The state machine maintains the
// concept of a "currently open content block" (text or tool_use) so that
// content_block_start / content_block_stop frames can be synthesized from
// the flat OpenAI delta stream.
type ClaudeStreamWriter struct {
	w           http.ResponseWriter
	model       string
	messageID   string

	headerSent  bool
	startSent   bool
	finished    bool

	currentIdx  int    // -1 means no block is currently open
	currentKind string // "text" or "tool"

	toolIdxMap   map[int]int // OpenAI tool_calls.index → Claude content_block index
	nextBlockIdx int

	stopReason   string // captured from finish_reason
	stopEmitted  bool   // emitted message_delta with stop_reason yet?

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
```

- [ ] **Step 4: Run, confirm tests still fail at next step (handleChunk undefined)**

```bash
cd backend && go test ./internal/adapter/ -run TestClaudeStreamWriter
```

Expected: build error referencing `s.handleChunk`.

- [ ] **Step 5: Implement handleChunk + delta dispatch**

Append to `converter_reverse.go`:

```go
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

	if chunk.ID != "" && !s.startSent {
		// keep our own msg_ id format; don't overwrite messageID
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
			s.closeCurrentBlock()
			s.emitMessageDelta()
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
		// If the stop_reason has already been emitted without usage, emit a
		// supplementary message_delta carrying usage so billing sees output.
		if s.stopEmitted {
			s.writeEvent("message_delta", map[string]any{
				"type":  "message_delta",
				"delta": map[string]any{},
				"usage": map[string]any{"output_tokens": s.outputTokens},
			})
		}
	}
}
```

- [ ] **Step 6: Implement block-management helpers**

Append:

```go
func (s *ClaudeStreamWriter) emitMessageStart() {
	s.startSent = true
	s.writeEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":      s.messageID,
			"type":    "message",
			"role":    "assistant",
			"model":   s.model,
			"content": []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":               0,
				"output_tokens":              0,
				"cache_read_input_tokens":    0,
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
		"type":  "content_block_start",
		"index": s.currentIdx,
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
```

- [ ] **Step 7: Run tests, confirm pass**

```bash
cd backend && go test ./internal/adapter/ -run TestClaudeStreamWriter -v
```

Expected: both tests pass.

- [ ] **Step 8: Run full adapter test suite to catch regressions**

```bash
cd backend && go test ./internal/adapter/ -v
```

Expected: all green including pre-existing `OpenAIToClaude` tests.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/adapter/converter_reverse.go backend/internal/adapter/converter_reverse_test.go
git commit -m "adapter: add ClaudeStreamWriter (OpenAI SSE → Claude SSE state machine)"
```

---

## Task 7: Refactor Handler — Pass Raw Body + InboundProto

**Files:**
- Modify: `backend/internal/handler/proxy.go:103-216`
- Modify: `backend/internal/service/proxy.go:27-36`

**Goal:** Move conversion responsibility out of the handler. Handler becomes a thin tagger; service decides what to convert based on the channel it picks.

- [ ] **Step 1: Add InboundProto to ProxyRequest struct**

In `backend/internal/service/proxy.go` modify lines 27-36:

```go
// ProxyRequest carries all the context needed to forward a single request.
type ProxyRequest struct {
	UserID        int64
	ApiKeyID      int64
	Model         string
	Body          []byte    // raw body, in InboundProto format
	Stream        bool
	Type          string    // request log: "native" or "openai_compat"
	InboundProto  string    // wire protocol of Body: "claude" or "openai"
	IP            string
	ClientHeaders http.Header
}
```

- [ ] **Step 2: Simplify NativeMessages handler**

Replace `proxy.go` lines 105-140 with:

```go
// NativeMessages handles POST /v1/messages (native Claude protocol).
// Forwards the raw body to ProxyService, which handles conversion if the
// selected channel speaks OpenAI.
func (h *ProxyHandler) NativeMessages(c *gin.Context) {
	if msg := checkBalance(c); msg != "" {
		h.preflightErr(c, http.StatusPaymentRequired, "insufficient_balance",
			"native", "", "balance_insufficient", msg)
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.preflightErr(c, http.StatusBadRequest, "invalid_request",
			"native", "", "body_read", "cannot read request body: "+err.Error())
		return
	}
	model := service.ExtractModel(body)
	if model == "" {
		h.preflightErr(c, http.StatusBadRequest, "invalid_request",
			"native", "", "missing_model", "model field is required")
		return
	}

	pr := &service.ProxyRequest{
		UserID:        getUserID(c),
		ApiKeyID:      getApiKeyID(c),
		Model:         model,
		Body:          body,
		Stream:        service.ExtractStream(body),
		Type:          "native",
		InboundProto:  "claude",
		IP:            c.ClientIP(),
		ClientHeaders: c.Request.Header,
	}
	h.ProxyService.HandleProxy(c.Request.Context(), c.Writer, pr)
}
```

- [ ] **Step 3: Simplify ChatCompletions handler**

Replace `proxy.go` lines 142-216 with:

```go
// ChatCompletions handles POST /v1/chat/completions (OpenAI-compatible).
// Forwards the raw body. If the selected channel speaks Claude, ProxyService
// converts the OpenAI body to Claude format and wraps the response.
func (h *ProxyHandler) ChatCompletions(c *gin.Context) {
	if msg := checkBalance(c); msg != "" {
		h.preflightErr(c, http.StatusPaymentRequired, "insufficient_balance",
			"openai_compat", "", "balance_insufficient", msg)
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.preflightErr(c, http.StatusBadRequest, "invalid_request",
			"openai_compat", "", "body_read", "cannot read request body: "+err.Error())
		return
	}
	model := service.ExtractModel(body)
	if model == "" {
		h.preflightErr(c, http.StatusBadRequest, "invalid_request",
			"openai_compat", "", "missing_model", "model field is required")
		return
	}

	pr := &service.ProxyRequest{
		UserID:        getUserID(c),
		ApiKeyID:      getApiKeyID(c),
		Model:         model,
		Body:          body,
		Stream:        service.ExtractStream(body),
		Type:          "openai_compat",
		InboundProto:  "openai",
		IP:            c.ClientIP(),
		ClientHeaders: c.Request.Header,
	}
	h.ProxyService.HandleProxy(c.Request.Context(), c.Writer, pr)
}
```

Also remove now-unused imports from `proxy.go` if `adapter` was imported only for the converters: keep it if still needed elsewhere. Run `goimports` after.

- [ ] **Step 4: Build to confirm syntactic correctness**

```bash
cd backend && go build ./...
```

Expected: build error in `service/proxy.go` because `HandleProxy` doesn't yet apply conversions and the file may still reference `responseBuffer` from the handler. (We'll fix in next task.)

If the build passes, that's also fine — it just means we haven't broken anything yet.

- [ ] **Step 5: Commit (handler refactor only)**

```bash
git add backend/internal/handler/proxy.go backend/internal/service/proxy.go
git commit -m "handler: pass raw body and InboundProto to ProxyService instead of converting"
```

---

## Task 8: Service-Layer Conversion Dispatch

**Files:**
- Modify: `backend/internal/service/proxy.go:41-103`

**Goal:** Inside `HandleProxy`, after channel + adapter selection, branch on `(pr.InboundProto, adapter.Protocol())` and apply conversions when they differ.

- [ ] **Step 1: Add a small response buffer type to service package**

At top of `backend/internal/service/proxy.go` (after the existing imports), add:

```go
// bufferedResponse captures response bytes in memory for post-processing.
// Used when the inbound and upstream protocols differ in non-streaming mode.
type bufferedResponse struct {
	header     http.Header
	statusCode int
	body       bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header {
	if b.header == nil {
		b.header = make(http.Header)
	}
	return b.header
}
func (b *bufferedResponse) Write(p []byte) (int, error)  { return b.body.Write(p) }
func (b *bufferedResponse) WriteHeader(statusCode int)   { b.statusCode = statusCode }
```

Add `"bytes"` to the imports.

- [ ] **Step 2: Insert dispatch logic in HandleProxy**

Locate `HandleProxy` (currently around line 41) and after step "2. Look up adapter" (line 53-59), before step "3. Forward the request" (line 61-62), insert:

```go
	// 2b. Decide whether request/response conversion is needed.
	upstreamProto := adpt.Protocol()
	body := pr.Body
	writer := w

	if pr.InboundProto != upstreamProto {
		converted, convErr := s.convertRequest(pr.InboundProto, upstreamProto, pr.Body)
		if convErr != nil {
			writeProxyError(w, http.StatusBadRequest, "converter", convErr.Error())
			s.LogPreflightError(pr, "converter_request", convErr.Error())
			return
		}
		body = converted

		var bufferForPostConvert *bufferedResponse
		writer, bufferForPostConvert = s.wrapResponseWriter(w, pr, upstreamProto)
		// If we got a buffer (non-streaming mismatch), defer the post-convert.
		if bufferForPostConvert != nil {
			defer s.flushBuffered(w, bufferForPostConvert, pr, upstreamProto)
		}
	}
```

Replace the existing call at line 62:

```go
	result, err := adpt.ProxyRequest(ctx, writer, body, pr.Model, ch.ApiKey, ch.BaseURL, pr.Stream, pr.ClientHeaders)
```

(Was: `result, err := adpt.ProxyRequest(ctx, w, pr.Body, ...)` — substitute `writer` for `w` and `body` for `pr.Body`.)

- [ ] **Step 3: Implement convertRequest helper**

Append to `backend/internal/service/proxy.go`:

```go
// convertRequest re-encodes the inbound request body into the upstream
// protocol's format. Called only when the two protocols differ.
func (s *ProxyService) convertRequest(inbound, upstream string, body []byte) ([]byte, error) {
	switch {
	case inbound == "openai" && upstream == "claude":
		out, _, err := adapter.OpenAIToClaude(body)
		return out, err
	case inbound == "claude" && upstream == "openai":
		out, _, err := adapter.ClaudeToOpenAIRequest(body)
		return out, err
	}
	return nil, fmt.Errorf("unsupported conversion %s→%s", inbound, upstream)
}
```

- [ ] **Step 4: Implement wrapResponseWriter helper**

```go
// wrapResponseWriter returns a writer that converts the upstream's response
// (in upstreamProto) back to the inbound protocol on the fly. For streaming
// requests it returns a stream-converting wrapper and a nil buffer; for
// non-streaming it returns a buffer that flushBuffered will post-convert.
func (s *ProxyService) wrapResponseWriter(
	w http.ResponseWriter,
	pr *ProxyRequest,
	upstreamProto string,
) (http.ResponseWriter, *bufferedResponse) {
	if pr.Stream {
		switch {
		case pr.InboundProto == "openai" && upstreamProto == "claude":
			return adapter.NewOpenAIStreamWriter(w, pr.Model, adapter.IncludeUsageRequested(pr.Body)), nil
		case pr.InboundProto == "claude" && upstreamProto == "openai":
			return adapter.NewClaudeStreamWriter(w, pr.Model), nil
		}
	}
	buf := &bufferedResponse{}
	return buf, buf
}

// flushBuffered post-converts a buffered upstream response back to the inbound
// protocol and writes it to the real client writer.
func (s *ProxyService) flushBuffered(
	w http.ResponseWriter,
	buf *bufferedResponse,
	pr *ProxyRequest,
	upstreamProto string,
) {
	statusCode := buf.statusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	// Forward error responses verbatim — translating a 4xx/5xx body would
	// destroy diagnostic info from the upstream.
	if statusCode != http.StatusOK {
		ct := buf.Header().Get("Content-Type")
		if ct == "" {
			ct = "application/json"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(statusCode)
		_, _ = w.Write(buf.body.Bytes())
		return
	}

	var converted []byte
	var err error
	switch {
	case pr.InboundProto == "openai" && upstreamProto == "claude":
		converted, err = adapter.ClaudeToOpenAIResponse(buf.body.Bytes(), pr.Model)
	case pr.InboundProto == "claude" && upstreamProto == "openai":
		converted, err = adapter.OpenAIToClaudeResponse(buf.body.Bytes(), pr.Model)
	default:
		converted = buf.body.Bytes()
	}
	if err != nil {
		// Conversion failed — forward original body. Better than zero output.
		log.Printf("proxy: response conversion %s→%s failed: %v", upstreamProto, pr.InboundProto, err)
		converted = buf.body.Bytes()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(converted)
}
```

Add `"ai-relay/internal/adapter"` to imports if not already present.

- [ ] **Step 5: Build and run all tests**

```bash
cd backend && go build ./... && go test ./...
```

Expected: green.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/proxy.go
git commit -m "service: dispatch request/response conversion when inbound != upstream protocol"
```

(Coverage of the dispatch logic comes from end-to-end runs in Task 10. Adding a unit test here would require refactoring `ChannelService` to take a fake repo; out of scope for this plan.)

---

## Task 9: Seed Data and Frontend Updates

**Files:**
- Modify: `backend/migration/seed.sql`
- Modify: `frontend/src/pages/admin/Channels.tsx:35-56`

- [ ] **Step 1: Append Azure model to seed.sql**

In `backend/migration/seed.sql`, after the existing claude inserts, add:

```sql
-- Seed: Azure OpenAI model configs
INSERT INTO model_configs (model_name, provider, display_name, rate, input_price, output_price, enabled, created_at, updated_at)
VALUES
    ('gpt-5.4-nano', 'azure', 'Azure GPT-5.4 Nano', 1.0, 0.150000, 0.600000, true, NOW(), NOW())
ON CONFLICT (model_name) DO NOTHING;
```

(Adjust input_price / output_price to whatever your contract is; the values above are placeholders. If you don't know yet, use 0 to mark unbilled.)

- [ ] **Step 2: Update frontend providers list**

In `frontend/src/pages/admin/Channels.tsx`, line 35, add `azure` to TYPE_COLORS:

```tsx
const TYPE_COLORS: Record<string, string> = {
  claude: 'blue',
  openai: 'green',
  gemini: 'orange',
  azure: 'purple',
};
```

Line 46:

```tsx
const DEFAULT_PROVIDERS = ['claude', 'openai', 'gemini', 'azure', 'deepseek', 'mistral'];
```

Line 48-56:

```tsx
const BASE_URLS: Record<string, string> = {
  claude: 'https://api.anthropic.com',
  openai: 'https://api.openai.com',
  gemini: 'https://generativelanguage.googleapis.com',
  deepseek: 'https://api.deepseek.com',
  mistral: 'https://api.mistral.ai',
  azure: 'https://<resource>.openai.azure.com/openai/deployments/<deployment>?api-version=2024-10-21',
};
```

- [ ] **Step 3: Add a help hint above the Base URL input**

Find the form rendering block (around line 393-400, the "base_url" input). Just above it, conditionally render:

```tsx
{form.type === 'azure' && (
  <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4 }}>
    {t('Azure: include the deployment path and ?api-version=… in Base URL')}
  </div>
)}
```

If the project uses i18n keys, add `azure_base_url_hint` to the translation files; otherwise inline the English text.

- [ ] **Step 4: Restart dev stack and apply migrations**

```bash
make seed
```

Expected: new row inserted, no errors.

- [ ] **Step 5: Build frontend**

```bash
cd frontend && npm run build
```

Expected: build succeeds.

- [ ] **Step 6: Commit**

```bash
git add backend/migration/seed.sql frontend/src/pages/admin/Channels.tsx
git commit -m "seed+ui: add azure provider option, model config, base url hint"
```

---

## Task 10: End-to-End Verification

**Files:** none (manual run)

This is the proof that all 4 conversion paths actually work end-to-end. Use the real Azure deployment from earlier (`gpt-5.4-nano`).

- [ ] **Step 1: Bring up stack**

```bash
make deps && make dev
```

In another terminal:

```bash
cd frontend && npm run dev
```

- [ ] **Step 2: Create azure channel via admin UI**

- Log in to admin.
- Channels → Add.
- Type: `azure`
- API Key: <your azure api key>
- Base URL: `https://juezhou.openai.azure.com/openai/deployments/gpt-5.4-nano?api-version=2024-10-21`
- Models: `gpt-5.4-nano`
- Save.

Expected: channel appears in list with status `active`.

- [ ] **Step 3: Path 1 — OpenAI client → Azure (no conversion)**

```bash
curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <user-api-key>" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5.4-nano","messages":[{"role":"user","content":"hi"}],"max_tokens":50}'
```

Expected: OpenAI `chat.completion` JSON with assistant content.

- [ ] **Step 4: Path 2 — OpenAI client streaming → Azure (no conversion)**

Same but with `"stream":true,"stream_options":{"include_usage":true}`.

Expected: SSE stream of `chat.completion.chunk` events ending with `[DONE]`.

- [ ] **Step 5: Path 3 — Claude client → Azure (request+response conversion)**

```bash
curl -s -X POST http://localhost:8080/v1/messages \
  -H "x-api-key: <user-api-key>" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5.4-nano","max_tokens":100,"messages":[{"role":"user","content":"hi"}]}'
```

Expected: Claude-format response JSON with `content:[{type:"text",text:"..."}]`, `stop_reason:"end_turn"`, usage object.

- [ ] **Step 6: Path 4 — Claude client streaming → Azure (request+stream conversion)**

```bash
curl -sN -X POST http://localhost:8080/v1/messages \
  -H "x-api-key: <user-api-key>" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5.4-nano","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"count to 3"}]}'
```

Expected: SSE stream with `event: message_start`, multiple `event: content_block_delta` with `text_delta`, `event: content_block_stop`, `event: message_delta`, `event: message_stop`.

- [ ] **Step 7: Regression check — Claude client → Claude upstream still works**

Repeat Step 5 but with the original Claude channel still present (configure model routing so this request goes to Claude, not Azure). Verify nothing broke.

- [ ] **Step 8: Check request_logs for billing**

```bash
psql -U postgres -d ai_relay -c "SELECT model, prompt_tokens, completion_tokens, status, error_stage FROM request_logs ORDER BY id DESC LIMIT 10;"
```

Expected: rows for each test path with non-zero token counts and `status=success`.

- [ ] **Step 9: Final commit (if any tweaks made)**

```bash
git status
# only commit if there were follow-up fixes during E2E
```

- [ ] **Step 10: Open PR**

```bash
git push -u origin feature/azure-bidirectional
# then `gh pr create` per repo conventions
```

---

## Summary of Deliverables

By plan completion you will have:

1. New `azure` channel type that proxies to Azure OpenAI deployments
2. Bidirectional conversion: any client protocol routes to any upstream protocol
3. Three new pure converters with unit tests:
   - `ClaudeToOpenAIRequest`
   - `OpenAIToClaudeResponse`
   - `ClaudeStreamWriter` (OpenAI SSE → Claude SSE)
4. Service-layer dispatch that applies conversions only when needed (zero overhead when client and upstream proto match)
5. Admin UI option for `azure` with deployment-URL guidance
6. Seed data for `gpt-5.4-nano`
7. End-to-end verification across all 4 (client × upstream) paths

## Known Gotchas

- **`max_tokens` rewrite**: GPT-5/o-series Azure deployments require `max_completion_tokens`; the AzureAdapter rewrites this unconditionally. If a future deployment requires the original name, gate the rewrite by model.
- **Tool-call ID stability**: IDs are passed through verbatim in both directions. Don't introduce ID rewriting later — it will break multi-turn tool conversations.
- **Streaming usage timing**: OpenAI emits `usage` only in the final chunk (when `stream_options.include_usage=true`). The ClaudeStreamWriter emits `message_start` with zero usage and patches it via a supplementary `message_delta`. Some Claude clients may briefly see zero input_tokens until that supplementary event arrives — acceptable per protocol.
- **Error responses are not converted**: 4xx/5xx upstream responses are forwarded verbatim. A Claude client hitting Azure will see an OpenAI-format error JSON. If that's a problem, add a separate error-translation path in `flushBuffered` later.
