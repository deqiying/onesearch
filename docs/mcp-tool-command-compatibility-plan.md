# Onesearch MCP 原始工具命令兼容技术方案

## 背景

`onesearch` 当前已经提供一组能力导向命令，例如 `search`、`fetch`、`map`、`crawl`、`repo-wiki`、`exa-search`、`context7-docs`。这套命令适合日常 agent 工作流，但如果目标是逐步替换 Codex 配置中的 MCP 工具，还需要让用户和 agent 能按原 MCP tool 名快速定位等价命令。

本方案定义新的命令体系：主推 provider 分组式命令，同时兼容原始 MCP tool 名。

## 目标

- 主推统一格式：`onesearch <provider> <command> [args] [flags]`。
- 兼容原始 MCP tool 名：允许在 provider 分组下直接使用 MCP 暴露的 tool 名，例如 `onesearch exa web_fetch_exa "https://example.com"`。
- 保留现有平铺命令作为兼容入口，例如 `onesearch exa-search`、`onesearch context7-docs`。
- 所有新增命令默认输出稳定 JSON，并沿用现有 `--format json|markdown|content`、`--output`、`--quiet|--verbose`。
- 对高成本、异步、账号管理、监控或浏览器交互类工具，先纳入命名规划，分阶段实现。
- `onesearch-cli` 主技能只作为路由入口；每个 provider 独立维护自己的技能，说明该工具有哪些命令、作用和使用方法。

## 非目标

- 不把 `onesearch` 改造成 MCP Server。
- 不一次性实现所有 MCP 的私有账号管理、自动化、Unity、Excel、Node REPL 等非 web research 能力。
- 不为兼容原始 tool 名牺牲主命令的可读性；原始 MCP 名只作为别名或低摩擦迁移入口。

## 命名结论

### Tavily

当前 Codex MCP 工具面中，Tavily 原始工具名带 `tavily_` 前缀，例如：

- `tavily_search`
- `tavily_extract`
- `tavily_map`
- `tavily_crawl`

因此 `onesearch` 应主推短命令：

```powershell
onesearch tavily search "query"
onesearch tavily extract "https://example.com"
onesearch tavily map "https://example.com"
onesearch tavily crawl "https://example.com"
```

同时兼容原始 MCP tool 名：

```powershell
onesearch tavily tavily_search "query"
onesearch tavily tavily_extract "https://example.com"
onesearch tavily tavily_map "https://example.com"
onesearch tavily tavily_crawl "https://example.com"
```

### Freecrawl

当前 Codex 工具命名里，Freecrawl 在工具 namespace 下显示为 `mcp__freecrawl__search`、`mcp__freecrawl__scrape`、`mcp__freecrawl__crawl`、`mcp__freecrawl__deep_research`。这是 Codex/MCP 暴露层的全限定工具名；底层语义工具名仍是 `search`、`scrape`、`crawl`、`deep_research`。

因此 `onesearch` 应主推短命令：

```powershell
onesearch freecrawl search "query"
onesearch freecrawl scrape "https://example.com"
onesearch freecrawl crawl "https://example.com"
onesearch freecrawl deep-research "topic"
```

同时兼容全限定 MCP tool 名：

```powershell
onesearch freecrawl mcp__freecrawl__search "query"
onesearch freecrawl mcp__freecrawl__scrape "https://example.com"
onesearch freecrawl mcp__freecrawl__crawl "https://example.com"
onesearch freecrawl mcp__freecrawl__deep_research "topic"
```

## 命令设计

### 主推格式

```text
onesearch <provider> <command> [positional args] [flags]
```

规则：

- `<provider>` 使用稳定 provider ID：`exa`、`tavily`、`firecrawl`、`context7`、`deepwiki`、`ddg`、`freecrawl`、`anysearch`。
- `<command>` 使用人类友好的 kebab-case 或短动词，例如 `web-fetch`、`query-docs`、`ask-question`。
- 原始 MCP tool 名使用 snake_case 保留，例如 `web_fetch_exa`、`tavily_search`、`read_wiki_structure`。
- 对 Codex 全限定工具名，保留双下划线原样别名，例如 `mcp__freecrawl__scrape`。

### 兼容格式

兼容入口分两层：

1. Provider 分组内兼容原始 MCP tool 名。

```powershell
onesearch exa web_search_exa "query"
onesearch tavily tavily_search "query"
onesearch deepwiki ask_question "owner/repo" "question"
```

2. 可选增加全局 `mcp` 兼容入口，用于脚本迁移和机械映射。

