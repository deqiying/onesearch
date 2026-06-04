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
	{ID: "onesearch-cli", Folder: "onesearch-cli", Aliases: []string{"base", "onesearch", "cli"}, Capabilities: []string{"all"}, Description: "Full Onesearch CLI orchestration skill."},
	{ID: "search", Folder: "onesearch-search", Aliases: []string{"web-search", "source-search"}, Capabilities: []string{"answer_search", "source_search"}, Description: "Search and source discovery workflow."},
	{ID: "docs", Folder: "onesearch-docs", Aliases: []string{"api-docs", "documentation"}, Capabilities: []string{"docs_search"}, Description: "API, SDK, library, and framework documentation workflow."},
	{ID: "fetch", Folder: "onesearch-fetch", Aliases: []string{"page-fetch", "evidence"}, Capabilities: []string{"page_fetch", "site_map"}, Description: "URL fetch, evidence extraction, and site map workflow."},
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

func Names() []string {
	out := make([]string, 0, len(definitions))
	for _, def := range definitions {
		out = append(out, def.ID)
	}
	return out
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
