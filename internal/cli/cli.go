package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/deqiying/onesearch/internal/app"
	"github.com/deqiying/onesearch/internal/config"
	"github.com/deqiying/onesearch/internal/output"
	"github.com/deqiying/onesearch/internal/redact"
	"github.com/deqiying/onesearch/internal/service"
)

func Execute(args []string) int {
	if len(args) == 0 {
		printRegistryHelp()
		return 2
	}
	if args[0] == "--version" || args[0] == "-v" || args[0] == "--v" {
		fmt.Printf("%s %s\n", app.Name, app.Version)
		return 0
	}
	if definition, namespace, requested := helpTarget(args); requested {
		switch {
		case namespace != nil:
			printNamespaceHelp(*namespace)
		case definition.ID != "":
			printCommandHelp(definition)
		default:
			printRegistryHelp()
		}
		return 0
	}

	invocation, ok := resolveInvocation(args)
	if !ok {
		if namespace, consumed, found := resolveNamespacePrefix(args); found {
			if consumed == len(args) {
				printNamespaceHelp(namespace)
				return 2
			}
			message := fmt.Sprintf("unknown %s subcommand: %s", strings.Join(namespace.Path, " "), args[consumed])
			return printParameterError(nil, namespace.Path[0], message, formatOutput{format: "json", verbosity: "quiet"})
		}
		fmt.Fprintln(os.Stderr, "unknown command: "+args[0])
		return 2
	}
	parsed, parseErr := parseCommand(invocation.Definition, args[invocation.Consumed:])
	if invocation.Definition.ID == "schema" {
		if parseErr != nil {
			return writeStaticParameterError(parseErr.Error(), parsed.String("output"), parsed.Bool("pretty"))
		}
		return runSchema(parsed)
	}
	if parseErr != nil {
		return printParsedParameterError(parsed, parseErr.Error(), nil)
	}
	if invocation.Definition.ID == "skills.list" || invocation.Definition.ID == "skills.show" {
		return runStaticSkillCommand(parsed)
	}

	cfg := config.Load()
	svc := service.New(cfg)
	return runParsedCommand(context.Background(), svc, parsed)
}

func parseSearchProviderFilters(raw string) (string, map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "auto") {
		return "auto", nil, nil
	}
	if !strings.Contains(raw, "=") && !strings.Contains(raw, ":") {
		return raw, nil, nil
	}
	filters := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		index := strings.Index(part, "=")
		if index < 0 {
			index = strings.Index(part, ":")
		}
		if index < 0 {
			return "", nil, fmt.Errorf("provider filter %q must use capability=providers", part)
		}
		capability := normalizeCapabilityFilterKey(strings.TrimSpace(part[:index]))
		value := strings.TrimSpace(part[index+1:])
		if capability == "" || value == "" {
			return "", nil, fmt.Errorf("provider filter %q must include capability and provider list", part)
		}
		filters[capability] = value
	}
	if len(filters) == 0 {
		return "auto", nil, nil
	}
	return "auto", filters, nil
}

func overlayProviderFilter(filters map[string]string, capability, value string) map[string]string {
	value = strings.TrimSpace(value)
	if value == "" {
		return filters
	}
	if filters == nil {
		filters = map[string]string{}
	}
	filters[config.V2CapabilityName(capability)] = value
	return filters
}

func normalizeCapabilityFilterKey(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "answer", "answer_search", "search":
		return "answer_search"
	case "source", "sources", "source_search":
		return "source_search"
	case "docs", "doc", "documentation", "docs_search":
		return "docs_search"
	case "fetch", "page", "page_fetch":
		return "page_fetch"
	case "repo", "repo_wiki", "repository", "repository_wiki":
		return "repo_wiki"
	case "site_map", "map":
		return "site_map"
	case "site_crawl", "crawl":
		return "site_crawl"
	case "vertical", "vertical_search":
		return "vertical_search"
	default:
		return config.V2CapabilityName(value)
	}
}

type formatOutput struct {
	format    string
	pretty    bool
	output    string
	verbosity string
}

func printCommand(svc *service.Service, command string, data map[string]any, fo formatOutput, transientSecrets ...string) int {
	if fo.format == "" {
		fo.format = "json"
	}
	secrets := append([]string{}, transientSecrets...)
	if svc != nil {
		secrets = append(secrets, svc.OutputSecretValues()...)
	}
	rendered := output.RenderWithOptions(command, data, output.Options{Format: fo.format, Pretty: fo.pretty, Verbosity: fo.verbosity, SecretValues: secrets})
	if err := output.Write(fo.output, rendered); err != nil {
		fmt.Fprintln(os.Stderr, redact.Text(err.Error(), secrets))
		return 5
	}
	fmt.Print(rendered)
	return output.ExitCode(data)
}

func printParameterError(svc *service.Service, command, message string, fo formatOutput, transientSecrets ...string) int {
	return printCommand(svc, command, map[string]any{"ok": false, "error_type": "parameter_error", "error": message}, fo, transientSecrets...)
}

func defaultVerbosity(svc *service.Service) string {
	if svc != nil {
		data := svc.ConfigList(false)
		defaults, _ := data["defaults"].(map[string]any)
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(defaults["log_level"])), "debug") {
			return "verbose"
		}
	}
	return "quiet"
}
