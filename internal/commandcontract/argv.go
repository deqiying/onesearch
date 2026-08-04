package commandcontract

import (
	"fmt"
	"math"
	"reflect"
	"strings"
)

// BuildArgv resolves id in the default registry and builds canonical argv
// tokens. It is intended for callers that use the complete repository
// definitions; use Registry.BuildArgv when working with a smaller registry in
// tests or an embedding application.
func BuildArgv(id string, values map[string]any) ([]string, error) {
	registry, err := DefaultRegistry()
	if err != nil {
		return nil, err
	}
	return registry.BuildArgv(id, values)
}

// BuildArgv builds `onesearch` followed by the command's canonical path. It
// emits only explicitly supplied options (defaults are metadata, not argv
// tokens), repeats array flags, and emits boolean flags only when true.
func (r *Registry) BuildArgv(id string, values map[string]any) ([]string, error) {
	command, ok := r.LookupID(id)
	if !ok {
		return nil, fmt.Errorf("unknown command id %q", id)
	}
	if values == nil {
		values = map[string]any{}
	}
	resolved := map[string]any{}
	known := map[string]bool{}
	for _, positional := range command.Positionals {
		known[positional.Name] = true
	}
	for _, option := range command.Options {
		known[option.Name] = true
		known[option.Flag] = true
	}
	for key, value := range values {
		if !known[key] {
			return nil, fmt.Errorf("unknown value %q for command %q", key, id)
		}
		optionName := key
		for _, option := range command.Options {
			if option.Name == key || option.Flag == key {
				optionName = option.Name
				break
			}
		}
		if optionName != key {
			if _, exists := resolved[optionName]; exists {
				return nil, fmt.Errorf("value supplied more than once for option %q", optionName)
			}
		}
		resolved[optionName] = value
	}

	if err := validateBuildConstraints(command, resolved); err != nil {
		return nil, err
	}
	for _, positional := range command.Positionals {
		value, present := resolved[positional.Name]
		if !present {
			if positional.Required || positional.MinItems > 0 {
				return nil, fmt.Errorf("missing required positional %q", positional.Name)
			}
			continue
		}
		if positional.Variadic {
			items, err := stringOrTypedValues(value, positional.Type)
			if err != nil {
				return nil, fmt.Errorf("positional %q: %w", positional.Name, err)
			}
			if positional.MinItems > 0 && len(items) < positional.MinItems {
				return nil, fmt.Errorf("positional %q requires at least %d items", positional.Name, positional.MinItems)
			}
			if positional.MaxItems > 0 && len(items) > positional.MaxItems {
				return nil, fmt.Errorf("positional %q accepts at most %d items", positional.Name, positional.MaxItems)
			}
			for _, item := range items {
				if err := validatePositionalToken(positional, item); err != nil {
					return nil, fmt.Errorf("positional %q: %w", positional.Name, err)
				}
			}
		} else if err := validatePositionalValue(positional, value); err != nil {
			return nil, fmt.Errorf("positional %q: %w", positional.Name, err)
		}
	}

	optionTokens := []string{}
	for _, option := range command.Options {
		value, present := resolved[option.Name]
		if !present {
			continue
		}
		if option.Type == TypeBoolean {
			boolean, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("option %q: want boolean, got %T", option.Name, value)
			}
			if boolean {
				optionTokens = append(optionTokens, "--"+option.Flag)
			}
			continue
		}
		if option.Type == TypeStringArray {
			items, err := stringValues(value)
			if err != nil {
				return nil, fmt.Errorf("option %q: %w", option.Name, err)
			}
			for _, item := range items {
				encoded, err := EncodeListValue(item)
				if err != nil {
					return nil, fmt.Errorf("option %q: %w", option.Name, err)
				}
				optionTokens = append(optionTokens, "--"+option.Flag, encoded)
			}
			continue
		}
		text, err := scalarString(option.Type, value)
		if err != nil {
			return nil, fmt.Errorf("option %q: %w", option.Name, err)
		}
		optionTokens = append(optionTokens, "--"+option.Flag, text)
	}
	positionals := []string{}
	for _, positional := range command.Positionals {
		value, present := resolved[positional.Name]
		if !present {
			continue
		}
		if positional.Variadic {
			items, _ := stringOrTypedValues(value, positional.Type)
			positionals = append(positionals, items...)
			continue
		}
		text, _ := scalarString(positional.Type, value)
		positionals = append(positionals, text)
	}
	argv := append([]string{"onesearch"}, command.Path...)
	if requiresEndOfOptions(positionals) {
		argv = append(argv, optionTokens...)
		argv = append(argv, "--")
		return append(argv, positionals...), nil
	}
	argv = append(argv, positionals...)
	argv = append(argv, optionTokens...)
	return argv, nil
}

