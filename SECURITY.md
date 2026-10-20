# Security Policy

[中文说明见下文](#中文说明)

## Supported versions

Review Service is under active development and is released from `main`. Security
fixes are applied to the latest commit on `main` and released as a new tag.

| Version | Supported |
|---|---|
| `main` / latest tag | ✅ |
| Older tags | ❌ |

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Report it privately through **GitHub Security Advisories**:

1. Go to <https://github.com/yi-nology/code-review-service/security/advisories/new>
2. Describe the issue, the impact, and the steps to reproduce.

If private advisory reporting is unavailable to you, open a regular issue that
contains only a one-line description of the class of the problem and a request
to contact you privately — no technical detail, no exploit, no proof of
concept.

You can expect an acknowledgement within a few days. Please allow time for a
fix and a coordinated release before disclosing publicly.

## What counts as a vulnerability

Especially welcome:

- webhook signature verification bypass, or any way to forge a webhook
  delivery that the service accepts as authentic;
- authentication or authorization bypass on any API endpoint, including the
  MCP endpoint;
- path traversal, command injection, or arbitrary file read/write reachable
  through a review request, a patch, or an OCR configuration value;
- SQL injection or cross-tenant data access in the repository layer;
- credential or token disclosure through logs, API responses, or error
  messages;
- denial of service reachable by an unauthenticated caller.

Not considered vulnerabilities:

- findings from automated scanners without a demonstrated impact;
- missing rate limits on an endpoint that requires a valid API token;
- the service trusting a correctly configured Git platform or LLM provider;
- issues that require an already-privileged position (for example, a database
  administrator with access to `DATABASE_URL`).

## Sensitive areas

The following areas require a security-focused review before merge. Please
mention explicitly in your pull request if you touch them:

- provider / VCS cloning, and URL or SHA validation;
- webhook signature verification (`internal/service/webhook`, and the
  `VerifySignature` implementation of each normalizer);
- OCR command execution and the environment variables passed to it
  (`internal/ocr`, `internal/ocr/internal/command.go`);
- write-back flows, including inline comment and commit-status creation;
- anything that handles API tokens, webhook secrets, or Git platform
  credentials — including the redaction layer
  (`internal/observability/redact`).

## Deployment notes for operators

- `REVIEW_TOKEN` / `MCP_TOKEN` are the only authentication for the API. Use a
  long random value; the development default `dev-token` is not safe.
- Set `WEBHOOK_ALLOW_INSECURE` only in a local or isolated test environment.
  It makes the service accept webhook deliveries for platforms that have no
  secret configured, which allows forged deliveries.
- `DATABASE_URL` contains the database credentials — do not paste it into
  issues, logs, or screenshots.
- The service clones repositories and runs the OCR binary as a subprocess.
  Run it in an environment where that is acceptable, and keep the set of
  trusted Git platforms and LLM providers explicit.

---

# 中文说明

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
