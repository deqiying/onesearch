package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deqiying/onesearch/internal/redact"
)

type Options struct {
	Format       string
	Verbosity    string
	SecretValues []string
}

func Render(command string, data map[string]any, format string) string {
	return RenderWithOptions(command, data, Options{Format: format})
}

func RenderWithOptions(command string, data map[string]any, options Options) string {
	if safe, ok := redact.Data(data, options.SecretValues).(map[string]any); ok {
		data = safe
	}
	format := options.Format
	verbosity := strings.ToLower(strings.TrimSpace(options.Verbosity))
	if verbosity != "verbose" {
		if !truthy(data["ok"]) {
			data = compactError(command, data)
		} else if command == "search" && format != "content" && format != "markdown" {
			data = compactSearchResult(data)
		} else if command == "repo-wiki" && format != "content" && format != "markdown" {
			data = compactContentResult(data)
		} else if stringValue(data["tool"]) != "" && format != "content" && format != "markdown" {
			data = compactContentResult(data)
		}
	}
	var rendered string
	switch format {
	case "content":
		rendered = content(command, data)
	case "markdown":
		rendered = markdown(command, data)
	default:
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(data)
		rendered = buf.String()
	}
	return redact.Text(rendered, options.SecretValues)
}

func compactError(command string, data map[string]any) map[string]any {
	out := map[string]any{
		"ok":         false,
		"error_type": data["error_type"],
		"error":      compactErrorMessage(stringValue(data["error"])),
		"elapsed_ms": data["elapsed_ms"],
	}
	for _, key := range []string{"session_id", "query", "url", "provider", "tool", "mode"} {
		if value := data[key]; stringValue(value) != "" {
			out[key] = value
		}
	}
	if command == "doctor" {
		for _, key := range []string{"status", "config", "schema", "minimum_profile", "issues", "effective_environment"} {
			if value, ok := data[key]; ok {
				out[key] = value
			}
		}
		return out
	}
	out["hint"] = "Retry with --verbose for diagnostics, or run `onesearch doctor`."
	return out
}

func compactSearchResult(data map[string]any) map[string]any {
	out := map[string]any{
		"ok":    true,
		"query": data["query"],
		"used":  compactSearchUsed(data["used"]),
		"meta":  compactSearchMeta(data),
	}
	if len(asMap(out["used"])) == 0 {
		out["used"] = fallbackSearchUsed(data)
	}
	return out
}

func compactContentResult(data map[string]any) map[string]any {
	out := map[string]any{"ok": data["ok"]}
	for _, key := range []string{"query", "url", "repo", "library_id", "provider", "tool", "mode", "id", "job_id", "status", "success", "total", "elapsed_ms", "fallback_used"} {
		if value, ok := data[key]; ok && stringValue(value) != "" {
			out[key] = value
		}
	}
	if urls := asStrings(data["urls"]); len(urls) > 0 {
		out["urls"] = urls
	}
	if content := stringValue(data["content"]); content != "" {
		out["content_preview"] = previewText(content, 1200)
		out["content_length"] = len(content)
	}
	if result := asMap(data["result"]); len(result) > 0 {
		if compacted := compactProviderResult(result); keepNestedProviderResult(out, compacted) {
			out["result"] = compacted
		}
	}
	if results := compactToolResults(data["results"]); len(results) > 0 {
		out["results"] = results
		out["results_count"] = len(results)
	}
	return out
}

func compactToolResults(value any) []map[string]any {
	rawItems := asAnySlice(value)
	out := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		item := compactToolItem(raw)
		if len(item) > 0 {
			out = append(out, item)
		}
	}
	return out
}

func compactToolItem(value any) map[string]any {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return map[string]any{"url": typed}
	case map[string]any:
		out := map[string]any{}
		for _, key := range []string{"capability", "provider", "title", "url", "id", "library_id", "description", "snippet", "published_date", "publishedDate", "author", "score", "status"} {
			if value, ok := typed[key]; ok && stringValue(value) != "" {
				out[key] = value
			}
		}
		content := firstNonEmpty(stringValue(typed["content"]), stringValue(typed["text"]), stringValue(typed["raw_content"]), stringValue(typed["markdown"]))
		if content != "" {
			out["content_preview"] = previewText(content, 1200)
			out["content_length"] = len(content)
		}
		return out
	default:
		return nil
	}
}

