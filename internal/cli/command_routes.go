package cli

import (
	"strings"

	"github.com/deqiying/onesearch/internal/commandcontract"
)

var commandRegistry = commandcontract.MustDefaultRegistry()

type resolvedInvocation struct {
	Definition commandcontract.CommandDefinition
	Consumed   int
	Alias      bool
}

func resolveInvocation(args []string) (resolvedInvocation, bool) {
	best := resolvedInvocation{}
	found := false
	for _, definition := range commandRegistry.Commands() {
		if pathPrefix(args, definition.Path) && (!found || len(definition.Path) > best.Consumed) {
			best = resolvedInvocation{Definition: definition, Consumed: len(definition.Path)}
			found = true
		}
		for _, alias := range definition.Aliases {
			if pathPrefix(args, alias) && (!found || len(alias) > best.Consumed) {
				best = resolvedInvocation{Definition: definition, Consumed: len(alias), Alias: true}
				found = true
			}
		}
	}
	if found {
		return best, true
	}

	if namespace, consumed, ok := resolveNamespacePrefix(args); ok && namespace.DefaultCommandID != "" {
		if len(args) == consumed || (len(args) > consumed && (looksLikeFlagToken(args[consumed]) || args[consumed] == "--")) {
			definition, exists := commandRegistry.LookupID(namespace.DefaultCommandID)
			if exists {
				return resolvedInvocation{Definition: definition, Consumed: consumed, Alias: !equalPath(args[:consumed], namespace.Path)}, true
			}
		}
	}
	return resolvedInvocation{}, false
}

func resolveNamespacePrefix(args []string) (commandcontract.NamespaceDefinition, int, bool) {
	var best commandcontract.NamespaceDefinition
	bestLength := 0
	found := false
	for _, namespace := range commandRegistry.Namespaces() {
		if pathPrefix(args, namespace.Path) && len(namespace.Path) > bestLength {
			best, bestLength, found = namespace, len(namespace.Path), true
		}
		for _, alias := range namespace.Aliases {
			if pathPrefix(args, alias) && len(alias) > bestLength {
				best, bestLength, found = namespace, len(alias), true
			}
		}
	}
	return best, bestLength, found
}

func helpTarget(args []string) (commandcontract.CommandDefinition, *commandcontract.NamespaceDefinition, bool) {
	helpIndex := -1
	for index, arg := range args {
		if arg == "--" {
			break
		}
		if isHelpToken(arg) {
			helpIndex = index
			break
		}
	}
	if helpIndex < 0 {
		return commandcontract.CommandDefinition{}, nil, false
	}
	if helpIndex == 0 {
		return commandcontract.CommandDefinition{}, nil, true
	}
	prefix := args[:helpIndex]
	if namespace, consumed, ok := resolveNamespacePrefix(prefix); ok && consumed == len(prefix) {
		return commandcontract.CommandDefinition{}, &namespace, true
	}
	if invocation, ok := resolveInvocation(prefix); ok {
		return invocation.Definition, nil, true
	}
	return commandcontract.CommandDefinition{}, nil, true
}

func pathPrefix(args, path []string) bool {
	if len(args) < len(path) {
		return false
	}
	for index := range path {
		if args[index] != path[index] {
			return false
		}
	}
	return true
}

func equalPath(left, right []string) bool {
	return len(left) == len(right) && strings.Join(left, "\x00") == strings.Join(right, "\x00")
}
