package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/deqiying/onesearch/internal/providers"
)

type deepSeekTestServer struct {
	*httptest.Server
	mu       sync.Mutex
	payloads []map[string]any
	paths    []string
}

func (s *deepSeekTestServer) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.paths)
}

func newDeepSeekTestServer(t *testing.T, handler func(path string) (int, string)) *deepSeekTestServer {
	t.Helper()
	server := &deepSeekTestServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(data, &payload)
		server.mu.Lock()
		server.paths = append(server.paths, r.URL.Path)
		server.payloads = append(server.payloads, payload)
		server.mu.Unlock()
		status, body := handler(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func deepSeekProviderConfig(baseURL string, enabled any) map[string]any {
	return map[string]any{
		"enabled":      enabled,
		"adapter":      "deepseek_tools",
		"capabilities": []any{"source_search", "page_fetch"},
		"base_url":     baseURL,
		"api_key":      "deepseek-secret-key",
	}
}

func TestSearchExtraSourcesUsesDeepSeekWithTruncatedQueryAndClampedCount(t *testing.T) {
	deepseek := newDeepSeekTestServer(t, func(path string) (int, string) {
		if path != "/v1/tools/search" {
			return http.StatusNotFound, `{"error":{"message":"bad path"}}`
		}
		return http.StatusOK, `{"results":[{"title":"DeepSeek source","url":"https://deepseek.example/a","snippet":"short snippet","summary":"long summary","content":"","site_name":"Example","date":null,"rerank_score":2.1,"authority":3}]}`
	})
	openai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"output":"answer","results":[]}`))
	}))
	defer openai.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes": map[string]any{
			"answer_search": []any{"openai_responses"},
			"source_search": []any{"deepseek"},
		},
		"profiles": map[string]any{
			"standard": map[string]any{"required_capabilities": []any{"answer_search"}},
		},
		"providers": map[string]any{
			"openai_responses": map[string]any{
				"enabled":      true,
				"adapter":      "openai_responses",
				"capabilities": []any{"answer_search"},
				"base_url":     openai.URL,
				"api_key":      "test-key",
				"settings":     map[string]any{"model": "gpt-test"},
			},
			"deepseek": deepSeekProviderConfig(deepseek.URL, true),
		},
	}); err != nil {
		t.Fatal(err)
	}

	longQuery := strings.Repeat("长", 140)
	got := New(cfg).Search(t.Context(), longQuery, SearchOptions{Validation: "fast", Providers: "openai_responses", ProviderFilters: map[string]string{"source_search": "deepseek"}, ExtraSources: 80})
	if got["ok"] != true {
		t.Fatalf("search failed: %#v", got)
	}
	if deepseek.calls() != 1 {
		t.Fatalf("deepseek calls = %d", deepseek.calls())
	}
	payload := deepseek.payloads[0]
	query, _ := payload["query"].(string)
	if utf8.RuneCountInString(query) != 128 || payload["count"] != float64(50) || payload["need_content"] != false {
		t.Fatalf("payload = %#v (query runes %d)", payload, utf8.RuneCountInString(query))
	}
	for _, key := range []string{"ori_question", "localization"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("workflow payload must not contain %s: %#v", key, payload)
		}
	}

	var found bool
	for _, item := range got["used"].([]map[string]any) {
		if item["capability"] != "source_search" {
			continue
		}
		for _, provider := range item["providers"].([]map[string]any) {
			if provider["provider"] == "deepseek" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("used missing source_search/deepseek: %#v", got["used"])
	}
	extra := got["extra_sources"].([]map[string]any)
	if len(extra) != 1 || extra[0]["description"] != "short snippet" || extra[0]["provider"] != "deepseek" {
		t.Fatalf("extra_sources = %#v", extra)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "deepseek-secret-key") {
		t.Fatalf("search output leaks API key: %s", encoded)
	}
}

func TestFetchFallsBackToDeepSeekAndKeepsTypedAttempts(t *testing.T) {
	tavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer tavily.Close()
	deepseek := newDeepSeekTestServer(t, func(path string) (int, string) {
		if path != "/v1/tools/fetch" {
			return http.StatusNotFound, `{"error":{"message":"bad path"}}`
		}
		return http.StatusOK, `{"title":"Article","content":"# Article\n\nbody"}`
	})

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes":         map[string]any{"page_fetch": []any{"tavily", "deepseek"}},
		"providers": map[string]any{
			"tavily": map[string]any{
				"enabled":      true,
				"adapter":      "tavily",
				"capabilities": []any{"page_fetch"},
				"base_url":     tavily.URL,
				"api_key":      "tavily-key",
			},
			"deepseek": deepSeekProviderConfig(deepseek.URL, "auto"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	svc := New(cfg)
	got := svc.Fetch(t.Context(), "https://example.com/article", FetchOptions{})
	if got["ok"] != true || got["provider"] != "deepseek" || got["content"] != "# Article\n\nbody" || got["fallback_used"] != true {
		t.Fatalf("fetch result = %#v", got)
	}
	attempts := got["provider_attempts"].([]map[string]any)
	if len(attempts) != 2 || attempts[0]["provider"] != "tavily" || attempts[0]["status"] != "error" || attempts[1]["provider"] != "deepseek" || attempts[1]["status"] != "ok" {
		t.Fatalf("attempts = %#v", attempts)
	}
	if deepseek.payloads[0]["url"] != "https://example.com/article" || len(deepseek.payloads[0]) != 1 {
		t.Fatalf("fetch payload = %#v", deepseek.payloads[0])
	}

	explicit := svc.Fetch(t.Context(), "https://example.com/article", FetchOptions{Provider: "deepseek"})
	if explicit["ok"] != true || len(explicit["provider_attempts"].([]map[string]any)) != 1 {
		t.Fatalf("fetch --provider deepseek = %#v", explicit)
	}

	// --fallback off 只尝试第一个 provider。
	_, offAttempts := svc.runWebFetchFallback(t.Context(), "https://example.com/article", "auto", "off")
	if len(offAttempts) != 1 || offAttempts[0].Provider != "tavily" {
		t.Fatalf("fallback off attempts = %#v", offAttempts)
	}
}

func TestDeepSeekFetch404AttemptKeepsEmptyResultAndContinues(t *testing.T) {
	deepseek := newDeepSeekTestServer(t, func(string) (int, string) {
		return http.StatusNotFound, `{"error":{"message":"no usable content","type":"not_found_error","code":"not_found","param":null}}`
	})
	exa := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com","title":"Example","text":"exa page body"}]}`))
	}))
	defer exa.Close()

	clearProviderEnv(t)
	cfg := testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes":         map[string]any{"page_fetch": []any{"deepseek", "exa"}},
		"providers": map[string]any{
			"deepseek": deepSeekProviderConfig(deepseek.URL, true),
			"exa": map[string]any{
				"enabled":      true,
				"adapter":      "exa",
				"capabilities": []any{"page_fetch"},
				"base_url":     exa.URL,
				"api_key":      "exa-key",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := New(cfg).Fetch(t.Context(), "https://example.com", FetchOptions{})
	if got["ok"] != true || got["provider"] != "exa" {
		t.Fatalf("fetch result = %#v", got)
	}
	first := got["provider_attempts"].([]map[string]any)[0]
	if first["provider"] != "deepseek" || first["status"] != "error" || first["error_type"] != "empty_result" {
		t.Fatalf("deepseek attempt = %#v", first)
	}
	if !strings.Contains(stringValue(first["error"]), "DeepSeek fetch: no usable content") {
		t.Fatalf("deepseek attempt error = %#v", first)
	}
}

func TestDeepSeekDisabledByDefaultSendsNoRequests(t *testing.T) {
	deepseek := newDeepSeekTestServer(t, func(string) (int, string) {
		return http.StatusOK, `{"results":[],"title":"","content":"x"}`
	})
	clearProviderEnv(t)
	t.Setenv("DEEPSEEK_API_KEY", "env-deepseek-key")
	cfg := testConfig(t)
	// 只覆盖 base_url，enabled 继承内置默认值 false。
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes":         map[string]any{},
		"providers":      map[string]any{"deepseek": map[string]any{"base_url": deepseek.URL}},
	}); err != nil {
		t.Fatal(err)
	}
	svc := New(cfg)
	for _, capability := range []string{"source_search", "page_fetch"} {
		if got := svc.resolvedProviders(capability, "deepseek"); len(got) != 0 {
			t.Fatalf("%s resolved = %#v, want none", capability, got)
		}
	}
	fetch := svc.Fetch(t.Context(), "https://example.com", FetchOptions{Provider: "deepseek"})
	if fetch["ok"] != false || fetch["error_type"] != "config_error" {
		t.Fatalf("fetch = %#v", fetch)
	}
	if sources, attempts := svc.extraSources(t.Context(), "q", 5, "deepseek"); len(sources) != 0 || len(attempts) != 0 {
		t.Fatalf("extra sources = %#v %#v", sources, attempts)
	}
	if deepseek.calls() != 0 {
		t.Fatalf("disabled deepseek received %d requests", deepseek.calls())
	}
}

