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
	{ID: "onesearch-cli", Folder: "onesearch-cli", Aliases: []string{"base", "onesearch", "cli", "router"}, Capabilities: []string{"routing"}, Description: "Onesearch CLI capabilities for web search, evidence fetch, docs lookup, site crawl, repo wiki, deep research planning, and provider-direct MCP-compatible commands."},
	{ID: "exa", Folder: "onesearch-exa", Aliases: []string{"exa-tools", "web_search_exa", "web_fetch_exa"}, Capabilities: []string{"source_search", "docs_search", "page_fetch"}, Description: "Exa provider commands, original MCP aliases, and usage guidance."},
	{ID: "tavily", Folder: "onesearch-tavily", Aliases: []string{"tavily-tools", "tavily_search", "tavily_extract", "tavily_map", "tavily_crawl"}, Capabilities: []string{"source_search", "page_fetch", "site_map", "site_crawl"}, Description: "Tavily provider commands, original MCP aliases, and usage guidance."},
	{ID: "firecrawl", Folder: "onesearch-firecrawl", Aliases: []string{"firecrawl-tools", "firecrawl_search", "firecrawl_scrape", "firecrawl_map", "firecrawl_crawl"}, Capabilities: []string{"source_search", "page_fetch", "site_map", "site_crawl"}, Description: "Firecrawl provider commands, original MCP aliases, and usage guidance."},
	{ID: "context7", Folder: "onesearch-context7", Aliases: []string{"context7-tools", "ctx7", "resolve_library_id", "query_docs"}, Capabilities: []string{"docs_search"}, Description: "Context7 provider commands, original MCP aliases, and usage guidance."},
	{ID: "deepwiki", Folder: "onesearch-deepwiki", Aliases: []string{"deepwiki-tools", "ask_question", "read_wiki_structure", "read_wiki_contents"}, Capabilities: []string{"repo_wiki"}, Description: "DeepWiki provider commands, original MCP aliases, and usage guidance."},
	{ID: "anysearch", Folder: "onesearch-anysearch", Aliases: []string{"anysearch-tools", "as"}, Capabilities: []string{"vertical_search", "page_fetch"}, Description: "AnySearch provider commands and usage guidance."},
	{ID: "zhipu", Folder: "onesearch-zhipu", Aliases: []string{"zhipu-tools", "zhipu-search", "zp"}, Capabilities: []string{"source_search"}, Description: "Zhipu search command and usage guidance."},
	{ID: "search", Folder: "onesearch-search", Aliases: []string{"web-search", "source-search"}, Capabilities: []string{"answer_search", "source_search"}, Description: "Search and source discovery workflow."},
	{ID: "docs", Folder: "onesearch-docs", Aliases: []string{"api-docs", "documentation"}, Capabilities: []string{"docs_search"}, Description: "API, SDK, library, and framework documentation workflow."},
	{ID: "fetch", Folder: "onesearch-fetch", Aliases: []string{"page-fetch", "evidence"}, Capabilities: []string{"page_fetch", "site_map"}, Description: "URL fetch, evidence extraction, and site map workflow."},
	{ID: "mcp-tools", Folder: "onesearch-mcp-tools", Aliases: []string{"mcp", "mcp-compat", "mcp-tool-compat", "provider-tools"}, Capabilities: []string{"source_search", "docs_search", "page_fetch", "site_map", "site_crawl", "repo_wiki"}, Description: "MCP original tool-name compatibility workflow."},
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
