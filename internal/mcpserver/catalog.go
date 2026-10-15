package mcpserver

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/deqiying/onesearch/internal/commandcontract"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var registry = commandcontract.MustDefaultRegistry()

var answerFields = strings.Fields("platform model extra_sources fetch_sources validation fallback providers answer_providers source_providers docs_providers fetch_providers repo_providers repo_wiki repo_wiki_mode repo_wiki_query")
var sourceFields = strings.Fields("num_results search_type include_text include_highlights start_published_date category include_domains exclude_domains")

type toolDefinition struct {
	tool  *mcp.Tool
	input *jsonschema.Resolved
}

func object(properties map[string]any, required ...string) map[string]any {
	out := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func textSchema() map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "maxLength": 16384}
}
func enumSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}
func timeoutSchema() map[string]any {
	return map[string]any{"type": "number", "exclusiveMinimum": 0, "maximum": 300, "description": "Overall timeout in seconds; includes all upstream work."}
}

// 仅投影明确批准的业务字段；新增 CLI 参数不会自动扩大 MCP 的能力。
func projected(id string, names []string) map[string]any {
	d, ok := registry.LookupID(id)
	if !ok {
		panic("missing MCP command binding: " + id)
	}
	properties := d.Manifest().InputSchema["properties"].(map[string]any)
	out := map[string]any{}
	for _, name := range names {
		p, ok := properties[name].(map[string]any)
		if !ok {
			panic("missing MCP parameter: " + name)
		}
		for key := range p {
			if strings.HasPrefix(key, "x-cli-") {
				delete(p, key)
			}
		}
		if p["type"] == "string" {
			p["maxLength"] = 16384
		}
		if p["type"] == "array" {
			p["maxItems"] = 100
		}
		out[name] = p
	}
	return out
}

func bounded(p map[string]any, name string, min, max int) {
	field := p[name].(map[string]any)
	field["minimum"], field["maximum"] = min, max
}

