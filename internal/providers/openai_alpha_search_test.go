package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/deqiying/onesearch/internal/app"
	"github.com/deqiying/onesearch/internal/sources"
)

func TestBuildOpenAIAlphaSearchRequestUsesMinimalContract(t *testing.T) {
	payload, err := buildOpenAIAlphaSearchRequest("onesearch-a1b2c3d4e5f6", " gpt-4.1 ", " OpenAI Codex latest release ", " GitHub ")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"id":    "onesearch-a1b2c3d4e5f6",
		"model": "gpt-4.1",
		"commands": map[string]any{
			"search_query": []any{map[string]any{"q": "OpenAI Codex latest release\n\nPreferred platform or source: GitHub"}},
		},
		"settings": map[string]any{
			"allowed_callers":     []any{"direct"},
			"external_web_access": true,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payload = %#v, want %#v", got, want)
	}
}

func TestOpenAIAlphaSearchWireAndBaseURLs(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  func(string) string
		wantPath string
	}{
		{name: "root", baseURL: func(serverURL string) string { return serverURL }, wantPath: "/v1/alpha/search"},
		{name: "root_with_v1", baseURL: func(serverURL string) string { return serverURL + "/v1" }, wantPath: "/v1/alpha/search"},
		{name: "relay_subpath", baseURL: func(serverURL string) string { return serverURL + "/openai" }, wantPath: "/openai/v1/alpha/search"},
		{name: "relay_subpath_with_v1", baseURL: func(serverURL string) string { return serverURL + "/openai/v1/" }, wantPath: "/openai/v1/alpha/search"},
	}

	seenIDs := map[string]struct{}{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != tt.wantPath {
					t.Fatalf("request = %s %s, want POST %s", r.Method, r.URL.Path, tt.wantPath)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Fatalf("Authorization = %q", got)
				}
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Fatalf("Content-Type = %q", got)
				}
				if got := r.Header.Get("Accept"); got != "application/json" {
					t.Fatalf("Accept = %q", got)
				}
				if got := r.Header.Get("User-Agent"); got != app.UserAgent {
					t.Fatalf("User-Agent = %q, want %q", got, app.UserAgent)
				}
				for _, forbidden := range []string{"OpenAI-Beta", "Session_ID", "Conversation_ID", "ChatGPT-Account-ID", "X-Codex-Beta-Features", "X-Codex-Turn-State"} {
					if got := r.Header.Get(forbidden); got != "" {
						t.Fatalf("forbidden header %s = %q", forbidden, got)
					}
				}

				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				requestID, ok := got["id"].(string)
				if !ok || !strings.HasPrefix(requestID, "onesearch-") || len(requestID) != len("onesearch-")+12 {
					t.Fatalf("request id = %#v", got["id"])
				}
				if _, exists := seenIDs[requestID]; exists {
					t.Fatalf("duplicate request id %q", requestID)
				}
				seenIDs[requestID] = struct{}{}
				want := map[string]any{
					"id":    requestID,
					"model": "gpt-test",
					"commands": map[string]any{
						"search_query": []any{map[string]any{"q": "query"}},
					},
					"settings": map[string]any{
						"allowed_callers":     []any{"direct"},
						"external_web_access": true,
					},
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("payload = %#v, want %#v", got, want)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"output":"ok"}`))
			}))
			defer server.Close()

			got, err := (OpenAIResponses{APIURL: tt.baseURL(server.URL), APIKey: "test-key", Model: "gpt-test"}).Search(t.Context(), "query", "")
			if err != nil {
				t.Fatal(err)
			}
			if got != "ok" {
				t.Fatalf("response = %q, want ok", got)
			}
		})
	}
}

func TestOpenAIAlphaSearchRejectsEmptyQueryWithoutRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"output":"unexpected"}`))
	}))
	defer server.Close()

	_, err := (OpenAIResponses{APIURL: server.URL, APIKey: "key", Model: "gpt-test"}).Search(t.Context(), " \n\t ", "")
	errorType, _ := ErrorPayload(err)
	if errorType != "parameter_error" {
		t.Fatalf("error type = %q, want parameter_error; err=%v", errorType, err)
	}
	if calls != 0 {
		t.Fatalf("server calls = %d, want 0", calls)
	}
}

func TestParseOpenAIAlphaSearchResponseNormalizesSources(t *testing.T) {
	data := []byte(`{
		"encrypted_output":"opaque-secret",
		"output":"  answer text  ",
		"results":[
			{"type":"future_result","url":"HTTPS://Example.COM/a","title":"  Example A  ","unknown":true},
			{"type":"text_result","url":"https://example.com/a","title":"duplicate"},
			{"url":"http://example.com/b","title":""},
			{"url":"ftp://example.com/file"},
			{"url":"https://"},
			"ignored"
		]
	}`)
	got, err := parseOpenAIAlphaSearchResponse(data, http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "opaque-secret") {
		t.Fatalf("response leaked encrypted_output: %q", got)
	}
	answer, gotSources := sources.SplitAnswerAndSources(got)
	if answer != "answer text" {
		t.Fatalf("answer = %q", answer)
	}
	wantSources := []map[string]any{
		{"url": "https://example.com/a", "title": "Example A"},
		{"url": "http://example.com/b"},
	}
	if !reflect.DeepEqual(gotSources, wantSources) {
		t.Fatalf("sources = %#v, want %#v", gotSources, wantSources)
	}
}

