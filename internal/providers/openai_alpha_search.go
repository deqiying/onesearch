package providers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/app"
)

const openAIAlphaSearchResponseLimit = 8 * 1024 * 1024

type OpenAIResponses struct {
	APIURL string
	APIKey string
	Model  string
}

type openAIAlphaSearchRequest struct {
	ID       string                    `json:"id"`
	Model    string                    `json:"model"`
	Commands openAIAlphaSearchCommands `json:"commands"`
	Settings openAIAlphaSearchSettings `json:"settings"`
}

type openAIAlphaSearchCommands struct {
	SearchQuery []openAIAlphaSearchQuery `json:"search_query"`
}

type openAIAlphaSearchQuery struct {
	Query string `json:"q"`
}

type openAIAlphaSearchSettings struct {
	AllowedCallers    []string `json:"allowed_callers"`
	ExternalWebAccess bool     `json:"external_web_access"`
}

func (p OpenAIResponses) Name() string {
	return "OpenAI Responses"
}

func (p OpenAIResponses) Search(ctx context.Context, query, platform string) (string, error) {
	requestID, err := newOpenAIAlphaSearchRequestID()
	if err != nil {
		return "", err
	}
	payload, err := buildOpenAIAlphaSearchRequest(requestID, p.Model, query, platform)
	if err != nil {
		return "", err
	}
	data, header, err := postOpenAIAlphaSearch(ctx, p.APIURL, p.APIKey, payload)
	if err != nil {
		return "", err
	}
	return parseOpenAIAlphaSearchResponse(data, header)
}

func buildOpenAIAlphaSearchRequest(requestID, model, query, platform string) (openAIAlphaSearchRequest, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return openAIAlphaSearchRequest{}, &ProviderError{Type: "parameter_error", Message: "alpha search request id is required"}
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return openAIAlphaSearchRequest{}, &ProviderError{Type: "parameter_error", Message: "alpha search model is required"}
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return openAIAlphaSearchRequest{}, &ProviderError{Type: "parameter_error", Message: "search query is required"}
	}
	if platform = strings.TrimSpace(platform); platform != "" {
		query += "\n\nPreferred platform or source: " + platform
	}
	return openAIAlphaSearchRequest{
		ID:    requestID,
		Model: model,
		Commands: openAIAlphaSearchCommands{
			SearchQuery: []openAIAlphaSearchQuery{{Query: query}},
		},
		Settings: openAIAlphaSearchSettings{
			AllowedCallers:    []string{"direct"},
			ExternalWebAccess: true,
		},
	}, nil
}

func newOpenAIAlphaSearchRequestID() (string, error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate alpha search request id: %w", err)
	}
	return "onesearch-" + hex.EncodeToString(random), nil
}

func postOpenAIAlphaSearch(ctx context.Context, apiURL, apiKey string, payload openAIAlphaSearchRequest) ([]byte, http.Header, error) {
	if err := validateOpenAIAlphaSearchBaseURL(apiURL); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpointURL(apiURL, "alpha/search"), bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", app.UserAgent)

	resp, err := Client(120 * time.Second).Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	data, readErr := io.ReadAll(io.LimitReader(resp.Body, openAIAlphaSearchResponseLimit+1))
	if readErr != nil {
		return nil, resp.Header, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited := data
		if len(limited) > openAIAlphaSearchResponseLimit {
			limited = limited[:openAIAlphaSearchResponseLimit]
		}
		sanitized := sanitizeOpenAIAlphaSearchErrorBody(limited)
		errorType, message := extractProviderError(sanitized)
		return nil, resp.Header, &HTTPError{
			StatusCode:   resp.StatusCode,
			Status:       resp.Status,
			Body:         trimBody(sanitized, 500),
			ProviderType: errorType,
			Message:      message,
		}
	}
	if len(data) > openAIAlphaSearchResponseLimit {
		return nil, resp.Header, &ProviderError{Type: "upstream_error", Message: "alpha search response exceeds 8 MiB limit"}
	}
	return data, resp.Header, nil
}

