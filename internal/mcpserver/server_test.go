package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func testSnapshot(t *testing.T, enabled bool) *config.Config {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.ConfigDirEnvName, dir)
	raw := config.InitialRuntimeSchema()
	providers := raw["providers"].(map[string]any)
	for _, value := range providers {
		value.(map[string]any)["enabled"] = false
	}
	providers["context7"].(map[string]any)["enabled"] = enabled
	providers["context7"].(map[string]any)["api_key"] = "test-secret-credential"
	body, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "config.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadReadOnlySnapshot()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func resultEnvelope(t *testing.T, r *mcp.CallToolResult) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), &envelope); err != nil {
		t.Fatal(err)
	}
	schema, err := resolveSchema(resultSchema())
	if err != nil {
		t.Fatal(err)
	}
	if err = schema.Validate(envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

const resolveArgs = `{"mode":"docs","query":"q","docs_options":{"library_name":"test"}}`

func TestHandlerDispatchReadinessAndSnapshotRotation(t *testing.T) {
	cfg := testSnapshot(t, true)
	catalog := catalogForTest(t)
	var ids []string
	h := handler{load: config.LoadReadOnlySnapshot, slots: make(chan struct{}, 4), execute: func(_ context.Context, _ *service.Service, id string, _ map[string]any) map[string]any {
		ids = append(ids, id)
		return map[string]any{"ok": true, "results": []map[string]any{{"id": "/org/lib", "title": "only candidate"}}, "content": "complete test-secret-credential text"}
	}}
	r := h.call(context.Background(), catalog["onesearch_search"], []byte(resolveArgs), "2026-07-28")
	envelope := resultEnvelope(t, r)
	data := envelope["data"].(map[string]any)
	if r.IsError || len(ids) != 1 || ids[0] != "context7.resolve-library-id" || data["kind"] != "library_candidates" || data["needs_selection"] != true {
		t.Fatalf("resolve: %#v, %v", envelope, ids)
	}
	if strings.Contains(r.Content[0].(*mcp.TextContent).Text, "test-secret-credential") {
		t.Fatal("credential leak")
	}
	r = h.call(context.Background(), catalog["onesearch_search"], []byte(`{"mode":"docs","query":"q","docs_options":{"library_id":"/org/lib"}}`), "2026-07-28")
	if r.IsError || len(ids) != 2 || ids[1] != "context7.query-docs" {
		t.Fatalf("docs dispatch: %v", ids)
	}
	// 仅 docs 可用时 search 仍部分 available，answer 不因此获准。
	status := statusData(cfg, service.New(cfg))
	search := status["tools"].(map[string]any)["onesearch_search"].(map[string]any)
	if search["available"] != true || search["fully_available"] != false {
		t.Fatalf("readiness: %#v", search)
	}
	r = h.call(context.Background(), catalog["onesearch_search"], []byte(`{"mode":"answer","query":"q"}`), "2026-07-28")
	if resultEnvelope(t, r)["error_type"] != "config_error" || len(ids) != 2 {
		t.Fatal("answer readiness bypass")
	}
	raw := cfg.LoadFile()
	raw["providers"].(map[string]any)["context7"].(map[string]any)["enabled"] = false
	body, _ := json.Marshal(raw)
	if err := os.WriteFile(cfg.ConfigFile, body, 0600); err != nil {
		t.Fatal(err)
	}
	if !operationReady(cfg, service.New(cfg), "docs.resolve", "") {
		t.Fatal("old snapshot changed")
	}
	r = h.call(context.Background(), catalog["onesearch_search"], []byte(resolveArgs), "2026-07-28")
	if resultEnvelope(t, r)["error_type"] != "config_error" || len(ids) != 2 {
		t.Fatal("disabled provider executed")
	}
	if err := os.WriteFile(cfg.ConfigFile, []byte(`{"secret":"do-not-leak",`), 0600); err != nil {
		t.Fatal(err)
	}
	r = h.call(context.Background(), catalog["onesearch_status"], []byte(`{}`), "2026-07-28")
	data = resultEnvelope(t, r)["data"].(map[string]any)
	if r.IsError || data["status"] != "initialization_error" {
		t.Fatalf("bad-config status: %#v", data)
	}
	if strings.Contains(r.Content[0].(*mcp.TextContent).Text, "do-not-leak") {
		t.Fatal("bad config leaked")
	}
}

func TestHandlerLimitsAndRecovery(t *testing.T) {
	cfg := testSnapshot(t, true)
	definition := catalogForTest(t)["onesearch_search"]
	h := handler{load: func() (*config.Config, error) { return cfg, nil }, slots: make(chan struct{}, 4)}
	h.execute = func(ctx context.Context, _ *service.Service, _ string, _ map[string]any) map[string]any {
		<-ctx.Done()
		return nil
	}
	r := h.call(context.Background(), definition, []byte(`{"mode":"docs","query":"q","timeout":0.01,"docs_options":{"library_name":"test"}}`), "2026-07-28")
	if resultEnvelope(t, r)["error_type"] != "timeout" {
		t.Fatal("timeout not enforced")
	}
	h.execute = func(context.Context, *service.Service, string, map[string]any) map[string]any {
		panic("secret panic value")
	}
	r = h.call(context.Background(), definition, []byte(resolveArgs), "2026-07-28")
	if resultEnvelope(t, r)["error_type"] != "internal_error" || strings.Contains(r.Content[0].(*mcp.TextContent).Text, "secret panic value") {
		t.Fatal("unsafe panic recovery")
	}
	started, release := make(chan struct{}, 4), make(chan struct{})
	h.execute = func(context.Context, *service.Service, string, map[string]any) map[string]any {
		started <- struct{}{}
		<-release
		return map[string]any{"ok": true, "results": []any{}}
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); h.call(context.Background(), definition, []byte(resolveArgs), "2026-07-28") }()
	}
	for range 4 {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			close(release)
			t.Fatal("concurrency did not start")
		}
	}
	r = h.call(context.Background(), definition, []byte(resolveArgs), "2026-07-28")
	close(release)
	wg.Wait()
	if resultEnvelope(t, r)["error_type"] != "server_busy" {
		t.Fatal("concurrency limit not enforced")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = h.call(ctx, definition, []byte(resolveArgs), "2026-07-28")
	if resultEnvelope(t, r)["error_type"] != "cancelled" {
		t.Fatal("cancelled request executed")
	}
}

