package cli

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/deqiying/onesearch/internal/commandcontract"
)

type parsedCommand struct {
	Definition commandcontract.CommandDefinition
	Values     map[string]any
	Present    map[string]bool
}

func parseCommand(definition commandcontract.CommandDefinition, args []string) (*parsedCommand, error) {
	parsed := &parsedCommand{
		Definition: definition,
		Values:     map[string]any{},
		Present:    map[string]bool{},
	}
	flags := make(map[string]commandcontract.OptionDefinition, len(definition.Options))
	for _, option := range definition.Options {
		flags[option.Flag] = option
		if option.HasDefault {
			parsed.Values[option.Name] = cloneParsedValue(option.Default)
		}
	}

	positionals := []string{}
	for index := 0; index < len(args); index++ {
		token := args[index]
		if token == "--" {
			positionals = append(positionals, args[index+1:]...)
			break
		}
		if !looksLikeFlagToken(token) {
			positionals = append(positionals, token)
			continue
		}

		name, inlineValue, hasInlineValue := splitFlagToken(token)
		option, ok := flags[name]
		if !ok {
			if definition.ID == "config.setup" && name == "api-key" {
				return parsed, fmt.Errorf("--api-key is not supported; use hidden input or --api-key-stdin")
			}
			return parsed, fmt.Errorf("unknown flag --%s", name)
		}
		parsed.Present[option.Name] = true

		if option.Type == commandcontract.TypeBoolean {
			value := true
			if hasInlineValue {
				parsedValue, err := strconv.ParseBool(inlineValue)
				if err != nil {
					if isSensitiveInputActivator(definition, option.Name) {
						return parsed, fmt.Errorf("invalid value for --%s: expected boolean", option.Flag)
					}
					return parsed, fmt.Errorf("invalid value %q for --%s: expected boolean", inlineValue, option.Flag)
				}
				value = parsedValue
			}
			parsed.Values[option.Name] = value
			continue
		}

		values := []string{}
		if hasInlineValue {
			values = append(values, inlineValue)
		} else {
			if index+1 >= len(args) || args[index+1] == "--" {
				return parsed, fmt.Errorf("flag --%s requires a value", option.Flag)
			}
			index++
			values = append(values, args[index])
		}
		if option.Type == commandcontract.TypeStringArray && option.Greedy && !hasInlineValue {
			for index+1 < len(args) && args[index+1] != "--" && !looksLikeFlagToken(args[index+1]) {
				index++
				values = append(values, args[index])
			}
		}

		if option.Type == commandcontract.TypeStringArray {
			items := parsed.Strings(option.Name)
			for _, value := range values {
				items = append(items, commandcontract.DecodeListValues(value)...)
			}
			parsed.Values[option.Name] = items
			continue
		}
		value, err := parseScalarOption(option, values[0])
		if err != nil {
			return parsed, err
		}
		parsed.Values[option.Name] = value
	}

	if err := bindPositionals(parsed, positionals); err != nil {
		return parsed, err
	}
	if err := validateParsedOptions(parsed); err != nil {
		return parsed, err
	}
	if err := validateParsedConstraints(parsed); err != nil {
		return parsed, err
	}
	return parsed, nil
}

func isSensitiveInputActivator(definition commandcontract.CommandDefinition, optionName string) bool {
	for _, channel := range definition.InputChannels {
		if !channel.Sensitive {
			continue
		}
		for _, binding := range channel.Bindings {
			if binding.ActivatedBy == optionName {
				return true
			}
		}
	}
	return false
}

func bindPositionals(parsed *parsedCommand, values []string) error {
	definition := parsed.Definition
	valueIndex := 0
	for _, positional := range definition.Positionals {
		if positional.Variadic {
			items := append([]string{}, values[valueIndex:]...)
			if len(items) < positional.MinItems {
				return fmt.Errorf("%s requires at least %d positional value(s) for %s", strings.Join(definition.Path, " "), positional.MinItems, positional.Name)
			}
			if positional.MaxItems > 0 && len(items) > positional.MaxItems {
				return fmt.Errorf("%s accepts at most %d positional value(s) for %s", strings.Join(definition.Path, " "), positional.MaxItems, positional.Name)
			}
			for _, item := range items {
				if positional.MinLength > 0 && len([]rune(item)) < positional.MinLength {
					return fmt.Errorf("%s requires non-empty %s values", strings.Join(definition.Path, " "), positional.Name)
				}
			}
			if len(items) > 0 {
				parsed.Values[positional.Name] = items
				parsed.Present[positional.Name] = true
			}
			valueIndex = len(values)
			continue
		}
		if valueIndex >= len(values) {
			if positional.Required {
				return fmt.Errorf("%s requires %s", strings.Join(definition.Path, " "), positional.Name)
			}
			continue
		}
		value := values[valueIndex]
		if positional.MinLength > 0 && len([]rune(value)) < positional.MinLength {
			return fmt.Errorf("%s requires non-empty %s", strings.Join(definition.Path, " "), positional.Name)
		}
		parsed.Values[positional.Name] = value
		parsed.Present[positional.Name] = true
		valueIndex++
	}
	if valueIndex < len(values) {
		return fmt.Errorf("%s accepts at most %d positional argument(s)", strings.Join(definition.Path, " "), len(definition.Positionals))
	}
	return nil
}

