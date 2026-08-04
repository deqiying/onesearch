package commandcontract

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Registry is an immutable, validated index of command and namespace
// definitions. The constructor copies its input so callers may safely reuse
// or mutate their source slices after construction.
type Registry struct {
	commands   []CommandDefinition
	namespaces []NamespaceDefinition
	byID       map[string]int
	byPath     map[string]int
	byAlias    map[string]int
}

// NewRegistry validates and indexes commands and namespaces.
func NewRegistry(commands []CommandDefinition, namespaces []NamespaceDefinition) (*Registry, error) {
	if err := ValidateDefinitions(commands, namespaces); err != nil {
		return nil, err
	}
	r := &Registry{
		commands:   cloneCommands(commands),
		namespaces: cloneNamespaces(namespaces),
		byID:       map[string]int{},
		byPath:     map[string]int{},
		byAlias:    map[string]int{},
	}
	sort.Slice(r.commands, func(i, j int) bool { return pathKey(r.commands[i].Path) < pathKey(r.commands[j].Path) })
	sort.Slice(r.namespaces, func(i, j int) bool { return pathKey(r.namespaces[i].Path) < pathKey(r.namespaces[j].Path) })
	for i := range r.commands {
		command := &r.commands[i]
		r.byID[command.ID] = i
		r.byPath[pathKey(command.Path)] = i
		for _, alias := range command.Aliases {
			r.byAlias[pathKey(alias)] = i
		}
	}
	return r, nil
}

// DefaultRegistry returns the repository's complete static command registry.
// Definition fragments are intentionally package-private; this is the public
// aggregate used by CLI, status and planner consumers.
func DefaultRegistry() (*Registry, error) {
	return NewRegistry(
		append(append(append([]CommandDefinition{}, workflowDefinitions()...), providerDefinitions()...), utilityDefinitions()...),
		namespaceDefinitions(),
	)
}

// MustDefaultRegistry is a convenience for package initialization and tests.
func MustDefaultRegistry() *Registry {
	r, err := DefaultRegistry()
	if err != nil {
		panic(err)
	}
	return r
}

// Commands returns public command definitions in canonical path order.
func (r *Registry) Commands() []CommandDefinition {
	if r == nil {
		return nil
	}
	out := make([]CommandDefinition, 0, len(r.commands))
	for _, command := range r.commands {
		if command.Visibility == VisibilityPublic {
			out = append(out, cloneCommand(command))
		}
	}
	return out
}

// AllCommands returns public and hidden definitions in canonical path order.
func (r *Registry) AllCommands() []CommandDefinition {
	if r == nil {
		return nil
	}
	return cloneCommands(r.commands)
}

// Namespaces returns public namespace definitions in canonical path order.
func (r *Registry) Namespaces() []NamespaceDefinition {
	if r == nil {
		return nil
	}
	out := make([]NamespaceDefinition, 0, len(r.namespaces))
	for _, namespace := range r.namespaces {
		if namespace.Visibility == VisibilityPublic {
			out = append(out, cloneNamespace(namespace))
		}
	}
	return out
}

// AllNamespaces returns public and hidden namespace definitions.
func (r *Registry) AllNamespaces() []NamespaceDefinition {
	if r == nil {
		return nil
	}
	return cloneNamespaces(r.namespaces)
}

// LookupID finds a command by its stable ID.
func (r *Registry) LookupID(id string) (CommandDefinition, bool) {
	if r == nil {
		return CommandDefinition{}, false
	}
	index, ok := r.byID[strings.TrimSpace(id)]
	if !ok {
		return CommandDefinition{}, false
	}
	return cloneCommand(r.commands[index]), true
}

// LookupCanonical finds a command by its exact canonical path.
func (r *Registry) LookupCanonical(path ...string) (CommandDefinition, bool) {
	if r == nil {
		return CommandDefinition{}, false
	}
	index, ok := r.byPath[pathKey(path)]
	if !ok {
		return CommandDefinition{}, false
	}
	return cloneCommand(r.commands[index]), true
}

// LookupAlias finds a command by an exact compatibility alias path.
func (r *Registry) LookupAlias(path ...string) (CommandDefinition, bool) {
	if r == nil {
		return CommandDefinition{}, false
	}
	index, ok := r.byAlias[pathKey(path)]
	if !ok {
		return CommandDefinition{}, false
	}
	return cloneCommand(r.commands[index]), true
}