func buildCatalog() ([]toolDefinition, error) {
	answer := projected("search", answerFields)
	bounded(answer, "extra_sources", 0, 10)
	bounded(answer, "fetch_sources", 0, 10)
	sources := projected("exa.web-search", sourceFields)
	bounded(sources, "num_results", 1, 20)
	docs := object(map[string]any{"library_name": textSchema(), "library_id": map[string]any{"type": "string", "maxLength": 1024, "pattern": `^/[^\s/?#]+/[^\s?#]+$`}})
	docs["oneOf"] = []any{map[string]any{"required": []string{"library_name"}}, map[string]any{"required": []string{"library_id"}}}
	search := object(map[string]any{
		"mode": enumSchema("answer", "sources", "docs"), "query": textSchema(), "timeout": timeoutSchema(),
		"answer_options": object(answer), "sources_options": object(sources), "docs_options": docs,
	}, "mode", "query")
	var branches []any
	for _, mode := range []string{"answer", "sources", "docs"} {
		props := map[string]any{"mode": map[string]any{"const": mode}}
		for _, other := range []string{"answer", "sources", "docs"} {
			if other != mode {
				props[other+"_options"] = false
			}
		}
		branch := map[string]any{"properties": props}
		if mode == "docs" {
			branch["required"] = []string{"docs_options"}
		}
		branches = append(branches, branch)
	}
	search["oneOf"] = branches

	fetchProps := projected("fetch", []string{"url", "provider"})
	fetchProps["timeout"] = timeoutSchema()
	fetch := object(fetchProps, "url")
	siteProps := projected("map", []string{"url", "provider", "max_depth", "limit"})
	for _, name := range []string{"max_depth", "limit"} {
		delete(siteProps[name].(map[string]any), "default")
	}
	bounded(siteProps, "max_depth", 1, 3)
	bounded(siteProps, "limit", 1, 200)
	siteProps["action"] = enumSchema("map", "crawl")
	siteProps["timeout"] = timeoutSchema()
	siteProps["timeout"].(map[string]any)["type"] = "integer"
	mapProps := projected("map", []string{"instructions", "max_breadth"})
	bounded(mapProps, "max_breadth", 1, 50)
	siteProps["map_options"] = object(mapProps)
	site := object(siteProps, "action", "url")
	site["oneOf"] = []any{
		map[string]any{"properties": map[string]any{"action": map[string]any{"const": "map"}}},
		map[string]any{"properties": map[string]any{"action": map[string]any{"const": "crawl"}, "map_options": false, "limit": map[string]any{"maximum": 50}}},
	}
	repoProps := projected("repo-wiki", []string{"repo", "question", "provider"})
	repoProps["mode"], repoProps["timeout"] = enumSchema("ask", "structure", "contents"), timeoutSchema()
	repoProps["question"] = textSchema()
	repo := object(repoProps, "repo", "mode")
	repo["oneOf"] = []any{
		map[string]any{"properties": map[string]any{"mode": map[string]any{"const": "ask"}}, "required": []string{"question"}},
		map[string]any{"properties": map[string]any{"mode": enumSchema("structure", "contents"), "question": false}},
	}
	open, closed, nonDestructive := true, false, false
	tools := []*mcp.Tool{
		{Name: "onesearch_search", Description: "Search by explicit mode: answer generates an answer using configured routes (minimum profile required); sources discovers URLs with Exa; docs uses Context7. For docs supply library_name to resolve candidates ONLY, then call again with a selected library_id to retrieve documentation. Never guess an ID. Only the selected mode's options are allowed. Candidates are not verified evidence; fetch important URLs. Calls may incur provider fees.", InputSchema: search, Annotations: &mcp.ToolAnnotations{OpenWorldHint: &open}},
		{Name: "onesearch_fetch", Description: "Fetch full text from one known HTTP(S) URL using configured page-fetch routes. Use for evidence after source discovery; does not crawl a website. May incur provider fees.", InputSchema: fetch, Annotations: &mcp.ToolAnnotations{OpenWorldHint: &open}},
		{Name: "onesearch_site", Description: "Operate on a website: action=map returns links; action=crawl retrieves multiple pages or submits an upstream crawl job. A submitted job is NOT completed page evidence. Choose action explicitly; map_options are forbidden for crawl. May start local provider processes or paid upstream jobs.", InputSchema: site, Annotations: &mcp.ToolAnnotations{OpenWorldHint: &open}},
		{Name: "onesearch_repo_wiki", Description: "Read a repository wiki: mode=ask requires question; structure and contents forbid question. Use owner/repo or a supported GitHub repository URL. Uses configured repository routes.", InputSchema: repo, Annotations: &mcp.ToolAnnotations{OpenWorldHint: &open}},
		{Name: "onesearch_status", Description: "Inspect local configuration and per-operation readiness without network calls or creating configuration. available means any operation is ready; inspect operations before choosing search mode or site action. Does not prove network, credential validity or quota.", InputSchema: object(map[string]any{}), Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &nonDestructive, OpenWorldHint: &closed, IdempotentHint: true}},
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	out := make([]toolDefinition, 0, len(tools))
	for _, tool := range tools {
		tool.OutputSchema = resultSchema()
		input, err := resolveSchema(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("MCP schema %s: %w", tool.Name, err)
		}
		out = append(out, toolDefinition{tool: tool, input: input})
	}
	return out, nil
}

func resultSchema() map[string]any {
	return object(map[string]any{
		"ok": map[string]any{"type": "boolean"}, "tool": map[string]any{"type": "string"},
		"operation": enumSchema("answer", "sources", "docs.resolve", "docs.query", "fetch", "map", "crawl", "ask", "structure", "contents", "status"),
		"data":      map[string]any{"type": "object"}, "error_type": map[string]any{"type": "string"}, "error": map[string]any{"type": "string"},
	}, "ok", "tool", "data")
}

func resolveSchema(value any) (*jsonschema.Resolved, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(body, &schema); err != nil {
		return nil, err
	}
	return schema.Resolve(nil)
}
