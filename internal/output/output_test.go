package output

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuietSearchOutputMergesFetchedPages(t *testing.T) {
	data := map[string]any{
		"ok":    true,
		"query": "rank",
		"used": []map[string]any{
			{
				"capability": "page_fetch",
				"role":       "page_evidence",
				"providers": []map[string]any{
					{
						"provider": "tavily",
						"status":   "ok",
						"result": map[string]any{
							"pages": []map[string]any{
								{"url": "https://example.com/a", "content_preview": "alpha", "content_length": 5},
							},
						},
					},
				},
			},
			{
				"capability": "page_fetch",
				"role":       "source_evidence",
				"providers": []map[string]any{
					{
						"provider": "tavily",
						"status":   "ok",
						"result": map[string]any{
							"pages": []map[string]any{
								{"url": "https://example.com/b", "content_preview": "beta", "content_length": 4},
							},
						},
					},
				},
			},
		},
	}

	rendered := RenderWithOptions("search", data, Options{Format: "json", Verbosity: "quiet"})
	var got map[string]any
	if err := json.Unmarshal([]byte(rendered), &got); err != nil {
		t.Fatal(err)
	}
	pageFetch := got["used"].(map[string]any)["page_fetch"].(map[string]any)
	roles := pageFetch["roles"].([]any)
	if len(roles) != 2 {
		t.Fatalf("roles = %#v, want both page_fetch roles", roles)
	}
	result := pageFetch["providers"].(map[string]any)["tavily"].(map[string]any)["result"].(map[string]any)
	pages := result["pages"].([]any)
	if len(pages) != 2 || result["pages_count"].(float64) != 2 {
		t.Fatalf("pages result = %#v", result)
	}
}

func TestQuietProviderToolOutputKeepsJobEnvelopeWithoutRawResult(t *testing.T) {
	rendered := RenderWithOptions("firecrawl", map[string]any{
		"ok":       true,
		"provider": "firecrawl",
		"tool":     "firecrawl_crawl",
		"url":      "https://example.com",
		"id":       "crawl-1",
		"status":   "submitted",
		"success":  true,
		"result": map[string]any{
			"success": true,
			"id":      "crawl-1",
			"url":     "https://example.com",
			"debug":   strings.Repeat("raw ", 100),
		},
	}, Options{Format: "json", Verbosity: "quiet"})

	var got map[string]any
	if err := json.Unmarshal([]byte(rendered), &got); err != nil {
		t.Fatal(err)
	}
	if got["provider"] != "firecrawl" || got["tool"] != "firecrawl_crawl" || got["id"] != "crawl-1" || got["status"] != "submitted" {
		t.Fatalf("quiet job envelope = %#v", got)
	}
	if _, ok := got["result"]; ok {
		t.Fatalf("quiet provider tool output should omit raw result: %s", rendered)
	}
}
