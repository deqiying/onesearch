package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTavilySearchResultSendsMCPCompatiblePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("path = %q, want /search", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tavily-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["query"] != "golang" || payload["max_results"] != float64(3) || payload["search_depth"] != "basic" {
			t.Fatalf("search payload = %#v", payload)
		}
		if payload["include_raw_content"] != true || payload["include_images"] != true || payload["include_favicon"] != true {
			t.Fatalf("include flags payload = %#v", payload)
		}
		includeDomains := payload["include_domains"].([]any)
		if len(includeDomains) != 1 || includeDomains[0] != "go.dev" {
			t.Fatalf("include_domains = %#v", includeDomains)
		}
		_, _ = w.Write([]byte(`{"results":[{"title":"Go","url":"https://go.dev","content":"docs","score":0.9,"raw_content":"full"}],"response_time":0.1}`))
	}))
	defer server.Close()

	got := (Tavily{APIURL: server.URL, APIKey: "tavily-key"}).SearchResult(context.Background(), "golang", TavilySearchOptions{MaxResults: 3, SearchDepth: "basic", IncludeRawContent: true, IncludeImages: true, IncludeFavicon: true, IncludeDomains: []string{"go.dev"}})
	if got["ok"] != true || got["provider"] != "tavily" || got["tool"] != "tavily_search" {
		t.Fatalf("search result envelope = %#v", got)
	}
	results := got["results"].([]map[string]any)
	if len(results) != 1 || results[0]["raw_content"] != "full" {
		t.Fatalf("search results = %#v", results)
	}
}

func TestTavilyExtractResultSupportsMultipleURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/extract" {
			t.Fatalf("path = %q, want /extract", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		urls := payload["urls"].([]any)
		if len(urls) != 2 || urls[0] != "https://example.com/a" || urls[1] != "https://example.com/b" {
			t.Fatalf("urls payload = %#v", urls)
		}
		if payload["format"] != "text" || payload["extract_depth"] != "advanced" || payload["query"] != "only api" {
			t.Fatalf("extract payload = %#v", payload)
		}
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com/a","raw_content":"alpha"},{"url":"https://example.com/b","raw_content":"beta"}]}`))
	}))
	defer server.Close()

	got := (Tavily{APIURL: server.URL, APIKey: "tavily-key"}).ExtractResult(context.Background(), []string{"https://example.com/a", "https://example.com/b"}, TavilyExtractOptions{Format: "text", ExtractDepth: "advanced", Query: "only api"})
	if got["ok"] != true || got["tool"] != "tavily_extract" || got["content"] != "alpha\n\nbeta" {
		t.Fatalf("extract result = %#v", got)
	}
}

func TestTavilyCrawlSendsLimitAndSelectionPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crawl" {
			t.Fatalf("path = %q, want /crawl", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["url"] != "https://example.com" || payload["max_depth"] != float64(3) || payload["limit"] != float64(7) || payload["allow_external"] != true {
			t.Fatalf("crawl payload = %#v", payload)
		}
		selectPaths := payload["select_paths"].([]any)
		if len(selectPaths) != 1 || selectPaths[0] != "/docs" {
			t.Fatalf("select_paths = %#v", selectPaths)
		}
		_, _ = w.Write([]byte(`{"results":[{"url":"https://example.com/docs","raw_content":"docs"}]}`))
	}))
	defer server.Close()

	got := (Tavily{APIURL: server.URL, APIKey: "tavily-key"}).Crawl(context.Background(), "https://example.com", TavilyCrawlOptions{MaxDepth: 3, Limit: 7, AllowExternal: true, SelectPaths: []string{"/docs"}})
	if got["ok"] != true || got["provider"] != "tavily" || got["tool"] != "tavily_crawl" {
		t.Fatalf("crawl result envelope = %#v", got)
	}
	if got["content"] != "docs" {
		t.Fatalf("crawl content = %#v", got["content"])
	}
}

func TestFirecrawlCrawlExposesSubmittedJobEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crawl" {
			t.Fatalf("path = %q, want /crawl", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["maxDiscoveryDepth"] != float64(2) || payload["limit"] != float64(5) {
			t.Fatalf("crawl payload = %#v", payload)
		}
		_, _ = w.Write([]byte(`{"success":true,"id":"crawl-1","url":"https://example.com"}`))
	}))
	defer server.Close()

	got := (Firecrawl{APIURL: server.URL, APIKey: "firecrawl-key"}).Crawl(context.Background(), "https://example.com", 2, 5)
	if got["ok"] != true || got["provider"] != "firecrawl" || got["tool"] != "firecrawl_crawl" || got["id"] != "crawl-1" || got["status"] != "submitted" {
		t.Fatalf("crawl result envelope = %#v", got)
	}
}
