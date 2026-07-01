# onesearch

`onesearch` 是一个 CLI-first 的研究与证据工具，面向 AI 助手、脚本和终端用户。它把综合搜索、来源发现、文档检索、网页抓取、站点结构读取、Deep Research 离线规划、配置诊断和内置 skill 分发放在同一个可复现命令层里。

`onesearch` 本身不是 MCP Server。AI 工具可以通过 `skills show` 读取内置工作流，再按 CLI 命令执行搜索、抓取和验证。

## 核心能力

| 能力 | 命令 | 默认路由 |
| --- | --- | --- |
| 综合搜索 | `search` | `answer_search`: xAI、OpenAI-compatible、OpenAI Responses |
| 来源发现 | `search --extra-sources`、`exa web-search`、`zhipu search`、`ddg search`、`freecrawl search` | `source_search`: Exa、Zhipu、Tavily、Firecrawl；DDG、Freecrawl 默认 direct-only；实际可用性以 `status` 为准 |
| 文档检索 | `context7 resolve-library-id`、`context7 query-docs`、`exa web-search` | `docs_search`: Exa、Context7 |
| 网页抓取 | `fetch --provider`、`exa web-fetch`、`tavily extract`、`firecrawl scrape`、`ddg fetch-content`、`freecrawl scrape` | `page_fetch`: Tavily、Firecrawl、Exa；DDG、Freecrawl 默认 direct-only |
| 站点结构 | `map --provider`、`tavily map`、`firecrawl map` | `site_map`: Tavily、Firecrawl |
| 站点爬取 | `crawl --provider`、`tavily crawl`、`firecrawl crawl`、`freecrawl crawl` | `site_crawl`: Tavily、Firecrawl；Freecrawl 默认 direct-only |
| 仓库 Wiki | `repo-wiki`、`search --repo-wiki` | `repo_wiki`: DeepWiki |
| 垂直搜索 | `anysearch domains/search/extract/batch` | `vertical_search`: AnySearch |
| 深度研究规划 | `deep` / `dr` | 本地离线 planner |
| 内置技能路由 | `skills list`、`skills show` | `onesearch-cli` 主路由，`exa`、`tavily` 等 provider 技能，`search`、`docs`、`fetch` 等工作流技能 |
| 配置诊断 | `doctor`、`status`、`config list`、`smoke` | 整体健康、能力/provider 实际可用状态、本地配置 |

## 构建

```powershell
go test ./...
go build -o .\bin\onesearch.exe .\cmd\onesearch
.\bin\onesearch.exe --version
```

开发时如果系统 Go cache 无写权限，可以把缓存放到项目目录：

```powershell
$env:GOCACHE='D:\Projects\Goland\onesearch\.gocache'
go test ./...
```

发布构建会把 `.deploy/version` 注入到二进制内部，不依赖同目录 `version` 文件：

```powershell
$version = (Get-Content .\.deploy\version -Raw).Trim()
go build -trimpath -ldflags "-s -w -X github.com/deqiying/onesearch/internal/app.BuildVersion=$version" -o .\bin\onesearch.exe .\cmd\onesearch
.\bin\onesearch.exe --version
```

推送 `v*.*.*` tag 后，GitHub Actions 会先执行共享测试，然后并行构建 GitHub Release 多平台二进制和 npm 平台包。GitHub Release 产物包含各平台压缩包与 `checksums.txt`，npm 发布沿用 `onesearch` 与 `@deqiying/onesearch-*` 包。

## 配置架构

配置文件默认位置：

| 系统 | 路径 |
| --- | --- |
| Windows | `%LOCALAPPDATA%\onesearch\config.json` |
| macOS / Linux | `~/.config/onesearch/config.json` |

可用 `ONESEARCH_CONFIG_DIR` 指定配置目录。环境变量只用于密钥和少量本机覆盖项，整体能力编排以配置文件中的运行时 schema 为准。

顶层结构：

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

各层职责：

