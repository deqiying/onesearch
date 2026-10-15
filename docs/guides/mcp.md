# MCP 接入 / MCP integration

[文档导航 / Documentation](../README.md) · [CLI Agent 接入](agent-integration.md) · [CLI agent integration](agent-integration.en.md)

## 发布与启动边界 / Release and launch boundary

当前源码实现本地 stdio MCP Server，入口为 `onesearch mcp`，不带运行 flags，也没有 `serve` 子命令。现有 npm launcher 原样转发参数，无需独立 MCP 包。**当前仓库 npm 清单版本 `1.0.6` 不包含新功能**；以下 npx 配置是包含该功能的版本发布后的用法，不是当前公开包可用性的承诺。

The source tree implements a local stdio MCP server at `onesearch mcp`, with no runtime flags or `serve` subcommand. The existing npm launcher forwards arguments. The repository's current npm manifest version, `1.0.6`, does not include this feature; the npx example below is for use after a release containing it.

### 发布后的 host 配置 / Host configuration after release

```json
{
  "mcpServers": {
    "onesearch": {
      "command": "npx",
      "args": ["-y", "onesearch", "mcp"]
    }
  }
}
```

`-y` 避免首次下载等待安装确认。正式使用时将 `onesearch` 替换为包含本功能且**实际已发布**的固定版本（`onesearch@<published-version>`），不要猜测版本号。npx 可能下载包，必须遵循 host 和用户的安装授权。

Use a verified, published version containing the feature (`onesearch@<published-version>`) for reproducible production use; no such version is asserted here. `-y` skips npm's install confirmation, so package download must already be authorized.

### 发布前本地构建 / Before release: local binary

按[开发指南](development.md)的工具链，从仓库根目录构建；Windows PowerShell 示例：

```powershell
mise exec -- go build -o ./bin/onesearch.exe ./cmd/onesearch
```

将 host 的 `command` 改为本地构建 binary 的路径，`args` 改为 `["mcp"]`；路径相对 host 的工作目录解析，不一定是仓库目录。macOS/Linux 构建输出使用 `./bin/onesearch`。不要在发布前以 npx 拉取的旧 binary 验收新入口。

Build from the repository root using the pinned toolchain. Point the host's `command` at the built executable and use `args: ["mcp"]`; resolve relative paths against the host's working directory. On macOS/Linux use `./bin/onesearch` as the build output. A downloaded older package cannot validate the new source implementation.

**Windows host 尚未实测。** 某些 host 需要 `npx.cmd` 或通过 `cmd` 启动批处理文件；应遵循该 host 的 executable、`cmd` 参数和 quoting 规则，不能将上面的通用 JSON 当成已验证的 Windows 通用配置。

Windows host integration has not been tested. Follow your host's requirements for `npx.cmd`, a `cmd` wrapper, argument handling, and quoting; the generic JSON is not a verified universal Windows recipe.

## 配置与发现 / Configuration and discovery

在 MCP 外部用 CLI 配置 provider，例如 `onesearch config setup openai-compatible`、`onesearch config setup exa`；凭据与配置目录见[配置参考](../reference/configuration.md) / [configuration reference](../reference/configuration.en.md)。host 必须继承正确的环境（包括可选的 `ONESEARCH_CONFIG_DIR`），不要把 API key 写入工具参数。

- 启动、协议发现和 `tools/list` 不读配置、不联网、不创建配置文件；缺配置也能发现固定的 5 个工具。
- 每次工具请求读取一个只读配置快照，不写配置；后续请求会看到外部配置更新，执行中的请求不混用新旧配置。
- `onesearch_status` 只做本机检查，不联网、不创建配置；不同于可能初始化缺失配置的 CLI `onesearch status`。

Configure providers outside MCP with the CLI. Discovery does not read configuration, contact providers, or create files. Each tool request uses one read-only configuration snapshot; later requests can see external changes without mixing configurations within an active request. MCP status is local-only and never initializes configuration, unlike CLI status.

## 五个工具 / Five tools

以 host 收到的 `tools/list` 为完整输入 schema，不要把 CLI flags 或 CLI manifest 直接作为 MCP 参数。未知字段及无关分支 options 会被拒绝。

Use `tools/list` for the complete MCP input schema, not CLI flags or the CLI manifest. Unknown fields and options belonging to another branch are rejected.

| 工具 / Tool | 必填与分支 / Required inputs and branches |
| --- | --- |
| `onesearch_search` | `mode`、`query` 必填。`mode=answer` 使用配置路由与 minimum profile，可带 `answer_options`；`sources` 固定 Exa，可带 `sources_options`；`docs` 固定 Context7，必须带 `docs_options`。 / Explicit mode and query; answer uses configured routes, sources uses Exa, docs uses Context7. |
| `onesearch_fetch` | `url` 必填；可选 `provider`，按 page-fetch 路由抓单页正文。 / Fetch one known URL, not a site crawl. |
| `onesearch_site` | `action`、`url` 必填，`action=map/crawl`；可选 `provider`、`max_depth`、`limit`。`map_options` 仅允许用于 map。 / Choose map or crawl explicitly; map options are forbidden for crawl. |
| `onesearch_repo_wiki` | `repo`、`mode` 必填，`mode=ask/structure/contents`；`ask` 必须带 `question`，其他模式禁止 `question`。 / Ask requires a question; structure and contents forbid it. |
| `onesearch_status` | 无参数，用 `{}`；返回配置与各 operation 的本机 readiness。 / No parameters; local readiness only. |

### 文档检索分两次 / Two-step documentation lookup

`library_name` 只返回候选，不自动选库或继续查询。选择返回的明确 ID 后，另发请求使用 `library_id`；两者互斥，不猜 ID。

