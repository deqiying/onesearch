package cli

import (
	"strings"

	"github.com/deqiying/onesearch/internal/service"
	"github.com/deqiying/onesearch/internal/skills"
)

func runSkills(svc *service.Service, args []string) int {
	subcommand := "list"
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		subcommand = canonicalSkillsSubcommand(args[0])
		rest = args[1:]
	}
	fs := flagSet("skills")
	capability := fs.String("capability", "", "")
	outputFlags := addOutputFlags(fs)
	if err := parse(fs, rest); err != nil {
		return printParameterError(svc, "skills", err.Error(), makeFormatOutput(outputFlags, svc))
	}
	switch subcommand {
	case "list":
		return printCommand(svc, "skills", skillsListData(*capability), makeFormatOutput(outputFlags, svc))
	case "show":
		if fs.NArg() < 1 {
			return printParameterError(svc, "skills", "skills show requires skill name", makeFormatOutput(outputFlags, svc))
		}
		data := skillShowData(fs.Arg(0))
		return printCommand(svc, "skills", data, makeFormatOutput(outputFlags, svc))
	default:
		return printParameterError(svc, "skills", "unknown skills subcommand: "+subcommand, makeFormatOutput(outputFlags, svc))
	}
}

func skillsListData(capability string) map[string]any {
	capability = strings.TrimSpace(capability)
	items := []map[string]any{}
	for _, def := range skills.Definitions() {
		if capability != "" && !skillHasCapability(def, capability) {
			continue
		}
		items = append(items, skillDefinitionData(def))
	}
	return map[string]any{
		"ok":     true,
		"skills": items,
		"total":  len(items),
	}
}

func skillShowData(name string) map[string]any {
	def, err := skills.Describe(name)
	if err != nil {
		return map[string]any{"ok": false, "error_type": "parameter_error", "error": err.Error()}
	}
	content, err := skills.ReadMarkdown(name)
	if err != nil {
		return map[string]any{"ok": false, "skill": def.ID, "error_type": "local_error", "error": err.Error()}
	}
	return map[string]any{
		"ok":      true,
		"skill":   skillDefinitionData(def),
		"content": content,
	}
}

func skillDefinitionData(def skills.Definition) map[string]any {
	return map[string]any{
		"id":           def.ID,
		"aliases":      append([]string{}, def.Aliases...),
		"capabilities": append([]string{}, def.Capabilities...),
		"description":  def.Description,
	}
}

func skillHasCapability(def skills.Definition, capability string) bool {
	target := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(capability)), "_", "-")
	for _, item := range def.Capabilities {
		normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(item)), "_", "-")
		if normalized == target || normalized == "all" {
			return true
		}
	}
	return false
}

func canonicalSkillsSubcommand(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "list", "ls", "l":
		return "list"
	case "show", "get", "read", "load":
		return "show"
	default:
		return value
	}
}