// Lookup resolves canonical paths first, then compatibility aliases. It does
// not resolve IDs; use LookupID when an ID is available.
func (r *Registry) Lookup(path ...string) (CommandDefinition, bool) {
	if command, ok := r.LookupCanonical(path...); ok {
		return command, true
	}
	return r.LookupAlias(path...)
}

// CommandsForProvider returns provider-direct commands in canonical path order.
func (r *Registry) CommandsForProvider(provider string) []CommandDefinition {
	provider = strings.TrimSpace(provider)
	out := []CommandDefinition{}
	for _, command := range r.Commands() {
		if command.Category == CategoryProvider && command.Provider == provider {
			out = append(out, command)
		}
	}
	return out
}

// PreferredFor returns the unique command preferred for a capability. It
// returns false when no command is preferred or when definitions are
// ambiguous (the latter should already be rejected by validation).
func (r *Registry) PreferredFor(capability string) (CommandDefinition, bool) {
	capability = strings.TrimSpace(capability)
	var found CommandDefinition
	count := 0
	for _, command := range r.Commands() {
		for _, preferred := range command.PreferredFor {
			if strings.TrimSpace(preferred) == capability {
				found = command
				count++
			}
		}
	}
	return found, count == 1
}

// Validate validates a registry's indexed definitions.
func (r *Registry) Validate() error {
	if r == nil {
		return fmt.Errorf("nil registry")
	}
	return ValidateDefinitions(r.commands, r.namespaces)
}

// ValidateDefinitions validates command and namespace contracts without
// constructing an index.
func ValidateDefinitions(commands []CommandDefinition, namespaces []NamespaceDefinition) error {
	ids := map[string]string{}
	paths := map[string]string{}
	for index, command := range commands {
		if err := validateCommand(command); err != nil {
			return fmt.Errorf("command[%d]: %w", index, err)
		}
		if _, exists := ids[command.ID]; exists {
			return fmt.Errorf("duplicate command id %q", command.ID)
		}
		ids[command.ID] = command.ID
		canonical := pathKey(command.Path)
		if previous, exists := paths[canonical]; exists {
			return fmt.Errorf("path %q conflicts with %q", displayPath(command.Path), previous)
		}
		paths[canonical] = command.ID
	}
	for index, namespace := range namespaces {
		if err := validateNamespace(namespace); err != nil {
			return fmt.Errorf("namespace[%d]: %w", index, err)
		}
		if namespace.DefaultCommandID != "" {
			if _, exists := ids[namespace.DefaultCommandID]; !exists {
				return fmt.Errorf("namespace %q default command %q does not exist", displayPath(namespace.Path), namespace.DefaultCommandID)
			}
		}
		canonical := pathKey(namespace.Path)
		if previous, exists := paths[canonical]; exists {
			return fmt.Errorf("namespace path %q conflicts with %q", displayPath(namespace.Path), previous)
		}
		paths[canonical] = "namespace"
	}
	for _, command := range commands {
		if err := validateAliases(command, paths); err != nil {
			return fmt.Errorf("command %q: %w", command.ID, err)
		}
	}
	for _, namespace := range namespaces {
		if err := validateNamespaceAliases(namespace, paths); err != nil {
			return fmt.Errorf("namespace %q: %w", displayPath(namespace.Path), err)
		}
	}
	if err := validatePreferred(commands); err != nil {
		return err
	}
	return nil
}

