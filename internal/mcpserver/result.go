package mcpserver

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/deqiying/onesearch/internal/redact"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxFrameBytes = 64 * 1024
const maxResponseBytes = 2 * 1024 * 1024

func failure(kind, message string) map[string]any {
	return map[string]any{"ok": false, "error_type": kind, "error": message}
}

// projectData 不做 CLI quiet 截断；移除不属于公开业务结果的调试/配置副本。
func projectData(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, item := range v {
			switch strings.ToLower(k) {
			case "raw", "raw_result", "raw_response", "debug", "stderr", "stderr_summary", "provider_stderr", "headers", "request_headers", "response_headers", "settings", "environment", "env", "config_file", "config_dir", "default_config_file", "config_dir_override_value":
				continue
			}
			if k == "error" {
				if text, ok := item.(string); ok {
					if index := strings.Index(text, "; stderr:"); index >= 0 {
						item = text[:index]
					}
				}
			}
			out[k] = projectData(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = projectData(item)
		}
		return out
	default:
		return value
	}
}

func makeResult(tool, operation, version string, data map[string]any, secrets []string) *mcp.CallToolResult {
	// 统一到 JSON 数据模型，覆盖具体 map/slice 类型并验证可序列化性。
	raw, err := json.Marshal(redact.Data(data, secrets))
	var safe map[string]any
	if err != nil || json.Unmarshal(raw, &safe) != nil || safe == nil {
		safe = failure("internal_error", "Unable to encode tool result")
	}
	safe = projectData(safe).(map[string]any)
	if operation == "docs.resolve" && safe["ok"] == true {
		candidates, _ := safe["results"].([]any)
		if candidates == nil {
			candidates = []any{}
		}
		safe["kind"], safe["candidates"], safe["needs_selection"] = "library_candidates", candidates, len(candidates) > 0
		delete(safe, "results")
		if len(candidates) == 0 {
			safe["hint"] = "No library candidates; refine library_name before querying documentation."
		}
	}
	if operation == "docs.query" {
		safe["kind"] = "documentation"
	}
	ok := safe["ok"] == true
	envelope := map[string]any{"ok": ok, "tool": tool, "data": safe}
	if operation != "" {
		envelope["operation"] = operation
	}
	if !ok {
		kind, _ := safe["error_type"].(string)
		if kind == "" {
			kind = "upstream_error"
		}
		message, _ := safe["error"].(string)
		if message == "" {
			message = "Tool operation failed"
		}
		envelope["error_type"], envelope["error"] = kind, message
	}
	result := encodeResult(envelope, version)
	encoded, err := json.Marshal(result)
	// 为 JSON-RPC 信封与最大合法 request ID 预留整帧空间，最终 writer 再检查。
	if err != nil || len(encoded) > maxResponseBytes-maxFrameBytes {
		small := map[string]any{"ok": false, "tool": tool, "data": map[string]any{}, "error_type": "result_too_large", "error": "Result exceeds the response limit; narrow the request."}
		if operation != "" {
			small["operation"] = operation
		}
		return encodeResult(small, version)
	}
	return result
}

func encodeResult(envelope map[string]any, version string) *mcp.CallToolResult {
	text, err := json.Marshal(envelope)
	if err != nil {
		panic("MCP envelope must be JSON serializable")
	}
	out := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(text)}}, IsError: envelope["ok"] != true}
	if version != "2025-03-26" {
		out.StructuredContent = envelope
	}
	return out
}

// 相对目录（尤其是 "."）不能直接作全局替换词，否则会破坏业务 URL。
func privatePaths(paths ...string) []string {
	var out []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil || absolute == filepath.Dir(absolute) {
			continue
		}
		out = append(out, absolute, filepath.ToSlash(absolute))
	}
	return out
}
