package execution

import (
	"fmt"
	"strings"

	"github.com/deqiying/onesearch/internal/config"
)

func parseSearchProviderFilters(raw string) (string, map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "auto") {
		return "auto", nil, nil
	}
	if !strings.Contains(raw, "=") && !strings.Contains(raw, ":") {
		return raw, nil, nil
	}
	filters := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		index := strings.Index(part, "=")
		if index < 0 {
			index = strings.Index(part, ":")
		}
		if index < 0 {
			return "", nil, fmt.Errorf("provider filter %q must use capability=providers", part)
		}
		capability := normalizeCapabilityFilterKey(strings.TrimSpace(part[:index]))
		value := strings.TrimSpace(part[index+1:])
		if capability == "" || value == "" {
			return "", nil, fmt.Errorf("provider filter %q must include capability and provider list", part)
		}
		filters[capability] = value
	}
	if len(filters) == 0 {
		return "auto", nil, nil
	}
	return "auto", filters, nil
}

func overlayProviderFilter(filters map[string]string, capability, value string) map[string]string {
	value = strings.TrimSpace(value)
	if value == "" {
		return filters
	}
	if filters == nil {
		filters = map[string]string{}
	}
	filters[config.V2CapabilityName(capability)] = value
	return filters
}

func normalizeCapabilityFilterKey(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "answer", "answer_search", "search":
		return "answer_search"
	case "source", "sources", "source_search":
		return "source_search"
	case "docs", "doc", "documentation", "docs_search":
		return "docs_search"
	case "fetch", "page", "page_fetch":
		return "page_fetch"
	case "repo", "repo_wiki", "repository", "repository_wiki":
		return "repo_wiki"
	case "site_map", "map":
		return "site_map"
	case "site_crawl", "crawl":
		return "site_crawl"
	case "vertical", "vertical_search":
		return "vertical_search"
	default:
		return config.V2CapabilityName(value)
	}
}
