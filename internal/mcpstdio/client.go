package mcpstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const defaultProtocolVersion = "2025-03-26"

type Config struct {
	Command         string
	Args            []string
	Env             map[string]string
	Timeout         time.Duration
	ProtocolVersion string
	SessionMode     string
}

type Tool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Meta         map[string]any `json:"_meta,omitempty"`
}

type CallResult struct {
	Result          map[string]any
	Tools           []Tool
	Stderr          string
	ProtocolVersion string
	SessionMode     string
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
	if e.Message != "" {
		return e.Message
	}
	return e.Type
}

type Client struct {
	cmd             *exec.Cmd
	stdin           io.WriteCloser
	events          chan rpcEvent
	nextID          int
	write           sync.Mutex
	stderr          *limitedBuffer
	modern          bool
	protocolVersion string
	pendingMu       sync.Mutex
	pending         map[string]chan rpcEvent
}

type rpcEvent struct {
	message rpcMessage
	err     error
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func CallTool(ctx context.Context, cfg Config, toolName string, arguments map[string]any) (CallResult, error) {
	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}
	client, err := Start(ctx, cfg)
	if err != nil {
		return CallResult{}, err
	}
	defer client.Close()
	if !client.modern {
		if err := client.Initialize(ctx); err != nil {
			return CallResult{Stderr: client.Stderr()}, err
		}
	} else {
		if _, err := client.Discover(ctx); err != nil {
			client.modern = false
			client.protocolVersion = defaultProtocolVersion
			if initErr := client.Initialize(ctx); initErr != nil {
				return CallResult{Stderr: client.Stderr(), ProtocolVersion: client.protocolVersion, SessionMode: sessionModeForClient(client)}, initErr
			}
		}
	}
	tools, err := client.ListTools(ctx)
	if err != nil {
		return CallResult{Stderr: client.Stderr()}, err
	}
	resolvedToolName, found := resolveToolName(tools, toolName)
	if !found {
		return CallResult{Tools: tools, Stderr: client.Stderr(), ProtocolVersion: client.protocolVersion, SessionMode: sessionModeForClient(client)}, &Error{Type: "capability_unavailable", Message: "mcp_stdio tool not found: " + toolName}
	}
	result, err := client.CallTool(ctx, resolvedToolName, arguments)
	return CallResult{Result: result, Tools: tools, Stderr: client.Stderr(), ProtocolVersion: client.protocolVersion, SessionMode: sessionModeForClient(client)}, err
}

func sessionModeForClient(client *Client) string {
	if client != nil && client.modern {
		return "modern"
	}
	return "legacy"
}

func Start(ctx context.Context, cfg Config) (*Client, error) {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		return nil, &Error{Type: "config_error", Message: "mcp_stdio command is empty"}
	}
	cmd := exec.CommandContext(ctx, command, cfg.Args...)
	cmd.Env = mergedEnv(cfg.Env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, normalizeStartError(command, err)
	}
	client := &Client{
		cmd:     cmd,
		stdin:   stdin,
		events:  make(chan rpcEvent, 16),
		stderr:  newLimitedBuffer(16 * 1024),
		pending: make(map[string]chan rpcEvent),
	}
	client.modern = strings.EqualFold(strings.TrimSpace(cfg.SessionMode), "modern")
	client.protocolVersion = strings.TrimSpace(cfg.ProtocolVersion)
	if client.protocolVersion == "" {
		client.protocolVersion = "2026-07-28"
	}
	go client.readLoop(stdout)
	go client.dispatchLoop()
	go func() {
		_, _ = io.Copy(client.stderr, stderrPipe)
	}()
	return client, nil
}

func (c *Client) Initialize(ctx context.Context) error {
	_, err := c.request(ctx, "initialize", map[string]any{
		"protocolVersion": defaultProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "onesearch",
			"version": "local",
		},
	})
	if err != nil {
		return err
	}
	return c.notify("notifications/initialized", map[string]any{})
}

func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var tools []Tool
	var cursor string
	for page := 0; page < 20; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := c.request(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		var payload struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(result, &payload); err != nil {
			return nil, &Error{Type: "protocol_error", Message: "invalid tools/list result: " + err.Error()}
		}
		tools = append(tools, payload.Tools...)
		cursor = strings.TrimSpace(payload.NextCursor)
		if cursor == "" || len(tools) >= 1000 {
			break
		}
	}
	return tools, nil
}

func (c *Client) Discover(ctx context.Context) (map[string]any, error) {
	result, err := c.request(ctx, "server/discover", map[string]any{})
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, &Error{Type: "protocol_error", Message: "invalid server/discover result: " + err.Error()}
	}
	return out, nil
}

func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (map[string]any, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, &Error{Type: "config_error", Message: "mcp_stdio tool name is empty"}
	}
	result, err := c.request(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(result, &out); err != nil {
		return nil, &Error{Type: "protocol_error", Message: "invalid tools/call result: " + err.Error()}
	}
	return out, nil
}

func (c *Client) Stderr() string {
	if c == nil || c.stderr == nil {
		return ""
	}
	return c.stderr.String()
}

