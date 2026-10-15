package commandcontract

import (
	"fmt"
	"math"
)

// Input 保留显式参数信息，避免把省略参数与显式 false 混为一谈。
type Input struct {
	Values  map[string]any
	Present map[string]bool
}

// NormalizeInput 校验结构化叶子参数并应用合同默认值，不构造或执行 argv。
func NormalizeInput(d CommandDefinition, raw map[string]any) (Input, error) {
	out := Input{Values: map[string]any{}, Present: map[string]bool{}}
	known := map[string]ValueType{}
	for _, p := range d.Positionals {
		if p.Variadic {
			known[p.Name] = TypeStringArray
		} else {
			known[p.Name] = p.Type
		}
	}
	for _, o := range d.Options {
		known[o.Name] = o.Type
	}
	for key, value := range raw {
		typ, ok := known[key]
		if !ok {
			return out, fmt.Errorf("unknown parameter for %s", d.ID)
		}
		if err := validateOptionValue(OptionDefinition{Type: typ}, value); err != nil {
			return out, fmt.Errorf("invalid type for %s", key)
		}
		switch typ {
		case TypeInteger, TypeNumber:
			n, _ := numericValue(value)
			if math.IsNaN(n) || math.IsInf(n, 0) || (typ == TypeInteger && (n >= float64(math.MaxInt) || n < float64(math.MinInt))) {
				return out, fmt.Errorf("invalid number for %s", key)
			}
			if typ == TypeInteger {
				value = int(n)
			} else {
				value = n
			}
		case TypeStringArray:
			items, err := stringValues(value)
			if err != nil {
				return out, fmt.Errorf("invalid array for %s", key)
			}
			value = items
		}
		out.Values[key], out.Present[key] = value, true
	}
	for _, p := range d.Positionals {
		v, exists := out.Values[p.Name]
		if !exists {
			if p.Required {
				return out, fmt.Errorf("missing required parameter %s", p.Name)
			}
			continue
		}
		if p.Variadic {
			items := v.([]string)
			if len(items) < p.MinItems || (p.MaxItems > 0 && len(items) > p.MaxItems) {
				return out, fmt.Errorf("invalid length for %s", p.Name)
			}
			for _, item := range items {
				if err := validatePositionalToken(p, item); err != nil {
					return out, fmt.Errorf("invalid value for %s", p.Name)
				}
			}
		} else if err := validatePositionalValue(p, v); err != nil {
			return out, fmt.Errorf("invalid value for %s", p.Name)
		}
	}
	if err := validateBuildConstraints(d, out.Values); err != nil {
		return out, err
	}
	for _, o := range d.Options {
		if _, exists := out.Values[o.Name]; !exists && o.HasDefault {
			value := o.Default
			if a, ok := value.([]string); ok {
				value = append([]string{}, a...)
			}
			out.Values[o.Name] = value
		}
	}
	return out, nil
}
