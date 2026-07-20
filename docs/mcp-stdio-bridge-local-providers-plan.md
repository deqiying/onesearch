# MCP Stdio Bridge 本地 Provider 接入技术方案

## 背景

`onesearch` 当前是 CLI-first 的研究与证据工具，核心能力通过 workflow 命令和 provider-direct 命令暴露。已有 provider 主要通过 HTTP API、HTTP JSON-RPC 或固定 SDK 风格协议调用，例如 `exa`、`tavily`、`firecrawl`、`context7`、`deepwiki`、`anysearch` 和 `zhipu`。

本方案要把两个已经可作为 MCP stdio server 运行的本地工具接入 `onesearch`：

- `ddg-search`：开源项目 `nickclyde/duckduckgo-mcp-server`，通过 `uvx duckduckgo-mcp-server --transport stdio` 暴露 DuckDuckGo 搜索和网页内容抓取能力。
- `freecrawl`：通过 `uvx freecrawl-mcp` 运行，并使用 `FREECRAWL_TRANSPORT=stdio` 明确 stdio transport；本机会话中已暴露 `search`、`scrape`、`crawl`、`deep_research` 类工具。

这里的“本地端点”指 `onesearch` 自己启动 MCP stdio 子进程并调用工具，而不是要求 Codex 再配置同一 MCP server。工具本身仍然可能访问公网；“本地”只表示桥接进程在本机启动。

## 目标

1. 新增通用 MCP stdio bridge，让 `onesearch` 可以按配置启动本地 MCP server，并发起 `tools/call`。
2. 把 `ddg` 和 `freecrawl` 作为独立 provider-direct 命令接入。
3. 默认不把这两个 provider 自动并入 `search`、`fetch`、`crawl` 等 workflow 路由，避免影响现有 provider 选择。
4. 允许用户通过 `onesearch` 配置显式启用、显式加入 workflow routes。
5. 保持 Codex MCP 配置与 `onesearch` 配置解耦：`onesearch` 可独立使用这些本地 provider。

## 非目标

- 不复刻 `duckduckgo-mcp-server` 的 Python 实现。
- 不复刻 `freecrawl-mcp` 的爬取、渲染、反爬或 deep research 实现。
- 不把 MCP server 变成长期后台 daemon。
- 不在本阶段支持远程 MCP HTTP/SSE server。
- 不把 `uvx ...@latest` 作为强制默认行为；版本固定由用户配置决定。

## 当前代码边界

当前接入点主要在以下模块：

- `internal/config/runtime.go`：runtime schema、provider 定义、routes、availability。
- `internal/cli/provider_commands.go`：provider-direct 命令分发、`status.direct_endpoints` 生成。
- `internal/service/provider_direct.go`：provider-direct service 方法。
- `internal/service/service.go`：workflow fallback、provider 解析、capability 状态。
- `internal/providers/`：各 provider 的协议封装与结果归一化。
- `internal/skills/assets/`：面向 agent 的 skill 文档和命令合同。

现有结构适合新增一个通用 `mcp_stdio` adapter，再由 `ddg`、`freecrawl` 两个 provider 负责工具参数和结果归一化。

## 复杂度判断

| 方案 | 复杂度 | 维护成本 | 结论 |
| --- | --- | --- | --- |
| MCP stdio bridge | 中 | 中 | 推荐先做。只实现 MCP 客户端、配置、命令分发和结果归一化。 |
| 复刻 `duckduckgo-mcp-server` | 中高 | 高 | 需要维护 DDG 搜索解析、速率限制、正文抽取、fetch backend 差异。 |
| 复刻 `freecrawl-mcp` | 高 | 高 | 需要维护抓取、JS/headless、anti-bot、crawl、deep research 等复杂行为。 |

`duckduckgo-mcp-server` 是 Python 项目并不直接导致接入复杂；复杂的是复刻其行为。通过 stdio bridge 调用 MCP server，可以把这部分复杂度保留在上游工具里。

## 总体设计

新增三层：

1. `internal/mcpstdio`
   - 负责启动 stdio MCP server。
   - 完成 `initialize`、`initialized`、`tools/list`、`tools/call`、shutdown。
   - 处理 JSON-RPC newline framing、超时、stderr 采集和错误归类。

2. `internal/providers/mcp_stdio.go`
   - 提供通用 `MCPStdio` client wrapper。
   - 提供 `CallTool(ctx, toolName, arguments)`。
   - 提供 MCP 内容抽取函数：`content[]` text、structured JSON text、plain text。

