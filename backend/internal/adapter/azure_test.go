package adapter

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestAzureAdapter_Stream_ForwardsAndParsesUsage(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		f, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
		if f != nil {
			f.Flush()
		}
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
