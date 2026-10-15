package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/service"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

type bufferCloser struct{ bytes.Buffer }

func (*bufferCloser) Close() error { return nil }

func TestCancellationResponseFilter(t *testing.T) {
	for _, batch := range []bool{false, true} {
		id, err := jsonrpc.MakeID("cancelled")
		if err != nil {
			t.Fatal(err)
		}
		state := &cancellationState{requests: map[jsonrpc.ID]bool{id: true}}
		var destination bufferCloser
		writer := &cancelWriter{WriteCloser: &destination, state: state}
		body := []byte(`{"jsonrpc":"2.0","id":"cancelled","result":{}}` + "\n")
		if batch {
			body = []byte(`[{"jsonrpc":"2.0","id":"cancelled","result":{}},{"jsonrpc":"2.0","id":2,"result":{}}]` + "\n")
		}
		n, err := writer.Write(body)
		if err != nil || n != len(body) {
			t.Fatalf("write: %d %v", n, err)
		}
		if len(state.requests) != 0 {
			t.Fatal("completed request was retained")
		}
		if !batch && destination.Len() != 0 {
			t.Fatal("cancelled response emitted")
		}
		if batch {
			var kept []map[string]any
			if err := json.Unmarshal(destination.Bytes(), &kept); err != nil {
				t.Fatal(err)
			}
			if len(kept) != 1 || kept[0]["id"] != float64(2) {
				t.Fatalf("batch corrupted: %s", destination.Bytes())
			}
		}
	}
}

func TestRelativeConfigDoesNotCorruptBusinessData(t *testing.T) {
	cfg := testSnapshot(t, true)
	t.Chdir(cfg.PathInfo().ConfigDir)
	t.Setenv(config.ConfigDirEnvName, ".")
	h := handler{load: config.LoadReadOnlySnapshot, slots: make(chan struct{}, 4), execute: func(context.Context, *service.Service, string, map[string]any) map[string]any {
		return map[string]any{"ok": true, "results": []any{}, "content": "Go 1.26. https://example.com/one.two", "mcp": map[string]any{"stderr_summary": "private-diagnostic-marker"}}
	}}
	r := h.call(context.Background(), catalogForTest(t)["onesearch_search"], []byte(resolveArgs), "2026-07-28")
	data := resultEnvelope(t, r)["data"].(map[string]any)
	if data["content"] != "Go 1.26. https://example.com/one.two" {
		t.Fatalf("relative path damaged content: %v", data["content"])
	}
	body, _ := json.Marshal(r)
	if strings.Contains(string(body), "private-diagnostic-marker") || strings.Contains(string(body), "stderr_summary") {
		t.Fatal("provider stderr summary leaked")
	}
	for _, path := range privatePaths(".", string(filepath.Separator), filepath.VolumeName(cfg.PathInfo().ConfigDir)+string(filepath.Separator)) {
		if !filepath.IsAbs(path) || filepath.Dir(path) == path || path == "." {
			t.Fatalf("unsafe path replacement %q", path)
		}
	}
}
