# 配置参考

[文档导航](../README.md) · [English](configuration.en.md)

本文描述当前 runtime 配置，不是 `onesearch schema` 输出的 CLI command manifest。

## 位置与初始化

Windows、macOS、Linux 默认使用 `~/.config/onesearch/config.json`；`ONESEARCH_CONFIG_DIR` 可覆盖目录。首次普通命令会创建缺失的目录和配置并继续执行；`schema`、命令级 `--help`、`skills list/show` 在 runtime 加载前返回，不初始化配置。

初始 schema 只启用 `deepwiki` 和 `anysearch`。`xai`、两个 OpenAI adapter、Exa、Context7、Zhipu、Tavily、Firecrawl、DeepSeek、DDG 和 Freecrawl 默认禁用，普通 `search` 需先完成 provider 配置。

## Schema 与路由

版本字段之外有五个配置区域：

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

| 区域 | 作用 |
| --- | --- |
| `defaults` | pipeline、fallback、validation、minimum profile、超时、日志、重试和输出清理 |
| `pipelines` | `default`、`research`、`docs`、`crawl` 等能力链 |
| `routes` | 每个能力的 provider 顺序；声明 capability 的 provider 自动追加，除非 `settings.direct_only=true` |
| `profiles` | 最低能力集合；`standard` 要求 `answer_search`、`docs_search`、`page_fetch` |
| `providers` | adapter、capabilities、base URL、凭据来源、启用状态和特有 settings |

完整可移植示例见 [config.example.json](../../config.example.json)。运行 `config list` 查看本机合并后的配置，不要把示例文件误认为已经启用的凭据配置。

## 安全配置 provider

```powershell
onesearch config path --format json
onesearch config list --format json
onesearch config setup exa
onesearch doctor --format json
onesearch status --format json
```

交互式配置隐藏 API key。脚本与 pipeline 应通过 stdin 传递密钥；CLI 不提供容易将密钥写入 shell history 的 `--api-key` 参数：

```powershell
$env:EXA_API_KEY |
  onesearch config setup exa --api-key-stdin --format json

$env:OPENAI_COMPATIBLE_API_KEY |
  onesearch config setup openai-compatible `
    --api-key-stdin `
    --base-url "https://gateway.example.com/v1" `
    --format json
```

`config setup` 更新 key 并将目标 provider 设为 `enabled: "auto"`。model、capabilities、routes、provider settings 和禁用操作仍需直接编辑 `config.json`。`api_key` 与 `api_key_env` 同时存在时，前者优先。

## Adapter 限制

- `openai_responses` 调用 JSON-only standalone SearchRequest `/v1/alpha/search`；这是本项目 adapter 的协议选择，不意味着公共 Responses API 整体迁移。
- `openai_chat_completions` 调用 `/v1/chat/completions`，支持 JSON/SSE，并可透传 `tools`、`tool_choice`。
- 两者不互相降级；Alpha 失败进入普通跨 provider fallback。
- `openai_responses` 的 base URL 必须是 API 根路径；以 `/alpha/search` 或 `/responses` 结尾的完整 endpoint 会被拒绝。
- Context7 请求需要 API key。
- `deepseek_tools` 调用 DeepSeek `POST /v1/tools/search` 与 `/v1/tools/fetch`，凭据环境变量为 `DEEPSEEK_API_KEY`，默认禁用；上游按成功请求计费。base URL 必须是 API 根路径（默认 `https://api.deepseek.com`，也接受 `/v1` 后缀），完整 endpoint（如 `/v1/tools/search`）会被拒绝。有效 fallback 位于 `source_search` 末段，`page_fetch` 位于 AnySearch 之后、Exa 之前。
- DDG 与 Freecrawl 使用本地 `mcp_stdio` bridge，默认禁用且 direct-only；只有显式启用并加入 routes 才参与 workflow。Freecrawl 应固定部署版本，以运行时 tool discovery 为准。

## Readiness 排查

1. 用 `config path` 确认实际配置目录及环境变量覆盖。
2. 用 `config list`、`doctor` 检查 provider 的启用状态、凭据来源、adapter、capabilities 与 route。
3. 用 `status` 检查 `minimum_profile.ok`、`capabilities.<capability>.ok` 或 `direct_endpoints.<provider>.available`，按目标命令选择 gate。
4. 本机 gate 通过仍不等于远端可达。实际调用失败时检查错误与 `--verbose` 的 provider attempts；不要输出凭据值。`smoke` / `regression` 的本地自检也不证明远端可用，模式与副作用见 [CLI 参考](cli.md)。

初次配置流程见[上手指南](../guides/getting-started.md)，输出与错误边界见 [CLI 参考](cli.md)。

## 依据与决定

- [配置路径与初始化](../../internal/config/config.go)、[runtime 合并与 readiness](../../internal/config/runtime.go)
- [Provider setup 实现](../../internal/config/provider_setup.go)与[测试](../../internal/config/provider_setup_test.go)
- [安全配置设计](../decisions/cli-provider-setup-technical-design.md)、[Alpha Search 协议决定](../decisions/openai-responses-alpha-search-technical-design.md)
