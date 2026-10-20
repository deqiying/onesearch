package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// DeepSeek 上游 /v1/tools/search 与 /v1/tools/fetch 的硬约束；违反时本地返回 parameter_error，不发请求。
const (
	DeepSeekMaxQueryRunes = 128
	DeepSeekMinCount      = 1
	DeepSeekMaxCount      = 50
	deepSeekMaxURLLength  = 8192
	deepSeekMaxMessageLen = 500
)

// DeepSeek 封装 DeepSeek 原始 JSON 工具接口。APIURL 为 API 根路径（可带 /v1），不能是完整 endpoint。
type DeepSeek struct {
	APIURL  string
	APIKey  string
	Timeout time.Duration
}

type DeepSeekSearchOptions struct {
	Count        int
	NeedContent  bool
	OriQuestion  string
	Localization string
}

type DeepSeekPage struct {
	Title   string
	Content string
}

type deepSeekSearchRequest struct {
	Query        string `json:"query"`
	Count        int    `json:"count"`
	OriQuestion  string `json:"ori_question,omitempty"`
	Localization string `json:"localization,omitempty"`
	NeedContent  bool   `json:"need_content"`
}

type deepSeekFetchRequest struct {
	URL string `json:"url"`
}

// Search 供 workflow 使用：返回归一化结果与带类型的错误，空结果返回空切片与 nil。
func (p DeepSeek) Search(ctx context.Context, query string, options DeepSeekSearchOptions) ([]map[string]any, error) {
	request, err := newDeepSeekSearchRequest(query, options)
	if err != nil {
		return nil, err
	}
	endpoint, err := deepSeekEndpoint(p.APIURL, "search")
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := PostJSON(ctx, Client(p.Timeout), endpoint, p.headers(), request, &data); err != nil {
		return nil, deepSeekError("search", err)
	}
	items, ok := data["results"].([]any)
	if !ok {
		return nil, &ProviderError{Type: "upstream_error", Message: "DeepSeek search: response is missing a results array"}
	}
	return normalizeDeepSeekResults(items, request.NeedContent), nil
}

// SearchResult 供 provider-direct 命令使用，返回统一 provider envelope。
func (p DeepSeek) SearchResult(ctx context.Context, query string, options DeepSeekSearchOptions) map[string]any {
	start := time.Now()
	results, err := p.Search(ctx, query, options)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "deepseek", "tool": "deepseek_search", "query": query, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	return map[string]any{
		"ok":         true,
		"provider":   "deepseek",
		"tool":       "deepseek_search",
		"query":      query,
		"results":    results,
		"total":      len(results),
		"elapsed_ms": Elapsed(start),
	}
}

// Fetch 供 workflow 使用：正文为空时返回 empty_result，便于 fallback 记录真实错误类型。
func (p DeepSeek) Fetch(ctx context.Context, targetURL string) (DeepSeekPage, error) {
	targetURL, err := validateDeepSeekFetchURL(targetURL)
	if err != nil {
		return DeepSeekPage{}, err
	}
	endpoint, err := deepSeekEndpoint(p.APIURL, "fetch")
	if err != nil {
		return DeepSeekPage{}, err
	}
	var data map[string]any
	if err := PostJSON(ctx, Client(p.Timeout), endpoint, p.headers(), deepSeekFetchRequest{URL: targetURL}, &data); err != nil {
		return DeepSeekPage{}, deepSeekError("fetch", err)
	}
	content, ok := data["content"].(string)
	if !ok {
		return DeepSeekPage{}, &ProviderError{Type: "upstream_error", Message: "DeepSeek fetch: response is missing string content"}
	}
	page := DeepSeekPage{Title: stringValue(data["title"]), Content: strings.TrimSpace(content)}
	if page.Content == "" {
		return page, &ProviderError{Type: "empty_result", Message: "DeepSeek fetch returned no page content"}
	}
	return page, nil
}

