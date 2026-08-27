package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	ModernProtocolVersion = "2026-07-28"
	LegacyProtocolVersion = "2025-03-26"
)

type Config struct {
	Endpoint        string
	APIKey          string
	Timeout         time.Duration
	ProtocolVersion string
	SessionMode     string
	Headers         map[string]string
}

type Request struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Tool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Meta         map[string]any `json:"_meta,omitempty"`
}

type ToolSnapshot struct {
	Tools  []Tool
	Pages  int
	Cursor string
}

type ProtocolInfo struct {
	SupportedVersions []string       `json:"supportedVersions,omitempty"`
	Capabilities      map[string]any `json:"capabilities,omitempty"`
	ServerInfo        map[string]any `json:"serverInfo,omitempty"`
}

func ResolveTool(snapshot ToolSnapshot, requested string) (string, bool) {
	requested = strings.TrimSpace(requested)
	for _, tool := range snapshot.Tools {
		if tool.Name == requested {
			return tool.Name, true
		}
	}
	parts := strings.Split(requested, "__")
	if len(parts) >= 3 && parts[0] == "mcp" {
		suffix := strings.Join(parts[2:], "__")
		for _, tool := range snapshot.Tools {
			if tool.Name == suffix {
				return tool.Name, true
			}
		}
	}
	return requested, false
}

type Client struct {
	cfg         Config
	http        *http.Client
	mu          sync.Mutex
	nextID      int
	version     string
	legacy      bool
	closed      bool
	initialized bool
}

type Error struct {
	Type    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Type != "" && e.Message != "" {
		return e.Type + ": " + e.Message
	}
	return e.Message
}

func NewHTTP(cfg Config) *Client {
	version := strings.TrimSpace(cfg.ProtocolVersion)
	if version == "" {
		version = ModernProtocolVersion
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}, version: version, legacy: strings.HasPrefix(version, "2025-") || strings.EqualFold(strings.TrimSpace(cfg.SessionMode), "legacy")}
}

func (c *Client) Prepare(ctx context.Context) error {
	if c == nil {
		return &Error{Type: "config_error", Message: "nil MCP client"}
	}
	// Modern stateless endpoints do not require a lifecycle handshake. The
	// first request carries the revision and can be retried in legacy shape only
	// when the server explicitly rejects the modern contract with HTTP 400.
	return nil
}

func (c *Client) Discover(ctx context.Context) (ProtocolInfo, error) {
	resp, err := c.call(ctx, "server/discover", map[string]any{}, "")
	if err != nil {
		return ProtocolInfo{}, err
	}
	var info ProtocolInfo
	if err := json.Unmarshal(resp.Result, &info); err != nil {
		return ProtocolInfo{}, &Error{Type: "protocol_error", Message: "invalid server/discover result: " + err.Error()}
	}
	return info, nil
}

func (c *Client) ListTools(ctx context.Context) (ToolSnapshot, error) {
	var all []Tool
	var cursor string
	pages := 0
	for pages < 20 {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		resp, err := c.call(ctx, "tools/list", params, "")
		if err != nil {
			return ToolSnapshot{}, err
		}
		var payload struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(resp.Result, &payload); err != nil {
			return ToolSnapshot{}, &Error{Type: "protocol_error", Message: "invalid tools/list result: " + err.Error()}
		}
		all = append(all, payload.Tools...)
		pages++
		cursor = strings.TrimSpace(payload.NextCursor)
		if cursor == "" || len(all) >= 1000 {
			break
		}
	}
	return ToolSnapshot{Tools: all, Pages: pages, Cursor: cursor}, nil
}

func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (map[string]any, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, &Error{Type: "config_error", Message: "MCP tool name is empty"}
	}
	if !safeHeaderValue(name) {
		return nil, &Error{Type: "parameter_error", Message: "MCP tool name contains invalid header characters"}
	}
	resp, err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": arguments}, name)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, &Error{Type: "protocol_error", Message: "invalid tools/call result: " + err.Error()}
	}
	return result, nil
}

func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *Client) ProtocolVersion() string {
	if c == nil {
		return ""
	}
	return c.version
}

func (c *Client) SessionMode() string {
	if c == nil {
		return ""
	}
	if c.legacy {
		return "legacy"
	}
	return "modern"
}

