package mcpserver

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/deqiying/onesearch/internal/commandcontract"
)

type invocation struct {
	id, operation string
	input         commandcontract.Input
	timeout       time.Duration
}

// operationFor 仅返回已知常量，错误结果不能回显任意 selector。
func operationFor(tool string, args map[string]any) string {
	switch tool {
	case "onesearch_search":
		switch args["mode"] {
		case "answer":
			return "answer"
		case "sources":
			return "sources"
		case "docs":
			docs, _ := args["docs_options"].(map[string]any)
			if _, ok := docs["library_name"]; ok {
				return "docs.resolve"
			}
			if _, ok := docs["library_id"]; ok {
				return "docs.query"
			}
		}
	case "onesearch_site":
		if args["action"] == "map" {
			return "map"
		}
		if args["action"] == "crawl" {
			return "crawl"
		}
	case "onesearch_repo_wiki":
		switch args["mode"] {
		case "ask":
			return "ask"
		case "structure":
			return "structure"
		case "contents":
			return "contents"
		}
	case "onesearch_fetch":
		return "fetch"
	case "onesearch_status":
		return "status"
	}
	return ""
}

func bind(tool string, args map[string]any) (invocation, error) {
	in := invocation{operation: operationFor(tool, args), timeout: 90 * time.Second}
	values := map[string]any{}
	copyFields := func(from map[string]any, names ...string) {
		for _, name := range names {
			if v, ok := from[name]; ok {
				values[name] = v
			}
		}
	}
	copyOptions := func(name string) {
		opts, _ := args[name].(map[string]any)
		for k, v := range opts {
			values[k] = v
		}
	}
	switch tool {
	case "onesearch_status":
		return in, nil
	case "onesearch_search":
		copyFields(args, "query")
		switch in.operation {
		case "answer":
			in.id = "search"
			copyOptions("answer_options")
		case "sources":
			in.id = "exa.web-search"
			copyOptions("sources_options")
		case "docs.resolve", "docs.query":
			opts, _ := args["docs_options"].(map[string]any)
			if in.operation == "docs.resolve" {
				in.id = "context7.resolve-library-id"
				values["name"] = opts["library_name"]
			} else {
				in.id = "context7.query-docs"
				values["library_id"] = opts["library_id"]
			}
		}
	case "onesearch_fetch":
		in.id = "fetch"
		copyFields(args, "url", "provider")
	case "onesearch_site":
		in.id = in.operation
		copyFields(args, "url", "provider", "max_depth", "limit")
		if in.operation == "map" {
			copyOptions("map_options")
		}
	case "onesearch_repo_wiki":
		in.id = "repo-wiki"
		copyFields(args, "repo", "question", "mode", "provider")
	}
	definition, ok := registry.LookupID(in.id)
	if !ok {
		return in, fmt.Errorf("invalid operation")
	}
	// 总超时只有部分 CLI 叶子命令具备；其余在 MCP 层统一包围整个请求。
	if timeout, exists := args["timeout"]; exists {
		n := timeout.(float64)
		in.timeout = time.Duration(n * float64(time.Second))
		if in.id == "search" || in.id == "map" || in.id == "crawl" || in.id == "repo-wiki" {
			values["timeout"] = n
		}
	}
	for _, key := range []string{"query", "name", "library_id", "repo", "question", "url"} {
		if value, exists := values[key]; exists && strings.TrimSpace(value.(string)) == "" {
			return in, fmt.Errorf("%s must not be blank", key)
		}
	}
	if value, ok := values["url"].(string); ok {
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
			return in, fmt.Errorf("url must be HTTP(S) without user information")
		}
	}
	input, err := commandcontract.NormalizeInput(definition, values)
	if err != nil {
		return in, fmt.Errorf("invalid operation parameters")
	}
	in.input = input
	if _, exists := args["timeout"]; !exists {
		switch n := input.Values["timeout"].(type) {
		case int:
			in.timeout = time.Duration(n) * time.Second
		case float64:
			in.timeout = time.Duration(n * float64(time.Second))
		}
	}
	return in, nil
}
