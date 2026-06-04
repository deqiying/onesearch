package sources

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	urlPattern            = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `，。、；：！？》）】\)]+`)
	mdLinkPattern         = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)]+)\)`)
	inlineCitationPattern = regexp.MustCompile(`\[\[(\d+)\]\]\((https?://[^)]+)\)`)
	sourcesCallPattern    = regexp.MustCompile(`(?is)(?:^|\n)\s*(sources|source|citations|citation|references|reference|citation_card|source_cards|source_card)\s*\((.*)\)\s*$`)
	sourcesHeadingPattern = regexp.MustCompile(`(?im)^ *(?:#{1,6} *)?(?:\*\*|__)? *(sources?|references?|citations?|信源|参考资料|参考|引用|来源列表|来源) *(?:\*\*|__)? *(?:[（(][^)\n]*[)）])? *[:：]? *$`)
	thinkBlockPattern     = regexp.MustCompile(`(?is)<think>.*?</think>`)
)

func NewSessionID() string {
	return randomHex12()
}

func ExtractUniqueURLs(text string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, match := range urlPattern.FindAllString(text, -1) {
		url := strings.TrimRight(match, ".,;:!?")
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, url)
	}
	return out
}

func Merge(lists ...[]map[string]any) []map[string]any {
	seen := map[string]struct{}{}
	var out []map[string]any
	for _, list := range lists {
		for _, item := range list {
			raw, _ := item["url"].(string)
			url := strings.TrimSpace(raw)
			if url == "" {
				continue
			}
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func SplitAnswerAndSources(text string) (string, []map[string]any) {
	raw := strings.TrimSpace(thinkBlockPattern.ReplaceAllString(text, ""))
	if raw == "" {
		return "", nil
	}
	inline := extractInlineCitationSources(raw)
	links := extractBodyLinkSources(raw)
	if answer, parsed, ok := splitFunctionSources(raw); ok {
		return strings.TrimSpace(answer), Merge(parsed, inline, links)
	}
	if answer, parsed, ok := splitHeadingSources(raw); ok {
		return strings.TrimSpace(answer), Merge(parsed, inline, links)
	}
	if answer, parsed, ok := splitTailLinkBlock(raw); ok {
		return strings.TrimSpace(answer), Merge(parsed, inline, links)
	}
	return raw, Merge(inline, links)
}

func splitFunctionSources(text string) (string, []map[string]any, bool) {
	match := sourcesCallPattern.FindStringSubmatchIndex(text)
	if match == nil || len(match) < 6 {
		return "", nil, false
	}
	payload := strings.TrimSpace(text[match[4]:match[5]])
	parsed := parseSourcesPayload(payload)
	if len(parsed) == 0 {
		return "", nil, false
	}
	return text[:match[0]], parsed, true
}

func splitHeadingSources(text string) (string, []map[string]any, bool) {
	matches := sourcesHeadingPattern.FindAllStringIndex(text, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		idx := matches[i][0]
		block := text[idx:]
		parsed := extractSourcesFromText(block)
		if len(parsed) > 0 {
			return text[:idx], parsed, true
		}
	}
	return "", nil, false
}

func splitTailLinkBlock(text string) (string, []map[string]any, bool) {
	lines := strings.Split(text, "\n")
	end := len(lines) - 1
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if end < 0 {
		return "", nil, false
	}
	start := end
	count := 0
	for start >= 0 {
		line := strings.TrimSpace(lines[start])
		if line == "" {
			start--
			continue
		}
		if !isLinkOnlyLine(line) {
			break
		}
		count++
		start--
	}
	if count < 2 {
		return "", nil, false
	}
	block := strings.Join(lines[start+1:end+1], "\n")
	parsed := extractSourcesFromText(block)
	if len(parsed) == 0 {
		return "", nil, false
	}
	return strings.Join(lines[:start+1], "\n"), parsed, true
}

func isLinkOnlyLine(line string) bool {
	line = strings.TrimSpace(regexp.MustCompile(`^\s*(?:[-*]|\d+\.)\s*`).ReplaceAllString(line, ""))
	return strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") || mdLinkPattern.MatchString(line)
}

func parseSourcesPayload(payload string) []map[string]any {
	payload = strings.TrimRight(strings.TrimSpace(payload), ";")
	if payload == "" {
		return nil
	}
	var data any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return extractSourcesFromText(payload)
	}
	return normalizeSources(data)
}

func normalizeSources(data any) []map[string]any {
	var items []any
	switch value := data.(type) {
	case []any:
		items = value
	case map[string]any:
		for _, key := range []string{"sources", "citations", "references", "urls"} {
			if nested, ok := value[key]; ok {
				return normalizeSources(nested)
			}
		}
		items = []any{value}
	case string:
		items = []any{value}
	default:
		return nil
	}
	seen := map[string]struct{}{}
	var out []map[string]any
	for _, item := range items {
		switch value := item.(type) {
		case string:
			for _, url := range ExtractUniqueURLs(value) {
				if _, ok := seen[url]; ok {
					continue
				}
				seen[url] = struct{}{}
				out = append(out, map[string]any{"url": url})
			}
		case map[string]any:
			url := firstString(value, "url", "href", "link")
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				continue
			}
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			source := map[string]any{"url": url}
			if title := firstString(value, "title", "name", "label"); strings.TrimSpace(title) != "" {
				source["title"] = strings.TrimSpace(title)
			}
			if desc := firstString(value, "description", "snippet", "content"); strings.TrimSpace(desc) != "" {
				source["description"] = strings.TrimSpace(desc)
			}
			out = append(out, source)
		}
	}
	return out
}

func extractSourcesFromText(text string) []map[string]any {
	seen := map[string]struct{}{}
	var out []map[string]any
	for _, match := range mdLinkPattern.FindAllStringSubmatch(text, -1) {
		title := strings.TrimSpace(match[1])
		url := strings.TrimSpace(match[2])
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		item := map[string]any{"url": url}
		if title != "" {
			item["title"] = title
		}
		out = append(out, item)
	}
	for _, url := range ExtractUniqueURLs(text) {
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, map[string]any{"url": url})
	}
	return out
}

func extractInlineCitationSources(text string) []map[string]any {
	seen := map[string]struct{}{}
	var out []map[string]any
	for _, match := range inlineCitationPattern.FindAllStringSubmatch(text, -1) {
		url := strings.TrimSpace(match[2])
		if url == "" {
			continue
		}
		if _, ok := seen[url]; ok {
			continue
		}
		seen[url] = struct{}{}
		out = append(out, map[string]any{"title": match[1], "url": url})
	}
	return out
}

func extractBodyLinkSources(text string) []map[string]any {
	return extractSourcesFromText(stripMarkdownCode(text))
}

func stripMarkdownCode(text string) string {
	var out strings.Builder
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			out.WriteByte('\n')
			continue
		}
		if inFence {
			out.WriteByte('\n')
			continue
		}
		out.WriteString(stripInlineCode(line))
		out.WriteByte('\n')
	}
	return out.String()
}

func stripInlineCode(line string) string {
	var out strings.Builder
	inCode := false
	for _, r := range line {
		if r == '`' {
			inCode = !inCode
			out.WriteRune(' ')
			continue
		}
		if inCode {
			out.WriteRune(' ')
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func firstString(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok {
			return value
		}
	}
	return ""
}
