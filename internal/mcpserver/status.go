package mcpserver

import (
	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/service"
)

var toolOperations = map[string][]string{
	"onesearch_search":    {"answer", "sources", "docs.resolve", "docs.query"},
	"onesearch_fetch":     {"fetch"},
	"onesearch_site":      {"map", "crawl"},
	"onesearch_repo_wiki": {"ask", "structure", "contents"},
	"onesearch_status":    {"status"},
}

func operationReady(cfg *config.Config, svc *service.Service, operation, filter string) bool {
	if operation == "status" {
		return true
	}
	if cfg.InitializationError != "" {
		return false
	}
	if operation == "answer" {
		return svc.ValidateMinimumProfile()["ok"] == true
	}
	capability, direct := "", ""
	switch operation {
	case "sources":
		capability, direct = "source_search", "exa"
	case "docs.resolve", "docs.query":
		capability, direct = "docs_search", "context7"
	case "fetch":
		capability = "page_fetch"
	case "map":
		capability = "site_map"
	case "crawl":
		capability = "site_crawl"
	case "ask", "structure", "contents":
		capability = "repo_wiki"
	default:
		return false
	}
	runtime := config.LoadRuntime(cfg)
	if direct != "" {
		// direct 命令使用固定 provider，而不是 capability route 的替代候选。
		runtime.Routes[capability] = []string{direct}
		filter = direct
	}
	if filter == "" {
		filter = "auto"
	}
	return len(runtime.ResolveProviders(cfg, capability, filter, false)) > 0
}

func statusData(cfg *config.Config, svc *service.Service) map[string]any {
	minimum := svc.ValidateMinimumProfile()
	tools := map[string]any{}
	for name, operations := range toolOperations {
		available, fully := false, true
		items := map[string]any{}
		for _, operation := range operations {
			ready := operationReady(cfg, svc, operation, "auto")
			item := map[string]any{"available": ready}
			if !ready {
				item["reason"] = "configuration_not_ready"
			}
			items[operation] = item
			available, fully = available || ready, fully && ready
		}
		tools[name] = map[string]any{"available": available, "fully_available": fully, "operations": items}
	}
	ready := cfg.InitializationError == "" && minimum["ok"] == true
	state := "degraded"
	if ready {
		state = "ready"
	}
	if cfg.InitializationError != "" {
		state = "initialization_error"
	}
	data := map[string]any{
		"ok": true, "ready": ready, "status": state,
		"config":          map[string]any{"exists": cfg.PathInfo().Exists, "missing": cfg.MissingConfigFile, "read_only": true, "valid": cfg.InitializationError == ""},
		"minimum_profile": map[string]any{"ok": minimum["ok"], "profile": minimum["profile"], "required": minimum["required"], "missing": minimum["missing"]},
		"tools":           tools,
	}
	if cfg.InitializationError != "" {
		data["error_type"], data["error"] = "config_error", "Configuration could not be loaded; repair it outside MCP."
	}
	return data
}