func TestDeepSeekDirectMethods(t *testing.T) {
	clearProviderEnv(t)
	cfg := testConfig(t)
	svc := New(cfg)
	if got := svc.DeepSeekSearch(t.Context(), "q", providersDeepSeekOptions(10, "")); got["ok"] != false || got["error_type"] != "config_error" || got["tool"] != "deepseek_search" {
		t.Fatalf("search without key = %#v", got)
	}
	if got := svc.DeepSeekFetch(t.Context(), "https://example.com"); got["ok"] != false || got["error_type"] != "config_error" || got["tool"] != "deepseek_fetch" {
		t.Fatalf("fetch without key = %#v", got)
	}

	deepseek := newDeepSeekTestServer(t, func(path string) (int, string) {
		if path == "/v1/tools/fetch" {
			return http.StatusOK, `{"title":"T","content":"body"}`
		}
		return http.StatusOK, `{"results":[{"title":"A","url":"https://a.example","snippet":"s"}]}`
	})
	// direct 方法只检查 key，与其他 HTTP provider 一致；enabled=false 不阻止显式调用。
	cfg = testConfig(t)
	if err := cfg.SetFile(map[string]any{
		"schema_version": 1,
		"routes":         map[string]any{},
		"providers":      map[string]any{"deepseek": deepSeekProviderConfig(deepseek.URL, false)},
	}); err != nil {
		t.Fatal(err)
	}
	svc = New(cfg)
	search := svc.DeepSeekSearch(t.Context(), "q", providersDeepSeekOptions(3, "198.51.100.23"))
	if search["ok"] != true || search["total"] != 1 {
		t.Fatalf("direct search = %#v", search)
	}
	if deepseek.payloads[0]["localization"] != "198.51.100.23" || deepseek.payloads[0]["count"] != float64(3) {
		t.Fatalf("direct payload = %#v", deepseek.payloads[0])
	}
	encoded, _ := json.Marshal(search)
	if strings.Contains(string(encoded), "198.51.100.23") || strings.Contains(string(encoded), "deepseek-secret-key") {
		t.Fatalf("direct output leaks localization or key: %s", encoded)
	}
	fetch := svc.DeepSeekFetch(t.Context(), "https://example.com")
	if fetch["ok"] != true || fetch["content"] != "body" || fetch["title"] != "T" {
		t.Fatalf("direct fetch = %#v", fetch)
	}
}

func providersDeepSeekOptions(count int, localization string) providers.DeepSeekSearchOptions {
	return providers.DeepSeekSearchOptions{Count: count, Localization: localization}
}