```powershell
onesearch mcp web_search_exa "query"
onesearch mcp tavily_search "query"
onesearch mcp mcp__freecrawl__scrape "https://example.com"
```

全局 `mcp` 入口不是主推使用方式，只用于从旧 MCP tool 名迁移时减少认知成本。

## 工具映射矩阵

### Exa

| 原 MCP tool 名 | 主推命令 | 兼容命令 | 当前状态 | 实现优先级 |
| --- | --- | --- | --- | --- |
| `web_search_exa` | `onesearch exa web-search "query"` | `onesearch exa web_search_exa "query"` | 已有 `exa-search` 主干能力 | P0 |
| `web_fetch_exa` | `onesearch exa web-fetch "url"` | `onesearch exa web_fetch_exa "url"` | 缺独立命令和 provider 方法 | P0 |

实现要点：

- `web-search` 复用现有 `Exa.Search`，补齐 `numResults` 默认值与 MCP 命名。
- `web-fetch` 新增 Exa contents/fetch provider 方法，支持多个 URL 和 `--max-characters`。
- 现有 `exa-search`、`exa-similar` 保持兼容。

### Tavily

| 原 MCP tool 名 | 主推命令 | 兼容命令 | 当前状态 | 实现优先级 |
| --- | --- | --- | --- | --- |
| `tavily_search` | `onesearch tavily search "query"` | `onesearch tavily tavily_search "query"` | 仅作为内部 route provider | P0 |
| `tavily_extract` | `onesearch tavily extract "url"` | `onesearch tavily tavily_extract "url"` | 有单 URL `Extract`，参数偏窄 | P0 |
| `tavily_map` | `onesearch tavily map "url"` | `onesearch tavily tavily_map "url"` | 已有 generic `map`，参数偏窄 | P1 |
| `tavily_crawl` | `onesearch tavily crawl "url"` | `onesearch tavily tavily_crawl "url"` | 缺 Tavily crawl provider | P1 |

实现要点：

- `search` 支持 `--max-results`、`--search-depth`、`--topic`、`--time-range`、`--start-date`、`--end-date`、`--include-domains`、`--exclude-domains`、`--country`、`--include-raw-content`、`--include-images`、`--include-favicon`。
- `extract` 支持多 URL：位置参数可重复，或 `--urls` 逗号列表；支持 `--format markdown|text`、`--extract-depth basic|advanced`、`--query`、`--include-images`、`--include-favicon`。
- `map` 支持 `--allow-external`、`--select-domains`、`--select-paths`，默认仍保持同域过滤，显式打开后不过滤。
- `crawl` 新增 Tavily provider 方法，并把 `site_crawl` route 从仅 Firecrawl 扩展为 `tavily`、`firecrawl`。

### Firecrawl

| 原 MCP tool 名 | 主推命令 | 兼容命令 | 当前状态 | 实现优先级 |
| --- | --- | --- | --- | --- |
| `firecrawl_search` | `onesearch firecrawl search "query"` | `onesearch firecrawl firecrawl_search "query"` | 有内部 search，参数偏窄 | P1 |
| `firecrawl_scrape` | `onesearch firecrawl scrape "url"` | `onesearch firecrawl firecrawl_scrape "url"` | 有内部 scrape，参数偏窄 | P1 |
| `firecrawl_map` | `onesearch firecrawl map "url"` | `onesearch firecrawl firecrawl_map "url"` | 有内部 map，参数偏窄 | P1 |
| `firecrawl_crawl` | `onesearch firecrawl crawl "url"` | `onesearch firecrawl firecrawl_crawl "url"` | 有内部 crawl，参数偏窄 | P1 |
| `firecrawl_extract` | `onesearch firecrawl extract --urls ...` | `onesearch firecrawl firecrawl_extract --urls ...` | 缺独立 provider 方法 | P2 |
| `firecrawl_parse` | `onesearch firecrawl parse "file"` | `onesearch firecrawl firecrawl_parse "file"` | 缺 multipart/上传流程 | P2 |
| `firecrawl_agent` | `onesearch firecrawl agent "prompt"` | `onesearch firecrawl firecrawl_agent "prompt"` | 缺异步任务支持 | P3 |
| `firecrawl_agent_status` | `onesearch firecrawl agent-status "id"` | `onesearch firecrawl firecrawl_agent_status "id"` | 缺异步任务支持 | P3 |
| `firecrawl_interact` | `onesearch firecrawl interact ...` | `onesearch firecrawl firecrawl_interact ...` | 浏览器交互类，暂缓 | P3 |
| `firecrawl_interact_stop` | `onesearch firecrawl interact-stop "id"` | `onesearch firecrawl firecrawl_interact_stop "id"` | 浏览器交互类，暂缓 | P3 |
| `firecrawl_feedback` | `onesearch firecrawl feedback ...` | `onesearch firecrawl firecrawl_feedback ...` | 可做轻量 passthrough | P3 |