A library-name request only resolves candidates. Select an explicit returned ID and issue a second call; never invent an ID or assume resolution fetched documentation.

```json
{"mode":"docs","query":"useEffect cleanup","docs_options":{"library_name":"react"}}
```

第二次请求 / Second request（`<selected-library-id>` 替换为候选中实际返回的 ID / replace with an actual returned candidate ID）：

```json
{"mode":"docs","query":"useEffect cleanup","docs_options":{"library_id":"<selected-library-id>"}}
```

### Readiness

读取 `onesearch_status` 结果的 `data.tools[tool_name]`：

- `available`：至少一个 operation 本机可用 / at least one operation is locally available。
- `fully_available`：所有 operation 本机可用 / all operations are locally available。
- `operations[operation].available`：所选分支是否可用，例如 `answer`、`sources`、`docs.resolve`、`docs.query`、`map`、`crawl`。
- `data.ready` 只表示 minimum profile 就绪，不代表所有工具或分支就绪。反之 `ready=false` 也不阻止独立就绪的分支。 / Overall ready covers only the minimum profile, not every branch; independent ready operations may still run when it is false.

这些检查不验证远端可达性、凭据有效性、额度或付费套餐。 / Readiness does not prove connectivity, valid credentials, quota, or subscription access.

## 限额、输出与错误 / Limits, output, and errors

- 输入 JSON-RPC 帧上限 **64 KiB**；完整序列化响应上限 **2 MiB**，包括协议外壳和结构化/文本结果，不只是正文。 / Limits apply to the input frame and complete serialized response, not just content.
- 同时最多 **4** 个工具调用；额外请求返回 `server_busy`，不无限排队。 / At most four concurrent calls; excess calls fail rather than queue indefinitely.
- 可选 `timeout` 单位为秒，满足 `0 < timeout <= 300`。`onesearch_site` 要求 integer，其他带 timeout 的工具接受 number；`onesearch_status` 无参数。默认 search/fetch **90**、map **150**、crawl **180**、repo_wiki **60** 秒。 / Timeout covers the entire call, including upstream work.
- 超时返回 `timeout`；结果过大返回 `result_too_large`，不要当成完整证据或静默截断后的成功。缩小查询、站点 limit 或改用单页 fetch。 / Narrow the request after an oversized-result failure; do not treat it as complete evidence.
- 业务结果使用 `ok`、`tool`、`operation`、`data`，失败带 `error_type` / `error`；工具失败使用 MCP `isError`。返回值经脱敏，但仍不应主动发送或回显凭据。 / Inspect both the MCP error marker and result envelope.

来源发现不是正文证据；重要结论需 fetch。crawl 返回 submitted job 只表示提交成功，不表示已取得页面。 / Source candidates are not verified page content, and a submitted crawl job is not completed page evidence.

## 协议与安全边界 / Protocol and safety boundaries

实现使用固定的 Go SDK `v1.8.0`；支持 `2026-07-28`、`2025-11-25`、`2025-06-18`、`2025-03-26`。最旧版本仅返回文本 JSON，无 `outputSchema` / `structuredContent`；其他版本支持结构化结果。现代 `2026-07-28` 使用逐请求版本元数据与 discovery，不要求 initialize；该版本无 `ping`，调用返回 JSON-RPC `-32601`，用 `tools/list` 检查存活。旧版本按 initialize / initialized 生命周期使用。

The pinned SDK is `v1.8.0`, with the four versions listed above. The oldest version is text-only. Modern `2026-07-28` uses per-request metadata/discovery without initialize and has no ping (`-32601`); use `tools/list` as the liveness check. Older versions use initialization negotiation.

现代 stdio 请求收到 `notifications/cancelled` 后不再发送该请求的响应；legacy 请求保留取消错误结果。超时不是客户端取消，仍返回 `timeout` 工具错误。 / After a cancellation notification, modern stdio suppresses the cancelled request's response; legacy calls retain their cancellation result. A server-side timeout still returns a tool error.

仅提供 **stdio + tools**：无 HTTP listener、resources、prompts、tasks、OAuth、任意 shell 或文件写工具。stdout 专用于协议，诊断使用 stderr。关闭 stdin / EOF 会结束会话并取消执行中的请求。工具本身不暴露 shell，不等于完全无副作用：网络请求可能收费，crawl 可能创建上游 job，已有配置可启动本地 provider 进程。不要未经授权安装 runtime、浏览器或依赖。

Only stdio/tools are exposed, with no HTTP listener, resources, prompts, tasks, OAuth, arbitrary shell, or file-writing tools. Network calls can incur fees, crawls can create upstream jobs, and configured providers can launch local processes. These effects require the normal authorization boundary; no silent dependency installation is authorized.

## 实施与验收边界 / Implementation and verification boundary

实现与测试路径见[批准设计的实施记录](../decisions/mcp-server-mode-technical-design.md)。全仓 Go 测试、vet 与定向 race 检测已通过。在 Windows x64、Node `v26.11.1` / npm `11.20.0` 上，[离线集成脚本](../../scripts/test-mcp.mjs)验证了四版本 binary 和现代/legacy 两版本本地 npx 链路，覆盖发现、docs 两步、错误恢复、取消、EOF 与 fake HTTP provider 清理；Linux x64/macOS arm64 仅完成交叉构建。

The full Go test suite, vet, and targeted race checks passed. On Windows x64 with Node `v26.11.1` / npm `11.20.0`, the offline fixture passed for all four binary protocols and modern/legacy local npx launch chains, including cancellation and EOF cleanup. Linux x64 and macOS arm64 were cross-compiled only.

这些验证不代表公开 npm 包、真实桌面 host、launcher 信号转发或 live provider 已验收；这些项目仍未执行。 / Public npm release, real desktop hosts, launcher signal forwarding, and live-provider acceptance remain unverified.