- `defaults`：默认 pipeline、fallback、validation、minimum profile、超时、日志、重试和输出清理策略。
- `pipelines`：把任务类型组织为能力链，例如 `default`、`research`、`docs`、`crawl`。
- `routes`：每个能力的 provider 优先顺序；未列入 routes 但在 `providers.<id>.capabilities` 声明了该能力的 provider 会自动追加到对应能力组。`settings.direct_only=true` 的 provider 不自动追加，但用户显式写进 routes 时仍可参与 workflow。
- `profiles`：最低可用能力集合，例如 `standard` 要求 `answer_search`、`docs_search`、`page_fetch`。
- `providers`：provider 的 adapter、capabilities、base_url、api_key、api_key_env、enabled 和 settings。

首次运行普通命令时，如果配置文件不存在，`onesearch` 会自动创建配置目录和初始 `config.json`，然后继续使用初始配置执行。只有 `doctor` 会在诊断输出中提示本次是否创建了配置文件。初始配置中，强制依赖 API key 的 provider 默认 `"enabled": false`；不强制 API key 的匿名端点默认启用。DeepWiki 默认使用公开 MCP 端点查询公开仓库文档；如果配置 `DEEPWIKI_API_KEY`，则可用于需要凭据的私有文档查询。

示例文件见 [config.example.json](D:/Projects/Goland/onesearch/config.example.json)。默认 `source_search` 路由为：

```json
["exa", "zhipu", "tavily", "firecrawl"]
```

## 常用配置命令

```powershell
onesearch config path --format json
onesearch config list --format json
onesearch doctor --format json
onesearch status --format json
```

provider 开关、base URL、模型、settings 和 capabilities 直接编辑 `config.json` 的 `providers.<id>`；密钥可以直接写入 `api_key`，也可以通过 `api_key_env` 指向环境变量读取。两者都配置时，`api_key` 优先。`config list` 输出会脱敏直配密钥。

OpenAI 协议适配器会自动补齐 `/v1` 路径：`openai_responses` 永远请求 `/v1/responses`，`openai_chat_completions` 永远请求 `/v1/chat/completions`，不会互相降级。两者都支持 JSON 与 SSE 响应解析；`providers.<id>.settings.stream` 控制是否主动发起流式请求，`search --stream/--no-stream` 可临时覆盖。`openai_responses` 默认启用 `tools: ["web_search"]` 和 `tool_choice: "required"`；`openai_compatible` 默认不附加工具，但可通过 `settings.tools` / `settings.tool_choice` 透传给支持 Chat Completions tools 的服务。`settings` 中的空字符串、空数组和空对象会保留内置默认值。

`doctor` 默认输出适合 agent 解析的紧凑诊断 JSON，只列 `ok/status/error`、配置文件初始化状态、最低 profile 和 `issues` 问题项；不会输出完整配置文件内容。`status` 输出当前能力命令和 provider-direct 端点的实际可用状态，例如 `answer_search`、`docs_search`、`page_fetch` 以及 `exa`、`tavily`、`zhipu` 是否可用。普通错误默认 `--quiet` 精简输出，需要 provider attempts、routing decision 等完整诊断时加 `--verbose`。需要人类短摘要时用 `doctor --format content`。完整 schema、routes、providers 和 defaults 用 `config list --format json` 查看。

## 常用命令

```powershell
onesearch status --format json
onesearch search "今天有什么值得关注的 AI 新闻" --validation balanced --extra-sources 2 --format json
onesearch search "微博热搜前十 当前官方榜单" --validation strict --source-providers tavily --fetch-providers tavily --fetch-sources 1 --format json
onesearch fetch "https://example.com/article" --provider tavily --format markdown --output evidence.md
onesearch fetch "https://example.com/article" --provider exa --format json
onesearch exa web-search "OpenAI Responses API documentation" --include-highlights --format json
onesearch exa web-fetch "https://example.com/article" --max-characters 12000 --format json
onesearch exa similar "https://example.com/article" --num-results 5 --format json
onesearch tavily search "今天国内 AI 新闻" --max-results 5 --format json
onesearch tavily extract "https://example.com/article" --format content
onesearch context7 resolve-library-id "react" "useEffect cleanup" --format json
onesearch context7 query-docs "/facebook/react" "useEffect cleanup" --format json
# 仅当 status.direct_endpoints.zhipu.available 为 true 时使用：
onesearch zhipu search "今天国内 AI 新闻" --count 5 --format json
onesearch map "https://docs.example.com" --provider firecrawl --instructions "Find API reference pages" --format json
onesearch crawl "https://docs.example.com" --provider tavily --max-depth 2 --limit 20 --format json
onesearch repo-wiki "microsoft/playwright" "MCP Browser Automation Server 是怎么实现的？" --provider deepwiki --format json
onesearch search "分析 Playwright MCP Browser Automation Server 架构" --repo-wiki microsoft/playwright --validation strict --format json
onesearch deep "OpenAI Responses API web_search 和 Chat Completions 联网搜索怎么选" --budget deep --format json
onesearch smoke --mock --format json
```