func validateParsedOptions(parsed *parsedCommand) error {
	for _, option := range parsed.Definition.Options {
		value, present := parsed.Values[option.Name]
		if !present {
			continue
		}
		if len(option.Enum) > 0 {
			text, ok := value.(string)
			if !ok || !containsString(option.Enum, text) {
				return fmt.Errorf("invalid value %q for --%s: expected one of %s", fmt.Sprint(value), option.Flag, strings.Join(option.Enum, ", "))
			}
		}
		if option.Minimum != nil || option.Maximum != nil {
			number, ok := parsedNumber(value)
			if !ok {
				return fmt.Errorf("invalid numeric value for --%s", option.Flag)
			}
			if option.Minimum != nil && number < *option.Minimum {
				return fmt.Errorf("--%s must be at least %g", option.Flag, *option.Minimum)
			}
			if option.Maximum != nil && number > *option.Maximum {
				return fmt.Errorf("--%s must be at most %g", option.Flag, *option.Maximum)
			}
		}
	}
	return nil
}

func validateParsedConstraints(parsed *parsedCommand) error {
	for _, constraint := range parsed.Definition.Constraints {
		set := []string{}
		for _, member := range constraint.Members {
			if parsed.ValueIsSet(member) {
				set = append(set, member)
			}
		}
		switch constraint.Kind {
		case "mutually_exclusive":
			if len(set) > 1 {
				return fmt.Errorf("%s are mutually exclusive", strings.Join(prefixedMembers(set), " and "))
			}
		case "at_least_one":
			if len(set) == 0 {
				return fmt.Errorf("at least one of %s is required", strings.Join(constraint.Members, ", "))
			}
		case "requires":
			if len(set) > 0 && len(set) != len(constraint.Members) {
				return fmt.Errorf("%s must be supplied together", strings.Join(constraint.Members, ", "))
			}
		}
	}
	return nil
}

func parseScalarOption(option commandcontract.OptionDefinition, raw string) (any, error) {
	switch option.Type {
	case commandcontract.TypeString:
		return raw, nil
	case commandcontract.TypeInteger:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value > math.MaxInt || value < math.MinInt {
			return nil, fmt.Errorf("invalid value %q for --%s: expected integer", raw, option.Flag)
		}
		return int(value), nil
	case commandcontract.TypeNumber:
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q for --%s: expected number", raw, option.Flag)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported value type %q for --%s", option.Type, option.Flag)
	}
}

func (p *parsedCommand) String(name string) string {
	value, _ := p.Values[name].(string)
	return value
}

func (p *parsedCommand) Int(name string) int {
	value, _ := p.Values[name].(int)
	return value
}

func (p *parsedCommand) Number(name string) float64 {
	value, _ := p.Values[name].(float64)
	return value
}

func (p *parsedCommand) Bool(name string) bool {
	value, _ := p.Values[name].(bool)
	return value
}

func (p *parsedCommand) Strings(name string) []string {
	value, _ := p.Values[name].([]string)
	return append([]string{}, value...)
}

func (p *parsedCommand) IsSet(name string) bool {
	return p.Present[name]
}

func (p *parsedCommand) ValueIsSet(name string) bool {
	if !p.Present[name] {
		return false
	}
	value := p.Values[name]
	switch typed := value.(type) {
	case bool:
		return typed
	case []string:
		return len(typed) > 0
	default:
		return value != nil
	}
}

func looksLikeFlagToken(value string) bool {
	return strings.HasPrefix(value, "-") && value != "-"
}

func splitFlagToken(value string) (string, string, bool) {
	name := strings.TrimPrefix(value, "-")
	if strings.HasPrefix(value, "--") {
		name = strings.TrimPrefix(value, "--")
	}
	if index := strings.IndexByte(name, '='); index >= 0 {
		return name[:index], name[index+1:], true
	}
	return name, "", false
}

func cloneParsedValue(value any) any {
	if values, ok := value.([]string); ok {
		return append([]string{}, values...)
	}
	return value
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func parsedNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

func prefixedMembers(values []string) []string {
	out := make([]string, len(values))
	for index, value := range values {
		out[index] = "--" + strings.ReplaceAll(value, "_", "-")
	}
	return out
}
