// Package config loads the bootstrap configuration from APP_* environment
// variables.
//
// There is no config file by design: deployment-time settings belong in
// the deployment manifest (systemd unit, docker-compose, k8s env).
// Everything user-facing (users, api keys, domain data) lives in the
// database — see internal/store.
//
// Every variable has a usable default, so a bare `./app` (or
// `docker run`) boots a working instance on SQLite with zero setup.
package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config is the process-level bootstrap configuration.
type Config struct {
	// HTTP server.
	Port int    // APP_PORT  — default 8080
	Bind string // APP_BIND  — "loopback" (default) or "all"

	// Storage.
	DataDir     string // APP_DATA_DIR      — sqlite home, default ./data
	DBType      string // APP_DB_TYPE       — "sqlite" (default) or "postgres"
	DBDSN       string // APP_DB_DSN        — empty = sqlite at $APP_DATA_DIR/app.db
	AutoMigrate bool   // APP_DB_AUTO_MIGRATE — default true

	// Security.
	CookieSecure bool // APP_COOKIE_SECURE — set true when served over HTTPS
	RateLimitRPM int  // APP_RATE_LIMIT_RPM — per-user requests/minute on /api/*; 0 = unlimited

	// Development.
	//
	// DevProxy, when set (e.g. http://localhost:3000), reverse-proxies
	// everything that is not /api/* or /ws to a `next dev` process. The
	// browser talks to THIS server only, so cookies stay same-origin and
	// frontend HMR works through the proxy (httputil.ReverseProxy handles
	// the WebSocket upgrade Next's dev server needs). Production builds
	// leave it unset and serve the embedded static export instead.
	DevProxy string // APP_DEV_PROXY

	LogLevel string // APP_LOG_LEVEL — "debug" / "info" (default) / "warn" / "error"

	// AI interviewer engine — any OpenAI-compatible
	// `POST {baseURL}/chat/completions` endpoint.
	//
	// These are the values from the environment / .env only. An admin can
	// override any of them at runtime in the UI (stored in the database);
	// the server merges that over these. See docs/API.md §6 and §7.
	LLMProvider    string  // APP_LLM_PROVIDER    — "custom" (default) | "siliconflow" | "openai"
	LLMBaseURL     string  // APP_LLM_BASE_URL
	LLMAPIKey      string  // APP_LLM_API_KEY
	LLMModel       string  // APP_LLM_MODEL
	LLMTimeoutSec  int     // APP_LLM_TIMEOUT_SEC — per-call budget, default 90
	LLMMaxTokens   int     // APP_LLM_MAX_TOKENS  — default 8192
	LLMTemperature float64 // APP_LLM_TEMPERATURE — default 0.4

	// LLMEnvPresent lists the APP_LLM_* names that were actually found (in
	// the real environment or in .env), so the admin UI can explain why a
	// field is falling back instead of guessing.
	LLMEnvPresent []string

	// DotEnvPath is the .env file that was read, "" when there was none.
	// Reported at boot so "why did my variables not apply?" is answerable.
	DotEnvPath string
}

// Provider names. A provider only supplies a default base URL and model —
// never a credential. There is deliberately no provider whose default is a
// private gateway: nothing about a deployment's endpoint belongs in the
// source tree, which is also why no API key appears anywhere in this
// package.
const (
	ProviderCustom      = "custom"
	ProviderSiliconFlow = "siliconflow"
	ProviderOpenAI      = "openai"
)

// LLMDefaults returns the (baseURL, model) a provider implies. A key is
// NEVER defaulted: an unconfigured install has no model and says so, and
// the bundled offline engine keeps the product usable.
func LLMDefaults(provider string) (baseURL, model string) {
	switch provider {
	case ProviderSiliconFlow:
		return "https://api.siliconflow.cn/v1", "Qwen/Qwen2.5-7B-Instruct"
	case ProviderOpenAI:
		return "https://api.openai.com/v1", "gpt-4o-mini"
	default: // ProviderCustom
		return "", ""
	}
}

// KnownProvider reports whether name is a provider this build understands.
func KnownProvider(name string) bool {
	switch name {
	case ProviderCustom, ProviderSiliconFlow, ProviderOpenAI:
		return true
	}
	return false
}

// llmEnvNames is the set of variables the admin panel reports on.
var llmEnvNames = []string{
	"APP_LLM_PROVIDER", "APP_LLM_BASE_URL", "APP_LLM_API_KEY", "APP_LLM_MODEL",
	"APP_LLM_TIMEOUT_SEC", "APP_LLM_MAX_TOKENS", "APP_LLM_TEMPERATURE",
}

// LoadDotEnv loads KEY=VALUE pairs from the first .env it finds and applies
// them to the process environment.
//
// Two rules matter:
//
//   - A variable already present in the REAL environment is never
//     overwritten. That is what lets a container override a .env baked
//     into an image, and it is the only sane precedence.
//   - Only the first existing candidate is read, so a .env next to the
//     running binary does not fight one in the current directory.
//
// It returns the path it used ("" if none) and the names it applied.
func LoadDotEnv(candidates ...string) (path string, applied []string) {
	for _, p := range candidates {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		path = p
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			// Strip one layer of matching quotes, the usual .env convention.
			if len(value) >= 2 {
				if (value[0] == '"' && value[len(value)-1] == '"') ||
					(value[0] == '\'' && value[len(value)-1] == '\'') {
					value = value[1 : len(value)-1]
				}
			}
			if key == "" {
				continue
			}
			if _, exists := os.LookupEnv(key); exists {
				continue // the real environment wins
			}
			if err := os.Setenv(key, value); err != nil {
				continue
			}
			applied = append(applied, key)
		}
		return path, applied
	}
	return "", nil
}