## Provider direct 命令

主推新格式：

```powershell
onesearch <provider> <command> [args] [--format json|markdown|content]
```

调用 provider-direct 命令前先运行 `onesearch status --format json`，确认 `direct_endpoints.<provider>.available` 为 `true`；如果只是要限制 workflow provider，也先确认对应 `capabilities.<capability>.available` 列表里有该 provider。

已支持的常用分组命令：

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
onesearch ddg search "query"
onesearch ddg fetch-content "https://example.com"
onesearch freecrawl search "query"
onesearch freecrawl scrape "https://example.com"
onesearch freecrawl crawl "https://example.com"
onesearch freecrawl deep-research "topic"
```

旧的平铺 provider 命令、provider 内 snake_case CLI alias 和全局 `mcp` router 不再属于 public contract。JSON 输出中的 `tool` 字段仍可保留上游语义名，用于审计实际调用的 provider 工具。

`ddg` 和 `freecrawl` 通过本地 `mcp_stdio` bridge 启动配置中的 MCP server 子进程，例如 `uvx duckduckgo-mcp-server --transport stdio` 或 `uvx freecrawl-mcp`。它们内置为 `enabled: false` 且 `settings.direct_only: true`，所以默认只出现在 provider-direct 命令中，不会改变普通 `search`、`fetch`、`crawl` 路由。要让 workflow 使用它们，需要在配置中显式启用 provider，并把 provider ID 写入对应 `routes.source_search`、`routes.page_fetch` 或 `routes.site_crawl`。

输出格式：

- `json`：给 agent 和脚本解析。
- `markdown`：给人看诊断、结果和抓取正文。
- `content`：只输出核心正文或短摘要。

`search --format json` 默认输出统一搜索结果对象，只包含 `ok`、`query`、`used` 和 `meta`。`used` 是按能力名索引的唯一结果树，`used.<capability>.providers.<provider>.result` 展示本次实际使用了哪些能力、哪些 provider，以及每个 provider 返回的正文预览或来源；默认不会在最外层重复输出 `content`、`answer`、`sources` 或 `sources_count`，也不会把 `answer_search` 完整正文塞进 JSON，只保留 `content_preview` 和 `content_length`。只需要答案正文时使用 `--format content`；需要完整 JSON 正文、路由决策、provider attempts、capability 状态、primary/extra source 拆分等内部诊断信息时加 `--verbose`。

`search` 支持按能力过滤 provider。可以用独立参数：

```powershell
onesearch search "query" --answer-providers openai_responses --source-providers tavily --fetch-providers firecrawl --format json
```

也可以用 `--providers` 传入能力级过滤表达式，分号分隔能力，逗号分隔 provider：

```powershell
onesearch search "query" --providers "answer_search=openai_responses;source_search=tavily;page_fetch=firecrawl" --format json
```

没有能力名前缀的 `--providers openai_responses` 保留为兼容写法，用于普通搜索路由中的 `answer_search`、`source_search`、`docs_search`；`page_fetch` 和 `repo_wiki` 默认仍按各自能力路由自动选择，除非显式传入 `--fetch-providers` / `--repo-providers` 或 scoped `--providers`。调用 agent 需要垂直意图时，应由 agent 先判断任务类型，再选择 `source_search`、`page_fetch`、`repo_wiki` 等能力及 provider 组合；`onesearch` 本身不内置业务垂直意图识别。

当 agent 已经把查询约束到高置信来源场景时，可以用 `--fetch-sources N` 让 `search` 在 `source_search` 发现候选 URL 后自动抓取前 N 个候选页面。抓取结果会出现在 `used.page_fetch`，角色为 `source_evidence`；这适合榜单、价格、政策、新闻等需要先发现再取正文的任务。

错误详略：

- 默认等同 `--quiet`：只输出 `ok/error_type/error`、耗时和少量命令上下文字段。
- `--verbose`：保留完整诊断字段，例如 `diagnostics`、`provider_attempts`、`routing_decision`。
- 当 `defaults.log_level` 为 `DEBUG` 时默认 verbose；显式 `--quiet` 会覆盖它。

## 内置 Skill

`onesearch-cli` 是主路由技能，只负责根据用户意图选择后续工作流技能或 provider 技能。每个 provider 独立维护自己的技能，例如 `exa`、`tavily`、`firecrawl`、`context7`、`deepwiki`、`anysearch`、`zhipu`、`ddg`、`freecrawl`；这些技能中描述对应工具有哪些命令、作用和使用方法。

查询内置 skill 清单和详情：

```powershell
onesearch skills list --format json
onesearch skills list --capability page_fetch --format json
onesearch skills show onesearch-cli --format content
onesearch skills show exa --format content
onesearch skills show tavily --format content
```

`skills show` 直接输出内置 skill 的 `SKILL.md` 内容，不联网、不写文件；需要确认 provider 或能力是否实际可用时，先运行 `onesearch status --format json`。

可用别名：

- `onesearch-cli`、`base`、`onesearch`、`cli`、`router`
- `search`、`web-search`、`source-search`
- `docs`、`api-docs`、`documentation`
- `fetch`、`page-fetch`、`evidence`
- `exa`、`exa-tools`、`exa-web`、`exa-fetch`、`exa-similar-pages`
- `tavily`、`tavily-tools`、`tavily-search`、`tavily-extract`、`tavily-map`、`tavily-crawl`
- `firecrawl`、`firecrawl-tools`、`firecrawl-search`、`firecrawl-scrape`、`firecrawl-map`、`firecrawl-crawl`
- `context7`、`context7-tools`、`ctx7`、`context7-provider`、`context7-library-docs`
- `deepwiki`、`deepwiki-tools`、`repo-wiki`、`repository-wiki`
- `anysearch`、`anysearch-tools`、`as`
- `zhipu`、`zhipu-tools`、`zhipu-web-search`、`zp`
- `ddg`、`ddg-search`、`duckduckgo`、`duckduckgo-mcp`
- `freecrawl`、`freecrawl-mcp`
- `deep-research`、`deep`、`research`

## 证据策略

`search` 默认 JSON 中每个 provider 的 `result.sources` 是候选来源，不等于已经校验正文。新闻、政策、财经、医疗、严肃评测和工具选型等高风险结论应先发现 URL，再用 `fetch` 或 `search --fetch-sources N` 抓取关键网页，最终只基于抓取正文下结论。

## Deep Research

`onesearch deep` 只生成离线计划，不调用 provider、不联网、不抓网页。计划里会给出 `intent_signals`、`decomposition`、`capability_plan`、`steps[]` 和 `gap_check`。真正研究从执行 `steps[].command` 开始。

```powershell
onesearch deep "帮我核验这个说法是真是假：某工具已经完全替代 Tavily 做 AI 搜索了" --format json
onesearch deep "https://example.com/source" --format json
```

Deep Research 默认要求 `fetch_before_claim`：关键结论必须有抓取正文支撑，否则降级成未验证候选。

## 项目结构

```text
cmd/onesearch/          CLI 入口
internal/cli/           命令解析、参数处理和退出码
internal/config/        配置路径、runtime schema、provider 解析、脱敏和默认值
internal/service/       业务编排、routes、fallback、doctor、smoke、deep planner
internal/providers/     Provider HTTP 调用与结果归一化
internal/sources/       来源解析、去重、answer/source 拆分
internal/output/        JSON / Markdown / content 渲染
internal/skills/        内置 skill 与附属资产
```

## License

MIT
