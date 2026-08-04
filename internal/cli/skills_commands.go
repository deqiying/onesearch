package cli

import (
	"strings"

	"github.com/deqiying/onesearch/internal/skills"
)

func skillsListData(capability string) map[string]any {
	capability = strings.TrimSpace(capability)
	items := []map[string]any{}
	for _, definition := range skills.Definitions() {
		if capability != "" && !skillHasCapability(definition, capability) {
			continue
		}
		items = append(items, skillDefinitionData(definition))
	}
	return map[string]any{"ok": true, "skills": items, "total": len(items)}
}

func skillShowData(name, filePath string) map[string]any {
	definition, err := skills.Describe(name)
	if err != nil {
		return map[string]any{"ok": false, "error_type": "parameter_error", "error": err.Error()}
	}
	file, err := skills.ReadFile(name, filePath)
	if err != nil {
		return map[string]any{"ok": false, "skill": definition.ID, "error_type": "parameter_error", "error": err.Error()}
	}
	return map[string]any{"ok": true, "skill": skillDefinitionData(definition), "file": file.Path, "content": string(file.Data)}
}

func runStaticSkillCommand(parsed *parsedCommand) int {
	fo := formatOutputFromParsed(parsed, nil)
	switch parsed.Definition.ID {
	case "skills.list":
		return printCommand(nil, "skills", skillsListData(parsed.String("capability")), fo)
	case "skills.show":
		return printCommand(nil, "skills", skillShowData(parsed.String("name"), parsed.String("file")), fo)
	default:
		return printParameterError(nil, "skills", "unsupported static skill command: "+parsed.Definition.ID, fo)
	}
}

func skillDefinitionData(definition skills.Definition) map[string]any {
	return map[string]any{
		"id":           definition.ID,
		"aliases":      append([]string{}, definition.Aliases...),
		"capabilities": append([]string{}, definition.Capabilities...),
		"description":  definition.Description,
	}
}

func skillHasCapability(definition skills.Definition, capability string) bool {
	target := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(capability)), "_", "-")
	for _, item := range definition.Capabilities {
		normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(item)), "_", "-")
		if normalized == target || normalized == "all" {
			return true
		}
	}
	return false
}
