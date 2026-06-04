package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/app"
)

const SearchPrompt = `You are a helpful research assistant. Answer the user's question thoroughly using web search results.

Guidelines:
- Infer the user's true intent even when the question is vague.
- Search broadly first, then go deep on the most relevant sources.
- Prioritize authoritative sources: official docs, Wikipedia, academic papers, reputable journalism.
- Search in English first for breadth, switch to Chinese when the topic demands it.
- Every factual claim should cite its source.
- Lead with the most likely answer, then provide supporting analysis.
- Format output in clean Markdown.
- Be direct and concise.`

type OpenAICompatible struct {
	APIURL     string
	APIKey     string
	Model      string
	Stream     bool
	Tools      []map[string]any
	ToolChoice any
}

func (p OpenAICompatible) Name() string {
	return "OpenAI-compatible"
}

func (p OpenAICompatible) Search(ctx context.Context, query, platform string) (string, error) {
	platformPrompt := ""
	if strings.TrimSpace(platform) != "" {
		platformPrompt = "\n\nYou should search the web for the information you need, and focus on these platform: " + platform + "\n"
	}
	payload := map[string]any{
		"model": p.Model,
		"messages": []map[string]string{
			{"role": "system", "content": SearchPrompt},
			{"role": "user", "content": localTimeContext() + "\n" + query + platformPrompt},
		},
		"stream": p.Stream,
	}
	if len(p.Tools) > 0 {
		payload["tools"] = p.Tools
	}
	if p.ToolChoice != nil {
		payload["tool_choice"] = p.ToolChoice
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpointURL(p.APIURL, "chat/completions"), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", app.UserAgent)
	resp, err := Client(120 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorType, message := extractProviderError(data)
		return "", &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: trimBody(data, 500), ProviderType: errorType, Message: message}
	}
	if isSSEBody(resp.Header, data) {
		return parseChatSSE(data)
	}
	return parseChatCompletion(data)
}

func parseChatCompletion(data []byte) (string, error) {
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return "", err
	}
	if errorType, message := extractProviderErrorFromMap(body); errorType != "" || message != "" {
		return "", &ProviderError{Type: errorType, Message: message}
	}
	content := ""
	if choices, ok := body["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if text, ok := message["content"].(string); ok {
					content = text
				}
			}
		}
	}
	sources := extractCitations(body)
	if content != "" && len(sources) > 0 {
		encoded, _ := json.Marshal(sources)
		content = strings.TrimSpace(content) + "\n\nsources(" + string(encoded) + ")"
	}
	return content, nil
}

func parseChatSSE(data []byte) (string, error) {
	var content strings.Builder
	for _, event := range parseSSEEvents(data) {
		payload := strings.TrimSpace(event.Data)
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(payload), &item); err != nil {
			continue
		}
		if errorType, message := extractProviderErrorFromMap(item); errorType != "" || message != "" {
			return "", &ProviderError{Type: errorType, Message: message}
		}
		if choices, ok := item["choices"].([]any); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]any); ok {
				if delta, ok := choice["delta"].(map[string]any); ok {
					if text, ok := delta["content"].(string); ok {
						content.WriteString(text)
					}
				}
			}
		}
	}
	return content.String(), nil
}

func extractCitations(body map[string]any) []map[string]any {
	seen := map[string]struct{}{}
	var out []map[string]any
	add := func(value any) {
		switch item := value.(type) {
		case string:
			if !strings.HasPrefix(item, "http://") && !strings.HasPrefix(item, "https://") {
				return
			}
			if _, ok := seen[item]; ok {
				return
			}
			seen[item] = struct{}{}
			out = append(out, map[string]any{"url": item})
		case map[string]any:
			url, _ := item["url"].(string)
			if url == "" {
				url, _ = item["href"].(string)
			}
			if url == "" {
				url, _ = item["link"].(string)
			}
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				return
			}
			if _, ok := seen[url]; ok {
				return
			}
			seen[url] = struct{}{}
			source := map[string]any{"url": url}
			for _, key := range []string{"title", "name", "label"} {
				if title, ok := item[key].(string); ok && strings.TrimSpace(title) != "" {
					source["title"] = strings.TrimSpace(title)
					break
				}
			}
			out = append(out, source)
		}
	}
	var walk func(any)
	walk = func(value any) {
		switch item := value.(type) {
		case []any:
			for _, nested := range item {
				walk(nested)
			}
		case map[string]any:
			if citations, ok := item["citations"]; ok {
				if list, ok := citations.([]any); ok {
					for _, citation := range list {
						add(citation)
					}
				} else {
					add(citations)
				}
			}
			for _, nested := range item {
				walk(nested)
			}
		}
	}
	walk(body)
	return out
}

func localTimeContext() string {
	now := time.Now()
	weekdays := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	return "[Current Time Context]\n- Date: " + now.Format("2006-01-02") + " (" + weekdays[int(now.Weekday())] + ")\n- Time: " + now.Format("15:04:05") + "\n- Timezone: " + now.Format("MST")
}

type sseEvent struct {
	Type string
	Data string
}

func openAIEndpointURL(apiURL, endpoint string) string {
	base := strings.TrimSpace(apiURL)
	if base == "" {
		base = "https://api.openai.com"
	}
	endpoint = strings.Trim(endpoint, "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		base = strings.TrimRight(base, "/")
		if !pathContainsV1(base) {
			base += "/v1"
		}
		return base + "/" + endpoint
	}
	path := strings.TrimRight(parsed.EscapedPath(), "/")
	if path == "" {
		path = "/v1"
	} else if !pathContainsV1(path) {
		path += "/v1"
	}
	parsed.Path = path + "/" + endpoint
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func pathContainsV1(path string) bool {
	normalized := "/" + strings.Trim(strings.ToLower(path), "/") + "/"
	return strings.Contains(normalized, "/v1/")
}

func isSSEBody(header http.Header, data []byte) bool {
	contentType := strings.ToLower(header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		return true
	}
	trimmed := bytes.TrimSpace(data)
	return bytes.HasPrefix(trimmed, []byte("data:")) || bytes.HasPrefix(trimmed, []byte("event:")) || bytes.Contains(trimmed, []byte("\nevent:")) || bytes.Contains(trimmed, []byte("\ndata:"))
}

func parseSSEEvents(data []byte) []sseEvent {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var events []sseEvent
	current := sseEvent{}
	var dataLines []string
	flush := func() {
		if len(dataLines) == 0 && current.Type == "" {
			return
		}
		current.Data = strings.Join(dataLines, "\n")
		events = append(events, current)
		current = sseEvent{}
		dataLines = nil
	}
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "event:") {
			current.Type = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	flush()
	return events
}
