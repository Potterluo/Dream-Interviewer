package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Potterluo/dream-interviewer/internal/auth"
	"github.com/Potterluo/dream-interviewer/internal/config"
	"github.com/Potterluo/dream-interviewer/internal/interview"
	"github.com/Potterluo/dream-interviewer/internal/llm"
)

// settings.go resolves the interviewer model configuration and serves the
// admin panel behind it (docs/API.md §6 and §7).
//
// THE POINT OF THIS FILE: no credential is compiled into the binary. The
// endpoint, the model and the key come from three places, and the merge is
// explicit and reportable so "why is it using that model?" is answerable
// from the UI rather than by reading a deployment.
//
// Precedence, per field, highest first:
//
//  1. database  — set by an admin in the UI
//  2. env/.env  — for headless deployments
//  3. default   — only the numeric knobs have one
//
// Fields are merged INDEPENDENTLY. Storing just a provider name therefore
// does not silently rewrite the endpoint behind the admin's back — the UI
// prefills the endpoint when a provider is picked, and what you see is what
// is stored. Surprise-by-default is the thing this avoids.

// Setting keys. Dotted and flat: a nested blob would be harder to update
// field-by-field, which is exactly what the admin panel does.
const (
	settingProvider    = "llm.provider"
	settingBaseURL     = "llm.base_url"
	settingAPIKey      = "llm.api_key"
	settingModel       = "llm.model"
	settingTimeoutSec  = "llm.timeout_sec"
	settingMaxTokens   = "llm.max_tokens"
	settingTemperature = "llm.temperature"
)

// Clamps for the numeric knobs, applied to values read back out of the
// database. The admin API REJECTS an out-of-range value with a 400 instead of
// clamping it (see handleUpdateAdminSettings): silently storing something
// other than what was typed is worse than an error. These clamps exist for
// rows that never went through the API — a hand-edited database, say.
const (
	minTimeoutSec = 5
	maxTimeoutSec = 600
	minMaxTokens  = 256
	maxMaxTokens  = 131072
	minTemp       = 0
	maxTemp       = 2
)

// llmSettings is the effective configuration. The API key lives here so the
// server can use it, and is never serialised — see toSettingsDTO.
type llmSettings struct {
	Provider    string
	BaseURL     string
	Model       string
	APIKey      string
	TimeoutSec  int
	MaxTokens   int
	Temperature float64
}

// Configured reports whether a model call is possible at all. A missing key
// is not disqualifying: local servers (Ollama, vLLM) accept none.
func (l llmSettings) Configured() bool {
	return l.BaseURL != "" && l.Model != ""
}

// effectiveLLM merges the database over env/.env over defaults, and reports
// which fields an admin has overridden.
func (s *Server) effectiveLLM(ctx context.Context) (llmSettings, map[string]bool, error) {
	stored, err := s.store.AllSettings(ctx)
	if err != nil {
		return llmSettings{}, nil, err
	}
	out := llmSettings{
		Provider:    s.cfg.LLMProvider,
		BaseURL:     s.cfg.LLMBaseURL,
		Model:       s.cfg.LLMModel,
		APIKey:      s.cfg.LLMAPIKey,
		TimeoutSec:  s.cfg.LLMTimeoutSec,
		MaxTokens:   s.cfg.LLMMaxTokens,
		Temperature: s.cfg.LLMTemperature,
	}
	overridden := map[string]bool{}

	// A stored empty string is treated as "no override", so clearing a field
	// in the UI (which sends "") behaves like deleting it.
	take := func(key string) (string, bool) {
		v, ok := stored[key]
		if !ok || strings.TrimSpace(v) == "" {
			return "", false
		}
		return v, true
	}

	if v, ok := take(settingProvider); ok {
		out.Provider, overridden["provider"] = v, true
	}
	if v, ok := take(settingBaseURL); ok {
		out.BaseURL, overridden["baseUrl"] = strings.TrimRight(v, "/"), true
	}
	if v, ok := take(settingModel); ok {
		out.Model, overridden["model"] = v, true
	}
	if v, ok := take(settingAPIKey); ok {
		out.APIKey, overridden["apiKey"] = v, true
	}
	if v, ok := take(settingTimeoutSec); ok {
		if n, err := strconv.Atoi(v); err == nil {
			out.TimeoutSec = clampInt(n, minTimeoutSec, maxTimeoutSec)
			overridden["timeoutSec"] = true
		}
	}
	if v, ok := take(settingMaxTokens); ok {
		if n, err := strconv.Atoi(v); err == nil {
			out.MaxTokens = clampInt(n, minMaxTokens, maxMaxTokens)
			overridden["maxTokens"] = true
		}
	}
	if v, ok := take(settingTemperature); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			out.Temperature = clampFloat(f, minTemp, maxTemp)
			overridden["temperature"] = true
		}
	}
	return out, overridden, nil
}