实现要点：

- P1 只补搜索、抓取、map、crawl 的常用参数。
- P2 开始支持 JSON schema、queryOptions、parse 本地文件。
- P3 涉及异步 job、交互 session、监控、feedback，先保持 MCP 兜底。

### Context7

| 原 MCP tool 名 | 主推命令 | 兼容命令 | 当前状态 | 实现优先级 |
| --- | --- | --- | --- | --- |
| `resolve_library_id` | `onesearch context7 resolve-library-id "React"` | `onesearch context7 resolve_library_id "React"` | 已有 `context7-library` | P0 |
| `query_docs` | `onesearch context7 query-docs "/facebook/react" "query"` | `onesearch context7 query_docs "/facebook/react" "query"` | 已有 `context7-docs` | P0 |

实现要点：

- 新命令作为现有 `context7-library`、`context7-docs` 的分组别名。
- 保持 Context7 先 resolve、再 query 的使用说明。

### DeepWiki

| 原 MCP tool 名 | 主推命令 | 兼容命令 | 当前状态 | 实现优先级 |
| --- | --- | --- | --- | --- |
| `ask_question` | `onesearch deepwiki ask-question "owner/repo" "question"` | `onesearch deepwiki ask_question "owner/repo" "question"` | 已由 `repo-wiki` 覆盖 | P0 |
| `read_wiki_structure` | `onesearch deepwiki read-wiki-structure "owner/repo"` | `onesearch deepwiki read_wiki_structure "owner/repo"` | 已由 `repo-wiki --mode structure` 覆盖 | P0 |
| `read_wiki_contents` | `onesearch deepwiki read-wiki-contents "owner/repo"` | `onesearch deepwiki read_wiki_contents "owner/repo"` | 已由 `repo-wiki --mode contents` 覆盖 | P0 |

实现要点：

- 只纳入公开 GitHub wiki 相关能力。
- 不纳入 Devin 私有自动化、session、knowledge、playbook 等非 `onesearch` 领域能力。

### DDG Search

| 原 MCP tool 名 | 主推命令 | 兼容命令 | 当前状态 | 实现优先级 |
| --- | --- | --- | --- | --- |
| `search` | `onesearch ddg search "query"` | `onesearch ddg search "query"` | 缺 DDG provider | P2 |
| `fetch_content` | `onesearch ddg fetch-content "url"` | `onesearch ddg fetch_content "url"` | 可由 generic `fetch` 部分替代 | P2 |

实现要点：

- DDG 可作为低成本 fallback provider，不进入默认高可信 route。
- `fetch-content` 支持 `--start-index`、`--max-length`、`--backend httpx|curl|auto`，但实现层可先忽略 backend，仅在输出中标注 unsupported backend。

### Freecrawl

| 原 MCP/Codex tool 名 | 主推命令 | 兼容命令 | 当前状态 | 实现优先级 |
| --- | --- | --- | --- | --- |
| `mcp__freecrawl__search` | `onesearch freecrawl search "query"` | `onesearch freecrawl mcp__freecrawl__search "query"` | 缺 Freecrawl provider | P2 |
| `mcp__freecrawl__scrape` | `onesearch freecrawl scrape "url"` | `onesearch freecrawl mcp__freecrawl__scrape "url"` | 缺 Freecrawl provider | P2 |
| `mcp__freecrawl__crawl` | `onesearch freecrawl crawl "url"` | `onesearch freecrawl mcp__freecrawl__crawl "url"` | 缺 Freecrawl provider | P3 |
| `mcp__freecrawl__deep_research` | `onesearch freecrawl deep-research "topic"` | `onesearch freecrawl mcp__freecrawl__deep_research "topic"` | 缺 Freecrawl provider | P3 |

实现要点：

- 新增 `freecrawl` provider，不默认进入 `source_search` 或 `page_fetch` route，先作为显式命令。
- `scrape` 支持 `--javascript`、`--anti-bot`、`--cache`、`--formats`、`--headers-json`、`--cookies-json`、`--wait-for`。
- `deep-research` 与 `onesearch deep` 语义不同，必须明确这是 Freecrawl provider 的联网研究，不和本地离线 planner 混淆。