func validateBuildConstraints(command CommandDefinition, values map[string]any) error {
	for _, constraint := range command.Constraints {
		members := []string{}
		for _, member := range constraint.Members {
			name := member
			for _, option := range command.Options {
				if option.Flag == member {
					name = option.Name
					break
				}
			}
			if value, ok := values[name]; ok && valueIsSet(value) {
				members = append(members, name)
			}
		}
		switch constraint.Kind {
		case "mutually_exclusive":
			if len(members) > 1 {
				return fmt.Errorf("options %s are mutually exclusive", strings.Join(members, ", "))
			}
		case "at_least_one":
			if len(members) == 0 {
				return fmt.Errorf("at least one of %s is required", strings.Join(constraint.Members, ", "))
			}
		case "requires":
			if len(members) > 0 && len(members) != len(constraint.Members) {
				return fmt.Errorf("options %s must be supplied together", strings.Join(constraint.Members, ", "))
			}
		}
	}
	for _, option := range command.Options {
		value, present := values[option.Name]
		if !present || !valueIsSet(value) {
			continue
		}
		if option.Overrides != "" && overrideApplies(option, value) {
			overridden := option.Overrides
			for _, candidate := range command.Options {
				if candidate.Flag == overridden {
					overridden = candidate.Name
					break
				}
			}
			delete(values, overridden)
		}
		if err := validateOptionValue(option, value); err != nil {
			return fmt.Errorf("option %q: %w", option.Name, err)
		}
		if len(option.Enum) > 0 && option.Type == TypeString {
			text := value.(string)
			if !contains(option.Enum, text) {
				return fmt.Errorf("option %q value %q is outside enum", option.Name, text)
			}
		}
		if option.Minimum != nil || option.Maximum != nil {
			number, _ := numericValue(value)
			if option.Minimum != nil && number < *option.Minimum {
				return fmt.Errorf("option %q is below minimum", option.Name)
			}
			if option.Maximum != nil && number > *option.Maximum {
				return fmt.Errorf("option %q is above maximum", option.Name)
			}
		}
	}
	return nil
}

func overrideApplies(option OptionDefinition, value any) bool {
	switch option.OverridesWhen {
	case "":
		return true
	case "positive":
		number, ok := numericValue(value)
		return ok && number > 0
	default:
		return false
	}
}

func valueIsSet(value any) bool {
	if value == nil {
		return false
	}
	if boolean, ok := value.(bool); ok {
		return boolean
	}
	if reflect.ValueOf(value).Kind() == reflect.Slice {
		return reflect.ValueOf(value).Len() > 0
	}
	return true
}

func validatePositionalValue(positional PositionalDefinition, value any) error {
	if err := validateOptionValue(OptionDefinition{Type: positional.Type}, value); err != nil {
		return err
	}
	return validatePositionalToken(positional, fmt.Sprint(value))
}

func validatePositionalToken(positional PositionalDefinition, value string) error {
	if positional.MinLength > 0 && len([]rune(value)) < positional.MinLength {
		return fmt.Errorf("requires at least %d characters", positional.MinLength)
	}
	return nil
}

func stringOrTypedValues(value any, valueType ValueType) ([]string, error) {
	if valueType != TypeString && valueType != TypeStringArray {
		return nil, fmt.Errorf("variadic values must use string-compatible type, got %s", valueType)
	}
	return stringValues(value)
}

func stringValues(value any) ([]string, error) {
	if value == nil {
		return nil, fmt.Errorf("want string array, got nil")
	}
	raw := reflect.ValueOf(value)
	if raw.Kind() != reflect.Slice && raw.Kind() != reflect.Array {
		return nil, fmt.Errorf("want string array, got %T", value)
	}
	out := make([]string, raw.Len())
	for i := 0; i < raw.Len(); i++ {
		item := raw.Index(i).Interface()
		text, err := scalarString(TypeString, item)
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i, err)
		}
		out[i] = text
	}
	return out, nil
}

func scalarString(valueType ValueType, value any) (string, error) {
	if err := validateOptionValue(OptionDefinition{Type: valueType}, value); err != nil {
		return "", err
	}
	switch valueType {
	case TypeString:
		return value.(string), nil
	case TypeInteger, TypeNumber:
		return fmt.Sprint(value), nil
	default:
		return "", fmt.Errorf("cannot encode %s as scalar", valueType)
	}
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

func isInteger(value any) bool {
	number, ok := numericValue(value)
	return ok && number == math.Trunc(number)
}

func isNumber(value any) bool {
	_, ok := numericValue(value)
	return ok
}

func requiresEndOfOptions(items []string) bool {
	for _, item := range items {
		if strings.HasPrefix(item, "-") && item != "-" {
			return true
		}
	}
	return false
}
