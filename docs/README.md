# 文档导航 / Documentation

[项目首页](../README.zh-CN.md) · [English README](../README.md)

本目录将**当前使用说明**与**设计决策记录**分开。用户指南提供中英文版本；开发、架构与设计记录以中文为主。命令参数以实际安装版本的 `onesearch schema <canonical path>` 为准，配置以本机 `config list` 和仓库实现为准。

Current user guides are available in Chinese and English below. Development, architecture, and design records are primarily in Chinese. Historical plans are not the current operating manual.

## 按目标阅读 / Find your task

| 目标 / Task | 简体中文 | English |
| --- | --- | --- |
| 安装、配置 readiness、完成首次搜索 / Get started | [上手指南](guides/getting-started.md) | [Getting started](guides/getting-started.en.md) |
| 将 CLI 接入 Agent / Connect an agent | [Agent 接入](guides/agent-integration.md) | [Agent integration](guides/agent-integration.en.md) |
| 查询配置、路由与 provider 限制 / Configure providers | [配置参考](reference/configuration.md) | [Configuration reference](reference/configuration.en.md) |
| 查询命令、输出和证据策略 / Use the CLI | [CLI 参考](reference/cli.md) | [CLI reference](reference/cli.en.md) |
| 构建、测试、维护文档与了解发布边界 / Develop | [开发与验证](guides/development.md) | Chinese guide |
| 理解模块和执行边界 / Understand architecture | [架构概览](architecture/README.md) | Chinese overview |
| 追溯理由、兼容取舍和替代关系 / Trace decisions | [设计记录](decisions/README.md) | Chinese records |

可复制资源：[host Skill 模板](assets/onesearch-host-skill.md)。配置示例：[config.example.json](../config.example.json)。

## 文档归属

```text
README.md / README.zh-CN.md     项目简介、最短上手路径和入口
 docs/
   README.md                   阅读导航与维护归属
   guides/                     按任务操作的指南
   reference/                  配置、CLI 与输出合同
   architecture/               当前系统边界与模块职责
   decisions/                  设计理由、状态和替代关系
   assets/                     可复制的文档资源
```

- 当前事实在对应指南或参考页详细维护，根 README 只保留入口。中文页面使用主题名 `.md`，英文对应页面使用 `.en.md`，行为或示例变化需同步两种语言。
- `decisions/` 沿用项目原有技术方案格式，不强制转换为 Agent Note，也不将历史验证结果视为本次验证。仍有效的理由继续保留；被替代的合同通过状态和相互链接说明。
- [npm 主包 README](../npm/onesearch/README.md)与[兼容包 README](../npm/deqiying-onesearch/README.md)随包独立发布，保留可独立阅读的安装说明，不改为指向包外的相对链接。主包内嵌 host 模板，兼容包仅链接仓库的 Agent 接入入口。
- [内置 Skills](../internal/skills/assets/)由 Go 嵌入到 binary，属于产品资源，保留原目录。仓库 [README 图片](../assets/readme/)同样保留，避免无意义地改动入口资源路径。
- Release 压缩包仅附带英文根 README 和 LICENSE，不包含完整文档树；完整阅读需使用源码仓库。验证方法与发布边界见[开发指南](guides/development.md)。