// buildEngine constructs the engine for a resolved configuration.
func buildEngine(l llmSettings) *interview.Engine {
	return interview.NewEngine(llm.New(llm.Config{
		BaseURL:     l.BaseURL,
		APIKey:      l.APIKey,
		Model:       l.Model,
		Timeout:     time.Duration(l.TimeoutSec) * time.Second,
		MaxTokens:   l.MaxTokens,
		Temperature: l.Temperature,
	}))
}

// engineHolder lets the running server swap its engine when an admin saves
// new settings, without a restart.
//
// What it guarantees, precisely: the pointer is read under a lock, so a swap
// can never race a read or tear. What it does NOT guarantee is that one
// request sees a single engine for its whole lifetime — the advance handler
// reads it once per phase (plan, grade, report), so an admin saving settings
// mid-interview can change the model between phases. That is accepted: each
// phase is internally consistent, the interview stays valid, and the
// alternative (pinning an engine for the duration of a long stream) would
// make "I fixed the model config" appear to do nothing.
type engineHolder struct {
	mu     sync.RWMutex
	engine *interview.Engine
}

func newEngineHolder(e *interview.Engine) *engineHolder { return &engineHolder{engine: e} }

func (h *engineHolder) get() *interview.Engine {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.engine
}

func (h *engineHolder) set(e *interview.Engine) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.engine = e
}

// reloadEngine recomputes the configuration and installs a fresh engine.
// Called at boot and after every settings write.
//
// It logs what it resolved (never the key) because "which model is actually
// in use?" is the first question when an interview behaves oddly, and the
// answer should not require reading the database.
func (s *Server) reloadEngine(ctx context.Context) (llmSettings, map[string]bool, error) {
	settings, overridden, err := s.effectiveLLM(ctx)
	if err != nil {
		return llmSettings{}, nil, err
	}
	s.engine.set(buildEngine(settings))

	source := "default"
	switch {
	case len(overridden) > 0:
		source = "database (set by an admin in the UI)"
	case len(s.cfg.LLMEnvPresent) > 0:
		source = "environment/.env"
	}
	slog.Info("interviewer model",
		"configured", settings.Configured(),
		"provider", settings.Provider,
		"base_url", settings.BaseURL,
		"model", settings.Model,
		"api_key_set", settings.APIKey != "",
		"max_tokens", settings.MaxTokens,
		"timeout_sec", settings.TimeoutSec,
		"source", source,
	)
	if !settings.Configured() {
		slog.Info("no interviewer model configured — interviews will use the " +
			"bundled question bank and the deterministic rubric; set one at /admin/model/ " +
			"or via .env")
	}
	return settings, overridden, nil
}

// --- DTOs -------------------------------------------------------------------

