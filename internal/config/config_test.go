package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The user-visible promise is that NO credential — and no private endpoint —
// ships in the binary. This is the guard.
//
// It works from an ALLOW-LIST on purpose. An earlier version named the private
// host and the real key prefix so it could assert they were absent, which put
// both back into tracked source: a "negative" test that spells out the secret
// is still a disclosure. Only public, well-known endpoints may be defaults,
// and the model names are never asserted against a literal either.
func TestNoCredentialOrPrivateEndpointIsCompiledIn(t *testing.T) {
	publicEndpoints := map[string]bool{
		"":                              true, // custom: nothing baked in
		"https://api.siliconflow.cn/v1": true,
		"https://api.openai.com/v1":     true,
	}
	for _, provider := range []string{ProviderCustom, ProviderSiliconFlow, ProviderOpenAI} {
		base, model := LLMDefaults(provider)
		if !publicEndpoints[base] {
			t.Fatalf("provider %q defaults to a non-public endpoint: %q", provider, base)
		}
		if provider == ProviderCustom {
			if base != "" || model != "" {
				t.Fatalf("custom must default to nothing, got base=%q model=%q", base, model)
			}
			continue
		}
		if base == "" || model == "" {
			t.Fatalf("provider %q should carry a public default, got base=%q model=%q",
				provider, base, model)
		}
	}

	// A clean load must produce no usable credential of any kind.
	dir := t.TempDir()
	t.Chdir(dir)
	for _, k := range []string{
		"APP_LLM_API_KEY", "APP_LLM_BASE_URL", "APP_LLM_MODEL", "APP_LLM_PROVIDER",
	} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
	cfg := Load()
	if cfg.LLMAPIKey != "" {
		t.Fatalf("a credential was supplied with no configuration: %q", cfg.LLMAPIKey)
	}
	if cfg.LLMBaseURL != "" || cfg.LLMModel != "" {
		t.Fatalf("custom provider must default to nothing, got base=%q model=%q",
			cfg.LLMBaseURL, cfg.LLMModel)
	}
	if cfg.LLMProvider != ProviderCustom {
		t.Fatalf("default provider = %q, want custom", cfg.LLMProvider)
	}
	if len(cfg.LLMEnvPresent) != 0 {
		t.Fatalf("envPresent should be empty in a clean environment: %v", cfg.LLMEnvPresent)
	}
	// The numeric knobs do have defaults — those are not secrets.
	if cfg.LLMTimeoutSec == 0 || cfg.LLMMaxTokens == 0 {
		t.Fatalf("numeric defaults missing: timeout=%d maxTokens=%d",
			cfg.LLMTimeoutSec, cfg.LLMMaxTokens)
	}
}

func TestLLMDefaultsCoverOnlyPublicEndpoints(t *testing.T) {
	base, model := LLMDefaults(ProviderSiliconFlow)
	if base == "" || model == "" {
		t.Fatalf("siliconflow should carry a public endpoint and model: %q %q", base, model)
	}
	base, model = LLMDefaults(ProviderOpenAI)
	if base == "" || model == "" {
		t.Fatalf("openai should carry a public endpoint and model: %q %q", base, model)
	}
	base, model = LLMDefaults("who-knows")
	if base != "" || model != "" {
		t.Fatalf("an unknown provider must not imply an endpoint: %q %q", base, model)
	}
	for _, p := range []string{ProviderCustom, ProviderSiliconFlow, ProviderOpenAI} {
		if !KnownProvider(p) {
			t.Fatalf("KnownProvider(%q) = false", p)
		}
	}
	if KnownProvider("dsh") || KnownProvider("") {
		t.Fatal("the removed provider name must not be accepted")
	}
}

func TestLoadDotEnvParsesAndNeverOverridesRealEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := strings.Join([]string{
		"# a comment",
		"",
		"CFG_PLAIN=value",
		`CFG_QUOTED="a quoted value"`,
		"CFG_SINGLE='single quoted'",
		"export CFG_EXPORTED=exported",
		"CFG_ALREADY_SET=from-file",
		"CFG_EMPTY=",
		"CFG_SPACED = spaced out ",
		"not-a-pair-line",
	}, "\n")
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// A real environment variable must win over the file.
	t.Setenv("CFG_ALREADY_SET", "from-env")
	for _, k := range []string{"CFG_PLAIN", "CFG_QUOTED", "CFG_SINGLE", "CFG_EXPORTED", "CFG_EMPTY", "CFG_SPACED"} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}

	path, applied := LoadDotEnv(envPath)
	if path != envPath {
		t.Fatalf("path = %q, want %q", path, envPath)
	}
	if got := os.Getenv("CFG_PLAIN"); got != "value" {
		t.Fatalf("CFG_PLAIN = %q", got)
	}
	if got := os.Getenv("CFG_QUOTED"); got != "a quoted value" {
		t.Fatalf("double quotes were not stripped: %q", got)
	}
	if got := os.Getenv("CFG_SINGLE"); got != "single quoted" {
		t.Fatalf("single quotes were not stripped: %q", got)
	}
	if got := os.Getenv("CFG_EXPORTED"); got != "exported" {
		t.Fatalf("the `export ` prefix was not handled: %q", got)
	}
	if got := os.Getenv("CFG_ALREADY_SET"); got != "from-env" {
		t.Fatalf("a real environment variable was overwritten: %q", got)
	}
	if _, ok := os.LookupEnv("CFG_EMPTY"); !ok {
		t.Fatal("an explicitly empty value should still be applied (and means 'no override' downstream)")
	}
	if got := os.Getenv("CFG_SPACED"); got != "spaced out" {
		t.Fatalf("surrounding whitespace was not trimmed: %q", got)
	}
	if !contains(applied, "CFG_PLAIN") {
		t.Fatalf("applied list is missing entries: %v", applied)
	}
	if contains(applied, "CFG_ALREADY_SET") {
		t.Fatalf("applied list claims to have set a pre-existing variable: %v", applied)
	}
}

