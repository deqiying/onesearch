# onesearch 项目协作规则

本文件适用于仓库根目录及所有子目录；若子目录存在更具体的 `AGENTS.md`，以更近的规则为准。

## 工具链与改动范围

- 运行项目命令前，以 `mise.toml` 和 `go.mod` 声明的工具链为准。
- 采用满足需求的最小改动，保留无关工作区变更，不做顺手重构或大范围格式化。
- 修改 CLI、配置 schema、命令示例或内置 Skill 后，同步更新直接受影响的文档与测试。

## 文档路径

- `README*`、`docs/**`、`npm/**/README*`、`internal/skills/**/*.md` 及其他文档不得包含开发机、用户目录或工作区的绝对文件系统路径。
- 仓库内文件链接使用相对于当前文档的路径；从仓库根目录执行的命令使用相对路径。
- 缓存、配置和临时目录示例优先使用环境变量、`Get-Location`、`~` 或 `<repo-root>` 等可移植表达方式，不写入个人机器上的实际路径。
- URL、HTTP API 路径和 Context7 的 `/org/project` 标识不属于文件系统路径，不应为满足本规则而改写。

## 验证

- 文档变更后检查所有文档中的 Windows drive path、UNC path 和 Unix user-directory path，并人工排除 URL、API 路径等非文件系统标识。
- 提交前检查目标文件 diff，并运行 `git diff --check`。
- Go 行为发生变化时，至少运行与改动范围匹配的测试；影响全局行为时运行 `go test ./...`。
