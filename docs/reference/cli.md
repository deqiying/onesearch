# CLI 参考

[文档导航](../README.md) · [English](cli.en.md)

适用于当前 CLI。完整参数、约束与 argv 绑定以实际 binary 的 targeted `schema` 为准；本文提供命令地图、输出边界和常用示例。

## 命令发现与合同

```powershell
onesearch schema --format json
onesearch schema search --format json
onesearch schema exa web-search --format json
onesearch schema --pretty
onesearch search --help
```

`schema` 返回 V2 CLI command manifest。每个 command 提供 `name`、`description`、`input_schema`，以及 canonical `path`、argv 绑定、约束、side effects、output contract 和 status preflight。它不是 native tool 注册或 runtime 配置 schema；适配器需将 `input_schema` 映射为目标平台 tool parameters，并执行 canonical `path`。

V2 用 `commands[].name` 替换 V1 `id`，用 `commands[].description` 替换 V1 `summary`；`input_schema` 不变。

`schema`、命令级 `--help`、`skills list/show` 在 runtime 配置与 provider 加载前返回，不创建 `config.json`、不读凭据、不联网。显式 `--output` 会写指定文件。已识别命令的未知 flag、多余位置参数、冲突选项，以及 schema 的未知 canonical path 返回 `parameter_error` 和退出码 `2`。

## 能力地图

| 任务 | 命令 | 能力 / provider |
| --- | --- | --- |
| 答案搜索 | `search` | `answer_search`：xAI、OpenAI-compatible、OpenAI Responses |
| 来源发现 | `search --extra-sources`、provider search | `source_search`：Exa、Zhipu、Tavily、Firecrawl、DeepSeek；DDG、Freecrawl 默认 direct-only |
| 文档检索 | `context7 resolve-library-id/query-docs`、`exa web-search` | `docs_search`：Context7、Exa |
| 网页抓取 | `fetch --provider`、provider fetch/extract/scrape | `page_fetch`：Tavily、Firecrawl、Exa、AnySearch、DeepSeek；DDG、Freecrawl 默认 direct-only |
| 站点结构 | `map --provider`、`tavily map`、`firecrawl map` | `site_map`：Tavily、Firecrawl |
| 站点爬取 | `crawl --provider`、provider crawl | `site_crawl`：Tavily、Firecrawl；Freecrawl 默认 direct-only |
| 仓库 Wiki | `repo-wiki`、`search --repo-wiki` | `repo_wiki`：DeepWiki |
| 垂直搜索 | `anysearch domains/search/extract/batch` | `vertical_search`：AnySearch |
| 离线规划 | `deep`、`dr` | 本地 planner，不调用 provider |
| Skill 发现 | `skills list/show` | router、workflow 和 provider Skill |
| 诊断 | `doctor`、`status`、`config list` | 配置健康与本机 preflight |
| 本地自检 | `smoke`、`regression` | 默认 / 固定 mock；`smoke --live` 当前仅检查本机 minimum profile |
| 下游 MCP Server | `mcp` | 本地 stdio，固定 5 工具；无运行 flags / `serve`，见 [MCP 指南](../guides/mcp.md)及发布边界 |

可用性由本机配置决定。执行前运行 `status` 并通过[对应 readiness gate](../guides/getting-started.md)。

`smoke` 默认使用 mock，`regression` 固定使用 mock；当前 `smoke --live` 仅调用本机 `doctor`，不验证远端连通性。这些命令仍会加载 runtime 配置，可能初始化配置文件，不能与静态发现的无配置副作用保证混同，也不能替代 `go test` 或真实 provider 验证。

## 常用场景

以下 PowerShell 示例需要相应能力与 provider 就绪；输出文件写入当前目录。

```powershell
# 答案搜索并发现额外来源
onesearch search "今天值得关注的 AI 新闻" --validation balanced --extra-sources 2 --format json

# 按能力选择 provider，同时发现候选并抓取其中一条
onesearch search "当前官方榜单" `
  --providers "answer_search=openai_responses;source_search=tavily;page_fetch=firecrawl" `
  --extra-sources 2 --fetch-sources 1 --validation strict --format json

# 抓取关键页面
onesearch fetch "https://example.com/article" --provider tavily --format markdown --output evidence.md

# 库文档
onesearch context7 resolve-library-id "react" "useEffect cleanup" --format json
onesearch context7 query-docs "/facebook/react" "useEffect cleanup" --format json

# 仓库上下文
onesearch repo-wiki "microsoft/playwright" "MCP Browser Server 是怎么实现的？" --provider deepwiki --format json

# 仅生成离线计划
onesearch deep "对比 OpenAI Responses 与 Chat Completions 的联网搜索" --budget deep --format json
```

`--fetch-sources N` 抓取 `source_search` 发现的前 `N` 个 URL；页面出现在 `used.page_fetch`，角色为 `source_evidence`。它不意味着所有候选都已经验证。

