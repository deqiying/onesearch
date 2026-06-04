---
name: onesearch-cli
description: CLI-first web research and source retrieval through the local onesearch command. Use when an AI agent needs current web search, source-backed fact checking, URL fetching, site mapping, official/API/documentation search, or reproducible search evidence through Skill + CLI.
---

# Onesearch CLI

Use the local `onesearch` command as the execution layer for web research. The skill decides the workflow; the CLI performs provider routing, JSON/Markdown output, URL fetching, source discovery, site mapping, configuration diagnostics, and offline Deep Research planning.

## Default workflow

1. Run `onesearch doctor --format json` when configuration or provider availability is uncertain.
2. If `doctor` reports missing configuration, inspect `config.json`, enable the needed `providers.<id>`, and configure either `api_key` or the environment variable named by `api_key_env` when a key is required. Never put raw API keys in the final answer.
3. Use `onesearch search` for broad answer discovery. For real-time or fast-changing information, prefer adding `--extra-sources 2` or `--extra-sources 3` on the first pass so the agent can compare source candidates. Also add it when the task needs multi-source coverage, freshness checks, competing-source comparison, rankings/lists, or source triage before fetching evidence.
4. Use `onesearch exa-search` for official documentation, API references, papers, trusted product pages, and low-noise source discovery.
5. Use `onesearch context7-library` and `onesearch context7-docs` only for library, SDK, API, framework, or documentation intent.
6. Use `onesearch zhipu-search` for Chinese, domestic, current, or domain-filtered web source discovery.
7. Use `onesearch fetch` when the user gives a URL or a claim depends on page content.
8. Use `onesearch map` when documentation-site or domain structure matters.
9. Use `onesearch repo-wiki` for GitHub repository architecture questions, or `search --repo-wiki owner/repo` when broad search should include DeepWiki repository context.
10. Use `onesearch deep` for offline Deep Research planning before executing live evidence commands.
11. Preserve key command lines and source URLs in the final answer. Treat search results as discovery candidates until fetched.

For current-news, policy, finance, health, security, legal, product-pricing, or other high-risk facts, do not answer from the broad `answer_search` result alone. Prefer `--extra-sources 2` or `--extra-sources 3` on the first search pass, then read answer preview from `used.answer_search.providers.<provider>.result.content_preview` and source candidates from `used.<capability>.providers.<provider>.result.sources`; use `--format content` only when the full answer text is needed, then fetch key URLs and summarize only what the fetched text supports.

For volatile or source-divergent tasks, prefer `onesearch search "query" --validation balanced --extra-sources 2 --format json` as the first pass. Use the extra sources to identify candidate URLs, then fetch the key pages before presenting claim-level conclusions. Do not make this decision from fixed keyword rules; decide from the semantics of the user request and the evidence standard needed for the answer.

For rankings, lists, schedules, prices, leaderboards, and other structured current results, treat `answer_search` as synthesis only. Prefer list/table content from `source_search` results or fetched pages as the final basis.

## Runtime schema

Provider orchestration is controlled by the runtime config schema:

```json
{
  "schema_version": 1,
  "defaults": {},
  "pipelines": {},
  "routes": {},
  "profiles": {},
  "providers": {}
}
```

Use these names when reasoning about routing:

- `answer_search`: answer generation through xAI Responses, OpenAI-compatible Chat Completions, or OpenAI Responses.
- `source_search`: source discovery through Exa, Zhipu, Tavily, or Firecrawl.
- `docs_search`: documentation discovery through Exa or Context7.
- `page_fetch`: page-level evidence through Tavily or Firecrawl.
- `site_map`: site structure discovery through Tavily or Firecrawl.
- `site_crawl`: crawl workflow through Firecrawl.
- `repo_wiki`: repository wiki lookup through DeepWiki. Public repositories work anonymously; `DEEPWIKI_API_KEY` is optional and only needed for private documentation access.
- `vertical_search`: explicit experimental vertical search through AnySearch.

Default `source_search` order is `exa`, `zhipu`, `tavily`, `firecrawl`. AnySearch is not the default source route; use it only through explicit `anysearch-*` commands or when runtime routes explicitly opt into it.

## Deep Research Mode

Use Deep Research Mode when the user asks for `深度搜索`, `深度调研`, `深入搜索`, `deep search`, `deep research`, multi-source verification, cross-checking, serious review, or selection/comparison research.

Start with:

```powershell
onesearch deep "question" --budget standard --format json
```

`onesearch deep` is an offline planner. It does not call search, fetch, map, doctor, or providers. Read `intent_signals`, `decomposition`, `capability_plan`, `steps`, and `gap_check`, then execute the returned `steps[].command` with existing CLI commands.

