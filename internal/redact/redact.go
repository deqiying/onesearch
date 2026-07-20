package redact

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

const Mask = "********"

var safeMetadataFields = map[string]struct{}{
	"api_key_env":           {},
	"api_key_src":           {},
	"api_key_set":           {},
	"api_key_env_set":       {},
	"has_api_key":           {},
	"effective_environment": {},
}

// IsSensitiveName reports whether a field or environment variable name is
// expected to contain a credential. Safe credential metadata is excluded.
func IsSensitiveName(name string) bool {
	normalized := normalizeName(name)
	if _, ok := safeMetadataFields[normalized]; ok {
		return false
	}
	compact := strings.ReplaceAll(normalized, "_", "")
	for _, value := range []string{
		"apikey", "authorization", "proxyauthorization", "token",
		"accesstoken", "refreshtoken", "idtoken", "secret", "clientsecret",
		"password", "passwd", "privatekey",
	} {
		if compact == value || strings.HasSuffix(compact, value) {
			return true
		}
	}
	return false
}

// IsSensitiveEnvironmentName broadens field classification for environment
// variable conventions such as KEY, SECRET_KEY, AWS_ACCESS_KEY_ID, and
// credential-bearing names while avoiding generic mode/transport settings.
func IsSensitiveEnvironmentName(name string) bool {
	if IsSensitiveName(name) {
		return true
	}
	normalized := normalizeName(name)
	compact := strings.ReplaceAll(normalized, "_", "")
	return normalized == "key" || strings.HasSuffix(normalized, "_key") ||
		strings.Contains(compact, "accesskey") || strings.Contains(compact, "secretkey") ||
		strings.Contains(compact, "credential")
}

// NormalizeSecrets trims, removes empty and duplicate values, and sorts longer
// values first so overlapping credentials are replaced deterministically.
func NormalizeSecrets(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		if value != "" {
			seen[value] = struct{}{}
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			seen[trimmed] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) == len(out[j]) {
			return out[i] < out[j]
		}
		return len(out[i]) > len(out[j])
	})
	return out
}

// Text replaces known credential values literally. It intentionally does not
// use regular expressions because credentials often contain metacharacters.
func Text(value string, secrets []string) string {
	for _, secret := range NormalizeSecrets(secrets) {
		value = strings.ReplaceAll(value, secret, Mask)
		if encoded, err := json.Marshal(secret); err == nil && len(encoded) >= 2 {
			escaped := string(encoded[1 : len(encoded)-1])
			if escaped != secret {
				value = strings.ReplaceAll(value, escaped, Mask)
			}
		}
	}
	return value
}

// Data returns a deep-copied value with sensitive fields masked and known
// credentials removed from all strings. Common concrete map/slice types are
// preserved so existing output formatters can keep their type assertions.
func Data(value any, secrets []string) any {
	safe := sanitizeValue(reflect.ValueOf(value), "", NormalizeSecrets(secrets))
	if !safe.IsValid() {
		return nil
	}
	return safe.Interface()
}

// CollectSensitiveValues finds credential-like values in JSON-shaped data.
// For an env map, only values whose variable names look sensitive are added;
// masking every generic env value here could corrupt unrelated rendered text.
func CollectSensitiveValues(value any) []string {
	var values []string
	collectValue(reflect.ValueOf(value), "", &values)
	return NormalizeSecrets(values)
}

// CollectAPIKeyEnvironmentNames returns variable names declared by api_key_env
// fields, including those in a malformed or otherwise unrecognized schema.
func CollectAPIKeyEnvironmentNames(value any) []string {
	names := []string{}
	collectAPIKeyEnvironmentNames(reflect.ValueOf(value), &names)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.ToUpper(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func sanitizeValue(value reflect.Value, parentKey string, secrets []string) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		child := sanitizeValue(value.Elem(), parentKey, secrets)
		out := reflect.New(value.Type()).Elem()
		out.Set(child)
		return out
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			key := iter.Key()
			mapValue := iter.Value()
			name := ""
			if key.Kind() == reflect.String {
				name = key.String()
			}
			var safeValue reflect.Value
			switch {
			case normalizeName(parentKey) == "env":
				safeValue = maskValue(mapValue)
			case IsSensitiveName(name):
				safeValue = maskValue(mapValue)
			default:
				safeValue = sanitizeValue(mapValue, name, secrets)
			}
			out.SetMapIndex(key, safeValue)
		}
		return out
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			out.Index(i).Set(sanitizeValue(value.Index(i), parentKey, secrets))
		}
		return out
	case reflect.Array:
		out := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			out.Index(i).Set(sanitizeValue(value.Index(i), parentKey, secrets))
		}
		return out
	case reflect.String:
		out := reflect.New(value.Type()).Elem()
		out.SetString(Text(value.String(), secrets))
		return out
	default:
		return value
	}
}

func maskValue(value reflect.Value) reflect.Value {
	if value.Kind() == reflect.Interface {
		out := reflect.New(value.Type()).Elem()
		mask := reflect.ValueOf(Mask)
		if mask.Type().AssignableTo(value.Type()) {
			out.Set(mask)
		}
		return out
	}
	if value.Kind() == reflect.String {
		out := reflect.New(value.Type()).Elem()
		out.SetString(Mask)
		return out
	}
	return reflect.Zero(value.Type())
}

func collectValue(value reflect.Value, parentKey string, values *[]string) {
	if !value.IsValid() {
		return
	}
	if value.Kind() == reflect.Interface {
		if !value.IsNil() {
			collectValue(value.Elem(), parentKey, values)
		}
		return
	}
	switch value.Kind() {
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			key := iter.Key()
			mapValue := iter.Value()
			name := ""
			if key.Kind() == reflect.String {
				name = key.String()
			}
			if IsSensitiveName(name) || (normalizeName(parentKey) == "env" && IsSensitiveEnvironmentName(name)) {
				appendStrings(mapValue, values)
				continue
			}
			collectValue(mapValue, name, values)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			collectValue(value.Index(i), parentKey, values)
		}
	}
}

func appendStrings(value reflect.Value, values *[]string) {
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return
		}
		appendStrings(value.Elem(), values)
		return
	}
	if value.Kind() == reflect.String {
		*values = append(*values, value.String())
		return
	}
	if value.IsValid() && value.CanInterface() {
		*values = append(*values, fmt.Sprint(value.Interface()))
	}
}

func collectAPIKeyEnvironmentNames(value reflect.Value, names *[]string) {
	if !value.IsValid() {
		return
	}
	if value.Kind() == reflect.Interface {
		if !value.IsNil() {
			collectAPIKeyEnvironmentNames(value.Elem(), names)
		}
		return
	}
	switch value.Kind() {
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			key := iter.Key()
			mapValue := iter.Value()
			if key.Kind() == reflect.String && normalizeName(key.String()) == "api_key_env" {
				appendStrings(mapValue, names)
				continue
			}
			collectAPIKeyEnvironmentNames(mapValue, names)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			collectAPIKeyEnvironmentNames(value.Index(i), names)
		}
	}
}

func normalizeName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var out strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			out.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(out.String(), "_")
}
