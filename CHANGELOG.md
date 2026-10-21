# Changelog

本文件记录本项目的显著变更。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

> 说明：本项目在开源前有约 4 个月的内部开发历史（377 个提交）。那次历史的变更
> 记录未逐条回填到本文件——开源首个版本起，本文件开始逐版本维护。

## [0.2.0] - 2026-10-21

架构解耦与库接入深化：把「层级规则」从散文变成 CI 可执行断言，并把散落在
各层的手搓实现收进 oekit / agent-harness 家族。150 文件，+3275/−953。

### 架构门禁（新增）

新增 `architecture-policy.yaml`，用 `agent-harness/archcheck` 做**可执行断言**
——此前层级规则只写在 AGENTS.md 里，靠人记、靠 review 靠不住，散文会随人手
改动而失效，而失效之后没人知道它曾经存在。

- **error 级规则**（违反即 CI 红）：领域层不得依赖 infra / 兄弟包；service 层
  不得 import ORM 与仓储实现；handler 不得直连 PostgreSQL 实现；router 不得
  碰仓储或服务；生成代码（`biz/model`）不得引入手写依赖。
- **接入 `make check` 与 CI**：独立 job 跑（它要编译门禁工具并扫全仓，与
  后端 job 叠在同一台 runner 上会推高峰值资源）。
- **棘轮只收紧**：每还清一笔欠账就把对应规则从 warn 升为 error。**0.2.0
  发布时门禁已无 warn 级欠账——9 条规则全部是 error 级。**

### 依赖方向修正

- **引擎层降级为纯适配层**：评审的**前置编排**（克隆工作区 → 项目策略 → 文件
  过滤 → 规则合并 → 记忆召回 → OCR 执行参数）此前住在 `internal/eino`（引擎
  适配层）里，直接持 6 个仓储、绕过 service 层直读 5 张业务表，又反向调用
  `service/review` 的 13 个导出函数——「谁编排评审」于是有两个答案。现下沉为
  `internal/service/review` 的 `Pipeline`（端口 `review.PipelineRunner`），
  **eino 已完全不 import 仓储包**。
- **评审领域类型迁到 `internal/domain/review`**：它本就是纯领域叶子包，却寄居
  在 `service/` 路径下，让 infra 层反向依赖 service。package 名保持 `domain`
  不变（Go 的包名来自 package 子句而非路径），1133 处 `domain.X` 引用一行
  都不用改。
- **进度词汇下沉 `internal/domain/progress`**：阶段名与上报端口是领域词汇，
  tracker / manager / websocket 桥才是机制。OCR 因此不再依赖任何
  `internal/service/*`。
- **仓储构造器搬进实现包**：接口包原先 import 实现包并 new 出 18 个实现，
  导致所有间接依赖仓储接口的包传递依赖上整个 GORM 栈。容器**类型**留在接口包
  （消费方不必直连实现包），**构造器**搬到 `repository/postgres`。
- **窄接口由消费方定义**：进度与 Agent 名册各用一个只含自有类型的小接口，
  实现方（agent / memory）无需 import 声明方——Go 的接口是结构化的。

### 库接入

- `oekit/pkg/cache`（Cache-Aside + 空值防穿透 + StaleIfError + SingleFlight
  并发回源合并）替换两处手写缓存；`oekit/concurrency/retry` 替换手写 HTTP
  重试（删掉整个 `internal/httpx` 包）；`oekit/pkg/stringx` 收编三处字符串
  工具；依赖地板抬到各家族最新 tag（oekit/oagentkit 共 12 个模块）。
- `agent-harness/toolfail` 的 `NeedsOperator`（凭证失效这类「机器无能为力」
  的失败）此前只落一行日志——现在接到通知渠道，并给出可执行提示。
- `agent-harness/statemachine` 的 failed 终态此前是**死代码**（只有
  completePhase 调它且只用 succeeded）——失败阶段永远停在 running，现已可达。

### 修复

