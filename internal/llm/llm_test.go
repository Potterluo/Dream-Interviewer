package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		want  string
		found bool
	}{
		{"bare object", `{"a":1}`, `{"a":1}`, true},
		{"fenced", "```json\n{\"a\":1}\n```", `{"a":1}`, true},
		{"fenced no lang", "```\n{\"a\": 1}\n```", `{"a": 1}`, true},
		{"leading prose", "好的，这是结果：{\"a\":1} 请查收", `{"a":1}`, true},
		{"trailing prose", `{"a":1} 以上。`, `{"a":1}`, true},
		{"nested", `{"a":{"b":[1,2,{"c":"}"}]}}`, `{"a":{"b":[1,2,{"c":"}"}]}}`, true},
		{"brace in string", `{"s":"} not the end"}`, `{"s":"} not the end"}`, true},
		{"escaped quote in string", `{"s":"a\"}"}`, `{"s":"a\"}"}`, true},
		{"array", `[1,2,3]`, `[1,2,3]`, true},
		{"array then object picks first", `[1] {"a":1}`, `[1]`, true},
		{"nothing", `no json here`, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found := ExtractJSON(tc.in)
			if found != tc.found {
				t.Fatalf("found = %v, want %v (got %q)", found, tc.found, got)
			}
			if found && got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// sseBody builds an OpenAI-style event stream from content deltas, with
// a reasoning chunk mixed in that must never reach the caller.
func sseBody(deltas ...string) string {
	var b strings.Builder
	for _, d := range deltas {
		chunk := map[string]any{
			"choices": []any{map[string]any{
				"delta": map[string]any{"content": d, "reasoning_content": "SECRET-THOUGHT"},
			}},
		}
		raw, _ := json.Marshal(chunk)
		fmt.Fprintf(&b, "data: %s\n\n", raw)
	}
	b.WriteString("data: [DONE]\n\n")
	return b.String()
}

func TestChatStreamDropsReasoningAndConcatenates(t *testing.T) {
	var gotAuth string
	var gotBody chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sseBody("Hello", " ", "world")))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "k_test", Model: "m", MaxTokens: 128})
	var got strings.Builder
	full, err := c.ChatStream(context.Background(), []Message{User("hi")}, func(d string) error {
		if strings.Contains(d, "SECRET-THOUGHT") {
			t.Errorf("reasoning content leaked into the visible stream: %q", d)
		}
		got.WriteString(d)
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if full != "Hello world" {
		t.Fatalf("full = %q, want %q", full, "Hello world")
	}
	if got.String() != "Hello world" {
		t.Fatalf("deltas = %q, want %q", got.String(), "Hello world")
	}
	if gotAuth != "Bearer k_test" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if !gotBody.Stream {
		t.Fatal("request did not ask for a stream")
	}
	if gotBody.Model != "m" {
		t.Fatalf("model = %q", gotBody.Model)
	}
}

func TestDeltaCallbackErrorStopsTheStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sseBody("aaa", "bbb", "ccc")))
	}))
	defer srv.Close()

	stop := errors.New("stop now")
	c := New(Config{BaseURL: srv.URL, Model: "m"})
	calls := 0
	_, err := c.ChatStream(context.Background(), []Message{User("hi")}, func(string) error {
		calls++
		if calls == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v, want %v", err, stop)
	}
	if calls != 2 {
		t.Fatalf("callback ran %d times after returning an error, want 2", calls)
	}
}