// FetchResult 供 provider-direct 命令使用，返回统一 provider envelope。
func (p DeepSeek) FetchResult(ctx context.Context, targetURL string) map[string]any {
	start := time.Now()
	targetURL = strings.TrimSpace(targetURL)
	page, err := p.Fetch(ctx, targetURL)
	if err != nil {
		errorType, message := ErrorPayload(err)
		return map[string]any{"ok": false, "provider": "deepseek", "tool": "deepseek_fetch", "url": targetURL, "error_type": errorType, "error": message, "elapsed_ms": Elapsed(start)}
	}
	return map[string]any{
		"ok":         true,
		"provider":   "deepseek",
		"tool":       "deepseek_fetch",
		"url":        targetURL,
		"title":      page.Title,
		"content":    page.Content,
		"elapsed_ms": Elapsed(start),
	}
}

func (p DeepSeek) headers() map[string]string {
	return map[string]string{"Authorization": "Bearer " + p.APIKey}
}

func newDeepSeekSearchRequest(query string, options DeepSeekSearchOptions) (deepSeekSearchRequest, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > DeepSeekMaxQueryRunes {
		return deepSeekSearchRequest{}, deepSeekParameterError("DeepSeek search query must be 1-128 characters and not blank")
	}
	if options.Count < DeepSeekMinCount || options.Count > DeepSeekMaxCount {
		return deepSeekSearchRequest{}, deepSeekParameterError("DeepSeek search count must be between 1 and 50")
	}
	// ori_question 的上游约束是 1-128 字符，空字符串本身非法，因此为空时省略字段。
	oriQuestion := strings.TrimSpace(options.OriQuestion)
	if utf8.RuneCountInString(oriQuestion) > DeepSeekMaxQueryRunes {
		return deepSeekSearchRequest{}, deepSeekParameterError("DeepSeek search ori_question must be 1-128 characters")
	}
	// localization 只做格式校验，错误消息不回显其值，避免把调用方 IP 写入输出。
	localization := strings.TrimSpace(options.Localization)
	if localization != "" && net.ParseIP(localization) == nil {
		return deepSeekSearchRequest{}, deepSeekParameterError("DeepSeek search localization must be a valid IP address")
	}
	return deepSeekSearchRequest{
		Query:        query,
		Count:        options.Count,
		OriQuestion:  oriQuestion,
		Localization: localization,
		NeedContent:  options.NeedContent,
	}, nil
}

func validateDeepSeekFetchURL(raw string) (string, error) {
	targetURL := strings.TrimSpace(raw)
	if targetURL == "" || len(targetURL) > deepSeekMaxURLLength {
		return "", deepSeekParameterError("DeepSeek fetch url must be 1-8192 characters")
	}
	parsed, err := url.Parse(targetURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", deepSeekParameterError("DeepSeek fetch url must be an http(s) URL with a host")
	}
	// 拒绝 userinfo，避免把 URL 中的凭据发送给第三方。
	if parsed.User != nil {
		return "", deepSeekParameterError("DeepSeek fetch url must not contain user credentials")
	}
	return targetURL, nil
}

// deepSeekEndpoint 把 API 根路径归一化为 /v1/tools/<op>。兼容 chat 用户常用的 .../v1，
// 拒绝已指向 /tools 的完整 endpoint，避免拼出 /v1/tools/search/v1/tools/search。
func deepSeekEndpoint(baseURL, op string) (string, error) {
	raw := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(raw)
	if raw == "" || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", &ProviderError{Type: "config_error", Message: "DeepSeek base_url must be an http(s) API root such as https://api.deepseek.com"}
	}
	path := strings.TrimRight(parsed.Path, "/")
	lower := strings.ToLower(path)
	for _, suffix := range []string{"/tools", "/tools/search", "/tools/fetch"} {
		if strings.HasSuffix(lower, suffix) {
			return "", &ProviderError{Type: "config_error", Message: "DeepSeek base_url must be the API root (for example https://api.deepseek.com), not a /tools endpoint"}
		}
	}
	if strings.HasSuffix(lower, "/v1") {
		path += "/tools/" + op
	} else {
		path += "/v1/tools/" + op
	}
	parsed.Path = path
	parsed.RawPath = ""
	return parsed.String(), nil
}