func TestResultsFullTextRedactionAndLimits(t *testing.T) {
	content := strings.Repeat("正文 evidence ", 5000)
	data := map[string]any{"ok": true, "content": content + "secret-value", "status": "submitted", "job_id": "123", "raw_result": map[string]any{"private": "debug-value"}, "attempts": []map[string]any{{"error": "failed secret-value; stderr: internal-stderr", "api_key": "unknown-secret", "stderr": "unrelated-private"}}}
	for _, version := range protocolVersions {
		r := makeResult("onesearch_site", "crawl", version, data, []string{"secret-value"})
		envelope := resultEnvelope(t, r)
		safe := envelope["data"].(map[string]any)
		if !strings.HasPrefix(safe["content"].(string), content) || safe["status"] != "submitted" || safe["job_id"] != "123" {
			t.Fatal("business data lost")
		}
		body, _ := json.Marshal(r)
		for _, secret := range []string{"secret-value", "unknown-secret", "internal-stderr", "unrelated-private", "debug-value"} {
			if strings.Contains(string(body), secret) {
				t.Fatalf("leak: %s", secret)
			}
		}
		if (r.StructuredContent == nil) != (version == "2025-03-26") {
			t.Fatal("version projection")
		}
	}
	r := makeResult("onesearch_fetch", "fetch", "2026-07-28", map[string]any{"ok": true, "content": strings.Repeat(`"\\`, maxResponseBytes)}, nil)
	if resultEnvelope(t, r)["error_type"] != "result_too_large" {
		t.Fatal("oversized response accepted")
	}
	for _, results := range []any{nil, []any{}, []map[string]any{{"id": "/a/b"}}} {
		r = makeResult("onesearch_search", "docs.resolve", "2026-07-28", map[string]any{"ok": true, "results": results}, nil)
		safe := resultEnvelope(t, r)["data"].(map[string]any)
		if safe["needs_selection"] != (len(safe["candidates"].([]any)) > 0) {
			t.Fatal("candidate state mismatch")
		}
	}
}

func TestRunEOFAndOversizedFrames(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		t.Run(map[bool]string{false: "eof", true: "oversized"}[oversized], func(t *testing.T) {
			client, server := net.Pipe()
			done := make(chan error, 1)
			go func() { done <- Run(context.Background(), server, server) }()
			if oversized {
				client.SetWriteDeadline(time.Now().Add(3 * time.Second))
				_, _ = io.WriteString(client, `{"jsonrpc":"2.0","id":1,"method":"`+strings.Repeat("x", maxFrameBytes)+`"}`+"\n")
			} else {
				client.Close()
			}
			select {
			case err := <-done:
				if (err != nil) != oversized {
					t.Fatalf("Run error = %v", err)
				}
			case <-time.After(5 * time.Second):
				client.Close()
				t.Fatal("Run did not exit")
			}
			client.Close()
		})
	}
}
