package mcpstdio

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCallToolWithFakeServer(t *testing.T) {
	result, err := CallTool(t.Context(), helperConfig("ok"), "search", map[string]any{"query": "golang"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Name != "search" {
		t.Fatalf("tools = %#v", result.Tools)
	}
	content := result.Result["content"].([]any)
	text := content[0].(map[string]any)["text"]
	if !strings.Contains(text.(string), "https://example.com/golang") {
		t.Fatalf("tool content = %#v", result.Result)
	}
}

func TestCallToolAcceptsCodexNamespacedToolName(t *testing.T) {
	result, err := CallTool(t.Context(), helperConfig("ok"), "mcp__freecrawl__search", map[string]any{"query": "golang"})
	if err != nil {
		t.Fatal(err)
	}
	content := result.Result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "https://example.com/golang") {
		t.Fatalf("tool content = %#v", result.Result)
	}
}

func TestCallToolReportsJSONRPCError(t *testing.T) {
	_, err := CallTool(t.Context(), helperConfig("rpc_error"), "search", map[string]any{"query": "golang"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want boom", err)
	}
}

func TestCallToolRejectsNonJSONStdout(t *testing.T) {
	_, err := CallTool(t.Context(), helperConfig("non_json"), "search", map[string]any{"query": "golang"})
	if err == nil || !strings.Contains(err.Error(), "non-JSON stdout") {
		t.Fatalf("error = %v, want protocol error", err)
	}
}

// 响应已入队但请求方因超时未读取时，stdout EOF 或读错误的广播不得阻塞 dispatchLoop。
func TestDispatchLoopDoesNotBlockWhenPendingEventIsUnread(t *testing.T) {
	cases := map[string]func(c *Client){
		"eof": func(c *Client) { close(c.events) },
		"read_error": func(c *Client) {
			c.events <- rpcEvent{err: errors.New("read failed")}
			close(c.events)
		},
	}
	for name, finish := range cases {
		t.Run(name, func(t *testing.T) {
			c := &Client{events: make(chan rpcEvent, 1), pending: map[string]chan rpcEvent{}}
			unread := make(chan rpcEvent, 1)
			unread <- rpcEvent{message: rpcMessage{ID: float64(1)}}
			c.pending["1"] = unread
			done := make(chan struct{})
			go func() {
				c.dispatchLoop()
				close(done)
			}()
			finish(c)
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("dispatchLoop blocked on a full pending channel")
			}
			// 请求方 defer 需要再次获取 pendingMu。
			c.pendingMu.Lock()
			delete(c.pending, "1")
			c.pendingMu.Unlock()
		})
	}
}

// 超时路径必须终止 mcp_stdio 子进程派生的整棵进程树，例如 uvx -> uv -> python。
func TestCallToolTimeoutTerminatesDescendants(t *testing.T) {
	t.Parallel()
	pid, err := runDescendantScenario(t, "orphan_hang", 3*time.Second)
	var mcpErr *Error
	if !errors.As(err, &mcpErr) || mcpErr.Type != "timeout" {
		t.Fatalf("error = %v, want timeout", err)
	}
	waitProcessExit(t, pid)
}

// 直接子进程正常退出后遗留的后代进程也必须在 Close 时清理。
func TestCallToolCloseTerminatesLeftoverDescendants(t *testing.T) {
	t.Parallel()
	pid, err := runDescendantScenario(t, "orphan_exit", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	waitProcessExit(t, pid)
}

func runDescendantScenario(t *testing.T, mode string, timeout time.Duration) (int, error) {
	t.Helper()
	pidFile := filepath.Join(t.TempDir(), "descendant.pid")
	cfg := helperConfig(mode)
	cfg.Env["MCPSTDIO_HELPER_PIDFILE"] = pidFile
	cfg.Timeout = timeout
	_, callErr := CallTool(t.Context(), cfg, "search", map[string]any{"query": "golang"})
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("descendant pid file: %v (call error: %v)", err, callErr)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		t.Fatalf("descendant pid = %q", data)
	}
	// 修复失效时避免测试遗留 60 秒的 sleep 进程。
	t.Cleanup(func() { killProcess(pid) })
	return pid, callErr
}

func waitProcessExit(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("descendant process %d is still running", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func helperConfig(mode string) Config {
	return Config{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestMCPStdioHelperProcess"},
		Env: map[string]string{
			"GO_WANT_MCPSTDIO_HELPER": "1",
			"MCPSTDIO_HELPER_MODE":    mode,
		},
		Timeout: 2 * time.Second,
	}
}

func TestMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCPSTDIO_HELPER") != "1" {
		return
	}
	mode := os.Getenv("MCPSTDIO_HELPER_MODE")
	if mode == "non_json" {
		os.Stdout.WriteString("not json\n")
		os.Exit(0)
	}
	if mode == "sleep" {
		time.Sleep(60 * time.Second)
		os.Exit(0)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			os.Exit(2)
		}
		id, hasID := request["id"]
		if !hasID {
			continue
		}
		switch request["method"] {
		case "initialize":
			helperRespond(id, map[string]any{"protocolVersion": defaultProtocolVersion, "capabilities": map[string]any{}})
		case "tools/list":
			helperRespond(id, map[string]any{"tools": []map[string]any{{"name": "search", "description": "fake search"}}})
		case "tools/call":
			if mode == "rpc_error" {
				helperRespondError(id, -32000, "boom")
				continue
			}
			if mode == "orphan_hang" || mode == "orphan_exit" {
				helperSpawnDescendant()
			}
			if mode == "orphan_hang" {
				time.Sleep(60 * time.Second)
				os.Exit(0)
			}
			params := request["params"].(map[string]any)
			args := params["arguments"].(map[string]any)
			query := args["query"].(string)
			helperRespond(id, map[string]any{
				"content": []map[string]any{{"type": "text", "text": `{"results":[{"title":"Go","url":"https://example.com/` + query + `"}]}`}},
			})
		default:
			helperRespondError(id, -32601, "method not found")
		}
	}
	os.Exit(0)
}

// helperSpawnDescendant 模拟 launcher 再派生真实 server：后代不持有协议管道，仅记录 PID。
func helperSpawnDescendant() {
	cmd := exec.Command(os.Args[0], "-test.run=TestMCPStdioHelperProcess")
	cmd.Env = append(os.Environ(), "MCPSTDIO_HELPER_MODE=sleep")
	if err := cmd.Start(); err != nil {
		os.Exit(3)
	}
	if err := os.WriteFile(os.Getenv("MCPSTDIO_HELPER_PIDFILE"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		os.Exit(4)
	}
}

func helperRespond(id any, result any) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
	os.Stdout.Write(append(body, '\n'))
}

func helperRespondError(id any, code int, message string) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
	os.Stdout.Write(append(body, '\n'))
}
