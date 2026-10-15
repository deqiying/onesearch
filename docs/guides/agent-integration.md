# Agent 接入

[文档导航](../README.md) · [English](agent-integration.en.md)

本文介绍 CLI + Skill 路径。支持原生 MCP tools 的 host 可改用 [MCP 接入指南（中英）](mcp.md)：`onesearch mcp` 已在源码实现，npm `1.0.6` 尚不包含本功能，发布前需本地构建 binary；host / 发布 / live 验收尚待完成。

## 安装薄入口

1. 按[上手指南](getting-started.md)安装 CLI 并完成所需能力的 readiness 配置。
2. 在 agent host 支持的自定义 Skills 目录中创建 `onesearch` 文件夹。
3. 将 [host Skill 模板](../assets/onesearch-host-skill.md)的完整原文保存为该文件夹的 `SKILL.md`，保留 frontmatter；不要复制网页渲染后的文本。
4. 如果 host 需要重新发现 Skill，按其机制刷新或重启。

模板只负责定位实际可执行文件、读取版本并加载同一个 binary 内的 `onesearch` router。不要将 provider 合同、端点配置或命令参数复制进 host Skill，也不要把加载 Skill 当成联网、安装或配置修改的授权。

## 读取内置 Skill

```powershell
onesearch skills list --format json
onesearch skills list --capability page_fetch --format json
onesearch skills show onesearch --format content
onesearch skills show onesearch --file references/agent-execution-contract.md --format content
onesearch skills show exa --format content
```

内置 `onesearch` 是薄 router。Workflow Skill 覆盖 `search`、`docs`、`fetch`、Deep Research；provider Skill 覆盖 Exa、Tavily、Firecrawl、Context7、DeepWiki、AnySearch、Zhipu、DDG、Freecrawl。

`skills show` 默认读 `SKILL.md`；`--file <relative-path>` 读取目标 Skill 内的一个内置文件。除显式 `--output` 外，发现过程只读、离线，不初始化 provider 配置。返回的 Markdown 是工作流指导，不是新注册的 host Skill。

## 版本与失败边界

可执行文件路径和版本不变时复用已加载 router，不递归重复加载。只在 router 选择子工作流或 provider 后加载该子 Skill。

如果 CLI 缺失或 canonical `skills show` 失败，应报告已解析路径、可得版本、退出码和简短错误；不得猜测别名或参数、静默升级，也不得回退到复制的旧合同。符合任务语义且获授权时，可以改用 host 自带的搜索能力，否则报告缺少的前提。

CLI 输出、argv 绑定、证据策略见 [CLI 参考](../reference/cli.md)。

## 维护边界

- 模板是仓库文档的单一来源；[npm 主包 README](../../npm/onesearch/README.md)内嵌同样的模板，供脱离仓库发布时使用，变更时需同步。[兼容包 README](../../npm/deqiying-onesearch/README.md)仅链接仓库的 Agent 接入入口，不另存模板。
- 内置工作流源文件保留在 [internal/skills/assets](../../internal/skills/assets/)，由 [skills.go](../../internal/skills/skills.go)嵌入 binary，不迁入 `docs/`。
- [内置 Skill 设计记录](../decisions/builtin-skills-agent-optimization-plan.md)保留路由、readiness 与按需加载的设计理由；[skills 测试](../../internal/skills/skills_test.go)核对资源与命令合同。
