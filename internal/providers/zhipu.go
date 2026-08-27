package providers

import (
	"context"
	"strings"
	"time"
)

type Zhipu struct {
	APIURL          string
	APIKey          string
	SearchEngine    string
	ProtocolProfile string
	SearchIntent    bool
	Timeout         time.Duration
}

type ZhipuOptions struct {
	ProtocolProfile     string
	Count               int
	SearchEngine        string
	SearchRecencyFilter string
	SearchDomainFilter  string
	ContentSize         string
	SearchIntent        bool
}

func (p Zhipu) Search(ctx context.Context, query string, options ZhipuOptions) map[string]any {
	start := time.Now()
	if options.Count <= 0 {
		options.Count = 10
	}
	if options.Count > 50 {
		return map[string]any{"ok": false, "query": query, "provider": "zhipu", "error_type": "parameter_error", "error": "Zhipu count must be between 1 and 50", "elapsed_ms": Elapsed(start)}
	}
	profile := normalizeZhipuProfile(firstNonEmpty(options.ProtocolProfile, p.ProtocolProfile, "bigmodel_cn"))
	if profile == "" {
		return map[string]any{"ok": false, "query": query, "provider": "zhipu", "error_type": "parameter_error", "error": "unsupported Zhipu protocol profile", "elapsed_ms": Elapsed(start)}
	}
	defaultEngine := "search_std"
	if profile == "zai_global" {
		defaultEngine = "search-prime"
	}
	engine := firstNonEmpty(options.SearchEngine, p.SearchEngine, defaultEngine)
	recency := firstNonEmpty(options.SearchRecencyFilter, "noLimit")
	contentSize := firstNonEmpty(options.ContentSize, "medium")
	searchQuery := query
	if profile == "bigmodel_cn" && len([]rune(searchQuery)) > 70 {
		searchQuery = string([]rune(searchQuery)[:70])
	}
	payload := map[string]any{
		"search_query":  searchQuery,
		"search_engine": engine,
		"count":         options.Count,
	}
	if profile == "bigmodel_cn" {
		payload["search_intent"] = options.SearchIntent || p.SearchIntent
		payload["search_recency_filter"] = recency
		payload["content_size"] = contentSize
	}
	if profile == "bigmodel_cn" && options.SearchDomainFilter != "" {
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
		"ok":               true,
		"query":            query,
		"provider":         "zhipu",
		"protocol_profile": profile,
		"search_engine":    engine,
		"results":          results,
		"total":            len(results),
		"search_intent":    data["search_intent"],
		"request_id":       stringValue(data["request_id"]),
		"id":               stringValue(data["id"]),
		"created":          data["created"],
		"elapsed_ms":       Elapsed(start),
	}
}

func normalizeZhipuProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "zai_global", "global", "zai":
		return "zai_global"
	case "bigmodel_cn", "cn", "china", "":
		if strings.TrimSpace(profile) == "" {
			return "bigmodel_cn"
		}
		return "bigmodel_cn"
	default:
		return ""
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