func compactSearchUsed(value any) map[string]any {
	capabilities := asAnySlice(value)
	used := map[string]any{}
	for _, rawCapability := range capabilities {
		capability := asMap(rawCapability)
		capabilityName := stringValue(capability["capability"])
		if capabilityName == "" {
			continue
		}
		providerItems := compactSearchProviders(capability["providers"])
		if len(providerItems) == 0 {
			continue
		}
		item := asMap(used[capabilityName])
		if len(item) == 0 {
			item = map[string]any{"providers": map[string]any{}}
			used[capabilityName] = item
		}
		if role := stringValue(capability["role"]); role != "" {
			appendCapabilityRole(item, role)
		}
		existingProviders := asMap(item["providers"])
		for provider, rawProvider := range providerItems {
			providerItem := asMap(rawProvider)
			existingProvider := asMap(existingProviders[provider])
			if len(existingProvider) == 0 {
				existingProviders[provider] = providerItem
				continue
			}
			existingProviders[provider] = mergeProviderResult(existingProvider, providerItem)
		}
	}
	return used
}

func compactSearchProviders(value any) map[string]any {
	providers := asAnySlice(value)
	out := map[string]any{}
	for _, rawProvider := range providers {
		provider := asMap(rawProvider)
		providerName := stringValue(provider["provider"])
		if providerName == "" {
			continue
		}
		result := compactProviderResult(provider["result"])
		if len(result) == 0 {
			continue
		}
		item := map[string]any{
			"status": provider["status"],
			"result": result,
		}
		for _, key := range []string{"mode", "model", "elapsed_ms"} {
			if value, ok := provider[key]; ok && stringValue(value) != "" {
				item[key] = value
			}
		}
		out[providerName] = item
	}
	return out
}

func appendCapabilityRole(item map[string]any, role string) {
	if existing := stringValue(item["role"]); existing != "" {
		if existing == role {
			return
		}
		delete(item, "role")
		item["roles"] = []string{existing, role}
		return
	}
	roles := asStrings(item["roles"])
	for _, existing := range roles {
		if existing == role {
			return
		}
	}
	if len(roles) > 0 {
		item["roles"] = append(roles, role)
		return
	}
	item["role"] = role
}

func mergeProviderResult(left, right map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range left {
		merged[key] = value
	}
	for _, key := range []string{"status", "mode", "model", "elapsed_ms"} {
		if value, ok := right[key]; ok && stringValue(value) != "" {
			merged[key] = value
		}
	}
	merged["result"] = mergeResult(asMap(left["result"]), asMap(right["result"]))
	return merged
}

func mergeResult(left, right map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range left {
		merged[key] = value
	}
	for _, key := range []string{"content", "url", "content_preview", "content_length"} {
		if value, ok := right[key]; ok && stringValue(value) != "" {
			merged[key] = value
		}
	}
	sources := mergeCompactSources(asAnySlice(left["sources"]), asAnySlice(right["sources"]))
	if len(sources) > 0 {
		merged["sources"] = sources
		merged["sources_count"] = len(sources)
	} else if value, ok := right["sources_count"]; ok && stringValue(value) != "" {
		merged["sources_count"] = value
	}
	pages := mergeCompactPages(asAnySlice(left["pages"]), asAnySlice(right["pages"]))
	if len(pages) > 0 {
		merged["pages"] = pages
		merged["pages_count"] = len(pages)
	} else if value, ok := right["pages_count"]; ok && stringValue(value) != "" {
		merged["pages_count"] = value
	}
	return merged
}

func compactProviderResult(value any) map[string]any {
	result := asMap(value)
	out := map[string]any{}
	if content := stringValue(result["content"]); content != "" {
		out["content_preview"] = previewText(content, 1200)
		out["content_length"] = len(content)
	}
	for _, key := range []string{"url", "repo", "tool", "query", "id", "status", "success", "content_preview", "content_length"} {
		if value, ok := result[key]; ok && stringValue(value) != "" {
			out[key] = value
		}
	}
	if sources := compactSearchSources(result["sources"]); len(sources) > 0 {
		out["sources"] = sources
		out["sources_count"] = len(sources)
	} else if value, ok := result["sources_count"]; ok && stringValue(value) != "" {
		out["sources_count"] = value
	}
	if pages := compactFetchedPages(result["pages"]); len(pages) > 0 {
		out["pages"] = pages
		out["pages_count"] = len(pages)
	} else if value, ok := result["pages_count"]; ok && stringValue(value) != "" {
		out["pages_count"] = value
	}
	return out
}

