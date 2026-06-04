package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/app"
)

type Result map[string]any

type Attempt struct {
	Capability  string  `json:"capability"`
	Provider    string  `json:"provider"`
	Status      string  `json:"status"`
	ErrorType   string  `json:"error_type"`
	Error       string  `json:"error"`
	ElapsedMS   float64 `json:"elapsed_ms"`
	ResultCount int     `json:"result_count"`
}

type HTTPError struct {
	StatusCode   int
	Status       string
	Body         string
	ProviderType string
	Message      string
}

type ProviderError struct {
	Type    string
	Message string
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.ProviderType != "" || e.Message != "" {
		message := firstNonEmpty(e.Message, e.Body, e.Status)
		if e.StatusCode > 0 {
			if e.ProviderType != "" {
				return fmt.Sprintf("%s: %s (HTTP %d)", e.ProviderType, message, e.StatusCode)
			}
			return fmt.Sprintf("%s (HTTP %d)", message, e.StatusCode)
		}
		if e.ProviderType != "" {
			return e.ProviderType + ": " + message
		}
		return message
	}
	if e.Body == "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Status)
	}
	return fmt.Sprintf("HTTP %d: %s - %s", e.StatusCode, e.Status, e.Body)
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	if e.Type != "" && e.Message != "" {
		return e.Type + ": " + e.Message
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Type
}

func PostJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", app.UserAgent)
	for key, value := range headers {
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorType, message := extractProviderError(data)
		return &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: trimBody(data, 500), ProviderType: errorType, Message: message}
	}
	if out == nil {
		if readErr != nil {
			return readErr
		}
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		if candidate := jsonRPCResponsePayload(sseJSONPayloads(string(data))); candidate != "" {
			if jsonErr := json.Unmarshal([]byte(candidate), out); jsonErr == nil {
				return nil
			}
		}
		if readErr != nil {
			return readErr
		}
		return err
	}
	if readErr != nil {
		return readErr
	}
	return nil
}

func GetJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/plain")
	req.Header.Set("User-Agent", app.UserAgent)
	for key, value := range headers {
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorType, message := extractProviderError(data)
		return &HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: trimBody(data, 500), ProviderType: errorType, Message: message}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

func Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func ErrorPayload(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	var httpErr *HTTPError
	if AsHTTPError(err, &httpErr) {
		if httpErr.ProviderType != "" {
			return normalizeProviderErrorType(httpErr.ProviderType, httpErr.StatusCode), httpErr.Error()
		}
		switch httpErr.StatusCode {
		case 400, 422:
			return "parameter_error", httpErr.Error()
		case 401, 403:
			return "auth_error", httpErr.Error()
		case 429:
			return "rate_limited", httpErr.Error()
		default:
			return "network_error", httpErr.Error()
		}
	}
	var providerErr *ProviderError
	if AsProviderError(err, &providerErr) {
		return normalizeProviderErrorType(providerErr.Type, 0), providerErr.Error()
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") {
		return "timeout", "request timed out"
	}
	return "network_error", err.Error()
}

func AsHTTPError(err error, target **HTTPError) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*HTTPError); ok {
		*target = e
		return true
	}
	return false
}

func AsProviderError(err error, target **ProviderError) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*ProviderError); ok {
		*target = e
		return true
	}
	return false
}

func Elapsed(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000
}

func extractProviderError(data []byte) (string, string) {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", ""
	}
	for _, candidate := range sseJSONPayloads(text) {
		if errorType, message := extractProviderErrorFromJSON([]byte(candidate)); errorType != "" || message != "" {
			return errorType, message
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "{") {
			if errorType, message := extractProviderErrorFromJSON([]byte(line)); errorType != "" || message != "" {
				return errorType, message
			}
		}
	}
	return extractProviderErrorFromJSON([]byte(text))
}

func extractProviderErrorFromJSON(data []byte) (string, string) {
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return "", ""
	}
	return extractProviderErrorFromMap(body)
}

func extractProviderErrorFromMap(body map[string]any) (string, string) {
	if body == nil {
		return "", ""
	}
	if errorBody, ok := body["error"].(map[string]any); ok {
		errorType := firstNonEmpty(stringValue(errorBody["type"]), stringValue(errorBody["code"]))
		message := firstNonEmpty(stringValue(errorBody["message"]), stringValue(errorBody["detail"]))
		if errorType != "" || message != "" {
			return errorType, message
		}
	}
	if response, ok := body["response"].(map[string]any); ok {
		if errorBody, ok := response["error"].(map[string]any); ok {
			errorType := firstNonEmpty(stringValue(errorBody["type"]), stringValue(errorBody["code"]))
			message := firstNonEmpty(stringValue(errorBody["message"]), stringValue(errorBody["detail"]))
			if errorType != "" || message != "" {
				return errorType, message
			}
		}
	}
	errorType := firstNonEmpty(stringValue(body["error_type"]), stringValue(body["type"]), stringValue(body["code"]))
	message := firstNonEmpty(stringValue(body["message"]), stringValue(body["detail"]))
	if strings.HasPrefix(errorType, "response.") && message == "" {
		return "", ""
	}
	return errorType, message
}

func sseJSONPayloads(text string) []string {
	var payloads []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func jsonRPCResponsePayload(candidates []string) string {
	selected := ""
	for _, candidate := range candidates {
		var item map[string]any
		if err := json.Unmarshal([]byte(candidate), &item); err != nil {
			continue
		}
		if _, ok := item["result"]; ok {
			selected = candidate
			continue
		}
		if _, ok := item["error"]; ok {
			selected = candidate
		}
	}
	return selected
}

func normalizeProviderErrorType(errorType string, statusCode int) string {
	text := strings.ToLower(strings.TrimSpace(errorType))
	switch {
	case strings.Contains(text, "upstream"):
		return "upstream_error"
	case strings.Contains(text, "auth") || strings.Contains(text, "forbidden") || strings.Contains(text, "permission"):
		return "auth_error"
	case strings.Contains(text, "rate"):
		return "rate_limited"
	case strings.Contains(text, "invalid") || strings.Contains(text, "parameter") || strings.Contains(text, "bad_request"):
		return "parameter_error"
	case text != "":
		return text
	}
	switch statusCode {
	case 400, 422:
		return "parameter_error"
	case 401, 403:
		return "auth_error"
	case 429:
		return "rate_limited"
	default:
		return "network_error"
	}
}

func trimBody(data []byte, limit int) string {
	text := strings.TrimSpace(string(data))
	if len(text) > limit {
		return text[:limit]
	}
	return text
}

func NormalizeSourceResults(results []map[string]any, provider string) []map[string]any {
	var out []map[string]any
	for _, item := range results {
		url := firstString(item, "url", "link")
		if strings.TrimSpace(url) == "" {
			continue
		}
		source := map[string]any{"url": strings.TrimSpace(url), "provider": valueOr(provider, firstString(item, "provider"))}
		if title := firstString(item, "title"); strings.TrimSpace(title) != "" {
			source["title"] = strings.TrimSpace(title)
		}
		if desc := firstString(item, "description", "content", "snippet", "text"); strings.TrimSpace(desc) != "" {
			source["description"] = strings.TrimSpace(desc)
		}
		if published := firstString(item, "published_date", "publishedDate", "publish_date"); published != "" {
			source["published_date"] = published
		}
		if media := firstString(item, "source", "media"); media != "" {
			source["source"] = media
		}
		out = append(out, source)
	}
	return out
}

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok {
			return value
		}
	}
	return ""
}

func valueOr(primary, fallback string) string {
	if fallback != "" {
		return fallback
	}
	return primary
}