3. provider-specific wrapper
   - `DDG`：`Search`、`FetchContent`。
   - `Freecrawl`：`Search`、`Scrape`、`Crawl`、`DeepResearch`。
   - wrapper 只做参数映射、tool name 映射和输出归一化。

### 进程模型

第一阶段采用“每次 CLI 调用启动一次 MCP server”的短生命周期模型：

```text
onesearch ddg search
  -> exec.CommandContext("uvx", ...)
  -> MCP initialize
  -> tools/call
  -> shutdown / kill on timeout
  -> normalize result
```

这个模型启动成本略高，但足够简单、可测试，也不会引入本地 daemon 生命周期问题。后续如果频繁调用成本明显，再考虑进程池或本地 proxy。

### 安全边界

- 只能通过配置中的 `command` 和 `args` 启动进程，不通过 shell 拼接命令。
- `env` 从 `settings.env` 读取，并叠加当前环境；不在输出中打印 env 值。
- stdout 仅用于 JSON-RPC；stderr 只在 `--verbose` 诊断中截断展示。
- 默认 provider 不自动加入 workflow routes，避免 agent 在普通搜索中意外触发本地可执行程序。

## 配置设计

新增 adapter：

```go
const AdapterMCPStdio = "mcp_stdio"
```

provider 配置继续使用现有 runtime schema，把 MCP stdio 细节放进 `settings`，避免扩大顶层 schema：

```json
{
  "providers": {
    "ddg": {
      "enabled": true,
      "adapter": "mcp_stdio",
      "capabilities": ["source_search", "page_fetch"],
      "api_key": "",
      "api_key_env": "",
      "aliases": ["ddg-search", "duckduckgo", "duckduckgo-mcp"],
      "settings": {
        "direct_only": true,
        "anonymous_allowed": true,
        "timeout_seconds": 60,
        "command": "uvx",
        "args": ["duckduckgo-mcp-server@latest", "--transport", "stdio"],
        "env": {
          "DDG_SAFE_SEARCH": "MODERATE",
          "DDG_REGION": "cn-zh"
        },
        "tools": {
          "search": "search",
          "fetch_content": "fetch_content"
        }
      }
    },
    "freecrawl": {
      "enabled": true,
      "adapter": "mcp_stdio",
      "capabilities": ["source_search", "page_fetch", "site_crawl"],
      "api_key": "",
      "api_key_env": "",
      "aliases": ["freecrawl-mcp"],
      "settings": {
        "direct_only": true,
        "anonymous_allowed": true,
        "timeout_seconds": 160,
        "command": "uvx",
        "args": ["freecrawl-mcp"],
        "env": {
          "FREECRAWL_TRANSPORT": "stdio",
          "FREECRAWL_HEADLESS": "true",
          "FREECRAWL_TIMEOUT": "160",
          "PYTHONIOENCODING": "utf-8"
        },
        "tools": {
          "search": "mcp__freecrawl__search",
          "scrape": "mcp__freecrawl__scrape",
          "crawl": "mcp__freecrawl__crawl",
          "deep_research": "mcp__freecrawl__deep_research"
        }
      }
    }
  }
}
```

### 默认配置策略

代码内置默认 provider 建议存在，但保持：

- `enabled: false`
- `settings.direct_only: true`
- `settings.anonymous_allowed: true`

这样新版本 `config list` 能看到可用配置模板，但不会在用户未确认时执行 `uvx`、下载包或改变普通 workflow 路由。

用户本机要启用时，只需要在 `~/.config/onesearch/config.json` 或 `ONESEARCH_CONFIG_DIR` 指向的配置里覆盖 `enabled: true`，并按需调整 `args`、`env`。

### direct_only 行为

当前 `routesWithProviderCapabilities` 会把声明了 capability 的 provider 自动追加到对应 route。为了满足“单独端点”的边界，需要新增规则：

- `settings.direct_only == true` 时，不自动追加到 workflow routes。
- 如果用户显式把 provider 写进 `routes.source_search`、`routes.page_fetch` 或 `routes.site_crawl`，仍允许 workflow 使用。

示例：

```json
{
  "routes": {
    "source_search": ["ddg", "exa", "zhipu", "tavily", "firecrawl"],
    "page_fetch": ["ddg", "freecrawl", "tavily", "firecrawl"],
    "site_crawl": ["freecrawl", "tavily", "firecrawl"]
  }
}
```

