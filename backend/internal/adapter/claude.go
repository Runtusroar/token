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

// Protocol identifies this adapter as speaking Anthropic Messages format.
func (a *ClaudeAdapter) Protocol() string { return "claude" }

// ProxyRequest forwards body to the Claude /v1/messages endpoint.
// When stream is true it sets SSE response headers and streams lines back to
// the client, parsing usage from message_start (input + cache tokens) and
// message_delta (running output tokens).
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

	// Upstream error: sample the body for diagnostics, then forward unchanged.
	// Handled here for both stream and non-stream so we never apply SSE headers
	// on top of a non-SSE error body.
	if resp.StatusCode >= 400 {
		const maxErrSample = 4096
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrSample))
		result.UpstreamError = string(errBody)

		// Copy upstream headers, but drop ones that describe the body shape —
		// we truncated it (so Content-Length is wrong) and Go already decoded
		// any compression (so Content-Encoding is stale). Without this, the
		// client HTTP parser hangs or rejects the response.
		for key, vals := range resp.Header {
			lk := strings.ToLower(key)
			if lk == "content-length" || lk == "content-encoding" {
				continue
			}
			for _, v := range vals {
				w.Header().Add(key, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(errBody)
		return result, nil
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
				result.applyUsage(&cr.Usage)
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
	// Default scanner line limit is 64KB; large tool_use deltas or chunky
	// text_delta payloads can exceed that and cause bufio.ErrTooLong, which
	// truncates the stream. Raise the ceiling well above any realistic line.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
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
		// Return result along with err so the service layer knows the SSE
		// response has already been (partially) written and skips its own
		// error-response path — otherwise it would write JSON after SSE.
		return result, fmt.Errorf("claude: stream scan: %w", err)
	}

	return result, nil
}

// parseStreamEvent inspects a single SSE data payload for usage information.
// Anthropic emits usage in two places:
//   - message_start: nested at message.usage; carries the full input
//     breakdown (input_tokens + cache_read + cache_creation, optionally
//     split 5m/1h). This is the only event that reports input/cache totals.
//   - message_delta: top-level usage with the running output_tokens (and on
//     newer API versions a final input tally — applyUsage handles either).
func (r *ProxyResult) parseStreamEvent(payload string) {
	var event struct {
		Type    string `json:"type"`
		Message *struct {
			Model string       `json:"model"`
			Usage *ClaudeUsage `json:"usage"`
		} `json:"message"`
		Usage *ClaudeUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return
	}

	if event.Message != nil {
		if event.Message.Model != "" {
			r.Model = event.Message.Model
		}
		if event.Message.Usage != nil {
			r.applyUsage(event.Message.Usage)
		}
	}
	if event.Usage != nil {
		r.applyUsage(event.Usage)
	}
}

// applyUsage merges a usage object into the result, only overwriting fields
// when the usage object carries a non-zero value. message_delta typically
// reports just OutputTokens — the zero-skip ensures it doesn't clobber the
// input breakdown that message_start already populated.
func (r *ProxyResult) applyUsage(u *ClaudeUsage) {
	in, cr, cw5, cw1 := u.Categorize()
	if in > 0 {
		r.InputTokens = in
	}
	if cr > 0 {
		r.CacheReadTokens = cr
	}
	if cw5 > 0 {
		r.CacheWrite5mTokens = cw5
	}
	if cw1 > 0 {
		r.CacheWrite1hTokens = cw1
	}
	r.PromptTokens = r.InputTokens + r.CacheReadTokens + r.CacheWrite5mTokens + r.CacheWrite1hTokens
	if u.OutputTokens > 0 {
		r.CompletionTokens = u.OutputTokens
	}
}
