# 安全策略 / Security Policy

[English](SECURITY.md) | [中文](SECURITY.zh-CN.md)

## 支持的版本

Review Service 处于活跃开发中，从 `main` 分支发布。安全修复会先落在 `main` 的
最新提交上，再以新 tag 发布。

| 版本 | 是否支持 |
|---|---|
| `main` / 最新 tag | ✅ |
| 更早的 tag | ❌ |

## 报告安全问题

**安全问题请不要提公开 issue。**

请通过 **GitHub Security Advisories** 私密报告：

1. 打开 <https://github.com/yi-nology/code-review-service/security/advisories/new>
2. 写明问题、影响与复现步骤。

如果无法使用私密报告，可以提一个普通 issue，但**只写问题类别和「希望私下联系」
的请求**，不要包含技术细节、漏洞利用代码或 PoC。

通常几天内会收到确认回复。在公开发布前，请留出修复与协同发布的时间。

## 什么算漏洞

特别欢迎以下类型的报告：

- webhook 签名校验被绕过，或存在让服务把伪造投递当作合法投递的路径；
- 任一 API 端点（含 MCP 端点）的鉴权或授权绕过；
- 通过评审请求、补丁或 OCR 配置值可达的路径穿越、命令注入、任意文件读写；
- 仓储层的 SQL 注入或跨租户数据访问；
- 通过日志、API 响应或错误信息泄露凭据或 token；
- 未鉴权调用方即可触发的拒绝服务。

不算漏洞的情况：

- 自动化扫描器报出但没有可证明影响的问题；
- 需要有效 API token 的端点缺少限流；
- 服务信任配置正确的 Git 平台或 LLM 提供方；
- 需要已具备特权的位置才能触发的问题（例如已持有 `DATABASE_URL` 的数据库管理员）。

## 敏感区域

以下区域合并前需要安全专项评审。如果你的 PR 触及它们，请在 PR 里显式说明：

- provider / VCS 克隆，以及 URL 或 SHA 校验；
- webhook 签名校验（`internal/service/webhook` 与各 normalizer 的
  `VerifySignature` 实现）；
- OCR 命令执行与传给它的环境变量（`internal/ocr`、
  `internal/ocr/internal/command.go`）；
- 回写链路，包括行内评论与 commit status 的创建；
- 任何处理 API token、webhook secret 或 Git 平台凭据的地方——包含脱敏层
  （`internal/observability/redact`）。

## 运维部署须知

- `REVIEW_TOKEN` / `MCP_TOKEN` 是 API 的唯一鉴权凭据，请使用足够长的随机值；
  开发默认值 `dev-token` 不安全。
- `WEBHOOK_ALLOW_INSECURE` 只在本地或隔离的测试环境开启。它会让服务接受未配置
  secret 的平台投递，等于允许伪造投递。
- `DATABASE_URL` 内含数据库凭据——不要贴进 issue、日志或截图。
- 服务会克隆仓库并以子进程方式运行 OCR 二进制，请部署在可接受这一点的环境中，
  并显式限定可信的 Git 平台与 LLM 提供方。
