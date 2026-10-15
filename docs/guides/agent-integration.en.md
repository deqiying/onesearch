# Agent integration

[Documentation hub](../README.md) · [简体中文](agent-integration.md)

## Install the thin host entry

1. Install the CLI and complete readiness setup for the required capabilities using [getting started](getting-started.en.md).
2. Create an `onesearch` folder in your agent host's supported custom Skills directory.
3. Save the complete raw [host Skill template](../assets/onesearch-host-skill.md) as `SKILL.md` in that folder, including frontmatter. Do not copy the rendered web page.
4. Refresh or restart the host if it requires Skill rediscovery.

The template only resolves the actual executable, records its version, and loads the `onesearch` router from that same binary. Do not copy provider contracts, endpoint configuration, or command parameters into the host Skill. Loading guidance does not authorize networking, installation, or configuration changes.

## Read built-in Skills

```powershell
onesearch skills list --format json
onesearch skills list --capability page_fetch --format json
onesearch skills show onesearch --format content
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
onesearch skills show exa --format content
```

The bundled `onesearch` Skill is a thin router. Workflow Skills cover `search`, `docs`, `fetch`, and deep research. Provider Skills cover Exa, Tavily, Firecrawl, Context7, DeepWiki, AnySearch, Zhipu, DDG, and Freecrawl.

`skills show` reads `SKILL.md` by default; `--file <relative-path>` reads one bundled file within the selected Skill. Discovery is read-only and offline without provider configuration initialization, except for explicit `--output`. Returned Markdown is guidance, not a newly registered host Skill.

## Version and failure boundaries

Reuse the loaded router while the executable path and version remain unchanged; do not reload recursively. Load a child workflow/provider only when selected by the router.

If the CLI is missing or canonical `skills show` fails, report the resolved path, available version, exit status, and concise error. Do not guess aliases/flags, silently upgrade, or fall back to copied stale contracts. Use a permitted host search capability only when it preserves task semantics; otherwise report the missing prerequisite.

See the [CLI reference](../reference/cli.en.md) for output, argv bindings, and evidence policy.

## Maintenance boundaries

- The template is the canonical repository documentation copy. The [main npm README](../../npm/onesearch/README.md) embeds a matching copy for standalone publishing; synchronize changes. The [compatibility package README](../../npm/deqiying-onesearch/README.md) links to the repository's agent integration entry instead of duplicating the template.
- Bundled workflow sources stay in [internal/skills/assets](../../internal/skills/assets/) and are embedded by [skills.go](../../internal/skills/skills.go), not moved into `docs/`.
- The [Skill design record](../decisions/builtin-skills-agent-optimization-plan.md) (Chinese) preserves routing, readiness, and on-demand loading rationale; [Skill tests](../../internal/skills/skills_test.go) check assets and command contracts.
