package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContext7LibraryUsesCanonicalSearchEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/libs/search" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("libraryName") != "react" || r.URL.Query().Get("query") != "hooks" {
			t.Fatalf("query = %#v", r.URL.Query())
		}
		if r.Header.Get("Authorization") != "Bearer ctx-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "/facebook/react", "title": "React", "description": "UI"}})
	}))
	defer server.Close()
	got := (Context7{APIURL: server.URL, APIKey: "ctx-key"}).Library(context.Background(), "react", "hooks")
	if got["ok"] != true || len(got["results"].([]map[string]any)) != 1 {
		t.Fatalf("result = %#v", got)
	}
}

func TestContext7DocsAddsJSONType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/context" || r.URL.Query().Get("type") != "json" {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"documentation": []map[string]any{{"title": "Hook", "content": "useEffect", "source": "docs"}}})
	}))
	defer server.Close()
	got := (Context7{APIURL: server.URL, APIKey: "ctx-key"}).Docs(context.Background(), "/facebook/react", "hooks")
	if got["ok"] != true || got["total"] != 1 {
		t.Fatalf("result = %#v", got)
	}
}

func TestContext7DocsAcceptsTopLevelDocumentationArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"title": "Guide", "content": "body", "source": "docs"}})
	}))
	defer server.Close()
	got := (Context7{APIURL: server.URL, APIKey: "ctx-key"}).Docs(context.Background(), "/org/lib", "api")
	if got["ok"] != true || got["total"] != 1 {
		t.Fatalf("result = %#v", got)
	}
	results := got["results"].([]any)
	if results[0].(map[string]any)["description"] != "body" {
		t.Fatalf("results = %#v", results)
	}
}
