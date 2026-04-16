package adapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ClaudeAdapter proxies requests to the Anthropic Claude API.
// HTTPClient should be a shared, long-lived client with connection pooling.
type ClaudeAdapter struct {
	HTTPClient *http.Client
}

// ProxyRequest forwards body to the Claude /v1/messages endpoint.
// When stream is true it sets SSE response headers and streams lines back to
// the client, parsing usage from message_delta / message_stop events.
// When stream is false it reads the full body, parses usage, and writes the
// response verbatim.
func (a *ClaudeAdapter) ProxyRequest(
	ctx context.Context,
	w http.ResponseWriter,
	body []byte,
	model, apiKey, baseURL string,
	stream bool,
	clientHeaders http.Header,
) (*ProxyResult, error) {
	url := strings.TrimRight(baseURL, "/") + "/v1/messages"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("claude: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	// Forward all anthropic-* headers from client (version, beta, etc.).
	for key, vals := range clientHeaders {
		lk := strings.ToLower(key)
		if strings.HasPrefix(lk, "anthropic-") {
			for _, v := range vals {
				req.Header.Add(key, v)
			}
		}
	}
	// Ensure anthropic-version is always set.
	if req.Header.Get("anthropic-version") == "" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude: upstream request: %w", err)
	}
	defer resp.Body.Close()

	result := &ProxyResult{
		StatusCode: resp.StatusCode,
		Model:      model,
	}

	if !stream {
		// ---------------------------------------------------------------
		// Non-streaming: read entire body, parse usage, then forward.
		// ---------------------------------------------------------------
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("claude: read response: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			var cr ClaudeResponse
			if jsonErr := json.Unmarshal(respBody, &cr); jsonErr == nil {
				result.PromptTokens = cr.Usage.InputTokens
				result.CompletionTokens = cr.Usage.OutputTokens
				if cr.Model != "" {
					result.Model = cr.Model
				}
			}
		}

		// Copy upstream headers then write the body.
		for key, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(key, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return result, nil
	}

	// -------------------------------------------------------------------
	// Streaming: set SSE headers and forward each line from upstream.
	// -------------------------------------------------------------------
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)

	flusher, canFlush := w.(http.Flusher)

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// Stop if client disconnected — avoid wasting upstream bandwidth.
		if ctx.Err() != nil {
			break
		}

		line := scanner.Text()

		// Try to parse usage from data: lines.
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			result.parseStreamEvent(payload)
		}

		_, _ = fmt.Fprintln(w, line)
		if canFlush {
			flusher.Flush()
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return nil, fmt.Errorf("claude: stream scan: %w", err)
	}

	return result, nil
}

// parseStreamEvent inspects a single SSE data payload for usage information
// present in message_delta and message_stop events.
func (r *ProxyResult) parseStreamEvent(payload string) {
	// We only care about objects that have a "usage" field.
	var event struct {
		Type  string `json:"type"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		// message_delta wraps usage differently
		Delta *struct {
			Type string `json:"type"`
		} `json:"delta"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return
	}
	if event.Usage != nil {
		if event.Usage.InputTokens > 0 {
			r.PromptTokens = event.Usage.InputTokens
		}
		if event.Usage.OutputTokens > 0 {
			r.CompletionTokens = event.Usage.OutputTokens
		}
	}
}
