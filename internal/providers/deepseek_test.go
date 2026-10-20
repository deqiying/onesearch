package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deqiying/onesearch/internal/app"
)

type deepSeekCapture struct {
	method  string
	path    string
	headers http.Header
	payload map[string]any
}

func newDeepSeekServer(t *testing.T, status int, body string, capture *deepSeekCapture, count *int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count != nil {
			atomic.AddInt32(count, 1)
		}
		if capture != nil {
			capture.method = r.Method
			capture.path = r.URL.Path
			capture.headers = r.Header.Clone()
			data, _ := io.ReadAll(r.Body)
			capture.payload = nil
			if err := json.Unmarshal(data, &capture.payload); err != nil {
				t.Errorf("request body is not JSON: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestDeepSeekEndpointNormalization(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
	}{
		{"https://api.deepseek.com", "https://api.deepseek.com/v1/tools/search"},
		{"https://api.deepseek.com/", "https://api.deepseek.com/v1/tools/search"},
		{" https://api.deepseek.com/v1 ", "https://api.deepseek.com/v1/tools/search"},
		{"https://api.deepseek.com/v1/", "https://api.deepseek.com/v1/tools/search"},
		{"https://relay.example.com/deepseek", "https://relay.example.com/deepseek/v1/tools/search"},
	}
	for _, tt := range tests {
		got, err := deepSeekEndpoint(tt.baseURL, "search")
		if err != nil || got != tt.want {
			t.Fatalf("deepSeekEndpoint(%q) = %q, %v; want %q", tt.baseURL, got, err, tt.want)
		}
	}
	for _, baseURL := range []string{"", "api.deepseek.com", "ftp://api.deepseek.com", "https://api.deepseek.com/v1/tools", "https://api.deepseek.com/v1/tools/search", "https://api.deepseek.com/v1/tools/fetch/"} {
		_, err := deepSeekEndpoint(baseURL, "search")
		if errorType, _ := ErrorPayload(err); errorType != "config_error" {
			t.Fatalf("deepSeekEndpoint(%q) error type = %q (%v), want config_error", baseURL, errorType, err)
		}
	}
}

func TestDeepSeekSearchWireContract(t *testing.T) {
	tests := []struct {
		name    string
		baseURL func(string) string
	}{
		{name: "root", baseURL: func(serverURL string) string { return serverURL }},
		{name: "root_with_v1", baseURL: func(serverURL string) string { return serverURL + "/v1/" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capture deepSeekCapture
			server := newDeepSeekServer(t, http.StatusOK, `{"results":[]}`, &capture, nil)
			client := DeepSeek{APIURL: tt.baseURL(server.URL), APIKey: "ds-key"}
			got := client.SearchResult(context.Background(), "  DeepSeek site:github.com lang:zh after:2025  ", DeepSeekSearchOptions{Count: 10})
			if got["ok"] != true || got["total"] != 0 || got["tool"] != "deepseek_search" || got["provider"] != "deepseek" {
				t.Fatalf("result = %#v", got)
			}
			if capture.method != http.MethodPost || capture.path != "/v1/tools/search" {
				t.Fatalf("request = %s %s", capture.method, capture.path)
			}
			for key, want := range map[string]string{
				"Authorization": "Bearer ds-key",
				"Content-Type":  "application/json",
				"Accept":        "application/json",
				"User-Agent":    app.UserAgent,
			} {
				if got := capture.headers.Get(key); got != want {
					t.Fatalf("header %s = %q, want %q", key, got, want)
				}
			}
			want := map[string]any{"query": "DeepSeek site:github.com lang:zh after:2025", "count": float64(10), "need_content": false}
			if !reflect.DeepEqual(capture.payload, want) {
				t.Fatalf("payload = %#v, want %#v", capture.payload, want)
			}
		})
	}
}

func TestDeepSeekSearchSendsOptionalFields(t *testing.T) {
	var capture deepSeekCapture
	server := newDeepSeekServer(t, http.StatusOK, `{"results":[]}`, &capture, nil)
	_, err := DeepSeek{APIURL: server.URL, APIKey: "ds-key"}.Search(context.Background(), "DeepSeek V4 发布", DeepSeekSearchOptions{
		Count:        5,
		NeedContent:  true,
		OriQuestion:  " DeepSeek V4 是什么时候发布的？ ",
		Localization: "203.0.113.7",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"query":        "DeepSeek V4 发布",
		"count":        float64(5),
		"ori_question": "DeepSeek V4 是什么时候发布的？",
		"localization": "203.0.113.7",
		"need_content": true,
	}
	if !reflect.DeepEqual(capture.payload, want) {
		t.Fatalf("payload = %#v, want %#v", capture.payload, want)
	}
}

func TestDeepSeekLocalValidationSendsNoRequest(t *testing.T) {
	var count int32
	server := newDeepSeekServer(t, http.StatusOK, `{"results":[]}`, nil, &count)
	client := DeepSeek{APIURL: server.URL, APIKey: "ds-key"}
	longQuery := strings.Repeat("中", DeepSeekMaxQueryRunes+1)
	searchCases := []struct {
		name    string
		query   string
		options DeepSeekSearchOptions
	}{
		{"blank_query", "   ", DeepSeekSearchOptions{Count: 10}},
		{"query_129_runes", longQuery, DeepSeekSearchOptions{Count: 10}},
		{"count_zero", "q", DeepSeekSearchOptions{Count: 0}},
		{"count_51", "q", DeepSeekSearchOptions{Count: 51}},
		{"ori_question_129_runes", "q", DeepSeekSearchOptions{Count: 10, OriQuestion: longQuery}},
		{"invalid_localization", "q", DeepSeekSearchOptions{Count: 10, Localization: "not-an-ip"}},
	}
	for _, tt := range searchCases {
		got := client.SearchResult(context.Background(), tt.query, tt.options)
		if got["ok"] != false || got["error_type"] != "parameter_error" {
			t.Fatalf("%s: result = %#v", tt.name, got)
		}
		if strings.Contains(stringValue(got["error"]), "not-an-ip") {
			t.Fatalf("%s: error leaks localization: %#v", tt.name, got)
		}
	}
	fetchCases := map[string]string{
		"empty":     "  ",
		"ftp":       "ftp://example.com/file",
		"no_host":   "https:///path",
		"relative":  "/docs",
		"userinfo":  "https://user:pass@example.com/",
		"too_long":  "https://example.com/" + strings.Repeat("a", 8192),
		"no_scheme": "example.com/page",
	}
	for name, targetURL := range fetchCases {
		got := client.FetchResult(context.Background(), targetURL)
		if got["ok"] != false || got["error_type"] != "parameter_error" {
			t.Fatalf("fetch %s: result = %#v", name, got)
		}
	}
	if count != 0 {
		t.Fatalf("local validation sent %d requests", count)
	}

	// 128 rune 恰好合法，应发出请求。
	got := client.SearchResult(context.Background(), strings.Repeat("中", DeepSeekMaxQueryRunes), DeepSeekSearchOptions{Count: 50})
	if got["ok"] != true || count != 1 {
		t.Fatalf("128-rune query: result = %#v, requests = %d", got, count)
	}
}

func TestDeepSeekSearchNormalizesResults(t *testing.T) {
	body := `{"results":[
		{"rerank_score":2.4,"title":" First ","url":" https://example.com/a ","snippet":"short snippet","summary":"long summary","content":"","site_name":"Example News","site_icon":"https://example.com/favicon.ico","date":"2026-06-01T00:00:00+00:00","authority":3},
		{"rerank_score":0.8,"title":"Second","url":"https://example.com/b","snippet":"","summary":"only summary","content":"body text","site_name":null,"site_icon":null,"date":null,"authority":-1}
	]}`
	server := newDeepSeekServer(t, http.StatusOK, body, nil, nil)
	client := DeepSeek{APIURL: server.URL, APIKey: "ds-key"}

	results, err := client.Search(context.Background(), "q", DeepSeekSearchOptions{Count: 2})
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{
			"provider":       "deepseek",
			"title":          "First",
			"url":            "https://example.com/a",
			"description":    "short snippet",
			"summary":        "long summary",
			"source":         "Example News",
			"icon":           "https://example.com/favicon.ico",
			"published_date": "2026-06-01T00:00:00+00:00",
			"rerank_score":   2.4,
			"authority":      float64(3),
		},
		{
			"provider":     "deepseek",
			"title":        "Second",
			"url":          "https://example.com/b",
			"description":  "only summary",
			"summary":      "only summary",
			"rerank_score": 0.8,
			"authority":    float64(-1),
		},
	}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("results = %#v\nwant %#v", results, want)
	}

	withContent, err := client.Search(context.Background(), "q", DeepSeekSearchOptions{Count: 2, NeedContent: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := withContent[0]["content"]; ok {
		t.Fatalf("empty content should be omitted: %#v", withContent[0])
	}
	if withContent[1]["content"] != "body text" {
		t.Fatalf("content = %#v", withContent[1])
	}

	sources := NormalizeSourceResults(results, "deepseek")
	if len(sources) != 2 || sources[0]["description"] != "short snippet" || sources[1]["description"] != "only summary" || sources[0]["source"] != "Example News" {
		t.Fatalf("workflow sources = %#v", sources)
	}
}

func TestDeepSeekMalformedSuccessResponses(t *testing.T) {
	for name, body := range map[string]string{
		"missing_results":   `{"data":[]}`,
		"results_not_array": `{"results":{}}`,
		"not_json":          `<html>oops</html>`,
		"json_array":        `[]`,
		"empty_body":        ``,
	} {
		server := newDeepSeekServer(t, http.StatusOK, body, nil, nil)
		got := DeepSeek{APIURL: server.URL, APIKey: "ds-key"}.SearchResult(context.Background(), "q", DeepSeekSearchOptions{Count: 1})
		if got["ok"] != false || got["error_type"] != "upstream_error" {
			t.Fatalf("%s: result = %#v", name, got)
		}
	}
}

func TestDeepSeekFetch(t *testing.T) {
	var capture deepSeekCapture
	server := newDeepSeekServer(t, http.StatusOK, `{"title":"Example article","content":"# Example article\n\nArticle content."}`, &capture, nil)
	got := DeepSeek{APIURL: server.URL + "/v1", APIKey: "ds-key"}.FetchResult(context.Background(), " https://example.com/article ")
	want := map[string]any{"url": "https://example.com/article"}
	if !reflect.DeepEqual(capture.payload, want) || capture.path != "/v1/tools/fetch" || capture.method != http.MethodPost {
		t.Fatalf("request = %s %s %#v", capture.method, capture.path, capture.payload)
	}
	if got["ok"] != true || got["tool"] != "deepseek_fetch" || got["title"] != "Example article" || got["content"] != "# Example article\n\nArticle content." || got["url"] != "https://example.com/article" {
		t.Fatalf("result = %#v", got)
	}

	for name, tc := range map[string]struct {
		body      string
		errorType string
	}{
		"empty_content":   {`{"title":"","content":"   "}`, "empty_result"},
		"missing_content": {`{"title":"x"}`, "upstream_error"},
		"not_json":        {`not json`, "upstream_error"},
	} {
		server := newDeepSeekServer(t, http.StatusOK, tc.body, nil, nil)
		got := DeepSeek{APIURL: server.URL, APIKey: "ds-key"}.FetchResult(context.Background(), "https://example.com/")
		if got["ok"] != false || got["error_type"] != tc.errorType {
			t.Fatalf("%s: result = %#v", name, got)
		}
	}
}

func TestDeepSeekErrorMapping(t *testing.T) {
	tests := []struct {
		op        string
		status    int
		errorType string
	}{
		{"search", 400, "parameter_error"},
		{"fetch", 413, "parameter_error"},
		{"search", 401, "auth_error"},
		{"search", 402, "provider_error"},
		{"fetch", 402, "provider_error"},
		{"fetch", 403, "provider_error"},
		{"search", 403, "auth_error"},
		{"fetch", 404, "empty_result"},
		{"search", 408, "timeout"},
		{"fetch", 504, "timeout"},
		{"search", 429, "rate_limited"},
		{"fetch", 429, "rate_limited"},
		{"search", 500, "upstream_error"},
		{"fetch", 502, "upstream_error"},
		{"search", 503, "upstream_error"},
	}
	for _, tt := range tests {
		body := `{"error":{"message":"upstream says no","type":"invalid_request_error","code":"x","param":null}}`
		server := newDeepSeekServer(t, tt.status, body, nil, nil)
		client := DeepSeek{APIURL: server.URL, APIKey: "secret-ds-key"}
		var got map[string]any
		if tt.op == "search" {
			got = client.SearchResult(context.Background(), "q", DeepSeekSearchOptions{Count: 1})
		} else {
			got = client.FetchResult(context.Background(), "https://example.com/")
		}
		message := stringValue(got["error"])
		if got["ok"] != false || got["error_type"] != tt.errorType {
			t.Fatalf("%s HTTP %d: result = %#v, want %s", tt.op, tt.status, got, tt.errorType)
		}
		if !strings.Contains(message, "DeepSeek "+tt.op+": upstream says no") || !strings.Contains(message, "(HTTP ") {
			t.Fatalf("%s HTTP %d: message = %q", tt.op, tt.status, message)
		}
		if strings.Contains(message, "secret-ds-key") {
			t.Fatalf("message leaks API key: %q", message)
		}
	}

	// workflow 方法返回的错误同样保留归一化类型。
	server := newDeepSeekServer(t, 404, `{"error":{"message":"no content","type":"not_found"}}`, nil, nil)
	_, err := DeepSeek{APIURL: server.URL, APIKey: "k"}.Fetch(context.Background(), "https://example.com/")
	if errorType, _ := ErrorPayload(err); errorType != "empty_result" {
		t.Fatalf("Fetch 404 error type = %q (%v)", errorType, err)
	}
}

func TestDeepSeekContextCancellation(t *testing.T) {
	server := newDeepSeekServer(t, http.StatusOK, `{"results":[]}`, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := DeepSeek{APIURL: server.URL, APIKey: "k"}.SearchResult(ctx, "q", DeepSeekSearchOptions{Count: 1})
	if got["ok"] != false || (got["error_type"] != "timeout" && got["error_type"] != "network_error") {
		t.Fatalf("result = %#v", got)
	}
}