func keepNestedProviderResult(parent, result map[string]any) bool {
	if len(result) == 0 {
		return false
	}
	for _, key := range []string{"content_preview", "content_length", "sources", "sources_count", "pages", "pages_count"} {
		if _, ok := result[key]; ok {
			return true
		}
	}
	return stringValue(parent["id"]) == "" && stringValue(parent["status"]) == "" && stringValue(parent["success"]) == ""
}

func compactSearchMeta(data map[string]any) map[string]any {
	meta := map[string]any{}
	raw := asMap(data["meta"])
	for _, key := range []string{"session_id", "validation_level", "elapsed_ms", "fallback_used"} {
		if value, ok := raw[key]; ok && stringValue(value) != "" {
			meta[key] = value
		}
	}
	for _, key := range []string{"session_id", "validation_level", "elapsed_ms", "fallback_used"} {
		if _, exists := meta[key]; exists {
			continue
		}
		if value, ok := data[key]; ok && stringValue(value) != "" {
			meta[key] = value
		}
	}
	return meta
}

func fallbackSearchUsed(data map[string]any) map[string]any {
	content := stringValue(data["content"])
	sources := compactSearchSources(data["sources"])
	if content == "" && len(sources) == 0 {
		return map[string]any{}
	}
	result := map[string]any{}
	if content != "" {
		result["content_preview"] = previewText(content, 1200)
		result["content_length"] = len(content)
	}
	if len(sources) > 0 {
		result["sources"] = sources
		result["sources_count"] = len(sources)
	}
	provider := "unknown"
	if providers := asStrings(data["providers_used"]); len(providers) > 0 {
		provider = providers[0]
	}
	return map[string]any{
		"answer_search": map[string]any{
			"role": "primary_answer",
			"providers": map[string]any{provider: map[string]any{
				"status": "ok",
				"result": result,
			}},
		},
	}
}

