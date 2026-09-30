// Package llm is a small OpenAI-compatible chat-completions client: the
// only thing the interviewer engine needs from a model provider.
//
// It speaks the `/chat/completions` dialect that essentially every
// gateway implements (DeepSeek, SiliconFlow, OpenAI, vLLM, Ollama,
// LiteLLM, …), in both flavours the engine uses:
//
//	Chat        — one blocking call, for structured JSON stages
//	ChatStream  — SSE deltas, for text the candidate watches arrive
//	ChatJSON    — Chat + tolerant JSON extraction into a Go value
//
// Three behaviours are deliberate and load-bearing:
//
//  1. `reasoning_content` (the private "thinking" channel that reasoning
//     models emit alongside `content`) is
//     DROPPED from streams. Streaming a model's inner monologue into a
//     candidate-facing UI would be a bug.
//
//  2. Reasoning tokens are billed against `max_tokens` too. A thinking
//     model can therefore spend the entire budget on reasoning and emit
//     an empty `content` with finish_reason "length". That failure mode
//     is detected and retried ONCE with a doubled budget whenever nothing
//     has been emitted yet, instead of silently degrading the interview
//     to the offline rubric.
//
//  3. JSON mode is not requested via `response_format`: gateway support is
//     uneven and a rejection costs a round trip. The prompt demands JSON
//     and ExtractJSON tolerates fences and surrounding prose instead.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config is the transport-level configuration for one provider.
type Config struct {
	BaseURL     string        // e.g. https://api.siliconflow.cn/v1
	APIKey      string        // sent as `Authorization: Bearer …` when non-empty
	Model       string        // e.g. gpt-4o-mini, Qwen/Qwen2.5-7B-Instruct
	Timeout     time.Duration // per-call budget (0 → 90s)
	MaxTokens   int           // response budget, reasoning included (0 → 8192)
	Temperature float64       // sampling temperature
}

// maxTokensCeiling bounds the automatic budget escalation.
const maxTokensCeiling = 32768

// Message is one turn of the conversation, in OpenAI's wire shape.
type Message struct {
	Role    string `json:"role"` // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// System/User/Assistant are the three constructors the engine uses; they
// exist so prompts read as prose instead of struct literals.
func System(content string) Message    { return Message{Role: "system", Content: content} }
func User(content string) Message      { return Message{Role: "user", Content: content} }
func Assistant(content string) Message { return Message{Role: "assistant", Content: content} }

// Client is safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client
}

// New builds a client. A zero timeout becomes 90s; a zero MaxTokens
// becomes 8192 — generous on purpose, because reasoning models charge
// their thinking against the same budget.
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 90 * time.Second
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 8192
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	return &Client{
		cfg: cfg,
		// No http.Client.Timeout: the per-request context deadline is the
		// real budget, and a client-level timeout would also cap the
		// long-lived streaming calls the interview room depends on.
		http: &http.Client{},
	}
}

// Config returns a copy of the client's configuration.
func (c *Client) Config() Config { return c.cfg }

// Model is the configured model id.
func (c *Client) Model() string { return c.cfg.Model }

// BaseURL is the configured endpoint root.
func (c *Client) BaseURL() string { return c.cfg.BaseURL }

// Configured reports whether the client can be used at all: an endpoint
// and a model. A missing API key is legal (local Ollama accepts none).
func (c *Client) Configured() bool {
	return c.cfg.BaseURL != "" && c.cfg.Model != ""
}

// HasKey reports whether a credential is configured.
func (c *Client) HasKey() bool { return c.cfg.APIKey != "" }

// Timeout exposes the per-call budget for display.
func (c *Client) Timeout() time.Duration { return c.cfg.Timeout }

// MaxTokens exposes the configured response budget for display.
func (c *Client) MaxTokens() int { return c.cfg.MaxTokens }

// APIError is a non-2xx response from the provider, with the provider's
// own code/message when it supplies one.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("llm: http %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("llm: http %d", e.Status)
}

// EmptyContentError means the call succeeded but produced nothing the
// user can read. FinishReason explains how: "length" means the budget was
// exhausted (reasoning models do this), anything else means the provider
// returned an empty completion for its own reasons.
type EmptyContentError struct {
	FinishReason   string
	Budget         int
	ReasoningChars int
}

func (e *EmptyContentError) Error() string {
	if e.FinishReason == "length" {
		return fmt.Sprintf(
			"模型在 %d token 的预算内只输出了推理内容（reasoning），没有输出正文。"+
				"这是一个推理型模型，思考过程同样消耗预算——请提高 APP_LLM_MAX_TOKENS（当前 %d），或换用非推理模型。",
			e.Budget, e.Budget)
	}
	return fmt.Sprintf("模型返回了空内容（finish_reason=%q）", e.FinishReason)
}

