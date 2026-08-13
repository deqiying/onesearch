package commandcontract

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ManifestKind    = "onesearch_cli_command_manifest"
	ManifestVersion = 2
)

type Category string

const (
	CategoryWorkflow Category = "workflow"
	CategoryProvider Category = "provider"
	CategoryUtility  Category = "utility"
)

type Visibility string

const (
	VisibilityPublic Visibility = "public"
	VisibilityHidden Visibility = "hidden"
)

type ValueType string

const (
	TypeString      ValueType = "string"
	TypeInteger     ValueType = "integer"
	TypeNumber      ValueType = "number"
	TypeBoolean     ValueType = "boolean"
	TypeStringArray ValueType = "string_array"
)

type PositionalDefinition struct {
	Name        string
	Type        ValueType
	Description string
	Required    bool
	Variadic    bool
	MinLength   int
	MinItems    int
	MaxItems    int
}

type OptionDefinition struct {
	Name          string
	Flag          string
	Type          ValueType
	Description   string
	Default       any
	HasDefault    bool
	Enum          []string
	Minimum       *float64
	Maximum       *float64
	Repeatable    bool
	Greedy        bool
	Deprecated    bool
	AliasFor      string
	Overrides     string
	OverridesWhen string
}

type ConstraintDefinition struct {
	Kind    string   `json:"kind"`
	Members []string `json:"members"`
}

type InputBindingDefinition struct {
	Kind        string `json:"kind"`
	When        string `json:"when,omitempty"`
	ActivatedBy string `json:"activated_by,omitempty"`
}

type InputChannelDefinition struct {
	Name                string                   `json:"name"`
	Sensitive           bool                     `json:"sensitive"`
	Bindings            []InputBindingDefinition `json:"bindings"`
	RequiredWhenRuntime string                   `json:"required_when_runtime,omitempty"`
	RuntimeCheckCommand []string                 `json:"runtime_check_command,omitempty"`
	RuntimeCheckScope   string                   `json:"runtime_check_scope,omitempty"`
	ForbiddenBinding    string                   `json:"forbidden_binding,omitempty"`
}

type AvailabilityDefinition struct {
	Dynamic        bool     `json:"dynamic"`
	CheckCommand   []string `json:"check_command,omitempty"`
	JSONPointer    string   `json:"json_pointer,omitempty"`
	PreflightLevel string   `json:"preflight_level,omitempty"`
	DoesNotProve   []string `json:"does_not_prove,omitempty"`
}

type OutputDefinition struct {
	DefaultFormat   string   `json:"default_format"`
	Formats         []string `json:"formats"`
	Variants        []string `json:"variants,omitempty"`
	Contract        string   `json:"contract"`
	ProviderPayload string   `json:"provider_payload,omitempty"`
}

type CommandDefinition struct {
	ID            string
	Path          []string
	Aliases       [][]string
	Category      Category
	Visibility    Visibility
	Description   string
	Capabilities  []string
	PreferredFor  []string
	Provider      string
	Positionals   []PositionalDefinition
	Options       []OptionDefinition
	Constraints   []ConstraintDefinition
	InputChannels []InputChannelDefinition
	Availability  AvailabilityDefinition
	SideEffects   []string
	Output        OutputDefinition
}

type NamespaceDefinition struct {
	Path             []string
	Aliases          [][]string
	Category         Category
	Visibility       Visibility
	Description      string
	DefaultCommandID string
}

type CLIInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ManifestScope struct {
	Mode string   `json:"mode"`
	Path []string `json:"path"`
}

type Manifest struct {
	OK              bool              `json:"ok"`
	Kind            string            `json:"kind"`
	ManifestVersion int               `json:"manifest_version"`
	CLI             CLIInfo           `json:"cli"`
	Scope           ManifestScope     `json:"scope"`
	Commands        []ManifestCommand `json:"commands"`
}

type ManifestCommand struct {
	Name          string                   `json:"name"`
	Description   string                   `json:"description"`
	InputSchema   map[string]any           `json:"input_schema"`
	Path          []string                 `json:"path"`
	Category      Category                 `json:"category"`
	Provider      string                   `json:"provider,omitempty"`
	Capabilities  []string                 `json:"capabilities,omitempty"`
	PreferredFor  []string                 `json:"preferred_for,omitempty"`
	Aliases       [][]string               `json:"aliases"`
	Constraints   []ConstraintDefinition   `json:"constraints,omitempty"`
	InputChannels []InputChannelDefinition `json:"input_channels,omitempty"`
	Availability  AvailabilityDefinition   `json:"availability"`
	SideEffects   []string                 `json:"side_effects"`
	Output        OutputDefinition         `json:"output"`
}

