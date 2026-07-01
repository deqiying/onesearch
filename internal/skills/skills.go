package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed assets/**
var assets embed.FS

type Definition struct {
	ID           string
	Folder       string
	Aliases      []string
	Capabilities []string
	Description  string
}

var definitions = []Definition{
	{ID: "onesearch-cli", Folder: "onesearch-cli", Aliases: []string{"base", "onesearch", "cli", "router"}, Capabilities: []string{"routing"}, Description: "Use for web search, current/latest public information, online claim checks, URL reading, website map/crawl, official docs lookup, public GitHub repo docs, and research planning."},
	{ID: "exa", Folder: "onesearch-exa", Aliases: []string{"exa-tools", "exa-web", "exa-fetch", "exa-similar-pages"}, Capabilities: []string{"source_search", "docs_search", "page_fetch"}, Description: "Status-gated Exa provider-direct guidance through Onesearch for web search, web fetch, similar-page discovery, official docs, papers, and product pages."},
	{ID: "tavily", Folder: "onesearch-tavily", Aliases: []string{"tavily-tools", "tavily-search", "tavily-extract", "tavily-map", "tavily-crawl"}, Capabilities: []string{"source_search", "page_fetch", "site_map", "site_crawl"}, Description: "Status-gated Tavily provider-direct guidance through Onesearch for current search, extraction, site maps, and crawls."},
	{ID: "firecrawl", Folder: "onesearch-firecrawl", Aliases: []string{"firecrawl-tools", "firecrawl-search", "firecrawl-scrape", "firecrawl-map", "firecrawl-crawl"}, Capabilities: []string{"source_search", "page_fetch", "site_map", "site_crawl"}, Description: "Status-gated Firecrawl provider-direct guidance through Onesearch for robust scraping, mapping, search, and crawl jobs."},
	{ID: "context7", Folder: "onesearch-context7", Aliases: []string{"context7-tools", "ctx7", "context7-provider", "context7-library-docs"}, Capabilities: []string{"docs_search"}, Description: "Status-gated Context7 provider-direct guidance through Onesearch for library resolution, current API docs, SDK docs, package docs, setup, and migration guidance."},
	{ID: "deepwiki", Folder: "onesearch-deepwiki", Aliases: []string{"deepwiki-tools", "repo-wiki", "repository-wiki"}, Capabilities: []string{"repo_wiki"}, Description: "Status-gated DeepWiki provider-direct guidance through Onesearch for GitHub repo questions, wiki structure, wiki contents, and architecture context."},
	{ID: "anysearch", Folder: "onesearch-anysearch", Aliases: []string{"anysearch-tools", "as"}, Capabilities: []string{"vertical_search", "page_fetch"}, Description: "Status-gated AnySearch provider commands and usage guidance."},
	{ID: "zhipu", Folder: "onesearch-zhipu", Aliases: []string{"zhipu-tools", "zhipu-web-search", "zp"}, Capabilities: []string{"source_search"}, Description: "Status-gated Zhipu provider-direct guidance for Chinese current search, hot searches, and source discovery."},
	{ID: "ddg", Folder: "onesearch-ddg", Aliases: []string{"ddg-search", "duckduckgo", "duckduckgo-mcp"}, Capabilities: []string{"source_search", "page_fetch"}, Description: "Status-gated DuckDuckGo MCP stdio provider-direct guidance through Onesearch for local DuckDuckGo search and page content fetching."},
	{ID: "freecrawl", Folder: "onesearch-freecrawl", Aliases: []string{"freecrawl-mcp"}, Capabilities: []string{"source_search", "page_fetch", "site_crawl"}, Description: "Status-gated Freecrawl MCP stdio provider-direct guidance through Onesearch for local search, scraping, crawling, and deep research."},
	{ID: "search", Folder: "onesearch-search", Aliases: []string{"web-search", "source-search"}, Capabilities: []string{"answer_search", "source_search"}, Description: "Status-aware current search, hot/trending list, and source discovery workflow."},
	{ID: "docs", Folder: "onesearch-docs", Aliases: []string{"api-docs", "documentation"}, Capabilities: []string{"docs_search"}, Description: "Status-aware documentation workflow through Onesearch for API, SDK, library, framework, Context7, and Exa official docs discovery."},
	{ID: "fetch", Folder: "onesearch-fetch", Aliases: []string{"page-fetch", "evidence"}, Capabilities: []string{"page_fetch", "site_map"}, Description: "Status-aware URL fetch, evidence extraction, and site map workflow."},
	{ID: "deep-research", Folder: "onesearch-deep-research", Aliases: []string{"deep", "research"}, Capabilities: []string{"deep_planner", "research"}, Description: "Offline Deep Research planning and execution workflow."},
}

type File struct {
	Path string
	Data []byte
}

func ReadMarkdown(name string) (string, error) {
	def, ok := resolve(name)
	if !ok {
		return "", fmt.Errorf("Unknown skill: %s. Available skills: %s", name, strings.Join(Names(), ", "))
	}
	data, err := assets.ReadFile("assets/" + def.Folder + "/SKILL.md")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func LoadFiles(name string) ([]File, error) {
	def, ok := resolve(name)
	if !ok {
		return nil, fmt.Errorf("Unknown skill: %s. Available skills: %s", name, strings.Join(Names(), ", "))
	}
	root := "assets/" + def.Folder
	var files []File
	err := fs.WalkDir(assets, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, root+"/")
		files = append(files, File{Path: rel, Data: normalizeNewlines(data)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func Definitions() []Definition {
	out := make([]Definition, 0, len(definitions))
	for _, def := range definitions {
		out = append(out, cloneDefinition(def))
	}
	return out
}

func Describe(name string) (Definition, error) {
	def, ok := resolve(name)
	if !ok {
		return Definition{}, fmt.Errorf("Unknown skill: %s. Available skills: %s", name, strings.Join(Names(), ", "))
	}
	return cloneDefinition(def), nil
}

func Names() []string {
	out := make([]string, 0, len(definitions))
	for _, def := range definitions {
		out = append(out, def.ID)
	}
	return out
}

func cloneDefinition(def Definition) Definition {
	def.Aliases = append([]string{}, def.Aliases...)
	def.Capabilities = append([]string{}, def.Capabilities...)
	return def
}

func resolve(name string) (Definition, bool) {
	normalized := normalize(name)
	for _, def := range definitions {
		if normalize(def.ID) == normalized || normalize(def.Folder) == normalized {
			return def, true
		}
		for _, alias := range def.Aliases {
			if normalize(alias) == normalized {
				return def, true
			}
		}
	}
	return Definition{}, false
}

func normalize(value string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "_", "-")
}

func normalizeNewlines(data []byte) []byte {
	return []byte(strings.ReplaceAll(string(data), "\r\n", "\n"))
}