- **月度成本预算检查从不触发**：单价键是 `provider|model`，而月度用量只按
  model 聚合，provider 为空时永远匹配不上 → 累计成本恒为 0，**block 模式
  也从来没拦过任何评审**。改为 provider 未知时按 model 兜底（多候选取最贵）。
- **OCR 子进程继承宿主凭据**：DATABASE_URL / REVIEW_TOKEN / MCP_TOKEN /
  ARGUS_BOT_TOKEN 落进了会在工作区跑 git 与 MCP server 的子进程；OCR 已显式
  拿到凭证时，环境里冗余的 provider 凭证也一并摘掉。
- **中文截断腰斩**：LLM 降级原因按字节切会截断多字节字符，产出非法 UTF-8。
- **记忆召回无超时上限**：手写配置丢掉底座默认的 `RecallTimeout=2s`，而召回在
  评审主链上——PG 一挂评审就无限等待。
- **阶段名第三份副本**：新增阶段时 OCR 侧漏改，该阶段进度被静默误报成
  analysis。
- **goroutine panic 会杀进程**：两处裸 `go` 启动改为 `async.GoSafeLabeled`；
  心跳的 stop 现在等协程真正退出（此前调用方无法依赖「stop 后不再 touch」）。
- **CI 稳定性**：架构门禁拆独立 job、限制编译并行度、关闭 `go test` 内建
  vet（vet 已由独立步骤覆盖）、构建临时目录移出 `/tmp`、格式检查步骤改为直接
  报出违规文件名。

## [0.1.0] - 2026-10-21

首个公开版本（Apache-2.0）。

### 功能

- **多平台接入**：GitHub、GitLab、Gitea、GitCode、腾讯云 Code，另支持 Gerrit。
- **AI 评审**：以 [Open Code Review](https://github.com/alibaba/open-code-review)
  为评审引擎，支持多智能体并行评审、结果聚合与讨论。
- **实时进度**：基于 WebSocket 的评审进度推送。
- **质量门禁**：按严重度阈值判定 pass/fail，并把结论写成 commit status，
  可直接作为 PR 的必需检查项。
- **行内评论**：评审结论回写为 PR/MR 行内评论。
- **评审地图**：基于 Mermaid 的代码架构图。
- **Web 控制台**：Vue 3 + Element Plus 单页应用，涵盖评审记录、项目策略、
  平台与模型设置、通知与成本看板。
- **会话管理**：多轮评审会话与历史留存。
- **意见生命周期**：跨评审稳定指纹识别复现问题，误报抑制避免重复上报。
- **通知**：站内通知中心 + 外部 webhook（飞书 / 企业微信 / 钉钉 / 通用）。
- **成本管理**：按模型价目表估算 LLM 成本，支持月度预算与超额处置策略。
- **可观测性**：Prometheus 指标（`/metrics`）与随仓库提供的 Grafana 看板。
- **鉴权与审计**：命名 API token（SHA256 存储）+ 写操作审计日志，
  兼容单 token 模式。
- **可靠性**：基于 PostgreSQL `SKIP LOCKED` 的持久化评审队列、重试上限与死信
  处理、失败可重试性分类。
- **安全**：补丁路径穿越防护、敏感文件护栏（CI / 构建 / hooks）、受保护分支的
  自动修复推送限制。
- **周报**：项目维度的 Markdown 评审汇总。
- **长期记忆**：基于 `agent-memory` 内核的跨会话记忆，可选启用。
- **MCP 端点**：独立进程暴露 MCP 服务，供 LLM 客户端接入。

### 变更

- 模块路径由内网域名改为 `github.com/yi-nology/code-review-service`。
- 依赖 `go-git-platform` 升级至 v0.95.0：新增 `SkipValidationSecret` 哨兵，
  为「宿主自行校验签名、SDK 只做 payload 解析」提供显式入口。
- 修正 webhook 解析在 SDK v0.77.0 空 secret 语义变更后的失效问题。

[0.2.0]: https://github.com/yi-nology/code-review-service/releases/tag/v0.2.0
[0.1.0]: https://github.com/yi-nology/code-review-service/releases/tag/v0.1.0
