package commandcontract

func utilityDefinitions() []CommandDefinition {
	return []CommandDefinition{
		utilityCommand("doctor", []string{"doctor"}, [][]string{{"d"}}, "Diagnose configuration and capability readiness.", nil, nil, "doctor_result", []string{"config_read", "filesystem_write_when_output_is_set"}),
		utilityCommand("status", []string{"status"}, [][]string{{"st"}}, "Report runtime capability and provider status.", nil, nil, "status_result", []string{"config_read", "filesystem_write_when_output_is_set"}),
		{
			ID: "smoke", Path: []string{"smoke"}, Aliases: [][]string{{"sm"}}, Category: CategoryUtility, Visibility: VisibilityPublic,
			Summary: "Run mock or live smoke checks.",
			Options: withOutput(
				enumOption("mode", "mode", "mock", []string{"mock", "live"}, "Smoke mode."),
				optionDefault("mock", "mock", TypeBoolean, false, "Select mock mode."),
				optionDefault("live", "live", TypeBoolean, false, "Select live mode."),
			),
			Constraints:  normalConstraints(ConstraintDefinition{Kind: "mutually_exclusive", Members: []string{"mock", "live"}}),
			Availability: localAvailability(), SideEffects: runtimeSideEffects("filesystem_write_when_output_is_set", "network_when_live"), Output: normalOutput("smoke_result"),
		},
		utilityCommand("model.current", []string{"model", "current"}, [][]string{{"model", "cur"}, {"model", "c"}, {"mdl", "current"}, {"mdl", "cur"}, {"mdl", "c"}}, "Show the effective model configuration.", nil, nil, "model_result", []string{"config_read", "filesystem_write_when_output_is_set"}),
		utilityCommand("config.path", []string{"config", "path"}, [][]string{{"config", "p"}, {"cfg", "path"}, {"cfg", "p"}}, "Show the active configuration path.", nil, nil, "config_result", []string{"config_read", "filesystem_write_when_output_is_set"}),
		utilityCommand("config.list", []string{"config", "list"}, [][]string{{"config", "ls"}, {"config", "l"}, {"cfg", "list"}, {"cfg", "ls"}, {"cfg", "l"}}, "Show the redacted runtime configuration schema and provider status.", nil, nil, "config_result", []string{"config_read", "filesystem_write_when_output_is_set"}),
		{
			ID: "config.setup", Path: []string{"config", "setup"}, Aliases: [][]string{{"cfg", "setup"}}, Category: CategoryUtility, Visibility: VisibilityPublic,
			Summary:     "Configure one provider using hidden TTY or stdin input.",
			Positionals: []PositionalDefinition{positional("provider", "Provider ID or runtime alias.", true)},
			Options: withOutput(
				option("base_url", "base-url", TypeString, "Provider base URL; an explicitly empty value preserves the current or built-in default."),
				optionDefault("api_key_stdin", "api-key-stdin", TypeBoolean, false, "Read one API-key line from stdin."),
			),
			Constraints: normalConstraints(),
			InputChannels: []InputChannelDefinition{{
				Name: "api_key", Sensitive: true,
				Bindings:            []InputBindingDefinition{{Kind: "tty_hidden", When: "interactive_and_api_key_stdin_is_false"}, {Kind: "stdin_line", ActivatedBy: "api_key_stdin"}},
				RequiredWhenRuntime: "provider_requires_key_and_has_no_effective_key", RuntimeCheckCommand: []string{"config", "list"}, RuntimeCheckScope: "/providers/{provider}", ForbiddenBinding: "argv",
			}},
			Availability: localAvailability(), SideEffects: runtimeSideEffects("config_read", "config_write", "stdin_read"), Output: normalOutput("config_result"),
		},
		{
			ID: "skills.list", Path: []string{"skills", "list"}, Aliases: [][]string{{"skills", "ls"}, {"skills", "l"}, {"skill", "list"}, {"skill", "ls"}, {"skill", "l"}}, Category: CategoryUtility, Visibility: VisibilityPublic,
			Summary: "List bundled Onesearch skills.", Options: withOutput(
				optionDefault("capability", "capability", TypeString, "", "Filter skills by capability."),
			), Constraints: normalConstraints(), Availability: localAvailability(), SideEffects: []string{"filesystem_read", "filesystem_write_when_output_is_set"}, Output: normalOutput("skill_result"),
		},
		{
			ID: "skills.show", Path: []string{"skills", "show"}, Aliases: [][]string{{"skills", "get"}, {"skills", "read"}, {"skills", "load"}, {"skill", "show"}, {"skill", "get"}, {"skill", "read"}, {"skill", "load"}}, Category: CategoryUtility, Visibility: VisibilityPublic,
			Summary: "Show one file from a bundled skill.", Positionals: []PositionalDefinition{positional("name", "Skill ID or alias.", true)}, Options: withOutput(
				optionDefault("capability", "capability", TypeString, "", "Accepted for compatibility; it does not filter a single skill."),
				optionDefault("file", "file", TypeString, "SKILL.md", "Relative bundled file path to read."),
			), Constraints: normalConstraints(), Availability: localAvailability(), SideEffects: []string{"filesystem_read", "filesystem_write_when_output_is_set"}, Output: normalOutput("skill_result"),
		},
		{
			ID: "regression", Path: []string{"regression"}, Aliases: [][]string{{"reg"}}, Category: CategoryUtility, Visibility: VisibilityPublic,
			Summary: "Run the fixed mock regression suite.", Options: []OptionDefinition{
				{Name: "format", Flag: "format", Type: TypeString, Default: "json", HasDefault: true, Enum: []string{"json"}, Description: "Regression output is always JSON."},
				prettyOption(),
			}, Availability: localAvailability(), SideEffects: runtimeSideEffects(), Output: OutputDefinition{DefaultFormat: "json", Formats: []string{"json"}, Variants: []string{"quiet"}, Contract: "smoke_result"},
		},
		{
			ID: "schema", Path: []string{"schema"}, Category: CategoryUtility, Visibility: VisibilityPublic,
			Summary:     "Return the versioned CLI command manifest for agents and tooling.",
			Positionals: []PositionalDefinition{variadicPositional("command_path", "Optional canonical command path to select.", 0)},
			Options: []OptionDefinition{
				{Name: "format", Flag: "format", Type: TypeString, Default: "json", HasDefault: true, Enum: []string{"json"}, Description: "Manifest format; V1 supports JSON only."},
				prettyOption(),
				optionDefault("output", "output", TypeString, "", "Also write the exact manifest bytes to this path."),
			},
			Availability: localAvailability(), SideEffects: []string{"filesystem_write_when_output_is_set"}, Output: OutputDefinition{DefaultFormat: "json", Formats: []string{"json"}, Contract: "command_manifest"},
		},
	}
}