Default evidence policy is `fetch_before_claim`: key claims in the final answer must be supported by fetched page text. Treat every `used.<capability>.providers.<provider>.result.sources` entry as a discovery candidate until the relevant URL has been fetched. Use `--verbose` only when you need internal fields such as `primary_sources`, `extra_sources`, provider attempts, or routing decisions.

Allowed Deep Research step tools are `search`, `exa-search`, `exa-similar`, `zhipu-search`, `context7-library`, `context7-docs`, `fetch`, and `map`. `doctor` is preflight and should not appear in `steps[]`.

## Command patterns

```powershell
onesearch search "query" --validation balanced --extra-sources 3 --format json
onesearch search "query" --validation strict --fallback auto --providers auto --format json
onesearch exa-search "query" --num-results 5 --include-highlights --format json
onesearch exa-search "OpenAI Responses API documentation" --include-domains platform.openai.com developers.openai.com --include-text --format json
onesearch exa-similar "https://example.com/article" --num-results 5 --format json
onesearch context7-library "react" "hooks" --format json
onesearch context7-docs "/facebook/react" "useEffect cleanup" --format json
onesearch zhipu-search "today China AI news" --count 5 --format json
onesearch fetch "https://example.com" --format markdown --output page.md
onesearch map "https://docs.example.com" --instructions "Find API reference pages" --max-depth 1 --max-breadth 20 --limit 50 --format json
onesearch crawl "https://docs.example.com" --max-depth 2 --limit 20 --format json
onesearch repo-wiki "microsoft/playwright" "MCP Browser Automation Server architecture" --format json
onesearch search "Explain Playwright MCP Browser Automation Server architecture" --repo-wiki microsoft/playwright --validation strict --format json
onesearch deep "research question" --budget standard --format json
onesearch doctor --format json
onesearch config list --format json
onesearch smoke --mock --format json
onesearch load_skill search
onesearch load_skill docs
onesearch load_skill fetch
onesearch load_skill deep-research
```

Short aliases are supported for interactive use:

```powershell
onesearch s "query" --format json
onesearch f "https://example.com" --format markdown
onesearch rw "microsoft/playwright" "architecture" --format json
onesearch exa "OpenAI Responses API documentation" --format json
onesearch z "today China AI news" --format json
onesearch c7 "react" "hooks" --format json
onesearch d --format markdown
onesearch sm --format json
```

## Guardrails

- Prefer JSON for agent parsing and Markdown for fetched page text intended for reading.
- Default `search --format json` emits `ok`, `query`, `used`, and `meta`. The `used` tree is indexed by capability and provider, for example `used.answer_search.providers.openai_responses.result.content_preview`; there is no default top-level `content`, `answer`, or flat `sources`, and full answer text is available through `--format content` or `--verbose`.
- Error output defaults to compact `--quiet`; retry with `--verbose` when diagnostics such as provider attempts or capability status are needed.
- Capability routing is driven by `providers.<id>.capabilities`; `routes` only controls preferred order and explicit fallback chains.
- `repo-wiki` accepts `owner/repo` or GitHub repository URLs. Bare repository names are rejected because the owner is ambiguous.
- `search --repo-wiki owner/repo` force-adds `repo_wiki` results under `used.repo_wiki`; DeepWiki content is repository context, not a fetched web source.
- OpenAI adapters honor `settings.stream`, `settings.tools`, and `settings.tool_choice`; blank setting values keep built-in defaults. `openai_responses` defaults to required `web_search`, while `openai_compatible` only passes tools when configured.
- Use `onesearch doctor` for compact diagnostic JSON containing errors and initialization status, not the full config file. Use `onesearch doctor --format content` when a short human-readable summary is needed.
- Use `onesearch config list --format json` when full runtime schema, routes, defaults, and provider definitions are needed.
- Use `--output` for multi-source work, long pages, or anything the answer may need to cite later.
- Keep `--extra-sources` small (`2` to `3`) for normal multi-source work; increase it only when the user asks for broad coverage, comparison research, or deeper source discovery.
- Do not cite search `result.sources` as proof for a claim; fetch the URL first or cite it only as a candidate source.
- Prefer `exa-search --include-domains` for official documentation when likely domains are known.
- Do not expose API keys. Treat `doctor` output as safe only because secrets are masked.
- If `doctor` or a command fails, report the failure and recovery steps; do not silently switch to another search route.

## Supporting reference

Read `references/cli-contract.md` when you need command details, output fields, exit codes, runtime schema expectations, or regression checks.
