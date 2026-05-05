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
//
//	https://juezhou.openai.azure.com/openai/deployments/gpt-5.4-nano?api-version=2024-10-21
//
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
	// clientHeaders is intentionally not read: Azure has no client-originating
	// pass-through header convention (no equivalent of Anthropic's anthropic-*).
	_ = clientHeaders

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
			if lk == "content-length" || lk == "content-encoding" || lk == "transfer-encoding" {
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
			lk := strings.ToLower(k)
			if lk == "content-length" || lk == "content-encoding" || lk == "transfer-encoding" {
				continue
			}
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
//
//	https://x.openai.azure.com/openai/deployments/<dep>?api-version=YYYY-MM-DD
//	https://x.openai.azure.com/openai/deployments/<dep>/?api-version=YYYY-MM-DD
//	https://x.openai.azure.com/openai/deployments/<dep>/chat/completions?api-version=YYYY-MM-DD
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