## 命令合同

新增 provider-direct 命令：

| Provider | 命令 | MCP tool | 主要用途 |
| --- | --- | --- | --- |
| `ddg` | `onesearch ddg search "query"` | `search` | DuckDuckGo 来源发现 |
| `ddg` | `onesearch ddg fetch-content "url"` | `fetch_content` | 网页正文抓取 |
| `freecrawl` | `onesearch freecrawl search "query"` | `mcp__freecrawl__search` | 搜索，可选抓取结果 |
| `freecrawl` | `onesearch freecrawl scrape "url"` | `mcp__freecrawl__scrape` | 单页抓取、JS、anti-bot、格式选择 |
| `freecrawl` | `onesearch freecrawl crawl "url"` | `mcp__freecrawl__crawl` | 站点爬取 |
| `freecrawl` | `onesearch freecrawl deep-research "topic"` | `mcp__freecrawl__deep_research` | 多来源研究；第一阶段仅作为 direct 命令，不新增 workflow capability |

### ddg 参数

```powershell
onesearch ddg search "OpenAI Responses API" --max-results 10 --region us-en --format json
onesearch ddg fetch-content "https://example.com" --start-index 0 --max-length 8000 --backend auto --format json
```

字段映射：

- `--max-results` -> `max_results`
- `--region` -> `region`
- `--start-index` -> `start_index`
- `--max-length` -> `max_length`
- `--backend` -> `backend`

### freecrawl 参数

```powershell
onesearch freecrawl search "OpenAI docs" --num-results 5 --search-engine duckduckgo --scrape-results --format json
onesearch freecrawl scrape "https://example.com" --formats markdown,text --javascript --anti-bot --cache --timeout 60000 --format json
onesearch freecrawl crawl "https://docs.example.com" --max-depth 2 --max-pages 20 --same-domain-only --format json
onesearch freecrawl deep-research "browser automation MCP comparison" --num-sources 8 --max-depth 3 --include-academic --format json
```

字段映射：

- `search`: `query`、`num_results`、`scrape_results`、`search_engine`
- `scrape`: `url`、`formats`、`javascript`、`anti_bot`、`cache`、`timeout`、`wait_for`
- `crawl`: `start_url`、`max_depth`、`max_pages`、`same_domain_only`、`include_patterns`、`exclude_patterns`
- `deep-research`: `topic`、`num_sources`、`max_depth`、`include_academic`、`search_queries`

## 输出归一化

provider-direct JSON 保持当前风格：

```json
{
  "ok": true,
  "provider": "ddg",
  "tool": "search",
  "query": "example",
  "results": [],
  "content": "",
  "raw_content": "",
  "elapsed_ms": 123.4
}
```

归一化规则：

- MCP `result.content[].text` 合并为 `content`。
- 如果 `content` 是 JSON 字符串，尝试解析出 `results`、`pages`、`markdown`、`text` 等字段。
- 如果只得到纯文本，保留 `raw_content`，并用 URL 正则提取候选 `results`。
- `--format content` 输出正文；`--format json` 输出 envelope。
- `--verbose` 才输出 MCP 初始化摘要、tool name、stderr 截断和 raw result。

## Status 与 Doctor

需要调整 `status.direct_endpoints` 的可用性计算：

- provider-direct availability 不应依赖 provider 是否在 workflow route 中。
- 对 `mcp_stdio` provider，availability 至少检查：
  - `enabled != false`
  - `adapter == "mcp_stdio"`
  - `settings.command` 非空
  - `settings.tools` 中存在对应 public command 的 tool name
  - 可选：`exec.LookPath(command)` 成功；失败时返回 `missing_command`

不建议 `status` 真正启动 MCP server，因为 `uvx` 可能下载包、拉网络或启动 headless 依赖。启动检查放到实际命令执行阶段。

## Codex MCP 配置关系

做了 `onesearch` MCP stdio bridge 后，Codex 不再必须配置：

```toml
[mcp_servers.ddg-search]
command = "uvx"
args = [ "duckduckgo-mcp-server@latest", "--transport", "stdio" ]

[mcp_servers.freecrawl]
command = "uvx"
args = [ "freecrawl-mcp" ]
```

如果保留这些 Codex MCP 配置，Codex 仍可直接调用原生 MCP tools；但 `onesearch` 会独立读取自己的 `config.json` 并启动自己的子进程。两套配置互不复用，也不会共享进程。

