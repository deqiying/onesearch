# Monorepo 文档分层

## 先识别真实工作区

读取实际工作区/构建声明、包清单、依赖和部署单元，确认各模块职责。`apps/`、`packages/` 只是示例名称；Java 多模块、Rust workspace、Python 多包和其他布局都映射到实际路径，不创建不存在的目录。

先按仓库规模选择一种布局，不同时维护两份相同内容：

- **前后端两个小项目、共同开发发布**：默认集中式 `docs/modules/<实际模块目录名>/`，共同内容放根级主题目录，见下方“小型前后端同仓”。
- **多个可独立维护、发布的应用或包**：默认就近式，根级 docs 管全局，各模块自己的 docs 管局部，目录示例如下。
- **已有文档布局**：沿用集中式或就近式约定，不因本技能自动迁移。小模块可以只用一个 README，不强建完整 docs。

无论采用哪种布局，全局事实只定义一次，局部事实归拥有该实现的模块；模块数量不是强制迁移的阈值。

## 多应用/包：就近式目录

```text
README.md
docs/                          # 根级：整仓及跨模块内容
  README.md                    # 阅读入口，链接根级主题和工作区入口
  overview.md                  # 产品边界、应用/包职责概览
  architecture/
    README.md                  # 依赖方向、部署单元、共享边界
    integration.md             # 跨模块契约和端到端数据流
  guides/
    development.md             # 整仓环境、构建、联调和工作目录
    testing.md                 # 测试分层、跨包集成测试
  reference/
    configuration.md           # 仓库共享配置；局部配置链接到所属模块
  operations/
    deployment.md              # 系统部署顺序、集成配置、回滚约束
apps/
  web/
    README.md                  # 职责、最短运行命令、局部文档入口
    docs/
      README.md                # 该应用文档导航
      architecture/README.md   # 应用内部结构
      reference/configuration.md
      operations/deployment.md # 仅应用特有步骤
packages/
  client/
    README.md                  # 包用途、支持范围、最小使用示例
    docs/
      README.md
      reference/interfaces.md  # 该包公开接口、错误和兼容边界
      guides/development.md    # 包特有构建/调试事项
.agents/notes/                 # 默认整仓共用一棵决策树
```

各级 docs 均采用 [单项目主题划分](docs-structure.md) 的必要子集。上图是位置规范，不是必须生成的清单。

## 归属规则

| 内容 | 详细说明归属 |
|---|---|
| 整仓安装、任务编排、共享工具链、统一测试入口 | 根级 docs |
| 模块地图、依赖方向、跨服务数据流 | 根级 architecture |
| 一个包公开的接口与错误语义 | 提供方局部 reference；消费者链接过去 |
| 应用特有配置、启动、部署或排障 | 应用局部 docs |
| 跨包协议及双方配合的时序 | 根级 integration；局部页面只说明自身职责 |
| 所有模块共同使用的规范 | 根级一处定义；局部只写真实差异 |

根 `docs/README.md` 链接各工作区 README 或局部 docs 入口；局部入口链接返回根级指南。路径随真实层级计算。包需要脱离仓库发布时，检查发布包是否包含被链接文件；不能随包携带的仓库相对链接改用项目已有的发布文档地址或保留必要说明。

## notes 的 Monorepo 规则

- 默认统一放根级 `.agents/notes/`；不为了包名增加分类层级，仍为 `{lifecycle}/{class}/日期-主题.md`。
- 在标题或主题文件名中包含必要的模块名；正文按需加入 `## Scope`，用真实路径列出提供方、消费者和受影响的 docs。
- 跨包决定只记一篇，各包通过链接引用。不要把同一决定复制到每个包，也不要在根与包内各维护一份状态。
- 宿主已有包级 notes 时沿用；先查根与相关包的决定。每次原生格式校验只针对一个 notes 根，需要检查多个根时逐个设置 `AGENT_NOTE_ROOT`。跨根链接另行检查。

## 跨包变更收尾

1. 从实际依赖与调用关系确定受影响包，不把“本次改了哪些文件”当成完整影响范围。
2. 检查提供方的接口/配置说明、消费方示例和根级集成/部署文档。
3. 新建、重命名或删除包时同步模块地图、导航、运行命令及相关 notes 的路径。
4. 命令标明仓库根或包目录；从声明的工作目录验证，不能混用根脚本与包脚本。
5. 只更新受影响部分；未受影响的包不顺手整理。

## 小型前后端同仓：集中式目录

适用于一个仓库包含 `frontend/`、`backend/` 两个小项目，作为同一个产品维护的情况。以下为通用结构，不代表某个示例仓库已经实现这些能力：

```text
frontend/                      # 前端源码，不再重复建同内容的 docs
backend/                       # 后端源码
docs/
  README.md                    # 全部文档的导航入口
  overview.md                  # 产品现状、范围与限制
  architecture/
    README.md                  # 系统结构、前后端职责、依赖方向
    integration.md             # 前后端交互、鉴权与错误传播
  guides/
    development.md             # 环境、分别启动、代理与联调
    testing.md                 # 两端测试与端到端验证
  reference/
    api/README.md              # 前后端共享 API 契约，唯一详细来源
    configuration.md           # 共同配置及两端配置入口
  operations/
    deployment.md              # 整体部署、升级和回滚
  modules/
    frontend/
      README.md                # 前端现状、目录职责、局部文档入口
      architecture/README.md   # 路由、状态、组件组织，按需
    backend/
      README.md                # 后端现状、目录职责、局部文档入口
      architecture/README.md   # 分层、存储、任务生命周期，按需
.agents/notes/
  proposed/<class>/YYYY-MM-DD-topic.md
  implemented/<class>/YYYY-MM-DD-topic.md
  rejected/<class>/YYYY-MM-DD-topic.md
  archived/<class>/YYYY-MM-DD-topic.md
  archived/manifest.json
```

局部指南、配置、协议或数据模型较多时，在相应 `docs/modules/<模块>/` 下继续使用 guides、reference 等标准主题目录；小项目每端一篇 README 就够，不预建空目录。

API 契约放共同 reference；前端如何调用和后端如何实现分别放模块文档，不复制接口定义。notes 不再按 frontend/backend 分树：使用主题文件名和 `## Scope` 标明范围，跨端决定只记一篇。

例如已有后端方案、开发计划、需求文档和探测报告：先核对内容和实施状态。已实现事实提炼到现状文档；未实现决定标 proposed；开发任务沿用宿主计划机制；报告保留证据来源。不能仅凭文件名自动搬迁、删除或改写状态。