// Retryable reports whether err is a transient failure worth a retry.
func Retryable(err error) bool {
	var empty *EmptyContentError
	if errors.As(err, &empty) {
		// A deterministic empty completion is not going to change on a
		// blind retry; escalation handles the budget case explicitly.
		return false
	}
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Status == http.StatusTooManyRequests || ae.Status >= 500
	}
	// Network/transport errors land here; a cancellation must not.
	return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// --- request / response wire shapes -----------------------------------------

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// --- plain completion -------------------------------------------------------

// Chat runs one blocking completion and returns the assistant text.
func (c *Client) Chat(ctx context.Context, msgs []Message) (string, error) {
	return c.do(ctx, chatRequest{
		Model:       c.cfg.Model,
		Messages:    msgs,
		MaxTokens:   c.cfg.MaxTokens,
		Temperature: c.cfg.Temperature,
	}, false, nil)
}

// ChatStream runs one streaming completion, calling onDelta for each
// visible content fragment. It returns the full concatenated text.
//
// onDelta returning an error aborts the stream and is returned to the
// caller — that is how the HTTP handler stops early when the browser
// disconnects.
func (c *Client) ChatStream(ctx context.Context, msgs []Message, onDelta func(string) error) (string, error) {
	return c.do(ctx, chatRequest{
		Model:       c.cfg.Model,
		Messages:    msgs,
		MaxTokens:   c.cfg.MaxTokens,
		Temperature: c.cfg.Temperature,
		Stream:      true,
	}, true, onDelta)
}

// outcome is one HTTP attempt: the text produced, whether any of it was
// already handed to the caller, and the failure.
type outcome struct {
	text    string
	emitted bool
	err     error
}

