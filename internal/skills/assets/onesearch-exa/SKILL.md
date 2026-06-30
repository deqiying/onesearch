---
name: onesearch-exa
description: Use when an AI agent needs Exa through Onesearch, especially when the user names Exa, semantic web discovery, low-noise source discovery, official docs discovery, product pages, papers, known-domain search, similar-page discovery, or clean page content fetch via the Onesearch CLI.
---

# Onesearch Exa

Use Exa for low-noise web discovery, official documentation, papers, product pages, known-domain searches, and Exa contents fetches.

## Bridge Contract

This skill is the source document for agent-facing Exa provider direct commands. When a task names Exa, semantic web discovery, best-page lookup, official docs discovery, papers, product pages, similar pages, or clean markdown/content fetch, route through Onesearch.

If command details may have changed, run:

```powershell
onesearch skills show exa --format content
```

Use the provider command family by default:

```powershell
onesearch exa web-search "query" --format json
onesearch exa web-fetch "https://example.com" --format json
onesearch exa similar "https://example.com" --format json
```

## Commands

| Purpose | Command |
| --- | --- |
| Web search | `onesearch exa web-search "query"` |
| Page fetch | `onesearch exa web-fetch "https://example.com"` |
| Similar pages | `onesearch exa similar "https://example.com"` |

## Usage

```powershell
onesearch exa web-search "OpenAI Responses API documentation" --max-results 5 --include-highlights --format json
onesearch exa web-search "vector database benchmark" --include-domains docs.example.com arxiv.org --format json
onesearch exa web-fetch "https://example.com/article" --max-characters 12000 --format json
onesearch exa web-fetch "https://example.com/article" --format content
onesearch exa similar "https://example.com/article" --num-results 5 --format json
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