### AnySearch

当前已有 `anysearch-domains`、`anysearch-search`、`anysearch-extract`、`anysearch-batch`。建议新增分组式别名：

```powershell
onesearch anysearch domains [domain]
onesearch anysearch search "query"
onesearch anysearch extract "url"
onesearch anysearch batch "query1" "query2"
```

## CLI 解析方案

### 路由结构

在 `internal/cli` 中增加 provider 分组 dispatcher：

```text
Execute
  -> runProviderCommand(provider, subcommand, args)
      -> runExaGroup
      -> runTavilyGroup
      -> runFirecrawlGroup
      -> runContext7Group
      -> runDeepWikiGroup
      -> runDDGGroup
      -> runFreecrawlGroup
      -> runAnySearchGroup
```

每个 group 内维护 alias map：

```text
web-search      -> web_search_exa
web_search_exa  -> web_search_exa
search          -> tavily_search
tavily_search   -> tavily_search
```

### 参数风格

- 简单标量继续使用 `flag.FlagSet`。
- 数组参数继续使用现有 `stringListFlag`，支持逗号和空格分隔。
- 复杂对象参数采用 JSON 字符串 flag，例如 `--schema-json`、`--scrape-options-json`、`--headers-json`。
- 不建议为复杂对象设计大量层级 flag，避免 CLI 变成不稳定的小 DSL。

### 输出风格

直接 provider 命令输出 provider 原始结果的归一化 envelope：

```json
{
  "ok": true,
  "provider": "tavily",
  "tool": "tavily_search",
  "query": "query",
  "results": [],
  "total": 0,
  "elapsed_ms": 0
}
```

规则：

- `provider` 使用 `onesearch` provider ID。
- `tool` 使用兼容的原始 MCP tool 名，方便迁移审计。
- `raw` 或 `result` 仅在 `--verbose` 输出完整 provider 原始响应。
- `content`、`results`、`pages` 等字段尽量沿用当前项目已有输出习惯。

## Runtime 配置调整

新增 provider 或 capability 时遵循现有 runtime schema：

- `exa` 增加 `page_fetch` capability，用于 `web_fetch_exa`。
- `tavily` 增加 `site_crawl` capability。
- 新增 `freecrawl` provider，默认 `enabled: false`，不自动加入默认 routes。
- 新增 `ddg` provider，默认 `enabled: false`，不自动加入默认 routes。

建议新增 capability 时克制：

- 可复用现有 `source_search`、`page_fetch`、`site_map`、`site_crawl` 的，不新增 capability。
- Firecrawl `parse`、`agent`、`interact` 这类不属于通用搜索/抓取流水线的工具，作为 provider direct command，不加入 profile required capabilities。

## 兼容策略

### 保留现有命令

以下命令继续保留：

- `exa-search`
- `exa-similar`
- `context7-library`
- `context7-docs`
- `repo-wiki`
- `anysearch-*`
- `search`、`fetch`、`map`、`crawl`

它们是当前 `README` 和 skill 中已有的 agent 入口，不应破坏。

### 新旧命令关系

| 旧命令 | 新主推命令 | 原始 MCP 名兼容 |
| --- | --- | --- |
| `exa-search` | `exa web-search` | `exa web_search_exa` |
| `context7-library` | `context7 resolve-library-id` | `context7 resolve_library_id` |
| `context7-docs` | `context7 query-docs` | `context7 query_docs` |
| `repo-wiki --mode ask` | `deepwiki ask-question` | `deepwiki ask_question` |
| `repo-wiki --mode structure` | `deepwiki read-wiki-structure` | `deepwiki read_wiki_structure` |
| `repo-wiki --mode contents` | `deepwiki read-wiki-contents` | `deepwiki read_wiki_contents` |
| `anysearch-search` | `anysearch search` | 如后续有原始 MCP 名再补 |

## 分阶段实现

### P0：低风险别名和 Exa/Tavily 核心补齐

范围：

- `onesearch exa web-search`
- `onesearch exa web_search_exa`
- `onesearch exa web-fetch`
- `onesearch exa web_fetch_exa`
- `onesearch tavily search`
- `onesearch tavily tavily_search`
- `onesearch tavily extract`
- `onesearch tavily tavily_extract`
- `onesearch context7 resolve-library-id/query-docs`
- `onesearch deepwiki ask-question/read-wiki-structure/read-wiki-contents`

验收：