// do performs the round trip, escalating the token budget when a
// reasoning model burns it all, and retrying transient transport faults.
//
// The `emitted` flag is what makes escalation safe: once a fragment has
// reached the candidate, replaying the request would show it twice, so at
// that point the failure is final.
func (c *Client) do(ctx context.Context, req chatRequest, stream bool, onDelta func(string) error) (string, error) {
	if !c.Configured() {
		return "", errors.New("llm: engine is not configured (need a base URL and a model)")
	}
	if len(req.Messages) == 0 {
		return "", errors.New("llm: no messages")
	}

	budget := req.MaxTokens
	if budget <= 0 {
		budget = c.cfg.MaxTokens
	}
	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Build the payload per attempt so the escalation can change the
		// budget; the request shape is otherwise identical every time.
		req.MaxTokens = budget
		payload, err := json.Marshal(req)
		if err != nil {
			return "", fmt.Errorf("llm: encode request: %w", err)
		}

		res := c.attempt(ctx, payload, stream, onDelta, budget)
		if res.err == nil {
			return res.text, nil
		}
		lastErr = res.err

		// Escalate on a budget-exhausted empty completion, but only while
		// the candidate has seen nothing.
		var empty *EmptyContentError
		if errors.As(res.err, &empty) && empty.FinishReason == "length" && !res.emitted {
			if budget >= maxTokensCeiling {
				return res.text, res.err
			}
			budget = min(budget*2, maxTokensCeiling)
			continue
		}
		if stream || res.emitted || !Retryable(res.err) {
			return res.text, res.err
		}
		select {
		case <-ctx.Done():
			return res.text, ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
	return "", lastErr
}

// attempt performs one HTTP round trip. For the blocking path it decodes
// the reply here (rather than in the caller) so the retry loop can see an
// EmptyContentError and escalate the token budget.
func (c *Client) attempt(ctx context.Context, payload []byte, stream bool, onDelta func(string) error, budget int) outcome {
	callCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	url := c.cfg.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return outcome{err: fmt.Errorf("llm: build request: %w", err)}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	res, err := c.http.Do(httpReq)
	if err != nil {
		return outcome{err: fmt.Errorf("llm: request failed: %w", err)}
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return outcome{err: decodeAPIError(res)}
	}
	if stream {
		return parseSSE(res.Body, onDelta, budget)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return outcome{err: fmt.Errorf("llm: read response: %w", err)}
	}
	content, err := extractMessage(string(raw), budget)
	if err != nil {
		return outcome{err: err}
	}
	return outcome{text: content}
}

// decodeAPIError turns a non-2xx body into an *APIError, preferring the
// provider's structured message over the raw text.
func decodeAPIError(res *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	ae := &APIError{Status: res.StatusCode}
	var envelope struct {
		Error *struct {
			Message string `json:"message"`
			Code    any    `json:"code"`
		} `json:"error"`
		// Some gateways (e.g. SiliconFlow) put the error at the top level.
		Message string `json:"message"`
		Code    any    `json:"code"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		switch {
		case envelope.Error != nil:
			ae.Message = envelope.Error.Message
			ae.Code = stringifyCode(envelope.Error.Code)
		case envelope.Message != "":
			ae.Message = envelope.Message
			ae.Code = stringifyCode(envelope.Code)
		}
	}
	if ae.Message == "" {
		ae.Message = strings.TrimSpace(string(raw))
	}
	if len(ae.Message) > 400 {
		ae.Message = ae.Message[:400] + "…"
	}
	return ae
}

func stringifyCode(code any) string {
	switch v := code.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractMessage pulls choices[0].message.content out of a buffered
// response, reporting provider errors and empty completions clearly.
func extractMessage(raw string, budget int) (string, error) {
	var parsed chatResponse
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", fmt.Errorf("llm: decode response: %w (body: %.200s)", err, raw)
	}
	if parsed.Error != nil {
		return "", &APIError{Status: http.StatusOK, Code: stringifyCode(parsed.Error.Code), Message: parsed.Error.Message}
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("llm: response carried no choices (body: %.200s)", raw)
	}
	choice := parsed.Choices[0]
	content := strings.TrimSpace(choice.Message.Content)
	if content == "" {
		return "", &EmptyContentError{
			FinishReason:   choice.FinishReason,
			Budget:         budget,
			ReasoningChars: len(choice.Message.ReasoningContent),
		}
	}
	return content, nil
}

// parseSSE consumes an OpenAI-style event stream. Reasoning content is
// dropped on purpose (see the package comment).
func parseSSE(body io.Reader, onDelta func(string) error, budget int) outcome {
	scanner := bufio.NewScanner(body)
	// A single frame can be large; grow well past bufio's 64 KiB default.
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)

	var (
		full         strings.Builder
		reasoning    int
		emitted      bool
		finishReason string
	)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue // keep-alive comment / blank frame separator
		}
		if !strings.HasPrefix(line, "data:") {
			continue // `event:` and `id:` lines carry nothing we need
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // tolerate non-JSON frames instead of failing the turn
		}
		if chunk.Error != nil {
			return outcome{text: full.String(), emitted: emitted,
				err: &APIError{Status: http.StatusOK, Code: stringifyCode(chunk.Error.Code), Message: chunk.Error.Message}}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if reason := chunk.Choices[0].FinishReason; reason != "" {
			finishReason = reason
		}
		reasoning += len(chunk.Choices[0].Delta.ReasoningContent)
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		full.WriteString(delta)
		if onDelta != nil {
			emitted = true
			if err := onDelta(delta); err != nil {
				return outcome{text: full.String(), emitted: true, err: err}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if full.Len() > 0 {
			// Partial text already reached the user; keep it rather than
			// discarding a mostly-complete answer.
			return outcome{text: full.String(), emitted: emitted}
		}
		return outcome{err: fmt.Errorf("llm: read stream: %w", err)}
	}
	if full.Len() == 0 {
		return outcome{err: &EmptyContentError{
			FinishReason: finishReason, Budget: budget, ReasoningChars: reasoning,
		}}
	}
	return outcome{text: full.String(), emitted: emitted}
}

// --- JSON helper ------------------------------------------------------------

// ChatJSON runs a blocking completion and decodes the reply into out,
// tolerating ```json fences and surrounding prose. The raw reply is
// returned alongside the error so callers can log what went wrong.
func (c *Client) ChatJSON(ctx context.Context, msgs []Message, out any) (string, error) {
	text, err := c.Chat(ctx, msgs)
	if err != nil {
		return text, err
	}
	payload, ok := ExtractJSON(text)
	if !ok {
		return text, fmt.Errorf("llm: reply contained no JSON object (got: %.200s)", text)
	}
	if err := json.Unmarshal([]byte(payload), out); err != nil {
		return text, fmt.Errorf("llm: decode JSON reply: %w", err)
	}
	return text, nil
}

// ExtractJSON returns the first complete JSON object or array in s: it
// skips a ```json fence, leading prose and trailing commentary, and
// respects string literals and escapes while balancing braces.
func ExtractJSON(s string) (string, bool) {
	start := -1
	var open, close byte
	for i := 0; i < len(s); i++ {
		if s[i] == '{' || s[i] == '[' {
			start = i
			open = s[i]
			break
		}
	}
	if start < 0 {
		return "", false
	}
	if open == '{' {
		close = '}'
	} else {
		close = ']'
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	// Unbalanced: hand back what we have and let Unmarshal produce the
	// error, which is more informative than "no JSON".
	return s[start:], true
}