func (c *Client) Close() {
	if c == nil || c.cmd == nil {
		return
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	done := make(chan struct{}, 1)
	go func() {
		_ = c.cmd.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = c.cmd.Process.Kill()
		<-done
	}
}

func (c *Client) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.write.Lock()
	c.nextID++
	id := c.nextID
	responseCh := make(chan rpcEvent, 1)
	c.pendingMu.Lock()
	c.pending[fmt.Sprint(id)] = responseCh
	c.pendingMu.Unlock()
	defer func() { c.pendingMu.Lock(); delete(c.pending, fmt.Sprint(id)); c.pendingMu.Unlock() }()
	if c.modern {
		if values, ok := params.(map[string]any); ok {
			values = cloneAnyMap(values)
			values["_meta"] = map[string]any{
				"io.modelcontextprotocol/protocolVersion": c.protocolVersion,
				"clientInfo":   map[string]any{"name": "onesearch", "version": "local"},
				"capabilities": map[string]any{},
			}
			params = values
		}
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		c.write.Unlock()
		return nil, err
	}
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		c.write.Unlock()
		return nil, err
	}
	c.write.Unlock()
	for {
		select {
		case <-ctx.Done():
			return nil, &Error{Type: "timeout", Message: ctx.Err().Error()}
		case event, ok := <-responseCh:
			if !ok {
				return nil, &Error{Type: "protocol_error", Message: c.closedMessage()}
			}
			if event.err != nil {
				return nil, event.err
			}
			if !idMatches(event.message.ID, id) {
				continue
			}
			if event.message.Error != nil {
				return nil, &Error{Type: "provider_error", Message: event.message.Error.Message}
			}
			if len(event.message.Result) == 0 {
				return json.RawMessage(`{}`), nil
			}
			return event.message.Result, nil
		}
	}
}

func (c *Client) dispatchLoop() {
	for event := range c.events {
		if event.err != nil {
			c.pendingMu.Lock()
			for _, channel := range c.pending {
				channel <- event
			}
			c.pendingMu.Unlock()
			continue
		}
		key := rpcIDKey(event.message.ID)
		c.pendingMu.Lock()
		channel := c.pending[key]
		c.pendingMu.Unlock()
		if channel != nil {
			channel <- event
		}
	}
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	for _, channel := range c.pending {
		channel <- rpcEvent{err: &Error{Type: "protocol_error", Message: c.closedMessage()}}
	}
}

func rpcIDKey(value any) string {
	switch typed := value.(type) {
	case float64:
		return fmt.Sprint(int(typed))
	case int:
		return fmt.Sprint(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func cloneAnyMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input)+1)
	for key, value := range input {
		out[key] = value
	}
	return out
}

func (c *Client) closedMessage() string {
	message := "mcp_stdio server stdout closed"
	if stderr := c.Stderr(); stderr != "" {
		message += "; stderr: " + truncate(strings.ReplaceAll(stderr, "\n", " "), 1000)
	}
	return message
}

func (c *Client) notify(method string, params any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	c.write.Lock()
	defer c.write.Unlock()
	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

func (c *Client) readLoop(stdout io.Reader) {
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && strings.TrimSpace(line) == "" {
				close(c.events)
				return
			}
			if strings.TrimSpace(line) != "" {
				c.emitLine(line)
			}
			if !errors.Is(err, io.EOF) {
				c.events <- rpcEvent{err: err}
			}
			close(c.events)
			return
		}
		c.emitLine(line)
	}
}

func (c *Client) emitLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	var message rpcMessage
	if err := json.Unmarshal([]byte(line), &message); err != nil {
		c.events <- rpcEvent{err: &Error{Type: "protocol_error", Message: "non-JSON stdout from mcp_stdio server: " + truncate(line, 240)}}
		return
	}
	c.events <- rpcEvent{message: message}
}

func mergedEnv(extra map[string]string) []string {
	env := os.Environ()
	if len(extra) == 0 {
		return env
	}
	seen := map[string]int{}
	for index, item := range env {
		if key, _, ok := strings.Cut(item, "="); ok {
			seen[strings.ToUpper(key)] = index
		}
	}
	for key, value := range extra {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		entry := key + "=" + value
		if index, ok := seen[strings.ToUpper(key)]; ok {
			env[index] = entry
			continue
		}
		env = append(env, entry)
	}
	return env
}

func normalizeStartError(command string, err error) error {
	if _, lookupErr := exec.LookPath(command); lookupErr != nil {
		return &Error{Type: "missing_command", Message: "mcp_stdio command not found: " + command}
	}
	return err
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

func resolveToolName(tools []Tool, name string) (string, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return name, true
		}
	}
	if suffix, ok := rawToolSuffix(name); ok {
		for _, tool := range tools {
			if tool.Name == suffix {
				return suffix, true
			}
		}
	}
	return name, false
}

func rawToolSuffix(name string) (string, bool) {
	parts := strings.Split(name, "__")
	if len(parts) >= 3 && parts[0] == "mcp" {
		suffix := strings.Join(parts[2:], "__")
		return suffix, suffix != ""
	}
	return "", false
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit-3]) + "..."
}

type limitedBuffer struct {
	mu    sync.Mutex
	limit int
	buf   bytes.Buffer
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := len(data)
	if b.limit <= 0 {
		return written, nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		return written, nil
	}
	if len(data) > remaining {
		data = data[:remaining]
	}
	_, _ = b.buf.Write(data)
	return written, nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(b.buf.String())
}
