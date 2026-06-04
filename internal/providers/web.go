package providers

import (
	"context"
	"strings"
	"time"
)

type Tavily struct {
	APIURL  string
	APIKey  string
	Timeout time.Duration
}

func (p Tavily) Search(ctx context.Context, query string, maxResults int) ([]map[string]any, error) {
	if maxResults <= 0 {
		maxResults = 6
	}
	payload := map[string]any{
		"query":               query,
		"max_results":         maxResults,
		"search_depth":        "advanced",
		"include_raw_content": false,
		"include_answer":      false,
	}
	var data map[string]any
	err := PostJSON(ctx, Client(p.Timeout), strings.TrimRight(p.APIURL, "/")+"/search", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		return nil, err
	}
	var results []map[string]any
	for _, item := range asMaps(data["results"]) {
		results = append(results, map[string]any{
			"title":   stringValue(item["title"]),
			"url":     stringValue(item["url"]),
			"content": stringValue(item["content"]),
			"score":   item["score"],
		})
	}
	return results, nil
}

func (p Tavily) Extract(ctx context.Context, targetURL string) (string, error) {
	payload := map[string]any{"urls": []string{targetURL}, "format": "markdown"}
	var data map[string]any
	err := PostJSON(ctx, Client(p.Timeout), strings.TrimRight(p.APIURL, "/")+"/extract", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		return "", err
	}
	results := asMaps(data["results"])
	if len(results) == 0 {
		return "", nil
	}
	return strings.TrimSpace(stringValue(results[0]["raw_content"])), nil
}

func (p Tavily) Map(ctx context.Context, targetURL, instructions string, maxDepth, maxBreadth, limit, timeoutSeconds int) map[string]any {
	start := time.Now()
	payload := map[string]any{"url": targetURL, "max_depth": maxDepth, "max_breadth": maxBreadth, "limit": limit, "timeout": timeoutSeconds}
	if instructions != "" {
		payload["instructions"] = instructions
	}
	var data map[string]any
	err := PostJSON(ctx, Client(time.Duration(timeoutSeconds+10)*time.Second), strings.TrimRight(p.APIURL, "/")+"/map", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "url": targetURL, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	return map[string]any{"ok": true, "url": targetURL, "base_url": data["base_url"], "results": data["results"], "response_time": data["response_time"], "elapsed_ms": Elapsed(start)}
}

type Firecrawl struct {
	APIURL string
	APIKey string
}

func (p Firecrawl) Search(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 14
	}
	payload := map[string]any{"query": query, "limit": limit}
	var data map[string]any
	err := PostJSON(ctx, Client(90*time.Second), strings.TrimRight(p.APIURL, "/")+"/search", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		return nil, err
	}
	var web []map[string]any
	if nested, ok := data["data"].(map[string]any); ok {
		web = asMaps(nested["web"])
	}
	results := make([]map[string]any, 0, len(web))
	for _, item := range web {
		results = append(results, map[string]any{
			"title":       stringValue(item["title"]),
			"url":         stringValue(item["url"]),
			"description": stringValue(item["description"]),
		})
	}
	return results, nil
}

func (p Firecrawl) Map(ctx context.Context, targetURL string, limit int) map[string]any {
	start := time.Now()
	if limit <= 0 {
		limit = 50
	}
	payload := map[string]any{"url": targetURL, "limit": limit}
	var data map[string]any
	err := PostJSON(ctx, Client(90*time.Second), strings.TrimRight(p.APIURL, "/")+"/map", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "url": targetURL, "provider": "firecrawl", "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	results := data["links"]
	if results == nil {
		results = data["data"]
	}
	return map[string]any{"ok": true, "url": targetURL, "provider": "firecrawl", "results": results, "elapsed_ms": Elapsed(start)}
}

func (p Firecrawl) Crawl(ctx context.Context, targetURL string, maxDepth, limit int) map[string]any {
	start := time.Now()
	if maxDepth <= 0 {
		maxDepth = 2
	}
	if limit <= 0 {
		limit = 20
	}
	payload := map[string]any{
		"url":               targetURL,
		"maxDiscoveryDepth": maxDepth,
		"limit":             limit,
		"scrapeOptions":     map[string]any{"formats": []string{"markdown"}, "onlyMainContent": true},
	}
	var data map[string]any
	err := PostJSON(ctx, Client(120*time.Second), strings.TrimRight(p.APIURL, "/")+"/crawl", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "url": targetURL, "provider": "firecrawl", "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	return map[string]any{"ok": true, "url": targetURL, "provider": "firecrawl", "result": data, "elapsed_ms": Elapsed(start)}
}

func (p Firecrawl) Scrape(ctx context.Context, targetURL string, attempts int) (string, error) {
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		payload := map[string]any{
			"url":     targetURL,
			"formats": []string{"markdown"},
			"timeout": 60000,
			"waitFor": (i + 1) * 1500,
		}
		var data map[string]any
		err := PostJSON(ctx, Client(90*time.Second), strings.TrimRight(p.APIURL, "/")+"/scrape", map[string]string{"Authorization": "Bearer " + p.APIKey}, payload, &data)
		if err != nil {
			lastErr = err
			continue
		}
		if nested, ok := data["data"].(map[string]any); ok {
			if markdown := strings.TrimSpace(stringValue(nested["markdown"])); markdown != "" {
				return markdown, nil
			}
		}
	}
	return "", lastErr
}