## Provider-direct 命令

需要明确 adapter 而不是 workflow 路由时使用：

```text
onesearch <provider> <command> [args] [--format json|markdown|content] [--pretty]
```

```powershell
onesearch exa web-search "query"
onesearch exa web-fetch "https://example.com"
onesearch exa similar "https://example.com"
onesearch tavily search "query"
onesearch tavily extract "https://example.com"
onesearch tavily map "https://example.com"
onesearch tavily crawl "https://example.com"
onesearch firecrawl search "query"
onesearch firecrawl scrape "https://example.com"
onesearch firecrawl map "https://example.com"
onesearch firecrawl crawl "https://example.com"
onesearch context7 resolve-library-id "react"
onesearch context7 query-docs "/facebook/react" "useEffect cleanup"
onesearch deepwiki ask-question "microsoft/playwright" "架构是什么？"
onesearch deepwiki read-wiki-structure "microsoft/playwright"
onesearch deepwiki read-wiki-contents "microsoft/playwright"
onesearch anysearch search "query"
onesearch zhipu search "query"
onesearch deepseek search "query site:example.com lang:zh,en"
onesearch deepseek fetch "https://example.com"
onesearch ddg search "query"
onesearch ddg fetch-content "https://example.com"
onesearch freecrawl search "query"
onesearch freecrawl scrape "https://example.com"
onesearch freecrawl crawl "https://example.com"
onesearch freecrawl deep-research "topic"
```

执行前检查 `direct_endpoints.<provider>.available == true`。协议边界：

- Exa wire search type 使用 `auto`；`neural` 仅为弃用兼容别名。
- Context7 library resolution 使用 `/api/v2/libs/search`，需要 API key。
- `firecrawl crawl` 返回异步提交状态 `job_id`/`status`，不是已完成页面。
- AnySearch 域发现必须提供 domain 或 `--domains`。抽取会解包上游 `url`/`title`/`content`（含 REST `data` 包装），`content` 与 `--format content` 是页面 Markdown 而非 JSON 外壳。
- DeepSeek 默认禁用，Search 与 Fetch 按成功请求计费（每个 API 约 5 QPS）；base URL 为 API 根 `https://api.deepseek.com`，不接受完整 endpoint。`deepseek search` 的 query 为 1–128 字符，透传 `site:`、`-site:`、`lang:`、`after:`、`before:` 等搜索语法；`--count` 默认 10（1–50），`--need-content` 才返回页面正文。`rerank_score` ≥ 1.5、`authority` ≥ 3 仅用于排序候选，不等于证据。`deepseek fetch` 接受不含 userinfo、不超过 8192 字符的 http(s) URL，返回标题与 Markdown 正文。
- DDG、Freecrawl 的启用与版本限制见[配置参考](configuration.md)。

## 输出与错误

| 选项 | 语义 |
| --- | --- |
| `--format json` | 默认单行 compact JSON，保留结尾换行 |
| `--pretty` | JSON 两空格缩进，不改变 quiet/verbose 数据字段 |
| `--format markdown` | 人类可读的诊断、结果和正文 |
| `--format content` | 核心答案、页面正文或短诊断摘要 |
| 默认 / `--quiet` 错误 | 精简、面向恢复的字段 |
| `--verbose` | provider attempts、routing decision、capability 状态等详细诊断 |
| `--output <path>` | 写入显式指定的输出文件 |

`search --format json` 返回 `ok`、`query`、`used`、`meta`；`used` 记录真正执行的能力和 provider。完整答案用 `--format content`，详细路由诊断用 `--verbose`。

所有动态输出经过凭据脱敏，包括 JSON、Markdown、content、stderr、quiet/verbose/pretty 和输出文件。敏感值替换为 `********`，没有关闭脱敏的 debug 开关。

## 证据策略与离线规划

Provider sources 是待核验候选，不证明网页正文已检查。新闻、政策、财经、医疗、严肃评测和工具选型应先发现 URL，再用 `fetch` 或 `search --fetch-sources N` 抓取关键页，基于抓取正文形成结论。

`deep` 仅生成含 `intent_signals`、`decomposition`、`capability_plan`、`preflight`、`steps`、`gap_check` 的计划，不调用 provider、不联网、不抓网页。Agent 执行其中的 `command_argv` token 数组，并满足默认 `fetch_before_claim` 要求后，才可将候选视为已验证证据。

## 依据与决定

- [命令合同](../../internal/commandcontract/)、[CLI 解析](../../internal/cli/)、[输出](../../internal/output/output.go)、[离线规划](../../internal/service/deep.go)
- [Manifest V2 决定](../decisions/cli-command-manifest-tool-declaration-alignment-technical-design.md)、[JSON 排版决定](../decisions/cli-json-output-format-technical-design.md)
- [Agent 接入与 Skill 发现](../guides/agent-integration.md)
