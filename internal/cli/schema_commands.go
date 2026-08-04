package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/deqiying/onesearch/internal/app"
	"github.com/deqiying/onesearch/internal/commandcontract"
	"github.com/deqiying/onesearch/internal/output"
)

func runSchema(parsed *parsedCommand) int {
	path := parsed.Strings("command_path")
	definitions := commandRegistry.Commands()
	scope := commandcontract.ManifestScope{Mode: "all", Path: []string{}}
	if len(path) > 0 {
		definition, ok := commandRegistry.LookupCanonical(path...)
		if !ok || definition.Visibility != commandcontract.VisibilityPublic {
			return writeStaticError("unknown canonical command path: "+strings.Join(path, " "), parsed.String("output"), schemaPaths())
		}
		definitions = []commandcontract.CommandDefinition{definition}
		scope = commandcontract.ManifestScope{Mode: "command", Path: append([]string{}, path...)}
	}
	sort.Slice(definitions, func(i, j int) bool {
		left, right := categoryRank(definitions[i].Category), categoryRank(definitions[j].Category)
		if left != right {
			return left < right
		}
		return strings.Join(definitions[i].Path, "\x00") < strings.Join(definitions[j].Path, "\x00")
	})
	commands := make([]commandcontract.ManifestCommand, 0, len(definitions))
	for _, definition := range definitions {
		commands = append(commands, definition.Manifest())
	}
	manifest := commandcontract.Manifest{
		OK: true, Kind: commandcontract.ManifestKind, ManifestVersion: commandcontract.ManifestVersion,
		CLI: commandcontract.CLIInfo{Name: app.Name, Version: app.Version}, Scope: scope, Commands: commands,
	}
	return writeStaticJSON(manifest, parsed.String("output"), 0)
}

func writeStaticParameterError(message, outputPath string) int {
	return writeStaticError(message, outputPath, nil)
}

func writeStaticError(message, outputPath string, availablePaths []string) int {
	data := struct {
		OK             bool     `json:"ok"`
		ErrorType      string   `json:"error_type"`
		Error          string   `json:"error"`
		AvailablePaths []string `json:"available_paths,omitempty"`
	}{OK: false, ErrorType: "parameter_error", Error: message, AvailablePaths: availablePaths}
	return writeStaticJSON(data, outputPath, 2)
}

func writeStaticJSON(value any, outputPath string, exitCode int) int {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 5
	}
	rendered := buffer.String()
	if err := output.Write(outputPath, rendered); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 5
	}
	fmt.Print(rendered)
	return exitCode
}

func schemaPaths() []string {
	paths := []string{}
	for _, definition := range commandRegistry.Commands() {
		paths = append(paths, strings.Join(definition.Path, " "))
	}
	sort.Strings(paths)
	return paths
}

func categoryRank(category commandcontract.Category) int {
	switch category {
	case commandcontract.CategoryWorkflow:
		return 0
	case commandcontract.CategoryProvider:
		return 1
	default:
		return 2
	}
}
