package commandcontract

import (
	"fmt"
	"strings"
	"unicode"
)

const ListEncoding = "backslash_escaped_csv_whitespace"

// EncodeListValue protects legacy comma and whitespace separators so one
// structured array item remains one item when parsed from argv.
func EncodeListValue(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("list values must be non-empty")
	}
	if value == "--" {
		return `\--`, nil
	}
	var encoded strings.Builder
	for _, char := range value {
		if char == '\\' || char == ',' || unicode.IsSpace(char) {
			encoded.WriteRune('\\')
		}
		encoded.WriteRune(char)
	}
	return encoded.String(), nil
}

// DecodeListValues preserves the existing comma/whitespace-separated form and
// decodes backslash-protected separators emitted by EncodeListValue.
func DecodeListValues(value string) []string {
	runes := []rune(value)
	items := []string{}
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		items = append(items, current.String())
		current.Reset()
	}
	for index := 0; index < len(runes); index++ {
		char := runes[index]
		if char == '\\' && index+1 < len(runes) {
			next := runes[index+1]
			if next == '\\' || next == ',' || next == '-' || unicode.IsSpace(next) {
				current.WriteRune(next)
				index++
				continue
			}
		}
		if char == ',' || unicode.IsSpace(char) {
			flush()
			continue
		}
		current.WriteRune(char)
	}
	flush()
	return items
}
