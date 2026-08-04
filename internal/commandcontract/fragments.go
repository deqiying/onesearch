package commandcontract

func positional(name, description string, required bool) PositionalDefinition {
	return PositionalDefinition{Name: name, Type: TypeString, Description: description, Required: required, MinLength: boolInt(required)}
}

func variadicPositional(name, description string, minItems int) PositionalDefinition {
	return PositionalDefinition{Name: name, Type: TypeString, Description: description, Required: minItems > 0, Variadic: true, MinLength: 1, MinItems: minItems}
}

func option(name, flag string, valueType ValueType, description string) OptionDefinition {
	return OptionDefinition{Name: name, Flag: flag, Type: valueType, Description: description}
}

func optionDefault(name, flag string, valueType ValueType, defaultValue any, description string) OptionDefinition {
	return OptionDefinition{Name: name, Flag: flag, Type: valueType, Default: defaultValue, HasDefault: true, Description: description}
}

func listOption(name, flag, description string) OptionDefinition {
	return OptionDefinition{Name: name, Flag: flag, Type: TypeStringArray, Description: description, Repeatable: true, Greedy: true}
}

func enumOption(name, flag string, defaultValue any, values []string, description string) OptionDefinition {
	return OptionDefinition{Name: name, Flag: flag, Type: TypeString, Default: defaultValue, HasDefault: true, Enum: values, Description: description}
}

func outputOptions() []OptionDefinition {
	return []OptionDefinition{
		{Name: "format", Flag: "format", Type: TypeString, Default: "json", HasDefault: true, Enum: []string{"json", "markdown", "content"}, Description: "Output format."},
		optionDefault("output", "output", TypeString, "", "Also write the rendered output to this path."),
		optionDefault("verbose", "verbose", TypeBoolean, false, "Include verbose diagnostics."),
		optionDefault("quiet", "quiet", TypeBoolean, false, "Return the compact result variant."),
	}
}

func withOutput(options ...OptionDefinition) []OptionDefinition {
	out := append([]OptionDefinition{}, options...)
	return append(out, outputOptions()...)
}

func normalConstraints(extra ...ConstraintDefinition) []ConstraintDefinition {
	out := []ConstraintDefinition{{Kind: "mutually_exclusive", Members: []string{"quiet", "verbose"}}}
	return append(out, extra...)
}

func normalOutput(contract string) OutputDefinition {
	return OutputDefinition{
		DefaultFormat: "json",
		Formats:       []string{"json", "markdown", "content"},
		Variants:      []string{"quiet", "verbose"},
		Contract:      contract,
	}
}

func providerOutput() OutputDefinition {
	out := normalOutput("provider_result")
	out.ProviderPayload = "opaque"
	return out
}

func localAvailability() AvailabilityDefinition {
	return AvailabilityDefinition{Dynamic: false, PreflightLevel: "static"}
}

func capabilityAvailability(capability string) AvailabilityDefinition {
	return AvailabilityDefinition{
		Dynamic:        true,
		CheckCommand:   []string{"status"},
		JSONPointer:    "/capabilities/" + capability + "/ok",
		PreflightLevel: "local_configuration",
		DoesNotProve:   []string{"credential_validity", "network_reachability"},
	}
}

func providerAvailability(provider string) AvailabilityDefinition {
	return AvailabilityDefinition{
		Dynamic:        true,
		CheckCommand:   []string{"status"},
		JSONPointer:    "/direct_endpoints/" + provider + "/available",
		PreflightLevel: "local_configuration",
		DoesNotProve:   []string{"credential_validity", "network_reachability", "remote_tool_presence"},
	}
}

func providerCommand(id, provider string, path []string, summary string, capabilities []string, positionals []PositionalDefinition, options []OptionDefinition) CommandDefinition {
	return CommandDefinition{
		ID:           id,
		Path:         path,
		Category:     CategoryProvider,
		Visibility:   VisibilityPublic,
		Summary:      summary,
		Capabilities: capabilities,
		Provider:     provider,
		Positionals:  positionals,
		Options:      withOutput(options...),
		Constraints:  normalConstraints(),
		Availability: providerAvailability(provider),
		SideEffects:  runtimeSideEffects("filesystem_write_when_output_is_set", "network"),
		Output:       providerOutput(),
	}
}

func runtimeSideEffects(values ...string) []string {
	return append([]string{"config_initialize_when_missing"}, values...)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