func parseOpenAIAlphaSearchResponse(data []byte, header http.Header) (string, error) {
	if isSSEBody(header, data) {
		return "", &ProviderError{Type: "upstream_error", Message: "alpha search returned an unsupported event stream"}
	}

	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil || body == nil {
		return "", &ProviderError{Type: "upstream_error", Message: "alpha search returned an invalid JSON object"}
	}
	if errorType, message := extractProviderErrorFromMap(body); errorType != "" || message != "" {
		return "", &ProviderError{
			Type:    firstNonEmpty(errorType, "upstream_error"),
			Message: firstNonEmpty(message, "alpha search returned an error"),
		}
	}

	output, ok := body["output"].(string)
	if !ok || strings.TrimSpace(output) == "" {
		return "", &ProviderError{Type: "upstream_error", Message: "alpha search response output is missing or empty"}
	}
	output = strings.TrimSpace(output)

	var sources []map[string]any
	seen := map[string]struct{}{}
	if rawResults, exists := body["results"]; exists {
		results, ok := rawResults.([]any)
		if !ok {
			return "", &ProviderError{Type: "upstream_error", Message: "alpha search response results must be an array"}
		}
		for _, rawResult := range results {
			result, ok := rawResult.(map[string]any)
			if !ok {
				continue
			}
			rawURL, ok := result["url"].(string)
			if !ok {
				continue
			}
			normalizedURL, ok := normalizeHTTPURL(rawURL)
			if !ok {
				continue
			}
			if _, ok := seen[normalizedURL]; ok {
				continue
			}
			seen[normalizedURL] = struct{}{}
			source := map[string]any{"url": normalizedURL}
			if title, ok := result["title"].(string); ok && strings.TrimSpace(title) != "" {
				source["title"] = strings.TrimSpace(title)
			}
			sources = append(sources, source)
		}
	}

	if len(sources) == 0 {
		return output, nil
	}
	encoded, err := json.Marshal(sources)
	if err != nil {
		return "", &ProviderError{Type: "upstream_error", Message: "alpha search sources could not be encoded"}
	}
	return output + "\n\nsources(" + string(encoded) + ")", nil
}

func normalizeHTTPURL(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) {
		return "", false
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String(), true
}

func validateOpenAIAlphaSearchBaseURL(apiURL string) error {
	base := strings.TrimSpace(apiURL)
	if base == "" {
		return nil
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return nil
	}
	path := strings.ToLower(strings.TrimRight(parsed.Path, "/"))
	if strings.HasSuffix(path, "/alpha/search") || strings.HasSuffix(path, "/responses") {
		return &ProviderError{Type: "parameter_error", Message: "openai_responses base_url must be an API root, not a complete endpoint"}
	}
	return nil
}

func sanitizeOpenAIAlphaSearchErrorBody(data []byte) []byte {
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil || body == nil {
		return genericOpenAIAlphaSearchErrorBody()
	}
	removeEncryptedOutput(body)
	errorType, message := extractProviderErrorFromMap(body)
	if errorType == "" && message == "" {
		return genericOpenAIAlphaSearchErrorBody()
	}
	safeError := map[string]any{}
	if errorType != "" {
		safeError["type"] = errorType
	}
	if message != "" {
		safeError["message"] = message
	}
	sanitized, err := json.Marshal(map[string]any{"error": safeError})
	if err != nil {
		return genericOpenAIAlphaSearchErrorBody()
	}
	return sanitized
}

func genericOpenAIAlphaSearchErrorBody() []byte {
	return []byte(`{"error":{"message":"alpha search returned an unrecognized error response"}}`)
}

func removeEncryptedOutput(value any) {
	switch item := value.(type) {
	case map[string]any:
		for key, nested := range item {
			if strings.EqualFold(key, "encrypted_output") {
				delete(item, key)
				continue
			}
			removeEncryptedOutput(nested)
		}
	case []any:
		for _, nested := range item {
			removeEncryptedOutput(nested)
		}
	}
}
