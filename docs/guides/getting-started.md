# 上手指南

[文档导航](../README.md) · [English](getting-started.en.md)

适用于安装后的 CLI；示例使用 PowerShell，联网命令需要可用的 provider。只读命令发现不需要凭据。

## 1. 安装与静态发现

```powershell
npm install -g onesearch
onesearch --version
onesearch schema search --format json
```

npm launcher 通过 optional dependencies 使用 Windows x64、Linux x64 或 macOS arm64 预构建二进制；其他构建见项目 [GitHub Releases](https://github.com/deqiying/onesearch/releases)。`schema` 离线执行，不创建配置文件。

## 2. 初始化并检查 readiness

```powershell
onesearch status --format json --pretty
```

普通命令会创建缺失的 runtime 配置。首次 `ready: false` 和 `minimum_profile.ok: false` 是预期状态，不代表安装失败。

| 能力 | 角色 | Provider 选择 / 初始状态 |
| --- | --- | --- |
| `answer_search` | `standard` 必需 | 配置 `xai`、`openai-compatible` 或 `openai-responses` |
| `docs_search` | `standard` 必需 | 配置 Exa 或 Context7 |
| `page_fetch` | `standard` 必需 | 配置 Exa、Tavily 或 Firecrawl；AnySearch 初始已启用 |
| `repo_wiki` | 独立能力 | DeepWiki 默认启用，公开仓库通常可匿名使用 |
| `vertical_search` | 可选实验能力 | AnySearch 默认启用 |

初始配置只启用 DeepWiki 和 AnySearch。其他 provider 的凭据、启用状态与路由配置见[配置参考](../reference/configuration.md)。

## 3. 可选：先研究公开仓库

当 `capabilities.repo_wiki.ok == true` 或 `direct_endpoints.deepwiki.available == true` 时，可以在完整 `standard` profile 就绪之前运行：

```powershell
onesearch repo-wiki "microsoft/playwright" `
  "MCP Browser Server 的架构是什么？" `
  --provider deepwiki --format json
```

DeepWiki 提供生成式仓库上下文、架构摘要、wiki structure 和 wiki contents，仅属于辅助上下文。精确实现结论应回到源码核对；私有仓库可能需要凭据。

## 4. 配齐完整搜索

一个答案 provider 加 Exa 可以覆盖 `answer_search`、`docs_search`、`page_fetch` 和可选的 `source_search`：

```powershell
onesearch config setup openai-compatible
onesearch config setup exa
onesearch status --format json --pretty
```

交互提示隐藏 API key。若端点使用 xAI 或 OpenAI Responses adapter，可替换对应的 setup 目标；兼容网关需显式提供 API 根路径：

```powershell
onesearch config setup openai-compatible `
  --base-url "https://gateway.example.com/v1"
```

执行前检查：

- 完整搜索：`minimum_profile.ok == true`。
- 能力命令：`capabilities.<capability>.ok == true`。
- Provider-direct 命令：`direct_endpoints.<provider>.available == true`。

这些字段只表示本机 preflight 通过，不证明远端可达。

## 5. 第一次搜索

```powershell
onesearch search "解释 Go context cancellation 的常用模式" `
  --validation balanced --format json
```

只有 `capabilities.source_search.ok == true` 时才添加 `--extra-sources 2`。发现候选 URL 后先抓取关键页面，再形成结论。

## 继续阅读

- [CLI、provider 命令与证据策略](../reference/cli.md)
- [Agent 接入与内置 Skill](agent-integration.md)
- [配置与 readiness 排查](../reference/configuration.md)

实现依据：[命令入口](../../internal/cli/cli.go)、[配置初始化](../../internal/config/config.go)、[runtime 能力与 profile](../../internal/config/runtime.go)。