func (c *Client) call(ctx context.Context, method string, params map[string]any, toolName string) (Response, error) {
	c.mu.Lock()
	legacyBefore := c.legacy
	c.mu.Unlock()
	if method != "initialize" && legacyBefore {
		if err := c.ensureLegacyInitialized(ctx); err != nil {
			return Response{}, err
		}
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return Response{}, &Error{Type: "session_error", Message: "MCP client is closed"}
	}
	c.nextID++
	id := c.nextID
	version := c.version
	legacy := c.legacy
	c.mu.Unlock()
	if params == nil {
		params = map[string]any{}
	}
	if legacy {
		delete(params, "_meta")
	}
	if !legacy {
		meta := map[string]any{"io.modelcontextprotocol/protocolVersion": version, "clientInfo": map[string]any{"name": "onesearch", "version": "local"}, "capabilities": map[string]any{}}
		params["_meta"] = meta
	}
	body, err := json.Marshal(Request{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(c.cfg.Endpoint), bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if !legacy {
		req.Header.Set("MCP-Protocol-Version", version)
		req.Header.Set("Mcp-Method", method)
		if toolName != "" && safeHeaderValue(toolName) {
			req.Header.Set("Mcp-Name", toolName)
		}
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	for key, value := range c.cfg.Headers {
		if strings.TrimSpace(value) != "" && safeHeaderValue(key) && safeHeaderValue(value) {
			req.Header.Set(key, value)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if readErr != nil {
		return Response{}, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusBadRequest && !legacy && strings.TrimSpace(string(data)) == "" {
			c.mu.Lock()
			c.legacy = true
			c.version = LegacyProtocolVersion
			c.mu.Unlock()
			return c.call(ctx, method, params, toolName)
		}
		return Response{}, &Error{Type: classifyHTTP(resp.StatusCode), Message: truncate(strings.TrimSpace(string(data)), 500)}
	}
	response, err := parseResponse(data, resp.Header.Get("Content-Type"))
	if err != nil {
		return Response{}, err
	}
	if !idMatches(response.ID, id) {
		return Response{}, &Error{Type: "protocol_error", Message: "MCP response id mismatch"}
	}
	if response.Error != nil {
		return Response{}, &Error{Type: "provider_error", Message: response.Error.Message}
	}
	return response, nil
}

func (c *Client) ensureLegacyInitialized(ctx context.Context) error {
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return nil
	}
	version := c.version
	c.mu.Unlock()
	if _, err := c.call(ctx, "initialize", map[string]any{"protocolVersion": version, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "onesearch", "version": "local"}}, ""); err != nil {
		return &Error{Type: "session_error", Message: err.Error()}
	}
	if err := c.notify(ctx, "notifications/initialized", map[string]any{}); err != nil {
		return &Error{Type: "session_error", Message: err.Error()}
	}
	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()
	return nil
}

func (c *Client) notify(ctx context.Context, method string, params map[string]any) error {
	body, err := json.Marshal(Request{JSONRPC: "2.0", Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(c.cfg.Endpoint), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusAccepted && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return &Error{Type: classifyHTTP(resp.StatusCode), Message: truncate(strings.TrimSpace(string(data)), 500)}
	}
	if resp.StatusCode == http.StatusAccepted && len(bytes.TrimSpace(data)) > 0 {
		return &Error{Type: "protocol_error", Message: "MCP notification returned a non-empty 202 body"}
	}
	return nil
}

func parseResponse(data []byte, contentType string) (Response, error) {
	var response Response
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") || !json.Valid(bytes.TrimSpace(data)) {
		for _, candidate := range ssePayloads(string(data)) {
			var item Response
			if json.Unmarshal([]byte(candidate), &item) == nil && (item.ID != nil || item.Error != nil) {
				response = item
			}
		}
		if response.ID == nil && response.Error == nil {
			return Response{}, &Error{Type: "protocol_error", Message: "MCP SSE response missing JSON-RPC message"}
		}
		return response, nil
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return Response{}, &Error{Type: "protocol_error", Message: "invalid MCP JSON response: " + err.Error()}
	}
	return response, nil
}

func ssePayloads(text string) []string {
	var out []string
	var data []string
	flush := func() {
		if len(data) > 0 {
			out = append(out, strings.Join(data, "\n"))
			data = nil
		}
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	flush()
	return out
}

func idMatches(raw any, want int) bool {
	switch value := raw.(type) {
	case float64:
		return int(value) == want
	case int:
		return value == want
	case string:
		return value == fmt.Sprint(want)
	default:
		return false
	}
}

func classifyHTTP(status int) string {
	switch status {
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

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit-3]) + "..."
}

func safeHeaderValue(value string) bool {
	return !strings.ContainsAny(value, "\r\n")
}
