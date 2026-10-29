# Changelog

本文件记录本项目的显著变更。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

> 说明：本项目在开源前有约 4 个月的内部开发历史（377 个提交）。那次历史的变更
> 记录未逐条回填到本文件——开源首个版本起，本文件开始逐版本维护。

## [0.3.0] - 2026-10-29

控制台改版与已知缺陷收口：把机器占位串从中文界面里清干净，修掉周报通过率与
进度 WebSocket 两处实打实的缺陷，并补齐 Gerrit 的回归测试。

### 控制台（界面修复）

- **机器占位串不再漏出英文**：控制台是中文界面，但入队 / 取消 / 扫描失败 /
  进度完成的审计消息、门禁原因、argus 裁决仍是英文，中文界面里夹着
  「review queued via webhook」这类原始串。
  分工明确：**Message 与门禁原因只面向人**，后端直接写中文；**Summary 同时是
  API 字段**（MCP 工具输出给 agent 用），保持机器可读的英文，由展示层翻译
  （`frontend/src/utils/reviewText.ts`）。映射表原先只覆盖
  `persistence.go` 的 `"review " + status` 一族，create / requeue /
  scan_orchestrator 的固定串与 argus 裁决串全漏——已补齐，升级前写下的
  历史行也一并翻译。
- **未知状态不再直出原始枚举**：后端新增状态而前端尚未同步时，界面会出现
  `success_with_publish_failed` 这类英文枚举混排（曾经真实发生过）。状态 /
  阶段 / 步骤状态三张表的兜底文案都改成人能读懂的。
- **`success_with_publish_failed` 与 `success` 区分开**：评审本身成功、但评论
  回写失败，此前与「已完成」同色，用户会以为评论已经发出去了。
- **审查 ID 取尾不取头**：ID 是 ULID，前 8 位是时间戳（同一时间创建的记录
  头部几乎一样），取头会让一列记录看起来完全相同。
- **token 数缩写去小数尾零**：776.0k → 776k。
- **品牌标识重做**：盾牌轮廓 → 放大镜（审查 / 检查代码的语义更贴切）；渐变
  id 按实例唯一——同一页面可能同时渲染多个标识（登录页的桌面/移动两处），
  固定 id 会让后一个实例复用前一个的定义。

### 缺陷修复

- **周报贡献者通过率重复放大 100 倍**：PassRate 已是百分数（仓储层把 SQL
  比值 ×100 后的值透传上来），导出 Markdown 时又乘了一次 100，表格里写成
  10000%。新增 `report_service_test.go` 钉住：仓储给 85.0 就该渲染 85%。
- **进度 WebSocket 被判跨站而 403**：同源校验拿 Origin 的 host:port 与 Host
  头严格比对，而 nginx 的 `$host` 会剥掉端口，凡是带端口的访问
  （`http://<ip>:8091`）都被判跨站，实时进度流直接断掉——实测 `$host` → 403，
  `$http_host` → 101。改用 `$http_host`。
- **配置内容非法时静默降级成空配置**：配置文件**不存在**不是错误（回落默认
  值），但**存在且内容非法**（未知键 / 解析失败）此前被静默吞掉，服务带着
  空配置继续启动，把「哪个键写错了」推迟到更远的地方才暴露。现在启动即失败。
- **静态资源权限 0600 导致容器内 403**：源文件权限被带成 0600，nginx worker
  读不到（实测 `frontend/public/icons.svg` 曾因此 403）。
- **SPA 入口缓存发版后引用已删除的旧 chunk**：入口改为协商缓存，带内容哈希
  的静态资源长缓存。

### 收编 oekit 原语

字符串截断（`stringx.TruncRunes`，按 rune 而非字节，避免腰斩多字节字符后产出
非法 UTF-8 进 prompt）、OCR 重试与退避（`concurrency/retry`）、LLM 宽容 JSON
解析（`pkg/jsonrepair`）、OCR settings 缓存（`pkg/cache`，过期判定注入假时钟
即可确定性驱动，无需 sleep）分别收编到单一实现，本地不再各写一份。

### 测试补齐

- Gerrit 的回归测试此前只随内网仓发布，开源仓缺少 `provider` 与 `writeback`
  两层的用例——本次补上，注册与写回路径都有测试钉住。

## [0.2.1] - 2026-10-22

Gerrit 平台支持与产品文档重写：把「支持 Gerrit」从文档里的声明变成真能配、真能跑的能力，
并按 AI Code Review 的产品定位重写了功能介绍与截图。

### Gerrit 平台支持

Gerrit 此前**只实现了客户端、没有打通平台注册**——`AddOrUpdate` 无条件调用 SDK 的
`provider.NewProvider`，而 Gerrit 不在 go-git-platform 的 backend 列表里，
于是保存 Gerrit 平台必然失败（`platform not registered`），配置进不了 `configs`，
`HasProvider` 恒为 false，评审编排层判定「平台未配置」，整条链路根本不会启动。
控制台的下拉里也没有 Gerrit，用户连选都选不到。

现在：

- **平台注册**：`usesSDKBackend()` 识别出 Gerrit 后不创建 SDK provider，
  改为建立独立的 go-gerrit 客户端并登记配置（clone URL / token / SkipTLS 都从
  `configs` 读，缺了会退化成「无凭据 clone 空 URL」）。
- **客户端热更新**：`ClientHolder` 下沉到 service 层，平台配置变更后写回路径
  立即生效，不用重启进程；平台被删除/禁用时一并清掉客户端。
- **写回适配器无条件注册**：不再因为「装配时还没配 Gerrit」而跳过注册，
  之后在控制台补上即可生效。
- **连接测试**：走 Gerrit 自己的 `accounts/self`（认证 REST API 的 `/a/` 前缀），
  顺带带出认证用户名。
- **clone URL**：认证 clone 用 `/a/` 前缀，多层 project（`group/project`）也带全。
- **控制台**：Git 平台配置页新增 Gerrit 选项与取 Token 指引。

注册路径由 `TestAddOrUpdateGerrit` 钉住（回退修复即失败）。

### 产品文档与素材

- `docs/FUNCTIONAL_OVERVIEW.md` 重写：产品名统一为 **AI Code Review**，支持平台更新为
  **8 个**（GitHub、GitLab、Gitea、Forgejo、Gitee、GitCode、腾讯工蜂、Gerrit），
  新增 Gerrit 专章——为什么它是例外、注册路径怎么走、diff 为什么从 git 算、
  webhook 为什么独立归一化。
- `docs/assets/screenshots/` 换为 16 张新截图（本地真实运行的控制台实拍，1920 宽）。
- `scripts/seed-demo.sql`：一键灌入合成演示数据（`acme/*` 仓库名、机器人提交者、
  `*.example.com` 地址），任何人 clone 后都能复现截图，不含真实代码或客户信息。

### 修复

- Gerrit 移植时模块路径改写（内网域名 → `github.com`）打乱了 import 分组，
  未过 `gofmt` 检查，CI 的 Check formatting 报红——已修正。
- 公共树清理：移除制作工具链（宣传片渲染工程、一次性移植脚本）与内含内网域名映射的
  截图脱敏工具；根 `.gitignore` 加 `*.mp4` 守卫，成片一律不进仓库。

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
