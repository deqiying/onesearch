package cli

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

var providerToolAliases = buildProviderToolAliases()

func buildProviderToolAliases() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, definition := range commandRegistry.Commands() {
		if definition.Provider == "" {
			continue
		}
		binding := commandBindings[definition.ID]
		if out[definition.Provider] == nil {
			out[definition.Provider] = map[string]string{}
		}
		out[definition.Provider][definition.Path[len(definition.Path)-1]] = binding.HandlerKey
	}
	return out
}

func shouldDispatchProviderCommand(command string, _ []string) bool {
	_, ok := providerToolAliases[command]
	return ok
}

func canonicalProviderTool(provider, subcommand string) (string, bool) {
	definition, ok := commandRegistry.LookupCanonical(provider, subcommand)
	if !ok || definition.Provider != provider {
		return "", false
	}
	binding, ok := bindingFor(definition.ID)
	return binding.HandlerKey, ok
}

func annotateProviderTool(data map[string]any, provider, tool string) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	data["provider"] = provider
	data["tool"] = tool
	return data
}

func annotateStatusCommands(data map[string]any) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	providers, _ := data["providers"].(map[string]any)
	direct := map[string]any{}
	names := make([]string, 0, len(providerToolAliases))
	for provider := range providerToolAliases {
		names = append(names, provider)
	}
	sort.Strings(names)
	for _, provider := range names {
		commands := providerCommands(provider)
		item := map[string]any{"commands": commands}
		if raw, ok := providers[provider]; ok {
			providerData, _ := raw.(map[string]any)
			if providerData != nil {
				providerData["direct"] = true
				providerData["commands"] = commands
				for _, key := range []string{"available", "enabled", "capabilities", "status", "base_url", "api_key_env", "api_key_set", "api_key_env_set", "api_key_src", "has_api_key"} {
					if value, ok := providerData[key]; ok {
						item[key] = value
					}
				}
				if cliString(providerData["adapter"]) == "mcp_stdio" {
					available, reason := mcpStdioDirectAvailability(provider, providerData)
					item["available"] = available
					providerData["available"] = available
					providerData["direct_reason"] = reason
					if reason != "" {
						item["reason"] = reason
					}
				}
			}
		} else {
			item["available"] = false
			item["reason"] = "unknown_provider"
		}
		direct[provider] = item
	}
	data["direct_endpoints"] = direct
	return data
}

func providerCommands(provider string) []string {
	return providerBindingCommands(provider)
}

func mcpStdioDirectAvailability(provider string, providerData map[string]any) (bool, string) {
	if directEnabledFalse(providerData["enabled"]) {
		return false, "disabled"
	}
	settings := cliMap(providerData["settings"])
	command := strings.TrimSpace(cliString(settings["command"]))
	if command == "" {
		return false, "missing_command"
	}
	if _, err := exec.LookPath(command); err != nil {
		return false, "missing_command"
	}
	tools := cliStringMap(settings["tools"])
	for _, publicTool := range providerToolAliases[provider] {
		if strings.TrimSpace(tools[publicTool]) == "" {
			return false, "missing_tool_mapping"
		}
	}
	return true, ""
}

func isHelpToken(value string) bool {
	return value == "--help" || value == "-h"
}

func splitCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if text := strings.TrimSpace(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func directEnabledFalse(value any) bool {
	if enabled, ok := value.(bool); ok {
		return !enabled
	}
	switch strings.ToLower(strings.TrimSpace(cliString(value))) {
	case "false", "0", "off", "no":
		return true
	default:
		return false
	}
}

func cliMap(value any) map[string]any {
	item, _ := value.(map[string]any)
	return item
}

func cliStringMap(value any) map[string]string {
	switch items := value.(type) {
	case map[string]string:
		return items
	case map[string]any:
		out := map[string]string{}
		for key, value := range items {
			out[key] = cliString(value)
		}
		return out
	default:
		return nil
	}
}

func cliString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