type settingsDTO struct {
	Provider     string          `json:"provider"`
	BaseURL      string          `json:"baseUrl"`
	Model        string          `json:"model"`
	APIKeyMasked string          `json:"apiKeyMasked"`
	APIKeySet    bool            `json:"apiKeySet"`
	TimeoutSec   int             `json:"timeoutSec"`
	MaxTokens    int             `json:"maxTokens"`
	Temperature  float64         `json:"temperature"`
	Configured   bool            `json:"configured"`
	Overridden   map[string]bool `json:"overridden"`
	EnvPresent   []string        `json:"envPresent"`
}

func (s *Server) toSettingsDTO(l llmSettings, overridden map[string]bool) settingsDTO {
	env := s.cfg.LLMEnvPresent
	if env == nil {
		env = []string{}
	}
	if overridden == nil {
		overridden = map[string]bool{}
	}
	return settingsDTO{
		Provider:     l.Provider,
		BaseURL:      l.BaseURL,
		Model:        l.Model,
		APIKeyMasked: config.MaskKey(l.APIKey),
		APIKeySet:    l.APIKey != "",
		TimeoutSec:   l.TimeoutSec,
		MaxTokens:    l.MaxTokens,
		Temperature:  l.Temperature,
		Configured:   l.Configured(),
		Overridden:   overridden,
		EnvPresent:   env,
	}
}

// --- handlers ---------------------------------------------------------------

func (s *Server) handleGetAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	settings, overridden, err := s.effectiveLLM(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, map[string]any{"settings": s.toSettingsDTO(settings, overridden)})
}

// adminSettingsRequest uses pointers so "absent" is distinguishable from
// "empty": absent keeps the current value, empty clears the override.
//
// The numeric fields use clearableInt/clearableFloat rather than *int, because
// docs/API.md §6 promises the absent-vs-"" rule is UNIFORM across fields — and
// a client clearing a number reasonably sends "". Decoding that straight into
// *int would 400, so the documented rule would be a lie.
type adminSettingsRequest struct {
	Provider    *string         `json:"provider"`
	BaseURL     *string         `json:"baseUrl"`
	Model       *string         `json:"model"`
	APIKey      *string         `json:"apiKey"`
	TimeoutSec  *clearableInt   `json:"timeoutSec"`
	MaxTokens   *clearableInt   `json:"maxTokens"`
	Temperature *clearableFloat `json:"temperature"`
}

// clearableInt accepts a number, or "" meaning "remove the override".
// JSON null is indistinguishable from absent once it reaches a pointer, so
// null is treated as absent (leave the value alone).
type clearableInt struct {
	Value int
	Clear bool
}

func (c *clearableInt) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == `""` || s == "null" {
		c.Clear = s == `""`
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return errors.New("expected a number or \"\" to clear")
	}
	c.Value = n
	return nil
}

// clearableFloat is the same idea for temperature.
type clearableFloat struct {
	Value float64
	Clear bool
}

func (c *clearableFloat) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == `""` || s == "null" {
		c.Clear = s == `""`
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return errors.New("expected a number or \"\" to clear")
	}
	c.Value = f
	return nil
}