// normalizeDeepSeekResults 保持上游顺序，不按 rerank_score/authority 过滤；null 与空字符串字段省略。
// description 显式取 snippet（回退 summary），避免 NormalizeSourceResults 被空 content 遮蔽。
func normalizeDeepSeekResults(items []any, needContent bool) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		result := map[string]any{"provider": "deepseek"}
		putDeepSeekString(result, "title", item["title"])
		putDeepSeekString(result, "url", item["url"])
		snippet := strings.TrimSpace(stringValue(item["snippet"]))
		summary := strings.TrimSpace(stringValue(item["summary"]))
		if description := firstNonEmpty(snippet, summary); description != "" {
			result["description"] = description
		}
		if summary != "" {
			result["summary"] = summary
		}
		if needContent {
			putDeepSeekString(result, "content", item["content"])
		}
		putDeepSeekString(result, "source", item["site_name"])
		putDeepSeekString(result, "icon", item["site_icon"])
		putDeepSeekString(result, "published_date", item["date"])
		for _, key := range []string{"rerank_score", "authority"} {
			if item[key] != nil {
				result[key] = item[key]
			}
		}
		out = append(out, result)
	}
	return out
}

func putDeepSeekString(target map[string]any, key string, value any) {
	if text := strings.TrimSpace(stringValue(value)); text != "" {
		target[key] = text
	}
}

func deepSeekParameterError(message string) error {
	return &ProviderError{Type: "parameter_error", Message: message}
}

// deepSeekError 按 HTTP 状态码改写错误类型，而不依赖上游 error.type 的字符串匹配；
// 修正 402/403/404 在默认映射下的误分类。2xx 非 JSON（含 4 MiB 截断）归为 upstream_error。
func deepSeekError(op string, err error) error {
	var httpErr *HTTPError
	if AsHTTPError(err, &httpErr) {
		mapped := *httpErr
		errorType, hint := deepSeekStatusError(op, httpErr.StatusCode)
		if errorType != "" {
			mapped.ProviderType = errorType
		}
		message := firstNonEmpty(strings.TrimSpace(httpErr.Message), strings.TrimSpace(httpErr.Body), httpErr.Status)
		if hint != "" && message != "" {
			message += "; " + hint
		} else if hint != "" {
			message = hint
		}
		mapped.Message = "DeepSeek " + op + ": " + truncateMessage(message, deepSeekMaxMessageLen)
		return &mapped
	}
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		message := "DeepSeek " + op + ": response is not a valid JSON object"
		if op == "search" {
			message += "; if results are large, reduce count or disable need_content"
		}
		return &ProviderError{Type: "upstream_error", Message: message}
	}
	return err
}

func deepSeekStatusError(op string, status int) (string, string) {
	switch status {
	case 400, 413:
		return "parameter_error", ""
	case 401:
		return "auth_error", "check the DeepSeek API key"
	case 402:
		return "provider_error", "insufficient account balance; top up before retrying"
	case 403:
		if op == "fetch" {
			return "provider_error", "URL rejected by DeepSeek safety policy; this is not a credential problem"
		}
		return "auth_error", ""
	case 404:
		if op == "fetch" {
			return "empty_result", "no usable page content for this URL"
		}
		return "", ""
	case 408, 504:
		return "timeout", ""
	case 429:
		return "rate_limited", "DeepSeek QPS limit exceeded"
	case 500, 502, 503:
		return "upstream_error", ""
	default:
		return "", ""
	}
}

func truncateMessage(message string, limit int) string {
	if utf8.RuneCountInString(message) <= limit {
		return message
	}
	return string([]rune(message)[:limit])
}