func validateCommand(command CommandDefinition) error {
	if strings.TrimSpace(command.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if err := validatePath(command.Path, "path"); err != nil {
		return err
	}
	if command.Category != CategoryWorkflow && command.Category != CategoryProvider && command.Category != CategoryUtility {
		return fmt.Errorf("invalid category %q", command.Category)
	}
	if command.Visibility != VisibilityPublic && command.Visibility != VisibilityHidden {
		return fmt.Errorf("invalid visibility %q", command.Visibility)
	}
	if strings.TrimSpace(command.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	if command.Category == CategoryProvider && strings.TrimSpace(command.Provider) == "" {
		return fmt.Errorf("provider is required for provider command")
	}
	optionalPositionalSeen := false
	for index, positional := range command.Positionals {
		if err := validatePositional(positional, index, len(command.Positionals)); err != nil {
			return err
		}
		if optionalPositionalSeen && positional.Required {
			return fmt.Errorf("required positional %q follows optional positional", positional.Name)
		}
		if !positional.Required {
			optionalPositionalSeen = true
		}
	}
	if err := validateOptions(command.Options); err != nil {
		return err
	}
	optionNames := map[string]bool{}
	for _, positional := range command.Positionals {
		optionNames[positional.Name] = true
	}
	for _, option := range command.Options {
		optionNames[option.Name] = true
		optionNames[option.Flag] = true
	}
	for _, constraint := range command.Constraints {
		if strings.TrimSpace(constraint.Kind) == "" || len(constraint.Members) < 2 {
			return fmt.Errorf("constraint kind and at least two members are required")
		}
		switch constraint.Kind {
		case "mutually_exclusive", "at_least_one", "requires":
		default:
			return fmt.Errorf("unsupported constraint kind %q", constraint.Kind)
		}
		seen := map[string]bool{}
		for _, member := range constraint.Members {
			member = strings.TrimSpace(member)
			if member == "" || seen[member] || !optionNames[member] {
				return fmt.Errorf("constraint %q references invalid member %q", constraint.Kind, member)
			}
			seen[member] = true
		}
	}
	channelNames := map[string]bool{}
	for _, channel := range command.InputChannels {
		if strings.TrimSpace(channel.Name) == "" || channelNames[channel.Name] {
			return fmt.Errorf("input channel name must be non-empty and unique")
		}
		channelNames[channel.Name] = true
		if len(channel.Bindings) == 0 {
			return fmt.Errorf("input channel %q requires at least one binding", channel.Name)
		}
		for _, binding := range channel.Bindings {
			if strings.TrimSpace(binding.Kind) == "" {
				return fmt.Errorf("input channel %q has a binding without kind", channel.Name)
			}
			if binding.ActivatedBy != "" && !optionNames[binding.ActivatedBy] {
				return fmt.Errorf("input channel %q references unknown activator %q", channel.Name, binding.ActivatedBy)
			}
		}
		if channel.Sensitive && channel.ForbiddenBinding != "argv" {
			return fmt.Errorf("sensitive input channel %q must forbid argv binding", channel.Name)
		}
	}
	if command.Availability.Dynamic && (len(command.Availability.CheckCommand) == 0 || strings.TrimSpace(command.Availability.JSONPointer) == "") {
		return fmt.Errorf("dynamic availability requires check_command and json_pointer")
	}
	if strings.TrimSpace(command.Output.DefaultFormat) == "" {
		return fmt.Errorf("output default format is required")
	}
	return nil
}

func validatePositional(positional PositionalDefinition, index, total int) error {
	if strings.TrimSpace(positional.Name) == "" {
		return fmt.Errorf("positional[%d] name is required", index)
	}
	if err := validateValueType(positional.Type); err != nil {
		return fmt.Errorf("positional %q: %w", positional.Name, err)
	}
	if positional.Variadic && index != total-1 {
		return fmt.Errorf("variadic positional %q must be last", positional.Name)
	}
	if positional.MinLength < 0 || positional.MinItems < 0 || positional.MaxItems < 0 {
		return fmt.Errorf("positional %q has negative bounds", positional.Name)
	}
	if positional.MaxItems > 0 && positional.MinItems > positional.MaxItems {
		return fmt.Errorf("positional %q min_items exceeds max_items", positional.Name)
	}
	if !positional.Variadic && (positional.MinItems > 0 || positional.MaxItems > 0) {
		return fmt.Errorf("non-variadic positional %q cannot use item bounds", positional.Name)
	}
	return nil
}

func validateOptions(options []OptionDefinition) error {
	names := map[string]bool{}
	flags := map[string]bool{}
	keys := map[string]bool{}
	for index, option := range options {
		if strings.TrimSpace(option.Name) == "" || strings.TrimSpace(option.Flag) == "" {
			return fmt.Errorf("option[%d] name and flag are required", index)
		}
		if names[option.Name] || flags[option.Flag] || keys[option.Name] || keys[option.Flag] {
			return fmt.Errorf("duplicate option name or flag %q", option.Name)
		}
		if strings.HasPrefix(option.Flag, "-") {
			return fmt.Errorf("option %q flag must omit leading dashes", option.Name)
		}
		if err := validateValueType(option.Type); err != nil {
			return fmt.Errorf("option %q: %w", option.Name, err)
		}
		if option.Minimum != nil && option.Maximum != nil && *option.Minimum > *option.Maximum {
			return fmt.Errorf("option %q minimum exceeds maximum", option.Name)
		}
		if (option.Minimum != nil || option.Maximum != nil) && option.Type != TypeInteger && option.Type != TypeNumber {
			return fmt.Errorf("option %q numeric bounds require integer or number type", option.Name)
		}
		if len(option.Enum) > 0 && option.Type != TypeString {
			return fmt.Errorf("option %q enum requires string type", option.Name)
		}
		if option.Repeatable && option.Type != TypeStringArray {
			return fmt.Errorf("option %q repeatable requires string_array type", option.Name)
		}
		if option.Greedy && !option.Repeatable {
			return fmt.Errorf("option %q greedy requires repeatable", option.Name)
		}
		if option.HasDefault {
			if err := validateOptionValue(option, option.Default); err != nil {
				return fmt.Errorf("option %q default: %w", option.Name, err)
			}
		}
		for _, value := range option.Enum {
			if strings.TrimSpace(value) == "" && option.Type != TypeString {
				return fmt.Errorf("option %q enum contains empty value for non-string type", option.Name)
			}
		}
		if option.HasDefault && len(option.Enum) > 0 {
			defaultValue, _ := option.Default.(string)
			found := false
			for _, value := range option.Enum {
				if value == defaultValue {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("option %q default is outside enum", option.Name)
			}
		}
		names[option.Name] = true
		flags[option.Flag] = true
		keys[option.Name] = true
		keys[option.Flag] = true
	}
	for _, option := range options {
		if option.AliasFor == option.Name || option.AliasFor == option.Flag || option.Overrides == option.Name || option.Overrides == option.Flag {
			return fmt.Errorf("option %q cannot reference itself", option.Name)
		}
		if option.AliasFor != "" && !names[option.AliasFor] && !flags[option.AliasFor] {
			return fmt.Errorf("option %q alias_for references unknown option %q", option.Name, option.AliasFor)
		}
		if option.Overrides != "" && !names[option.Overrides] && !flags[option.Overrides] {
			return fmt.Errorf("option %q overrides unknown option %q", option.Name, option.Overrides)
		}
		if option.OverridesWhen != "" && option.Overrides == "" {
			return fmt.Errorf("option %q overrides_when requires overrides", option.Name)
		}
		if option.OverridesWhen != "" && option.OverridesWhen != "positive" {
			return fmt.Errorf("option %q uses unsupported overrides_when %q", option.Name, option.OverridesWhen)
		}
	}
	return nil
}

func validateNamespace(namespace NamespaceDefinition) error {
	if err := validatePath(namespace.Path, "path"); err != nil {
		return err
	}
	if namespace.Category != CategoryWorkflow && namespace.Category != CategoryProvider && namespace.Category != CategoryUtility {
		return fmt.Errorf("invalid category %q", namespace.Category)
	}
	if namespace.Visibility != VisibilityPublic && namespace.Visibility != VisibilityHidden {
		return fmt.Errorf("invalid visibility %q", namespace.Visibility)
	}
	if strings.TrimSpace(namespace.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}

func validatePath(path []string, label string) error {
	if len(path) == 0 {
		return fmt.Errorf("%s is required", label)
	}
	for _, token := range path {
		if strings.TrimSpace(token) == "" || token == "." || token == ".." {
			return fmt.Errorf("%s contains invalid token", label)
		}
	}
	return nil
}

func validateAliases(command CommandDefinition, paths map[string]string) error {
	seen := map[string]bool{}
	for _, alias := range command.Aliases {
		if err := validatePath(alias, "alias"); err != nil {
			return err
		}
		key := pathKey(alias)
		if seen[key] || key == pathKey(command.Path) {
			return fmt.Errorf("duplicate alias %q", displayPath(alias))
		}
		if previous, exists := paths[key]; exists {
			return fmt.Errorf("alias %q conflicts with %q", displayPath(alias), previous)
		}
		seen[key] = true
		paths[key] = command.ID
	}
	return nil
}

func validateNamespaceAliases(namespace NamespaceDefinition, paths map[string]string) error {
	seen := map[string]bool{}
	for _, alias := range namespace.Aliases {
		if err := validatePath(alias, "alias"); err != nil {
			return err
		}
		key := pathKey(alias)
		if seen[key] || key == pathKey(namespace.Path) {
			return fmt.Errorf("duplicate alias %q", displayPath(alias))
		}
		if previous, exists := paths[key]; exists {
			return fmt.Errorf("alias %q conflicts with %q", displayPath(alias), previous)
		}
		seen[key] = true
		paths[key] = "namespace"
	}
	return nil
}

func validatePreferred(commands []CommandDefinition) error {
	preferred := map[string]string{}
	for _, command := range commands {
		for _, capability := range command.PreferredFor {
			capability = strings.TrimSpace(capability)
			if capability == "" {
				return fmt.Errorf("command %q has empty preferred capability", command.ID)
			}
			if !contains(command.Capabilities, capability) {
				return fmt.Errorf("command %q prefers unsupported capability %q", command.ID, capability)
			}
			if previous, exists := preferred[capability]; exists {
				return fmt.Errorf("capability %q has multiple preferred commands %q and %q", capability, previous, command.ID)
			}
			preferred[capability] = command.ID
		}
	}
	return nil
}

func validateOptionValue(option OptionDefinition, value any) error {
	if value == nil {
		return fmt.Errorf("value is nil")
	}
	switch option.Type {
	case TypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("want string, got %T", value)
		}
	case TypeInteger:
		if !isInteger(value) {
			return fmt.Errorf("want integer, got %T", value)
		}
	case TypeNumber:
		if !isNumber(value) {
			return fmt.Errorf("want number, got %T", value)
		}
	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("want boolean, got %T", value)
		}
	case TypeStringArray:
		if reflect.ValueOf(value).Kind() != reflect.Slice && reflect.ValueOf(value).Kind() != reflect.Array {
			return fmt.Errorf("want string array, got %T", value)
		}
		if _, err := stringValues(value); err != nil {
			return err
		}
	}
	return nil
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func cloneCommand(command CommandDefinition) CommandDefinition {
	command.Path = append([]string{}, command.Path...)
	command.Aliases = copyPaths(command.Aliases)
	command.Capabilities = append([]string{}, command.Capabilities...)
	command.PreferredFor = append([]string{}, command.PreferredFor...)
	command.Positionals = append([]PositionalDefinition{}, command.Positionals...)
	command.Options = append([]OptionDefinition{}, command.Options...)
	for i := range command.Options {
		command.Options[i].Enum = append([]string{}, command.Options[i].Enum...)
		if command.Options[i].HasDefault {
			command.Options[i].Default = cloneAny(command.Options[i].Default)
		}
		if command.Options[i].Minimum != nil {
			minimum := *command.Options[i].Minimum
			command.Options[i].Minimum = &minimum
		}
		if command.Options[i].Maximum != nil {
			maximum := *command.Options[i].Maximum
			command.Options[i].Maximum = &maximum
		}
	}
	command.Constraints = append([]ConstraintDefinition{}, command.Constraints...)
	for i := range command.Constraints {
		command.Constraints[i].Members = append([]string{}, command.Constraints[i].Members...)
	}
	command.InputChannels = append([]InputChannelDefinition{}, command.InputChannels...)
	for i := range command.InputChannels {
		command.InputChannels[i].Bindings = append([]InputBindingDefinition{}, command.InputChannels[i].Bindings...)
		command.InputChannels[i].RuntimeCheckCommand = append([]string{}, command.InputChannels[i].RuntimeCheckCommand...)
	}
	command.SideEffects = append([]string{}, command.SideEffects...)
	command.Output.Formats = append([]string{}, command.Output.Formats...)
	command.Output.Variants = append([]string{}, command.Output.Variants...)
	command.Availability.CheckCommand = append([]string{}, command.Availability.CheckCommand...)
	command.Availability.DoesNotProve = append([]string{}, command.Availability.DoesNotProve...)
	return command
}

func cloneCommands(commands []CommandDefinition) []CommandDefinition {
	out := make([]CommandDefinition, len(commands))
	for i, command := range commands {
		out[i] = cloneCommand(command)
	}
	return out
}

func cloneNamespaces(namespaces []NamespaceDefinition) []NamespaceDefinition {
	out := make([]NamespaceDefinition, len(namespaces))
	for i, namespace := range namespaces {
		out[i] = cloneNamespace(namespace)
	}
	return out
}

func cloneNamespace(namespace NamespaceDefinition) NamespaceDefinition {
	namespace.Path = append([]string{}, namespace.Path...)
	namespace.Aliases = copyPaths(namespace.Aliases)
	return namespace
}

func cloneAny(value any) any {
	if value == nil {
		return nil
	}
	return cloneReflect(reflect.ValueOf(value)).Interface()
}

func cloneReflect(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneReflect(value.Elem())
		wrapped := reflect.New(value.Type()).Elem()
		wrapped.Set(cloned)
		return wrapped
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			cloned.SetMapIndex(cloneReflect(iter.Key()), cloneReflect(iter.Value()))
		}
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(cloneReflect(value.Index(i)))
		}
		return cloned
	case reflect.Array:
		cloned := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			cloned.Index(i).Set(cloneReflect(value.Index(i)))
		}
		return cloned
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.New(value.Type().Elem())
		cloned.Elem().Set(cloneReflect(value.Elem()))
		return cloned
	default:
		return value
	}
}
