package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/app"
)

type XAIResponses struct {
	APIURL     string
	APIKey     string
	Model      string
	Tools      []map[string]any
	ToolChoice any
}

func (p XAIResponses) Name() string {
	return "xAI Responses"
}

func (p XAIResponses) Search(ctx context.Context, query, platform string) (string, error) {
	return responsesSearch(ctx, p.APIURL, p.APIKey, p.Model, p.Tools, p.ToolChoice, false, query, platform)
}

func responsesSearch(ctx context.Context, apiURL, apiKey, model string, tools []map[string]any, toolChoice any, stream bool, query, platform string) (string, error) {
	platformPrompt := ""
	if strings.TrimSpace(platform) != "" {
		platformPrompt = "\n\nYou should search the web for the information you need, and focus on these platform: " + platform + "\n"
	}
	payload := map[string]any{
		"model":        model,
		"instructions": SearchPrompt,
		"input": []map[string]string{
			{"role": "user", "content": localTimeContext() + "\n" + query + platformPrompt},
		},
		"stream": stream,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	if toolChoice != nil {
		payload["tool_choice"] = toolChoice
	}
	data, header, err := postOpenAIResponse(ctx, apiURL, apiKey, payload)
	if err != nil {
		return "", err
	}
	if isSSEBody(header, data) {
		return parseResponsesSSE(data)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return "", err
	}
	return parseResponsesOutput(body)
}

func postOpenAIResponse(ctx context.Context, apiURL, apiKey string, payload any) ([]byte, http.Header, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpointURL(apiURL, "responses"), strings.NewReader(string(body)))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", app.UserAgent)
	resp, err := Client(120 * time.Second).Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorType, message := extractProviderError(data)
		return nil, resp.Header, &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: trimBody(data, 500), ProviderType: errorType, Message: message}
	}
	return data, resp.Header, nil
}

func parseResponsesOutput(data map[string]any) (string, error) {
	if errorType, message := extractProviderErrorFromMap(data); errorType != "" || message != "" {
		return "", &ProviderError{Type: errorType, Message: message}
	}
	var textParts []string
	var sources []map[string]any
	seen := map[string]struct{}{}
	output, _ := data["output"].([]any)
	for _, rawItem := range output {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		content, _ := item["content"].([]any)
		for _, rawContent := range content {
			part, ok := rawContent.(map[string]any)
			if !ok || part["type"] != "output_text" {
				continue
			}
			if text, ok := part["text"].(string); ok && strings.TrimSpace(text) != "" {
				textParts = append(textParts, strings.TrimSpace(text))
			}
			annotations, _ := part["annotations"].([]any)
			for _, rawAnnotation := range annotations {
				annotation, ok := rawAnnotation.(map[string]any)
				if !ok || annotation["type"] != "url_citation" {
					continue
				}
				url, _ := annotation["url"].(string)
				if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
					continue
				}
				if _, ok := seen[url]; ok {
					continue
				}
				seen[url] = struct{}{}
				source := map[string]any{"url": url}
				if title, ok := annotation["title"].(string); ok && strings.TrimSpace(title) != "" {
					source["title"] = strings.TrimSpace(title)
				}
				sources = append(sources, source)
			}
		}
	}
	answer := strings.Join(textParts, "\n\n")
	if len(sources) > 0 {
		encoded, _ := json.Marshal(sources)
		answer = strings.TrimSpace(answer) + "\n\nsources(" + string(encoded) + ")"
	}
	return strings.TrimSpace(answer), nil
}

func parseResponsesSSE(data []byte) (string, error) {
	var text strings.Builder
	var terminal map[string]any
	for _, event := range parseSSEEvents(data) {
		payload := strings.TrimSpace(event.Data)
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(payload), &item); err != nil {
			continue
		}
		eventType := firstNonEmpty(stringValue(item["type"]), event.Type)
		if eventType == "response.failed" {
			errorType, message := extractProviderErrorFromMap(item)
			return "", &ProviderError{Type: firstNonEmpty(errorType, "upstream_error"), Message: firstNonEmpty(message, "Responses stream failed")}
		}
		if eventType == "response.output_text.delta" {
			text.WriteString(stringValue(item["delta"]))
			continue
		}
		if strings.HasPrefix(eventType, "response.completed") || eventType == "response.done" {
			terminal = item
		}
	}
	if text.String() != "" {
		return strings.TrimSpace(text.String()), nil
	}
	if terminal != nil {
		if response, ok := terminal["response"].(map[string]any); ok {
			return parseResponsesOutput(response)
		}
		return parseResponsesOutput(terminal)
	}
	return "", nil
}
