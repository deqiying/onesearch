# Configuration reference

[Documentation hub](../README.md) · [简体中文](configuration.md)

This page describes runtime configuration, not the CLI command manifest returned by `onesearch schema`.

## Location and initialization

Windows, macOS, and Linux default to `~/.config/onesearch/config.json`. Override the directory with `ONESEARCH_CONFIG_DIR`. Ordinary commands create missing configuration and continue; `schema`, command-level `--help`, and `skills list/show` return before runtime loading without initializing configuration.

Only `deepwiki` and `anysearch` are initially enabled. xAI, both OpenAI adapters, Exa, Context7, Zhipu, Tavily, Firecrawl, DDG, and Freecrawl start disabled. Configure providers before normal `search` use.

## Schema and routing

There are five configuration areas alongside the version field:

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

| Area | Responsibility |
| --- | --- |
| `defaults` | Pipeline, fallback, validation, minimum profile, timeout, logging, retry, and output cleaning |
| `pipelines` | Capability chains such as `default`, `research`, `docs`, and `crawl` |
| `routes` | Provider order per capability; providers declaring that capability are appended unless `settings.direct_only=true` |
| `profiles` | Minimum capability sets; `standard` requires `answer_search`, `docs_search`, and `page_fetch` |
| `providers` | Adapter, capabilities, base URL, credential source, enabled state, and provider-specific settings |

See [config.example.json](../../config.example.json) for the portable example. Use `config list` for the merged local configuration; the example does not supply enabled credentials.

## Configure providers safely

```powershell
onesearch config path --format json
onesearch config list --format json
onesearch config setup exa
onesearch doctor --format json
onesearch status --format json
```

Interactive setup hides API keys. Scripts and pipelines should pass secrets through stdin; there is intentionally no `--api-key` flag that would leave secrets in shell history:

```powershell
$env:EXA_API_KEY |
  onesearch config setup exa --api-key-stdin --format json

$env:OPENAI_COMPATIBLE_API_KEY |
  onesearch config setup openai-compatible `
    --api-key-stdin `
    --base-url "https://gateway.example.com/v1" `
    --format json
```

`config setup` updates the key and sets the provider to `enabled: "auto"`. Edit `config.json` for models, capabilities, routes, settings, or disabling providers. A direct `api_key` takes precedence over `api_key_env`.

## Adapter limits

- `openai_responses` calls the JSON-only standalone SearchRequest route `/v1/alpha/search`. This is this project's adapter choice, not a claim that the public Responses API migrated wholesale.
- `openai_chat_completions` calls `/v1/chat/completions`, supports JSON/SSE, and can pass through `tools` and `tool_choice`.
- The adapters do not downgrade into each other; Alpha failures enter normal cross-provider fallback.
- `openai_responses` requires an API root base URL. Full endpoints ending in `/alpha/search` or `/responses` are rejected.
- Context7 requests require an API key.
- DDG and Freecrawl use the local `mcp_stdio` bridge and are disabled/direct-only by default. Enable and explicitly route them to use them in workflows. Pin Freecrawl deployments and rely on runtime tool discovery.

## Diagnose readiness

1. Use `config path` to verify the active directory and environment override.
2. Use `config list` and `doctor` to inspect enabled state, credential source, adapters, capabilities, and routes.
3. Check `minimum_profile.ok`, `capabilities.<capability>.ok`, or `direct_endpoints.<provider>.available` in `status`, according to the target command.
4. Local readiness does not prove remote reachability. Inspect errors and `--verbose` provider attempts after failures; never expose credential values. Local `smoke` / `regression` checks do not prove remote availability either; see the [CLI reference](cli.en.md) for modes and side effects.

See [getting started](../guides/getting-started.en.md) for onboarding and the [CLI reference](cli.en.md) for output and errors.

## Implementation and rationale

- [Paths and initialization](../../internal/config/config.go), [runtime merging and readiness](../../internal/config/runtime.go)
- [Provider setup](../../internal/config/provider_setup.go) and [tests](../../internal/config/provider_setup_test.go)
- [Safe setup design](../decisions/cli-provider-setup-technical-design.md), [Alpha Search decision](../decisions/openai-responses-alpha-search-technical-design.md) (Chinese)