推荐迁移顺序：

1. 先在 `onesearch` 配置中启用 `ddg` 和 `freecrawl`。
2. 用 `onesearch status --format json` 确认 direct endpoint 配置有效。
3. 用 provider-direct 命令做最小 smoke。
4. 稳定后再决定是否删除 Codex 原生 MCP 配置。

## 实现步骤

1. 配置层
   - 新增 `AdapterMCPStdio`。
   - 新增 `mcp_stdio` availability 校验。
   - 新增 `direct_only` route 自动注册跳过逻辑。
   - 新增内置 `ddg`、`freecrawl` provider 模板。

2. MCP bridge
   - 新增 `internal/mcpstdio`。
   - 实现 subprocess 启动、JSON-RPC request/response、initialize、tools/call、shutdown。
   - 统一 timeout、stderr 截断、JSON-RPC error 映射。

3. Provider wrapper
   - 新增 `internal/providers/mcp_stdio.go`。
   - 新增 `internal/providers/ddg.go`。
   - 新增 `internal/providers/freecrawl.go`。

4. Service 与 CLI
   - 在 `providerToolAliases` 增加 `ddg`、`freecrawl`。
   - 增加 `runDDGGroup`、`runFreecrawlGroup`。
   - 增加 `Service.DDGSearch`、`Service.DDGFetchContent`、`Service.FreecrawlSearch`、`Service.FreecrawlScrape`、`Service.FreecrawlCrawl`、`Service.FreecrawlDeepResearch`。

5. 文档与 skill
   - 更新 `README.md` provider-direct 列表。
   - 更新 `internal/skills/assets/onesearch-cli/references/cli-contract.md`。
   - 新增 `onesearch-ddg`、`onesearch-freecrawl` provider skill，或先把用法收敛进 `onesearch-fetch` / `onesearch-search` 后续再拆分。

## 验证方案

### 单元测试

- `internal/mcpstdio`
  - fake stdio MCP server 返回 `initialize`、`tools/list`、`tools/call`。
  - 覆盖 JSON-RPC error、非 JSON stdout、stderr 截断、timeout。

- `internal/config`
  - `mcp_stdio` adapter 被支持。
  - `direct_only` provider 不自动追加到 routes。
  - 显式写入 routes 时可被 workflow 解析。
  - `missing_command`、`missing_tool_mapping` 能正确诊断。

- `internal/cli`
  - `ddg search`、`ddg fetch-content`、`freecrawl scrape`、`freecrawl crawl` 参数校验。
  - `status.direct_endpoints.ddg/freecrawl.commands` 包含新命令。

- `internal/service`
  - 使用 fake MCP stdio command 验证 direct 方法输出 envelope。
  - 验证 workflow 默认不会调用 direct-only provider。

### 本机 smoke

```powershell
$env:GOCACHE = Join-Path (Get-Location) ".gocache"
go test ./...
go run .\cmd\onesearch status --format json
go run .\cmd\onesearch ddg search "OpenAI Responses API" --max-results 2 --format json
go run .\cmd\onesearch ddg fetch-content "https://example.com" --max-length 1000 --format json
go run .\cmd\onesearch freecrawl scrape "https://example.com" --formats markdown --format json
```

`freecrawl crawl` 和 `freecrawl deep-research` 可能耗时更长，应放在手动验证或 live smoke，不作为默认 `go test` 依赖。

## 风险与取舍

- `uvx ...@latest` 可复现性弱。建议用户本机可先用 latest，发布默认文档给出可 pin 版本写法。
- `status` 不启动 MCP server，因此只能证明配置形态有效，不能证明上游包、网络和页面抓取一定成功。
- MCP stdio stdout 中如果混入非 JSON 日志会破坏协议，需要 bridge 对非 JSON 行给出明确错误。
- `freecrawl` 可能启动 headless 浏览器，超时和资源占用需要独立控制。
- `ddg` 搜索结果是发现候选，不是事实证明；高风险结论仍应 fetch 页面正文。
- 保留 Codex MCP 与 `onesearch` bridge 会产生两套配置，需要文档明确迁移边界。

## 参考资料

- `nickclyde/duckduckgo-mcp-server`: https://github.com/nickclyde/duckduckgo-mcp-server
- MCP stdio transport specification: https://modelcontextprotocol.io/specification/2025-03-26/basic/transports
- MCP lifecycle specification: https://modelcontextprotocol.io/specification/2025-03-26/basic/lifecycle