// DotEnvCandidates is the search order: the working directory first (where
// `go run` and `make dev` are invoked from), then next to the binary (how a
// deployed single-file build is usually arranged).
func DotEnvCandidates() []string {
	out := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		out = append(out, filepath.Join(filepath.Dir(exe), ".env"))
	}
	return out
}

// Load reads the bootstrap configuration.
//
// Order: built-in defaults → .env → real environment. .env is read first so
// that everything below can be configured from it, while a real environment
// variable still wins (see LoadDotEnv).
func Load() *Config {
	cfg := &Config{
		Port:        8080,
		Bind:        "loopback",
		DataDir:     "./data",
		DBType:      "sqlite",
		AutoMigrate: true,
		LogLevel:    "info",

		LLMProvider:   ProviderCustom,
		LLMTimeoutSec: 90,
		// Generous by default: reasoning models charge their thinking
		// against this budget, and 2048 was enough for the model to spend
		// the whole allowance on reasoning and emit an empty answer.
		LLMMaxTokens:   8192,
		LLMTemperature: 0.4,
	}

	// .env before anything else is read, since it may set any APP_* value.
	cfg.DotEnvPath, _ = LoadDotEnv(DotEnvCandidates()...)

	if v := os.Getenv("APP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.Port = p
		}
	}
	if v := os.Getenv("APP_BIND"); v != "" {
		cfg.Bind = v
	}
	if v := os.Getenv("APP_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("APP_DB_TYPE"); v != "" {
		cfg.DBType = v
	}
	if v := os.Getenv("APP_DB_DSN"); v != "" {
		cfg.DBDSN = v
	}
	if v := os.Getenv("APP_DB_AUTO_MIGRATE"); v != "" {
		cfg.AutoMigrate = !(v == "false" || v == "0")
	}
	if v := os.Getenv("APP_COOKIE_SECURE"); v != "" {
		cfg.CookieSecure = v == "true" || v == "1"
	}
	if v := os.Getenv("APP_RATE_LIMIT_RPM"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.RateLimitRPM = n
		}
	}
	if v := os.Getenv("APP_DEV_PROXY"); v != "" {
		cfg.DevProxy = v
	}
	if v := os.Getenv("APP_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	// AI interviewer engine. Provider supplies a default endpoint/model
	// (never a key); the explicit variables then override field by field,
	// so "provider preset, different model" is expressible.
	if v := os.Getenv("APP_LLM_PROVIDER"); v != "" && KnownProvider(v) {
		cfg.LLMProvider = v
	}
	defBase, defModel := LLMDefaults(cfg.LLMProvider)
	cfg.LLMBaseURL, cfg.LLMModel = defBase, defModel
	if v := os.Getenv("APP_LLM_BASE_URL"); v != "" {
		cfg.LLMBaseURL = v
	}
	if v := os.Getenv("APP_LLM_API_KEY"); v != "" {
		cfg.LLMAPIKey = v
	}
	if v := os.Getenv("APP_LLM_MODEL"); v != "" {
		cfg.LLMModel = v
	}
	if v := os.Getenv("APP_LLM_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.LLMTimeoutSec = n
		}
	}
	if v := os.Getenv("APP_LLM_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.LLMMaxTokens = n
		}
	}
	if v := os.Getenv("APP_LLM_TEMPERATURE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cfg.LLMTemperature = f
		}
	}
	// Report which of them actually exist, for the admin panel.
	for _, name := range llmEnvNames {
		if _, ok := os.LookupEnv(name); ok {
			cfg.LLMEnvPresent = append(cfg.LLMEnvPresent, name)
		}
	}
	return cfg
}

// ScrubBootSecrets removes credential-bearing env vars from the process
// environment AFTER they have been read into Config. Call once from main
// right after the store is open.
//
// Why: every subprocess spawned later (scripts, editors, debug tooling)
// inherits this env. Anything still set would be readable via
// /proc/<pid>/environ by anything running as the same user. Env is
// treated as one-time bootstrap input, not a live config source.
func ScrubBootSecrets() {
	for _, k := range []string{"APP_DB_DSN", "APP_LLM_API_KEY"} {
		_ = os.Unsetenv(k)
	}
}

// MaskKey renders a credential for display: first 4 and last 4
// characters, elided in between. Anything short is fully masked.
func MaskKey(key string) string {
	r := []rune(key)
	if len(r) == 0 {
		return ""
	}
	if len(r) <= 8 {
		return "****"
	}
	return string(r[:4]) + "…" + string(r[len(r)-4:])
}

// SetupLogging configures the default slog logger from APP_LOG_LEVEL.
func SetupLogging(level string) {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lv})))
}