func (d CommandDefinition) Manifest() ManifestCommand {
	aliases := copyPaths(d.Aliases)
	sort.Slice(aliases, func(i, j int) bool { return pathKey(aliases[i]) < pathKey(aliases[j]) })
	capabilities := append([]string{}, d.Capabilities...)
	preferredFor := append([]string{}, d.PreferredFor...)
	sideEffects := append([]string{}, d.SideEffects...)
	sort.Strings(capabilities)
	sort.Strings(preferredFor)
	sort.Strings(sideEffects)
	constraints := append([]ConstraintDefinition{}, d.Constraints...)
	for i := range constraints {
		constraints[i].Members = append([]string{}, constraints[i].Members...)
		sort.Strings(constraints[i].Members)
	}
	sort.Slice(constraints, func(i, j int) bool {
		return constraints[i].Kind+strings.Join(constraints[i].Members, "\x00") < constraints[j].Kind+strings.Join(constraints[j].Members, "\x00")
	})
	channels := append([]InputChannelDefinition{}, d.InputChannels...)
	sort.Slice(channels, func(i, j int) bool { return channels[i].Name < channels[j].Name })
	availability := d.Availability
	availability.CheckCommand = append([]string{}, availability.CheckCommand...)
	availability.DoesNotProve = append([]string{}, availability.DoesNotProve...)
	sort.Strings(availability.DoesNotProve)
	output := d.Output
	output.Formats = append([]string{}, output.Formats...)
	output.Variants = append([]string{}, output.Variants...)
	sort.Strings(output.Formats)
	sort.Strings(output.Variants)
	return ManifestCommand{
		Name:          d.ID,
		Description:   d.Description,
		InputSchema:   d.inputSchema(),
		Path:          append([]string{}, d.Path...),
		Category:      d.Category,
		Provider:      d.Provider,
		Capabilities:  capabilities,
		PreferredFor:  preferredFor,
		Aliases:       aliases,
		Constraints:   constraints,
		InputChannels: channels,
		Availability:  availability,
		SideEffects:   sideEffects,
		Output:        output,
	}
}

func (d CommandDefinition) inputSchema() map[string]any {
	properties := map[string]any{}
	required := []string{}
	for index, positional := range d.Positionals {
		property := map[string]any{
			"description": positional.Description,
			"x-cli-binding": map[string]any{
				"kind":     "positional",
				"index":    index,
				"variadic": positional.Variadic,
			},
		}
		if positional.Variadic {
			property["type"] = "array"
			items := map[string]any{"type": jsonType(positional.Type)}
			if positional.MinLength > 0 {
				items["minLength"] = positional.MinLength
			}
			property["items"] = items
			if positional.MinItems > 0 {
				property["minItems"] = positional.MinItems
			}
			if positional.MaxItems > 0 {
				property["maxItems"] = positional.MaxItems
			}
		} else {
			property["type"] = jsonType(positional.Type)
			if positional.MinLength > 0 {
				property["minLength"] = positional.MinLength
			}
		}
		properties[positional.Name] = property
		if positional.Required {
			required = append(required, positional.Name)
		}
	}
	for _, option := range d.Options {
		property := map[string]any{
			"description": option.Description,
			"x-cli-binding": map[string]any{
				"kind":       "flag",
				"token":      "--" + option.Flag,
				"repeatable": option.Repeatable,
			},
		}
		if option.Type == TypeStringArray {
			property["type"] = "array"
			property["items"] = map[string]any{"type": "string", "minLength": 1}
			property["x-cli-binding"].(map[string]any)["list_encoding"] = ListEncoding
		} else {
			property["type"] = jsonType(option.Type)
		}
		if option.HasDefault {
			property["default"] = option.Default
		}
		if len(option.Enum) > 0 {
			property["enum"] = append([]string{}, option.Enum...)
		}
		if option.Minimum != nil {
			property["minimum"] = *option.Minimum
		}
		if option.Maximum != nil {
			property["maximum"] = *option.Maximum
		}
		if option.Greedy {
			property["x-cli-binding"].(map[string]any)["greedy"] = true
		}
		if option.Deprecated {
			property["deprecated"] = true
		}
		if option.AliasFor != "" {
			property["x-cli-alias-for"] = option.AliasFor
		}
		if option.Overrides != "" {
			property["x-cli-overrides"] = option.Overrides
			if option.OverridesWhen != "" {
				property["x-cli-overrides-when"] = option.OverridesWhen
			}
		}
		properties[option.Name] = property
	}
	sort.Strings(required)
	schema := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func jsonType(value ValueType) string {
	switch value {
	case TypeInteger:
		return "integer"
	case TypeNumber:
		return "number"
	case TypeBoolean:
		return "boolean"
	default:
		return "string"
	}
}

func pathKey(path []string) string {
	return strings.Join(path, "\x00")
}

func displayPath(path []string) string {
	return strings.Join(path, " ")
}

func copyPaths(paths [][]string) [][]string {
	out := make([][]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, append([]string{}, path...))
	}
	return out
}

func validateValueType(value ValueType) error {
	switch value {
	case TypeString, TypeInteger, TypeNumber, TypeBoolean, TypeStringArray:
		return nil
	default:
		return fmt.Errorf("unsupported value type %q", value)
	}
}
