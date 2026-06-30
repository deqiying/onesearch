---
name: onesearch-deepwiki
description: Use when an AI agent needs DeepWiki through Onesearch, especially when the user names DeepWiki, ask_question, read_wiki_structure, read_wiki_contents, public GitHub repository architecture, module summaries, implementation overviews, generated repository wiki context, or DeepWiki MCP replacement through the Onesearch CLI.
---

# Onesearch DeepWiki

Use DeepWiki for public GitHub repository architecture questions, implementation overviews, wiki structure, and generated repository context.

## Bridge Contract

This skill is the source document for agent-facing DeepWiki bridge skills. When a task names DeepWiki, `ask_question`, `read_wiki_structure`, `read_wiki_contents`, public GitHub repository architecture, module summaries, implementation overviews, or generated repository wiki context, route through Onesearch instead of looking for a direct DeepWiki MCP tool.

If command details may have changed, run:

```powershell
onesearch skills show deepwiki --format content
```

Use the provider command family by default:

```powershell
onesearch deepwiki ask-question "owner/repo" "question" --format json
onesearch deepwiki read-wiki-structure "owner/repo" --format json
onesearch deepwiki read-wiki-contents "owner/repo" --format json
```

Use `onesearch mcp ask_question ...`, `onesearch mcp read_wiki_structure ...`, and `onesearch mcp read_wiki_contents ...` only for mechanical migration from original MCP tool names.

## Commands

| Purpose | Preferred command | MCP-compatible alias |
| --- | --- | --- |
| Ask repo question | `onesearch deepwiki ask-question "owner/repo" "question"` | `onesearch deepwiki ask_question "owner/repo" "question"` |
| Wiki structure | `onesearch deepwiki read-wiki-structure "owner/repo"` | `onesearch deepwiki read_wiki_structure "owner/repo"` |
| Wiki contents | `onesearch deepwiki read-wiki-contents "owner/repo"` | `onesearch deepwiki read_wiki_contents "owner/repo"` |
| Legacy repo wiki | `onesearch repo-wiki "owner/repo" "question"` | Legacy flat command |

Global MCP migration aliases:

```powershell
onesearch mcp ask_question "microsoft/playwright" "How is the MCP server structured?" --format json
onesearch mcp read_wiki_structure "microsoft/playwright" --format json
onesearch mcp read_wiki_contents "microsoft/playwright" --format json
```

## Usage

```powershell
onesearch deepwiki ask-question "microsoft/playwright" "How is the MCP server structured?" --format json
onesearch deepwiki read-wiki-structure "microsoft/playwright" --format json
onesearch deepwiki read-wiki-contents "microsoft/playwright" --format json
onesearch repo-wiki "microsoft/playwright" "architecture overview" --format json
```

## Input

Repository input accepts `owner/repo` or a GitHub URL such as `https://github.com/microsoft/playwright`. Bare repository names are rejected because the owner is ambiguous.

## Output

DeepWiki output includes `provider: "deepwiki"`, `tool`, `repo`, `content`, and timing fields. Quiet JSON compacts long content to `content_preview` and `content_length`.

## Guardrails

- Use DeepWiki for repository context, not current web facts.
- Treat DeepWiki content as generated repository documentation; verify exact code behavior locally when the repository is available.
- Public repositories work anonymously; private docs may require `DEEPWIKI_API_KEY`.
