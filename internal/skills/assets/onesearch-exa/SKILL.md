---
name: onesearch-exa
description: Use when an AI agent needs Exa through Onesearch for web_search_exa, web_fetch_exa, low-noise source discovery, official docs discovery, product pages, papers, or page content fetch via Exa contents.
---

# Onesearch Exa

Use Exa for low-noise web discovery, official documentation, papers, product pages, known-domain searches, and Exa contents fetches.

## Commands

| Purpose | Preferred command | MCP-compatible alias |
| --- | --- | --- |
| Web search | `onesearch exa web-search "query"` | `onesearch exa web_search_exa "query"` |
| Page fetch | `onesearch exa web-fetch "https://example.com"` | `onesearch exa web_fetch_exa "https://example.com"` |
| Similar pages | `onesearch exa-similar "https://example.com"` | Legacy flat command only |
| Legacy search | `onesearch exa-search "query"` | Legacy flat command |

Global MCP migration aliases:

```powershell
onesearch mcp web_search_exa "query" --format json
onesearch mcp web_fetch_exa "https://example.com" --format json
```

## Usage

```powershell
onesearch exa web-search "OpenAI Responses API documentation" --max-results 5 --include-highlights --format json
onesearch exa web-search "vector database benchmark" --include-domains docs.example.com arxiv.org --format json
onesearch exa web-fetch "https://example.com/article" --max-characters 12000 --format json
onesearch exa web-fetch "https://example.com/article" --format content
onesearch exa-similar "https://example.com/article" --num-results 5 --format json
```

## Options

- `--max-results` or `--num-results`: number of search results.
- `--search-type`: Exa search type, default `neural`.
- `--include-text`: include Exa text snippets in search results.
- `--include-highlights`: include highlights in search results.
- `--include-domains`: restrict search to domains; accepts comma-separated or space-separated values.
- `--exclude-domains`: exclude domains; accepts comma-separated or space-separated values.
- `--start-published-date`: lower bound for published date.
- `--category`: Exa category when needed.
- `--max-characters`: maximum fetched text length for `web-fetch`.

## Output

Provider-direct JSON includes:

- `provider: "exa"`
- `tool: "web_search_exa"` or `tool: "web_fetch_exa"`
- `results`, `total`, `elapsed_ms`
- `content_preview` and `content_length` in quiet JSON when fetched content is long

Use `--verbose` for full provider fields. Use `--format content` for fetched page body only.

## Guardrails

- Use `onesearch search` instead of Exa direct commands when answer synthesis or multi-provider routing is needed.
- Fetch key URLs before claim-level conclusions; search results are discovery candidates.
- Run `onesearch doctor --format json` when Exa returns `config_error`.