func TestLoadDotEnvMissingFileIsNotAnError(t *testing.T) {
	path, applied := LoadDotEnv(filepath.Join(t.TempDir(), "does-not-exist"))
	if path != "" || applied != nil {
		t.Fatalf("missing file: path=%q applied=%v", path, applied)
	}
}

// End-to-end through Load: .env is read, a real env var still wins, and the
// path is reported so "why didn't my .env apply?" is answerable.
func TestLoadReadsDotEnvAndEnvironmentWins(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"),
		[]byte("APP_PORT=9123\nAPP_LLM_MODEL=from-dotenv\nAPP_LLM_API_KEY=from-dotenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("APP_LLM_MODEL", "from-env")
	_ = os.Unsetenv("APP_LLM_API_KEY")

	cfg := Load()
	if cfg.Port != 9123 {
		t.Fatalf("APP_PORT from .env was not applied: %d", cfg.Port)
	}
	if cfg.LLMModel != "from-env" {
		t.Fatalf("the real environment did not win: %q", cfg.LLMModel)
	}
	if cfg.LLMAPIKey != "from-dotenv" {
		t.Fatalf("the key was not read from .env: %q", cfg.LLMAPIKey)
	}
	if cfg.DotEnvPath == "" {
		t.Fatal("DotEnvPath was not reported")
	}
	if !contains(cfg.LLMEnvPresent, "APP_LLM_API_KEY") {
		t.Fatalf("envPresent is missing the key: %v", cfg.LLMEnvPresent)
	}
}

func TestScrubBootSecretsRemovesTheCredential(t *testing.T) {
	t.Setenv("APP_LLM_API_KEY", "secret-value")
	t.Setenv("APP_DB_DSN", "postgres://user:pw@host/db")
	t.Setenv("APP_LLM_MODEL", "keep-me")

	ScrubBootSecrets()

	if _, ok := os.LookupEnv("APP_LLM_API_KEY"); ok {
		t.Fatal("the API key is still in the process environment")
	}
	if _, ok := os.LookupEnv("APP_DB_DSN"); ok {
		t.Fatal("the DB DSN is still in the process environment")
	}
	if got := os.Getenv("APP_LLM_MODEL"); got != "keep-me" {
		t.Fatalf("a non-secret variable was scrubbed: %q", got)
	}
}

func TestMaskKeyNeverRevealsTheWholeSecret(t *testing.T) {
	// Every input below is SYNTHETIC. Do not paste a real credential here:
	// this file is tracked, and a "looks masked" literal is still a
	// committed partial secret. That mistake was made once — a real key's
	// first 27 characters briefly sat in this very test while it was being
	// written, in the change whose whole purpose was to get credentials out
	// of the source tree. A made-up string proves the same property.
	cases := map[string]string{
		"":                            "",
		"short":                       "****",
		"01234567":                    "****",
		"0123456789":                  "0123…6789",
		"sk-aaaaaaaaaaaaaaaaaaaaaaaa": "sk-a…aaaa",
		"wxyz9876-5555-4444-3333":     "wxyz…3333",
	}
	for in, want := range cases {
		if got := MaskKey(in); got != want {
			t.Fatalf("MaskKey(%q) = %q, want %q", in, got, want)
		}
		if in != "" && len(in) > 8 && strings.Contains(MaskKey(in), in) {
			t.Fatalf("MaskKey leaked the input: %q", MaskKey(in))
		}
	}
}

// The mask must be useless to an attacker no matter how long the secret is:
// at most 4 leading and 4 trailing characters.
func TestMaskKeyExposesAtMostEightCharacters(t *testing.T) {
	masked := MaskKey("SYNTHETIC-" + strings.Repeat("x", 400) + "-SUFFIX")
	if len([]rune(masked)) > 9 { // 4 + ellipsis + 4
		t.Fatalf("mask reveals too much: %q (%d runes)", masked, len([]rune(masked)))
	}
	if strings.Contains(masked, strings.Repeat("x", 20)) {
		t.Fatalf("the mask reproduced a long run of the secret: %q", masked)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
