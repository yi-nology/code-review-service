# Changelog

本文件记录本项目的显著变更。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

> 说明：本项目在开源前有约 4 个月的内部开发历史（377 个提交）。那次历史的变更
> 记录未逐条回填到本文件——开源首个版本起，本文件开始逐版本维护。

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

[0.1.0]: https://github.com/yi-nology/code-review-service/releases/tag/v0.1.0