- 单元测试覆盖 alias 解析。
- provider httptest 覆盖 request payload。
- `go test ./...` 通过。
- `onesearch skills show exa/tavily/mcp-tools --format content` 能查询 provider 命令和 MCP 兼容路由。

### P1：Tavily map/crawl 与 Firecrawl 常用工具

范围：

- `onesearch tavily map/crawl` 和对应 `tavily_map/tavily_crawl`。
- `onesearch firecrawl search/scrape/map/crawl` 和对应 `firecrawl_*`。
- Tavily 加入 `site_crawl` route。

验收：

- `map` 默认同域过滤保持兼容；`--allow-external` 可放开。
- `crawl` 输出必须有数量限制和 quiet 裁剪，避免大响应打爆 agent 上下文。
- mock smoke 增加 Tavily crawl case。

### P2：DDG、Freecrawl、Firecrawl 结构化提取

范围：

- `onesearch ddg search/fetch-content`。
- `onesearch freecrawl search/scrape`。
- `onesearch firecrawl extract/parse`。

验收：

- 复杂对象参数只通过 `--*-json` 传入。
- JSON schema 参数有非法 JSON 错误测试。
- Firecrawl parse 本地文件只接受明确文件路径，不在输出中泄漏本地敏感路径之外的信息。

### P3：异步、交互、监控和反馈类命令

范围：

- Firecrawl `agent`、`agent-status`、`interact`、`interact-stop`、`feedback`、monitor 相关工具。
- Freecrawl `crawl`、`deep-research`。

验收：

- 异步命令不在 CLI 内无限等待，默认返回 job id。
- 可选 `--poll` 和 `--poll-timeout`，并有明确超时输出。
- 交互类命令默认要求显式 provider 命令，不进入通用 `fetch/search` pipeline。

## 测试方案

- `internal/cli`：覆盖 provider group alias、原始 MCP 名 alias、全局 `mcp` alias。
- `internal/providers`：每个 provider 使用 `httptest` 验证 endpoint、headers、payload 和错误解析。
- `internal/service`：覆盖 route capability 变更，例如 Exa 可作为 `page_fetch`、Tavily 可作为 `site_crawl`。
- `internal/output`：验证 quiet 输出裁剪大字段，verbose 保留原始诊断。
- regression：扩展 `smoke --mock`，但不在默认测试中发起真实联网请求。

## 文档更新

需要同步更新：

- `README.md`：增加“Provider 分组命令”和“兼容原始 MCP tool 名”章节。
- `internal/skills/assets/onesearch-cli/SKILL.md`：保持为主路由技能，只负责选择 workflow/provider skill。
- `internal/skills/assets/onesearch-cli/references/cli-contract.md`：列出新公共命令契约。
- `internal/skills/assets/onesearch-{exa,tavily,firecrawl,context7,deepwiki,anysearch,zhipu}/SKILL.md`：分别描述各 provider 命令、作用、用法和 MCP 兼容别名。
- `internal/skills/assets/onesearch-search/SKILL.md`：搜索类优先推荐 `onesearch tavily search`、`onesearch exa web-search`。
- `internal/skills/assets/onesearch-fetch/SKILL.md`：抓取类优先推荐 `onesearch tavily extract`、`onesearch firecrawl scrape`、`onesearch exa web-fetch`。
- npm README：发布前同步顶层 README 的命令示例。

## 风险与取舍

- 原始 MCP tool 名和 provider 分组短命令同时存在，会增加 help 输出长度。解决方式是在 `onesearch --help` 只展示分组入口，在 `onesearch <provider> --help` 展示详细 alias。
- Firecrawl/Freecrawl 高级参数很多，全部铺成 flag 会使 CLI 复杂化。解决方式是常用参数做 flag，复杂对象用 JSON flag。
- 某些 MCP 工具是 Codex namespace 产物，不一定是 provider 官方 API 命名。解决方式是在兼容层记录 `tool` 字段，主实现仍按 provider API 语义组织。
- 真实 provider API 可能和当前 MCP schema 有差异。实现前需以官方 API 文档或实际 MCP server 行为为准，避免只按工具描述猜 payload。

## 推荐结论

建议采用“分组式命令为主、原始 MCP tool 名为别名”的方案。

优先实现 P0 和 P1。完成后，`onesearch` 可以较自然地承接 Exa、Tavily、Context7、DeepWiki 和 Firecrawl 常用工具调用；P2/P3 再逐步覆盖 DDG、Freecrawl、Firecrawl 高级提取与异步工具。这样既能保持 `onesearch` 的统一体验，又能降低从 MCP tool 名迁移到 CLI 命令时的摩擦。