func (s *Server) handleUpdateAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req adminSettingsRequest
	if !readJSON(w, r, &req) {
		return
	}

	// Validate first, write second: a rejected request must not leave half
	// the settings applied. A field being CLEARED skips range validation —
	// removing a value cannot be out of range.
	if req.Provider != nil && *req.Provider != "" && !config.KnownProvider(*req.Provider) {
		writeError(w, http.StatusBadRequest,
			"unknown provider (want custom, siliconflow or openai)")
		return
	}
	if req.TimeoutSec != nil && !req.TimeoutSec.Clear &&
		(req.TimeoutSec.Value < minTimeoutSec || req.TimeoutSec.Value > maxTimeoutSec) {
		writeError(w, http.StatusBadRequest, "timeoutSec must be between 5 and 600")
		return
	}
	if req.MaxTokens != nil && !req.MaxTokens.Clear &&
		(req.MaxTokens.Value < minMaxTokens || req.MaxTokens.Value > maxMaxTokens) {
		writeError(w, http.StatusBadRequest, "maxTokens must be between 256 and 131072")
		return
	}
	if req.Temperature != nil && !req.Temperature.Clear &&
		(req.Temperature.Value < minTemp || req.Temperature.Value > maxTemp) {
		writeError(w, http.StatusBadRequest, "temperature must be between 0 and 2")
		return
	}

	ctx := r.Context()
	type write struct {
		key   string
		value string
	}
	var writes []write
	var deletes []string

	apply := func(key string, provided bool, value string) {
		if !provided {
			return
		}
		if strings.TrimSpace(value) == "" {
			deletes = append(deletes, key)
			return
		}
		writes = append(writes, write{key, value})
	}

	if req.Provider != nil {
		apply(settingProvider, true, *req.Provider)
	}
	if req.BaseURL != nil {
		apply(settingBaseURL, true, strings.TrimSpace(*req.BaseURL))
	}
	if req.Model != nil {
		apply(settingModel, true, strings.TrimSpace(*req.Model))
	}
	if req.APIKey != nil {
		// A key is never trimmed of internal characters; only surrounding
		// whitespace, which is always a paste artefact.
		apply(settingAPIKey, true, strings.TrimSpace(*req.APIKey))
	}
	if req.TimeoutSec != nil {
		if req.TimeoutSec.Clear {
			deletes = append(deletes, settingTimeoutSec)
		} else {
			writes = append(writes, write{settingTimeoutSec, strconv.Itoa(req.TimeoutSec.Value)})
		}
	}
	if req.MaxTokens != nil {
		if req.MaxTokens.Clear {
			deletes = append(deletes, settingMaxTokens)
		} else {
			writes = append(writes, write{settingMaxTokens, strconv.Itoa(req.MaxTokens.Value)})
		}
	}
	if req.Temperature != nil {
		if req.Temperature.Clear {
			deletes = append(deletes, settingTemperature)
		} else {
			writes = append(writes, write{settingTemperature,
				strconv.FormatFloat(req.Temperature.Value, 'f', -1, 64)})
		}
	}

	for _, d := range deletes {
		if err := s.store.DeleteSetting(ctx, d); err != nil {
			storeError(w, err)
			return
		}
	}
	for _, wr := range writes {
		if err := s.store.PutSetting(ctx, wr.key, wr.value); err != nil {
			storeError(w, err)
			return
		}
	}

	settings, overridden, err := s.reloadEngine(ctx)
	if err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, map[string]any{"settings": s.toSettingsDTO(settings, overridden)})
}

func (s *Server) handleDeleteAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if err := s.store.ClearSettings(r.Context()); err != nil {
		storeError(w, err)
		return
	}
	settings, overridden, err := s.reloadEngine(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	writeOK(w, map[string]any{"settings": s.toSettingsDTO(settings, overridden)})
}

// adminTestRequest optionally overrides fields for the probe, so a config
// can be validated BEFORE it is saved.
type adminTestRequest struct {
	Provider   *string `json:"provider"`
	BaseURL    *string `json:"baseUrl"`
	Model      *string `json:"model"`
	APIKey     *string `json:"apiKey"`
	TimeoutSec *int    `json:"timeoutSec"`
	MaxTokens  *int    `json:"maxTokens"`
}