func TestChatExtractsContentAndReportsAPIErrors(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":" PONG ","reasoning_content":"why"}}]}`))
		}))
		defer srv.Close()
		got, err := New(Config{BaseURL: srv.URL, Model: "m"}).Chat(context.Background(), []Message{User("ping")})
		if err != nil {
			t.Fatalf("Chat: %v", err)
		}
		if got != "PONG" {
			t.Fatalf("got %q, want PONG", got)
		}
	})

	t.Run("http error surfaces provider message", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = w.Write([]byte(`{"code":30001,"message":"Sorry, your account balance is insufficient"}`))
		}))
		defer srv.Close()
		_, err := New(Config{BaseURL: srv.URL, Model: "m"}).Chat(context.Background(), []Message{User("ping")})
		var ae *APIError
		if !errors.As(err, &ae) {
			t.Fatalf("err = %v, want *APIError", err)
		}
		if ae.Status != http.StatusPaymentRequired {
			t.Fatalf("status = %d", ae.Status)
		}
		if !strings.Contains(ae.Message, "insufficient") {
			t.Fatalf("message = %q", ae.Message)
		}
		// 402 is not transient: retrying would just double the latency.
		if Retryable(err) {
			t.Fatal("402 must not be retryable")
		}
	})

	t.Run("empty content is an error, not an empty question", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"","reasoning_content":"thinking only"}}]}`))
		}))
		defer srv.Close()
		_, err := New(Config{BaseURL: srv.URL, Model: "m"}).Chat(context.Background(), []Message{User("x")})
		var empty *EmptyContentError
		if !errors.As(err, &empty) {
			t.Fatalf("err = %v, want *EmptyContentError", err)
		}
		if empty.ReasoningChars == 0 {
			t.Fatal("the reasoning channel size should be reported for diagnosis")
		}
		if Retryable(err) {
			t.Fatal("a deterministic empty completion must not be blindly retried")
		}
	})
}

// A reasoning model charges its thinking against max_tokens. When it
// exhausts the budget it emits empty content with finish_reason "length";
// the client must escalate the budget instead of returning nothing, or a
// whole interview silently degrades to the offline rubric.
func TestEscalatesTokenBudgetForReasoningModels(t *testing.T) {
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body chatRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		budgets = append(budgets, body.MaxTokens)
		if len(budgets) == 1 {
			// First attempt: reasoning ate the whole budget.
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"long thinking"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"ok\":true}"}}]}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Model: "m", MaxTokens: 1024})
	got, err := c.Chat(context.Background(), []Message{User("x")})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != `{"ok":true}` {
		t.Fatalf("got %q", got)
	}
	if len(budgets) != 2 {
		t.Fatalf("attempts = %d, want 2 (one escalation)", len(budgets))
	}
	if budgets[0] != 1024 || budgets[1] != 2048 {
		t.Fatalf("budgets = %v, want [1024 2048]", budgets)
	}
}

func TestEscalationStopsAtTheCeiling(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"x"}}]}`))
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, Model: "m", MaxTokens: maxTokensCeiling}).Chat(context.Background(), []Message{User("x")})
	var empty *EmptyContentError
	if !errors.As(err, &empty) {
		t.Fatalf("err = %v, want *EmptyContentError", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (already at the ceiling)", attempts)
	}
	if !strings.Contains(err.Error(), "APP_LLM_MAX_TOKENS") {
		t.Fatalf("the error must tell the operator how to fix it: %v", err)
	}
}

func TestRetriesTransientFailureOnce(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"overloaded"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Model: "m", Timeout: 5 * time.Second})
	got, err := c.Chat(context.Background(), []Message{User("hi")})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != "ok" {
		t.Fatalf("got %q", got)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestChatJSONToleratesFences(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload := "这是结果：\n```json\n{\"score\": 88.5, \"verdict\": \"strong\"}\n```\n希望对你有帮助。"
		body, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": payload}}},
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	var out struct {
		Score   float64 `json:"score"`
		Verdict string  `json:"verdict"`
	}
	if _, err := New(Config{BaseURL: srv.URL, Model: "m"}).ChatJSON(context.Background(), []Message{User("x")}, &out); err != nil {
		t.Fatalf("ChatJSON: %v", err)
	}
	if out.Score != 88.5 || out.Verdict != "strong" {
		t.Fatalf("decoded = %+v", out)
	}
}

func TestUnconfiguredClientFailsClearly(t *testing.T) {
	c := New(Config{})
	if c.Configured() {
		t.Fatal("an empty config must not report itself as configured")
	}
	_, err := c.Chat(context.Background(), []Message{User("hi")})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("err = %v, want a clear not-configured error", err)
	}
}

func TestParseSSESkipsKeepalivesAndGarbage(t *testing.T) {
	body := ": keep-alive\n\ndata: not json\n\n" + sseBody("x")
	res := parseSSE(strings.NewReader(body), nil, 0)
	if res.err != nil {
		t.Fatalf("parseSSE: %v", res.err)
	}
	if res.text != "x" {
		t.Fatalf("text = %q, want x", res.text)
	}
}

// The stream path has the same reasoning-budget failure mode, and the
// escalation is only safe while nothing has been emitted.
func TestStreamEscalatesWhenNothingWasEmitted(t *testing.T) {
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body chatRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		budgets = append(budgets, body.MaxTokens)
		if len(budgets) == 1 {
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n"))
			return
		}
		_, _ = w.Write([]byte(sseBody("answer")))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Model: "m", MaxTokens: 512})
	deltas := 0
	full, err := c.ChatStream(context.Background(), []Message{User("x")}, func(string) error {
		deltas++
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if full != "answer" || deltas != 1 {
		t.Fatalf("full=%q deltas=%d", full, deltas)
	}
	if len(budgets) != 2 || budgets[1] != 1024 {
		t.Fatalf("budgets = %v, want [512 1024]", budgets)
	}
}

func TestStreamDoesNotReplayAfterEmitting(t *testing.T) {
	// Once a fragment reached the candidate, re-requesting would show it
	// twice — the failure must be final.
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Model: "m", MaxTokens: 512})
	full, err := c.ChatStream(context.Background(), []Message{User("x")}, func(string) error { return nil })
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if full != "partial" {
		t.Fatalf("full = %q", full)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (no replay after partial output)", attempts)
	}
}