func utilityCommand(id string, path []string, aliases [][]string, summary string, positionals []PositionalDefinition, options []OptionDefinition, contract string, sideEffects []string) CommandDefinition {
	return CommandDefinition{
		ID: id, Path: path, Aliases: aliases, Category: CategoryUtility, Visibility: VisibilityPublic, Summary: summary,
		Positionals: positionals, Options: withOutput(options...), Constraints: normalConstraints(), Availability: localAvailability(), SideEffects: runtimeSideEffects(sideEffects...), Output: normalOutput(contract),
	}
}

func namespaceDefinitions() []NamespaceDefinition {
	out := providerNamespaces()
	out = append(out,
		NamespaceDefinition{Path: []string{"model"}, Aliases: [][]string{{"mdl"}}, Category: CategoryUtility, Visibility: VisibilityPublic, Summary: "Model configuration commands."},
		NamespaceDefinition{Path: []string{"config"}, Aliases: [][]string{{"cfg"}}, Category: CategoryUtility, Visibility: VisibilityPublic, Summary: "Configuration commands."},
		NamespaceDefinition{Path: []string{"skills"}, Aliases: [][]string{{"skill"}}, Category: CategoryUtility, Visibility: VisibilityPublic, Summary: "Bundled skill discovery commands.", DefaultCommandID: "skills.list"},
	)
	return out
}