func (s *Server) handleTestAdminSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req adminTestRequest
	if r.ContentLength > 0 {
		if !readJSON(w, r, &req) {
			return
		}
	}
	tested := llmSettings{
		Provider: s.cfg.LLMProvider, BaseURL: s.cfg.LLMBaseURL, Model: s.cfg.LLMModel,
		APIKey: s.cfg.LLMAPIKey, TimeoutSec: s.cfg.LLMTimeoutSec,
		MaxTokens: s.cfg.LLMMaxTokens, Temperature: s.cfg.LLMTemperature,
	}
	if stored, _, err := s.effectiveLLM(r.Context()); err == nil {
		tested = stored
	} else {
		storeError(w, err)
		return
	}

	override := false
	applyStr := func(dst *string, src *string) {
		if src == nil {
			return
		}
		*dst = strings.TrimSpace(*src)
		override = true
	}
	applyStr(&tested.Provider, req.Provider)
	applyStr(&tested.BaseURL, req.BaseURL)
	applyStr(&tested.Model, req.Model)
	// An omitted key falls back to the stored one, which is what makes
	// "test without re-typing the key" work.
	applyStr(&tested.APIKey, req.APIKey)
	if req.TimeoutSec != nil {
		tested.TimeoutSec = clampInt(*req.TimeoutSec, minTimeoutSec, maxTimeoutSec)
		override = true
	}
	if req.MaxTokens != nil {
		tested.MaxTokens = clampInt(*req.MaxTokens, minMaxTokens, maxMaxTokens)
		override = true
	}
	tested.BaseURL = strings.TrimRight(tested.BaseURL, "/")

	probe := buildEngine(tested)
	// A probe must not be able to hang the UI; cap it well below the normal
	// generation budget.
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result := probe.Test(ctx, "")
	writeOK(w, map[string]any{
		"result": map[string]any{
			"ok":             result.OK,
			"latencyMs":      result.LatencyMs,
			"model":          result.Model,
			"reply":          result.Reply,
			"error":          result.Error,
			"testedOverride": override,
		},
	})
}

// --- interview-engine (read-only) -------------------------------------------

// engineStatusDTO is the everyone-can-see description of the current model.
// It is read-only, so it carries no `overridden` map — that is an admin
// concept; a normal user only needs to know which model assessed them.
//
// `baseUrl` and the key fields are populated for ADMINS ONLY and are empty
// strings for everyone else (see handleInterviewEngine), so a client must
// treat them as optional.
type engineStatusDTO struct {
	Configured        bool    `json:"configured"`
	Provider          string  `json:"provider"`
	BaseURL           string  `json:"baseUrl"`
	Model             string  `json:"model"`
	APIKeyMasked      string  `json:"apiKeyMasked"`
	APIKeySet         bool    `json:"apiKeySet"`
	Source            string  `json:"source"`
	FallbackAvailable bool    `json:"fallbackAvailable"`
	TimeoutSec        int     `json:"timeoutSec"`
	MaxTokens         int     `json:"maxTokens"`
	Temperature       float64 `json:"temperature"`
}

// handleInterviewEngine is available to every authenticated user: you should
// be able to see WHICH MODEL assessed you. It is read-only — only an admin
// can change it (see the /api/admin/settings routes).
//
// The endpoint and the key mask are withheld from non-admins. A regular user
// gains nothing from the provider's URL, and the deployment's endpoint is the
// operator's business — this product exists because credentials and private
// endpoints should not be handed out casually. Admins, who own that
// configuration, see the full picture.
func (s *Server) handleInterviewEngine(w http.ResponseWriter, r *http.Request) {
	ident, _ := auth.FromContext(r.Context())
	settings, overridden, err := s.effectiveLLM(r.Context())
	if err != nil {
		storeError(w, err)
		return
	}
	source := "default"
	if len(overridden) > 0 {
		source = "db"
	} else if len(s.cfg.LLMEnvPresent) > 0 {
		source = "env"
	}

	dto := engineStatusDTO{
		Configured:        settings.Configured(),
		Provider:          settings.Provider,
		Model:             settings.Model,
		Source:            source,
		FallbackAvailable: true,
		TimeoutSec:        settings.TimeoutSec,
		MaxTokens:         settings.MaxTokens,
		Temperature:       settings.Temperature,
	}
	if ident.IsAdmin() {
		dto.BaseURL = settings.BaseURL
		dto.APIKeyMasked = config.MaskKey(settings.APIKey)
		dto.APIKeySet = settings.APIKey != ""
	}
	writeOK(w, map[string]any{"engine": dto})
}

// --- helpers ----------------------------------------------------------------

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
