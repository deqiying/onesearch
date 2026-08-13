package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deqiying/onesearch/internal/app"
	"github.com/deqiying/onesearch/internal/commandcontract"
)

func printRegistryHelp() {
	fmt.Println(app.Description)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  onesearch <workflow> [args] [flags]")
	fmt.Println("  onesearch <provider> <command> [args] [flags]")
	fmt.Println("  onesearch schema [canonical-command-path...] [--pretty] [--output path]")
	fmt.Println()
	printTopCommandGroup("Workflow / capability", commandcontract.CategoryWorkflow)
	printTopNamespaces("Provider direct", commandcontract.CategoryProvider)
	printTopCommandGroup("Utility", commandcontract.CategoryUtility)
	fmt.Println()
	fmt.Println("Use `onesearch <command> --help` for command details.")
}

func printTopCommandGroup(title string, category commandcontract.Category) {
	items := []string{}
	seen := map[string]bool{}
	for _, definition := range commandRegistry.Commands() {
		if definition.Category != category {
			continue
		}
		name := definition.Path[0]
		if !seen[name] {
			items = append(items, name)
			seen[name] = true
		}
	}
	for _, namespace := range commandRegistry.Namespaces() {
		if namespace.Category == category && !seen[namespace.Path[0]] {
			items = append(items, namespace.Path[0])
			seen[namespace.Path[0]] = true
		}
	}
	sort.Strings(items)
	fmt.Println(title + ":")
	fmt.Println("  " + strings.Join(items, ", "))
	fmt.Println()
}

func printTopNamespaces(title string, category commandcontract.Category) {
	items := []string{}
	for _, namespace := range commandRegistry.Namespaces() {
		if namespace.Category == category {
			items = append(items, namespace.Path[0])
		}
	}
	sort.Strings(items)
	fmt.Println(title + ":")
	for _, item := range items {
		commands := commandRegistry.CommandsForProvider(item)
		subcommands := make([]string, 0, len(commands))
		for _, command := range commands {
			subcommands = append(subcommands, command.Path[len(command.Path)-1])
		}
		fmt.Printf("  %s %s\n", item, strings.Join(subcommands, "|"))
	}
	fmt.Println()
}

func printNamespaceHelp(namespace commandcontract.NamespaceDefinition) {
	fmt.Printf("%s\n\n", namespace.Description)
	fmt.Printf("Usage:\n  onesearch %s <command> [args] [flags]\n", strings.Join(namespace.Path, " "))
	if namespace.DefaultCommandID != "" {
		fmt.Printf("  onesearch %s [flags]\n", strings.Join(namespace.Path, " "))
	}
	fmt.Println()
	fmt.Println("Commands:")
	commands := namespaceCommands(namespace)
	for _, command := range commands {
		name := strings.Join(command.Path[len(namespace.Path):], " ")
		fmt.Printf("  %-24s %s\n", name, command.Description)
	}
}

func namespaceCommands(namespace commandcontract.NamespaceDefinition) []commandcontract.CommandDefinition {
	commands := []commandcontract.CommandDefinition{}
	for _, command := range commandRegistry.Commands() {
		if len(command.Path) > len(namespace.Path) && pathPrefix(command.Path, namespace.Path) {
			commands = append(commands, command)
		}
	}
	return commands
}

func printCommandHelp(definition commandcontract.CommandDefinition) {
	fmt.Println(definition.Description)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  onesearch %s", strings.Join(definition.Path, " "))
	for _, positional := range definition.Positionals {
		name := "<" + positional.Name + ">"
		if positional.Variadic {
			name = "<" + positional.Name + "...>"
		}
		if !positional.Required {
			name = "[" + name + "]"
		}
		fmt.Print(" " + name)
	}
	if len(definition.Options) > 0 {
		fmt.Print(" [flags]")
	}
	fmt.Println()
	if len(definition.Positionals) > 0 {
		fmt.Println()
		fmt.Println("Arguments:")
		for _, positional := range definition.Positionals {
			qualifier := "optional"
			if positional.Required {
				qualifier = "required"
			}
			if positional.Variadic {
				qualifier += ", repeatable"
			}
			fmt.Printf("  %-24s %s (%s)\n", positional.Name, positional.Description, qualifier)
		}
	}
	if len(definition.Options) > 0 {
		fmt.Println()
		fmt.Println("Flags:")
		for _, option := range definition.Options {
			flagUsage := "--" + option.Flag
			if option.Type != commandcontract.TypeBoolean {
				flagUsage += " <" + string(option.Type) + ">"
			}
			detail := option.Description
			if option.HasDefault {
				if text, ok := option.Default.(string); ok && text == "" {
					detail += " Default: empty."
				} else {
					detail += fmt.Sprintf(" Default: %v.", option.Default)
				}
			}
			if len(option.Enum) > 0 {
				values := nonEmptyHelpValues(option.Enum)
				if len(values) > 0 {
					detail += " Values: " + strings.Join(values, ", ") + "."
				}
			}
			fmt.Printf("  %-32s %s\n", flagUsage, detail)
		}
	}
	fmt.Println()
	fmt.Println("  -h, --help                     Show this help without loading runtime configuration.")
}

func nonEmptyHelpValues(values []string) []string {
	out := []string{}
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
