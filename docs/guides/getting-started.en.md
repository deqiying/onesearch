# Getting started

[Documentation hub](../README.md) · [简体中文](getting-started.md)

These PowerShell examples target an installed CLI. Network operations require available providers; static command discovery does not need credentials.

## 1. Install and discover commands

```powershell
npm install -g onesearch
onesearch --version
onesearch schema search --format json
```

The npm launcher uses optional dependencies for Windows x64, Linux x64, and macOS arm64 binaries. Additional builds are available from [GitHub Releases](https://github.com/deqiying/onesearch/releases). `schema` runs offline without creating configuration.

## 2. Initialize and inspect readiness

```powershell
onesearch status --format json --pretty
```

Ordinary commands initialize missing runtime configuration. A fresh `ready: false` and `minimum_profile.ok: false` is expected, not an installation failure.

| Capability | Role | Provider choices / initial state |
| --- | --- | --- |
| `answer_search` | Required by `standard` | Configure `xai`, `openai-compatible`, or `openai-responses` |
| `docs_search` | Required by `standard` | Configure Exa or Context7 |
| `page_fetch` | Required by `standard` | Configure Exa, Tavily, Firecrawl, or DeepSeek; AnySearch is initially enabled |
| `repo_wiki` | Independent capability | DeepWiki is enabled; public repositories may work anonymously |
| `vertical_search` | Optional experimental capability | AnySearch is enabled |

Only DeepWiki and AnySearch are initially enabled. See [configuration](../reference/configuration.en.md) for credentials, enabled state, and routes.

## 3. Optional: research a public repository first

When `capabilities.repo_wiki.ok == true` or `direct_endpoints.deepwiki.available == true`, this can run before the complete `standard` profile is ready:

```powershell
onesearch repo-wiki "microsoft/playwright" `
  "How is the MCP browser server structured?" `
  --provider deepwiki --format json
```

DeepWiki supplies generated repository context, architecture summaries, wiki structure, and wiki contents. Treat it as secondary context and verify exact implementation claims against source. Private repositories may require credentials.

## 4. Configure full search

One answer provider plus Exa covers `answer_search`, `docs_search`, `page_fetch`, and optional `source_search`:

```powershell
onesearch config setup openai-compatible
onesearch config setup exa
onesearch status --format json --pretty
```

Interactive prompts hide API keys. Substitute the xAI or OpenAI Responses setup target if that matches your endpoint. Compatible gateways need an explicit API root:

```powershell
onesearch config setup openai-compatible `
  --base-url "https://gateway.example.com/v1"
```

Before execution, check:

- Full search: `minimum_profile.ok == true`.
- Capability commands: `capabilities.<capability>.ok == true`.
- Provider-direct commands: `direct_endpoints.<provider>.available == true`.

These are local preflight signals, not proof of remote reachability.

## 5. Run the first search

```powershell
onesearch search "Explain Go context cancellation patterns" `
  --validation balanced --format json
```

Add `--extra-sources 2` only when `capabilities.source_search.ok == true`. Discover candidate URLs, then fetch important pages before drawing conclusions.

## Next steps

- [CLI, provider catalog, and evidence strategy](../reference/cli.en.md)
- [Agent integration and built-in Skills](agent-integration.en.md)
- [Configuration and readiness diagnostics](../reference/configuration.en.md)

Implementation: [CLI entry](../../internal/cli/cli.go), [configuration initialization](../../internal/config/config.go), and [runtime capabilities and profiles](../../internal/config/runtime.go).
