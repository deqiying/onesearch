package providers

import (
	"context"
	"strings"
	"time"
)

type Zhipu struct {
	APIURL       string
	APIKey       string
	SearchEngine string
	Timeout      time.Duration
}

type ZhipuOptions struct {
	Count               int
	SearchEngine        string
	SearchRecencyFilter string
	SearchDomainFilter  string
	ContentSize         string
}

func (p Zhipu) Search(ctx context.Context, query string, options ZhipuOptions) map[string]any {
	start := time.Now()
	if options.Count <= 0 {
		options.Count = 10
	}
	engine := firstNonEmpty(options.SearchEngine, p.SearchEngine, "search_std")
	recency := firstNonEmpty(options.SearchRecencyFilter, "noLimit")
	contentSize := firstNonEmpty(options.ContentSize, "medium")
	searchQuery := query
	if len([]rune(searchQuery)) > 70 {
		searchQuery = string([]rune(searchQuery)[:70])
	}
	payload := map[string]any{
		"search_query":          searchQuery,
		"search_engine":         engine,
		"search_intent":         true,
		"count":                 options.Count,
		"search_recency_filter": recency,
		"content_size":          contentSize,
	}
	if options.SearchDomainFilter != "" {
		payload["search_domain_filter"] = options.SearchDomainFilter
	}
	var data map[string]any
	err := PostJSON(
		ctx,
		Client(p.Timeout),
		strings.TrimRight(p.APIURL, "/")+"/paas/v4/web_search",
		map[string]string{"Authorization": "Bearer " + p.APIKey},
		payload,
		&data,
	)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "query": query, "provider": "zhipu", "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	results := normalizeZhipuResults(asMaps(data["search_result"]))
	return map[string]any{
		"ok":            true,
		"query":         query,
		"provider":      "zhipu",
		"search_engine": engine,
		"results":       results,
		"total":         len(results),
		"search_intent": data["search_intent"],
		"request_id":    stringValue(data["request_id"]),
		"elapsed_ms":    Elapsed(start),
	}
}

func normalizeZhipuResults(items []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"title":          stringValue(item["title"]),
			"url":            firstNonEmpty(stringValue(item["link"]), stringValue(item["url"])),
			"description":    stringValue(item["content"]),
			"provider":       "zhipu",
			"source":         stringValue(item["media"]),
			"published_date": stringValue(item["publish_date"]),
			"icon":           stringValue(item["icon"]),
			"refer":          stringValue(item["refer"]),
		})
	}
	return out
}