func TestParseOpenAIAlphaSearchResponseRejectsProtocolMismatch(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		header http.Header
	}{
		{name: "non_json", body: `not json`},
		{name: "null", body: `null`},
		{name: "array", body: `[]`},
		{name: "missing_output", body: `{}`},
		{name: "blank_output", body: `{"output":"   "}`},
		{name: "wrong_output_type", body: `{"output":[]}`},
		{name: "null_results", body: `{"output":"answer","results":null}`},
		{name: "wrong_results_type", body: `{"output":"answer","results":{}}`},
		{name: "encrypted_output_only", body: `{"encrypted_output":"opaque"}`},
		{name: "error_envelope", body: `{"error":{"type":"upstream_error","message":"failed"}}`},
		{name: "sse_header", body: `{"output":"answer"}`, header: http.Header{"Content-Type": []string{"text/event-stream"}}},
		{name: "sse_body", body: "data: {\"output\":\"answer\"}\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseOpenAIAlphaSearchResponse([]byte(tt.body), tt.header)
			errorType, _ := ErrorPayload(err)
			if errorType != "upstream_error" {
				t.Fatalf("error type = %q, want upstream_error; err=%v", errorType, err)
			}
		})
	}
}

func TestOpenAIAlphaSearchHTTPErrorClassification(t *testing.T) {
	tests := []struct {
		status   int
		wantType string
	}{
		{status: http.StatusBadRequest, wantType: "parameter_error"},
		{status: http.StatusUnprocessableEntity, wantType: "parameter_error"},
		{status: http.StatusUnauthorized, wantType: "auth_error"},
		{status: http.StatusForbidden, wantType: "auth_error"},
		{status: http.StatusTooManyRequests, wantType: "rate_limited"},
		{status: http.StatusNotFound, wantType: "network_error"},
		{status: http.StatusMethodNotAllowed, wantType: "network_error"},
		{status: http.StatusInternalServerError, wantType: "network_error"},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/v1/alpha/search" {
					t.Fatalf("path = %q", r.URL.Path)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":{"message":"request failed"}}`))
			}))
			defer server.Close()

			_, err := (OpenAIResponses{APIURL: server.URL, APIKey: "key", Model: "gpt-test"}).Search(t.Context(), "query", "")
			errorType, message := ErrorPayload(err)
			if errorType != tt.wantType || !strings.Contains(message, "HTTP ") {
				t.Fatalf("error type = %q, message = %q, want %q with status", errorType, message, tt.wantType)
			}
			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
		})
	}
}

func TestOpenAIAlphaSearchResponseLimitAndCancellation(t *testing.T) {
	t.Run("response_limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", openAIAlphaSearchResponseLimit+1)))
		}))
		defer server.Close()

		_, err := (OpenAIResponses{APIURL: server.URL, APIKey: "key", Model: "gpt-test"}).Search(t.Context(), "query", "")
		errorType, _ := ErrorPayload(err)
		if errorType != "upstream_error" {
			t.Fatalf("error type = %q, want upstream_error; err=%v", errorType, err)
		}
	})

	t.Run("canceled_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := (OpenAIResponses{APIURL: "http://127.0.0.1:1", APIKey: "key", Model: "gpt-test"}).Search(ctx, "query", "")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})

	t.Run("transport_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		serverURL := server.URL
		server.Close()
		_, err := (OpenAIResponses{APIURL: serverURL, APIKey: "key", Model: "gpt-test"}).Search(t.Context(), "query", "")
		errorType, _ := ErrorPayload(err)
		if errorType != "network_error" {
			t.Fatalf("error type = %q, want network_error; err=%v", errorType, err)
		}
	})
}

func TestOpenAIAlphaSearchHTTPErrorDoesNotExposeEncryptedOutput(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "nested_json", body: `{"error":{"type":"upstream_error"},"meta":{"encrypted_output":"opaque-secret"}}`},
		{name: "non_json", body: `encrypted_output=opaque-secret`},
		{name: "json_string", body: `"opaque-secret"`},
		{name: "json_array", body: `["opaque-secret"]`},
		{name: "object_without_error", body: `{"meta":{"token":"opaque-secret"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			_, err := (OpenAIResponses{APIURL: server.URL, APIKey: "key", Model: "gpt-test"}).Search(t.Context(), "query", "")
			_, message := ErrorPayload(err)
			if err == nil || strings.Contains(err.Error(), "opaque-secret") || strings.Contains(message, "opaque-secret") {
				t.Fatalf("error must not expose encrypted_output: err=%v message=%q", err, message)
			}
		})
	}
}

func TestOpenAIAlphaSearchRejectsCompleteEndpointBaseURL(t *testing.T) {
	tests := []string{"/v1/alpha/search", "/v1/responses"}
	for _, endpoint := range tests {
		t.Run(endpoint, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
			}))
			defer server.Close()

			_, err := (OpenAIResponses{APIURL: server.URL + endpoint, APIKey: "key", Model: "gpt-test"}).Search(t.Context(), "query", "")
			errorType, _ := ErrorPayload(err)
			if errorType != "parameter_error" {
				t.Fatalf("error type = %q, want parameter_error; err=%v", errorType, err)
			}
			if calls != 0 {
				t.Fatalf("calls = %d, want 0", calls)
			}
		})
	}
}