func mergeCompactSources(lists ...[]any) []map[string]any {
	seen := map[string]struct{}{}
	var out []map[string]any
	for _, list := range lists {
		for _, raw := range list {
			item := asMap(raw)
			if len(item) == 0 {
				continue
			}
			key := firstNonEmpty(stringValue(item["url"]), stringValue(item["id"]), stringValue(item["library_id"]), stringValue(item["title"]))
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func mergeCompactPages(lists ...[]any) []map[string]any {
	seen := map[string]struct{}{}
	var out []map[string]any
	for _, list := range lists {
		for _, raw := range list {
			item := asMap(raw)
			if len(item) == 0 {
				continue
			}
			key := stringValue(item["url"])
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func compactSearchSources(value any) []map[string]any {
	rawSources := asAnySlice(value)
	sources := make([]map[string]any, 0, len(rawSources))
	for _, raw := range rawSources {
		source := asMap(raw)
		if len(source) == 0 {
			continue
		}
		item := map[string]any{}
		for _, key := range []string{"capability", "provider", "title", "url", "published_date", "description", "snippet", "id", "library_id"} {
			if value, ok := source[key]; ok && stringValue(value) != "" {
				item[key] = value
			}
		}
		if len(item) > 0 {
			sources = append(sources, item)
		}
	}
	return sources
}

func compactFetchedPages(value any) []map[string]any {
	rawPages := asAnySlice(value)
	pages := make([]map[string]any, 0, len(rawPages))
	for _, raw := range rawPages {
		page := asMap(raw)
		if len(page) == 0 {
			continue
		}
		item := map[string]any{}
		for _, key := range []string{"url", "source_title", "source_provider", "content_preview", "content_length"} {
			if value, ok := page[key]; ok && stringValue(value) != "" {
				item[key] = value
			}
		}
		if len(item) > 0 {
			pages = append(pages, item)
		}
	}
	return pages
}

func compactErrorMessage(value string) string {
	text := oneLine(value, 0)
	if text == "" {
		return ""
	}
	for _, marker := range []string{": dial tcp ", ": connectex:", ": i/o timeout", ": context deadline exceeded"} {
		if idx := strings.Index(text, marker); idx >= 0 {
			return strings.TrimSpace(text[:idx])
		}
	}
	return oneLine(text, 220)
}

func Write(path, text string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func ExitCode(data map[string]any) int {
	if ok, _ := data["ok"].(bool); ok {
		return 0
	}
	switch data["error_type"] {
	case "parameter_error":
		return 2
	case "config_error":
		return 3
	case "network_error", "evidence_error":
		return 4
	default:
		return 5
	}
}

func content(command string, data map[string]any) string {
	if command == "search" || command == "fetch" || command == "repo-wiki" {
		if text := stringValue(data["content"]); text != "" {
			return text + "\n"
		}
		if ok, _ := data["ok"].(bool); ok {
			return ""
		}
		if err := errorSummary(data); err != "" {
			return status(data["ok"]) + ": " + err + "\n"
		}
		return ""
	}
	if text := stringValue(data["content"]); text != "" {
		return text + "\n"
	}
	if command == "deep" || data["mode"] == "deep_research" {
		return "Deep Research plan for: " + stringValue(data["question"]) + "\nThis command only plans; execute the listed CLI steps to perform live research.\n"
	}
	if command == "doctor" {
		minimum := asMap(data["minimum_profile"])
		config := asMap(data["config"])
		lines := []string{
			"Doctor: " + status(data["ok"]),
			"Config: " + stringValue(config["file"]),
			"Config source: " + stringValue(config["dir_source"]),
			"Profile: " + stringValue(minimum["profile"]) + " " + status(minimum["ok"]),
		}
		if dirEnv := stringValue(config["dir_env"]); dirEnv != "" {
			lines = append(lines, "Config environment: "+dirEnv)
		}
		lines = appendEffectiveEnvironmentContent(lines, data["effective_environment"])
		if created, _ := config["created"].(bool); created {
			lines = append(lines, "Initialized: config file was missing and has been created")
		} else if missing, _ := config["missing_before_start"].(bool); missing {
			lines = append(lines, "Initialization failed: config file is missing")
		}
		if err := stringValue(config["initialization_error"]); err != "" {
			lines = append(lines, "Initialization error: "+err)
		}
		if missing := asStrings(minimum["missing"]); len(missing) > 0 {
			lines = append(lines, "Missing: "+strings.Join(missing, ", "))
		}
		if issues := asAnySlice(data["issues"]); len(issues) > 0 {
			lines = append(lines, "Issues:")
			for _, raw := range issues {
				item := asMap(raw)
				line := "- " + stringValue(item["type"])
				if capability := stringValue(item["capability"]); capability != "" {
					line += " " + capability
				}
				if provider := stringValue(item["provider"]); provider != "" {
					line += "/" + provider
				}
				if reason := stringValue(item["reason"]); reason != "" {
					line += ": " + reason
				}
				lines = append(lines, line)
			}
		}
		lines = append(lines, "Details: run `onesearch doctor --format json` or `onesearch config list --format json`")
		return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
	}
	if command == "smoke" {
		return fmt.Sprintf("Smoke %s %s: %d cases, %d failed, %d degraded\n", stringValue(data["mode"]), status(data["ok"]), len(asAnySlice(data["cases"])), len(asAnySlice(data["failed_cases"])), len(asAnySlice(data["degraded_cases"])))
	}
	if command == "status" {
		return statusContent(data)
	}
	if command == "config" {
		parts := []string{strings.Title(command) + " " + status(data["ok"])}
		for _, key := range []string{"provider", "enabled", "api_key_set", "api_key_env", "api_key_env_set", "api_key_src", "has_api_key", "base_url"} {
			if value, ok := data[key]; ok && stringValue(value) != "" {
				parts = append(parts, key+"="+stringValue(value))
			}
		}
		if file := stringValue(data["config_file"]); file != "" {
			parts = append(parts, "file="+file)
		}
		if key := stringValue(data["key"]); key != "" {
			parts = append(parts, "key="+key)
		}
		if value := stringValue(data["value"]); value != "" {
			parts = append(parts, "value="+value)
		}
		if changed := asStrings(data["changed_fields"]); len(changed) > 0 {
			parts = append(parts, "changed_fields="+strings.Join(changed, ","))
		}
		if err := errorSummary(data); err != "" {
			parts = append(parts, "error="+err)
		}
		return strings.Join(parts, "; ") + "\n"
	}
	if command == "model" {
		if err := errorSummary(data); err != "" {
			return "Model " + status(data["ok"]) + ": " + err + "\n"
		}
		providers := asMap(data["providers"])
		xai := asMap(providers["xai"])
		compatible := asMap(providers["openai_compatible"])
		return "Models: xai=" + stringValue(xai["model"]) + ", openai_compatible=" + stringValue(compatible["model"]) + "\n"
	}
	if lines := plainResultLines(data); len(lines) > 0 {
		return strings.Join(lines, "\n") + "\n"
	}
	if err := errorSummary(data); err != "" {
		return status(data["ok"]) + ": " + err + "\n"
	}
	return command + ": " + status(data["ok"]) + "\n"
}

func markdown(command string, data map[string]any) string {
	switch command {
	case "doctor":
		return doctorMarkdown(data)
	case "status":
		return statusMarkdown(data)
	case "smoke":
		return smokeMarkdown(data)
	case "deep":
		return deepMarkdown(data)
	case "config":
		return configMarkdown(data)
	case "model":
		return modelMarkdown(data)
	default:
		return resultMarkdown(command, data, titleFor(command))
	}
}

func resultMarkdown(command string, data map[string]any, title string) string {
	lines := []string{"# " + title, "", "Status: " + status(data["ok"])}
	for _, key := range []string{"query", "url", "base_url", "provider", "tool"} {
		if value := stringValue(data[key]); value != "" {
			lines = append(lines, strings.Title(strings.ReplaceAll(key, "_", " "))+": `"+value+"`")
		}
	}
	results := asAnySlice(data["results"])
	if len(results) > 0 {
		lines = append(lines, "", "## Results", "| # | Title | URL / ID | Summary |", "| --- | --- | --- | --- |")
		for i, raw := range results {
			item, _ := raw.(map[string]any)
			lines = append(lines, fmt.Sprintf("| %d | %s | %s | %s |", i+1, mdCell(resultTitle(item, i+1)), mdCell(resultTarget(item)), mdCell(resultSummary(item))))
		}
	} else if text := stringValue(data["content"]); text != "" {
		lines = append(lines, "", "## Content", "```text", text, "```")
	} else if ok, _ := data["ok"].(bool); ok {
		lines = append(lines, "", "No results.")
	}
	if err := errorSummary(data); err != "" {
		lines = append(lines, "", "## Errors", "- "+err)
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func doctorMarkdown(data map[string]any) string {
	minimum := asMap(data["minimum_profile"])
	schema := asMap(data["schema"])
	config := asMap(data["config"])
	lines := []string{
		"# Onesearch Doctor", "",
		"Overall: " + status(data["ok"]),
		"Status: " + stringValue(data["status"]),
		"Schema: v" + stringValue(schema["version"]) + " (`" + stringValue(schema["source"]) + "`)",
		"Config file: `" + stringValue(config["file"]) + "`",
		"Config source: `" + stringValue(config["dir_source"]) + "`",
		"Minimum profile: `" + stringValue(minimum["profile"]) + "` " + status(minimum["ok"]),
	}
	if dirEnv := stringValue(config["dir_env"]); dirEnv != "" {
		lines = append(lines, "Config environment: `"+mdCell(dirEnv)+"`")
	}
	if created, _ := config["created"].(bool); created {
		lines = append(lines, "Config initialized: `"+stringValue(config["file"])+"`")
	} else if missing, _ := config["missing_before_start"].(bool); missing {
		lines = append(lines, "Config missing and initialization failed: `"+stringValue(config["file"])+"`")
	}
	if err := stringValue(config["initialization_error"]); err != "" {
		lines = append(lines, "Config initialization error: `"+err+"`")
	}
	if missing := asStrings(minimum["missing"]); len(missing) > 0 {
		lines = append(lines, "Missing: `"+mdCell(strings.Join(missing, ", "))+"`")
	}
	if issues := asAnySlice(data["issues"]); len(issues) > 0 {
		lines = append(lines, "", "## Issues", "| Type | Capability | Provider | Reason |", "| --- | --- | --- | --- |")
		for _, raw := range issues {
			item := asMap(raw)
			lines = append(lines, fmt.Sprintf("| %s | %s | %s | %s |", mdCell(stringValue(item["type"])), mdCell(stringValue(item["capability"])), mdCell(stringValue(item["provider"])), mdCell(stringValue(item["reason"]))))
		}
	}
	lines = appendEffectiveEnvironmentMarkdown(lines, data["effective_environment"])
	if err := errorSummary(data); err != "" {
		lines = append(lines, "", "## Errors", "- "+err)
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func statusContent(data map[string]any) string {
	lines := []string{"Status: " + stringValue(data["status"]) + " (ready: " + status(data["ready"]) + ")"}
	config := asMap(data["config"])
	if file := stringValue(config["file"]); file != "" {
		lines = append(lines, "Config: "+file)
	}
	if source := stringValue(config["dir_source"]); source != "" {
		lines = append(lines, "Config source: "+source)
	}
	if dirEnv := stringValue(config["dir_env"]); dirEnv != "" {
		lines = append(lines, "Config environment: "+dirEnv)
	}
	lines = appendEffectiveEnvironmentContent(lines, data["effective_environment"])
	minimum := asMap(data["minimum_profile"])
	if profile := stringValue(minimum["profile"]); profile != "" {
		lines = append(lines, "Profile: "+profile+" "+status(minimum["ok"]))
	}
	if missing := asStrings(minimum["missing"]); len(missing) > 0 {
		lines = append(lines, "Missing: "+strings.Join(missing, ", "))
	}
	capabilities := asMap(data["capabilities"])
	var availableCaps []string
	var unavailableCaps []string
	for _, key := range sortedAnyKeys(capabilities) {
		item := asMap(capabilities[key])
		if truthy(item["ok"]) {
			availableCaps = append(availableCaps, key+"="+strings.Join(asStrings(item["available"]), ","))
		} else {
			unavailableCaps = append(unavailableCaps, key)
		}
	}
	if len(availableCaps) > 0 {
		lines = append(lines, "Available capabilities: "+strings.Join(availableCaps, "; "))
	}
	if len(unavailableCaps) > 0 {
		lines = append(lines, "Unavailable capabilities: "+strings.Join(unavailableCaps, ", "))
	}
	direct := asMap(data["direct_endpoints"])
	if len(direct) > 0 {
		var endpoints []string
		for _, key := range sortedAnyKeys(direct) {
			item := asMap(direct[key])
			endpoints = append(endpoints, key+"="+status(item["available"]))
		}
		lines = append(lines, "Direct endpoints: "+strings.Join(endpoints, "; "))
	}
	lines = append(lines, "Details: run `onesearch status --format json`")
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func statusMarkdown(data map[string]any) string {
	lines := []string{"# Onesearch Status", "", "Overall: " + stringValue(data["status"]), "Ready: " + status(data["ready"])}
	config := asMap(data["config"])
	if file := stringValue(config["file"]); file != "" {
		lines = append(lines, "Config file: `"+mdCell(file)+"`")
	}
	if source := stringValue(config["dir_source"]); source != "" {
		lines = append(lines, "Config source: `"+mdCell(source)+"`")
	}
	if dirEnv := stringValue(config["dir_env"]); dirEnv != "" {
		lines = append(lines, "Config environment: `"+mdCell(dirEnv)+"`")
	}
	minimum := asMap(data["minimum_profile"])
	if profile := stringValue(minimum["profile"]); profile != "" {
		lines = append(lines, "Minimum profile: `"+mdCell(profile)+"` "+status(minimum["ok"]))
	}
	if missing := asStrings(minimum["missing"]); len(missing) > 0 {
		lines = append(lines, "Missing: `"+mdCell(strings.Join(missing, ", "))+"`")
	}
	if capabilities := asMap(data["capabilities"]); len(capabilities) > 0 {
		lines = append(lines, "", "## Capabilities", "| Capability | Status | Available providers | Command |", "| --- | --- | --- | --- |")
		for _, key := range sortedAnyKeys(capabilities) {
			item := asMap(capabilities[key])
			lines = append(lines, fmt.Sprintf("| `%s` | %s | %s | `%s` |", mdCell(key), status(item["ok"]), mdCell(strings.Join(asStrings(item["available"]), ", ")), mdCell(stringValue(item["command"]))))
		}
	}
	if direct := asMap(data["direct_endpoints"]); len(direct) > 0 {
		lines = append(lines, "", "## Direct Endpoints", "| Provider | Status | Commands |", "| --- | --- | --- |")
		for _, key := range sortedAnyKeys(direct) {
			item := asMap(direct[key])
			lines = append(lines, fmt.Sprintf("| `%s` | %s | %s |", mdCell(key), status(item["available"]), mdCell(strings.Join(asStrings(item["commands"]), ", "))))
		}
	}
	lines = appendEffectiveEnvironmentMarkdown(lines, data["effective_environment"])
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func smokeMarkdown(data map[string]any) string {
	cases := asAnySlice(data["cases"])
	lines := []string{"# Onesearch Smoke", "", "Mode: `" + stringValue(data["mode"]) + "`", "Overall: " + status(data["ok"]), fmt.Sprintf("Cases: %d total, %d failed, %d degraded", len(cases), len(asAnySlice(data["failed_cases"])), len(asAnySlice(data["degraded_cases"])))}
	if len(cases) > 0 {
		lines = append(lines, "", "## Cases", "| Case | Status | Details |", "| --- | --- | --- |")
		for _, raw := range cases {
			item, _ := raw.(map[string]any)
			lines = append(lines, fmt.Sprintf("| %s | %s | %s |", mdCell(stringValue(item["name"])), status(item["ok"]), mdCell(firstNonEmpty(stringValue(item["error"]), stringValue(item["error_type"]), stringValue(item["skipped"])))))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func deepMarkdown(data map[string]any) string {
	lines := []string{"# Deep Research Plan", "", "Question: `" + stringValue(data["question"]) + "`", "Evidence policy: `" + stringValue(data["evidence_policy"]) + "`", "Evidence dir: `" + stringValue(data["evidence_dir"]) + "`"}
	steps := asAnySlice(data["steps"])
	if len(steps) > 0 {
		lines = append(lines, "", "## Steps")
		for _, raw := range steps {
			item, _ := raw.(map[string]any)
			lines = append(lines, "- `"+stringValue(item["id"])+"` "+stringValue(item["tool"])+": "+stringValue(item["purpose"]))
			lines = append(lines, "  - Command: `"+stringValue(item["command"])+"`")
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func configMarkdown(data map[string]any) string {
	lines := []string{"# Onesearch Config", "", "Status: " + status(data["ok"])}
	metadata := asMap(data["metadata"])
	schema := asMap(data["schema"])
	if file := firstNonEmpty(stringValue(data["config_file"]), stringValue(metadata["config_file"])); file != "" {
		lines = append(lines, "Config file: `"+file+"`")
	}
	if provider := stringValue(data["provider"]); provider != "" {
		lines = append(lines,
			"Provider: `"+mdCell(provider)+"`",
			"Enabled: `"+mdCell(stringValue(data["enabled"]))+"`",
			"API key set directly: "+status(data["api_key_set"]),
			"API key environment: `"+mdCell(stringValue(data["api_key_env"]))+"`",
			"API key environment set: "+status(data["api_key_env_set"]),
			"API key source: `"+mdCell(stringValue(data["api_key_src"]))+"`",
			"Has API key: "+status(data["has_api_key"]),
		)
		if baseURL := stringValue(data["base_url"]); baseURL != "" {
			lines = append(lines, "Base URL: `"+mdCell(baseURL)+"`")
		}
		if changed := asStrings(data["changed_fields"]); len(changed) > 0 {
			lines = append(lines, "Changed fields: `"+mdCell(strings.Join(changed, ", "))+"`")
		}
	}
	if len(schema) > 0 {
		lines = append(lines, "Schema: v"+stringValue(schema["version"])+" (`"+stringValue(schema["source"])+"`)")
	}
	if routes, ok := data["routes"].(map[string][]string); ok && len(routes) > 0 {
		lines = append(lines, "", "## Routes", "| Capability | Providers |", "| --- | --- |")
		for _, key := range sortedKeys(routes) {
			lines = append(lines, "| `"+mdCell(key)+"` | "+mdCell(strings.Join(routes[key], ", "))+" |")
		}
	}
	if providers, ok := data["providers"].(map[string]any); ok && len(providers) > 0 {
		lines = append(lines, "", "## Providers", "| Provider | Enabled | Available | API key | Capabilities |", "| --- | --- | --- | --- | --- |")
		for _, key := range sortedAnyKeys(providers) {
			item := asMap(providers[key])
			lines = append(lines, fmt.Sprintf("| `%s` | %s | %s | %s | %s |", mdCell(key), mdCell(stringValue(item["enabled"])), status(item["available"]), status(item["has_api_key"]), mdCell(strings.Join(asStrings(item["capabilities"]), ", "))))
		}
	}
	if err := errorSummary(data); err != "" {
		lines = append(lines, "", "## Errors", "- "+err)
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func modelMarkdown(data map[string]any) string {
	return resultMarkdown("model", data, "Onesearch Models")
}

func appendEffectiveEnvironmentContent(lines []string, value any) []string {
	items := asAnySlice(value)
	if len(items) == 0 {
		return lines
	}
	parts := make([]string, 0, len(items))
	for _, raw := range items {
		item := asMap(raw)
		part := stringValue(item["name"]) + " (" + stringValue(item["purpose"])
		if provider := stringValue(item["provider"]); provider != "" {
			part += ", " + provider
		}
		parts = append(parts, part+")")
	}
	return append(lines, "Effective environment: "+strings.Join(parts, "; "))
}

func appendEffectiveEnvironmentMarkdown(lines []string, value any) []string {
	items := asAnySlice(value)
	if len(items) == 0 {
		return lines
	}
	lines = append(lines, "", "## Effective Environment", "| Variable | Purpose | Provider |", "| --- | --- | --- |")
	for _, raw := range items {
		item := asMap(raw)
		lines = append(lines, fmt.Sprintf("| `%s` | `%s` | `%s` |", mdCell(stringValue(item["name"])), mdCell(stringValue(item["purpose"])), mdCell(stringValue(item["provider"]))))
	}
	return lines
}

func plainResultLines(data map[string]any) []string {
	results := asAnySlice(data["results"])
	if len(results) == 0 {
		if ok, _ := data["ok"].(bool); ok {
			return []string{"No results."}
		}
		return nil
	}
	var lines []string
	for i, raw := range results {
		item, _ := raw.(map[string]any)
		line := fmt.Sprintf("%d. %s", i+1, resultTitle(item, i+1))
		if target := resultTarget(item); target != "" {
			line += " - " + target
		}
		if summary := resultSummary(item); summary != "" {
			line += " - " + oneLine(summary, 120)
		}
		lines = append(lines, line)
	}
	return lines
}

func titleFor(command string) string {
	switch command {
	case "search":
		return "Onesearch Search"
	case "fetch":
		return "Onesearch Fetch"
	case "map":
		return "Onesearch Map"
	default:
		return "Onesearch"
	}
}

func resultTitle(item map[string]any, index int) string {
	for _, key := range []string{"title", "id", "library_id", "url", "provider"} {
		if value := stringValue(item[key]); value != "" {
			return value
		}
	}
	return fmt.Sprintf("Result %d", index)
}

func resultTarget(item map[string]any) string {
	for _, key := range []string{"url", "id", "library_id"} {
		if value := stringValue(item[key]); value != "" {
			return value
		}
	}
	return ""
}

func resultSummary(item map[string]any) string {
	for _, key := range []string{"description", "content", "snippet", "text", "source"} {
		if value := stringValue(item[key]); value != "" {
			return value
		}
	}
	return ""
}

func errorSummary(data map[string]any) string {
	errorType := stringValue(data["error_type"])
	err := stringValue(data["error"])
	if errorType != "" && err != "" {
		return errorType + ": " + err
	}
	return firstNonEmpty(err, errorType)
}

func truthy(value any) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	text := strings.ToLower(strings.TrimSpace(stringValue(value)))
	return text == "true" || text == "ok"
}

func status(value any) string {
	if b, ok := value.(bool); ok {
		if b {
			return "OK"
		}
		return "FAIL"
	}
	text := strings.ToLower(strings.TrimSpace(stringValue(value)))
	switch text {
	case "ok", "true":
		return "OK"
	case "configured":
		return "CONFIGURED"
	case "warning":
		return "WARN"
	case "timeout":
		return "TIMEOUT"
	case "error":
		return "ERROR"
	case "config_error":
		return "CONFIG ERROR"
	case "not_configured":
		return "NOT CONFIGURED"
	case "false", "failed":
		return "FAIL"
	case "empty":
		return "EMPTY"
	case "skipped":
		return "SKIPPED"
	default:
		if text == "" {
			return "-"
		}
		return strings.ToUpper(text)
	}
}

func mdCell(value string) string {
	return strings.ReplaceAll(oneLine(value, 160), "|", `\|`)
}

func oneLine(value string, limit int) string {
	text := strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")), " ")
	if limit > 0 && len(text) > limit {
		return strings.TrimSpace(text[:limit-3]) + "..."
	}
	return text
}

func previewText(value string, limit int) string {
	text := strings.TrimSpace(value)
	runes := []rune(text)
	if limit > 0 && len(runes) > limit {
		return strings.TrimSpace(string(runes[:limit-3])) + "..."
	}
	return text
}

func asAnySlice(value any) []any {
	switch items := value.(type) {
	case []any:
		return items
	case []map[string]any:
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out
	case []string:
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func asMap(value any) map[string]any {
	if item, ok := value.(map[string]any); ok {
		return item
	}
	return map[string]any{}
}

func asStrings(value any) []string {
	switch items := value.(type) {
	case []string:
		return items
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			out = append(out, stringValue(item))
		}
		return out
	default:
		return nil
	}
}

func sortedKeys(items map[string][]string) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedAnyKeys(items map[string]any) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
