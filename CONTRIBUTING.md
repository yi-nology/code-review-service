# Contributing to Review Service

Thanks for your interest in improving Review Service. This document explains
how to set up a development environment, what we expect from a patch, and how
the review pipeline for this project itself works.

[中文说明见下文](#中文说明)

## Development setup

### Prerequisites

- Go 1.27+ (the `go` directive in `go.mod` is authoritative)
- PostgreSQL 14+ (the schema is created automatically by GORM AutoMigrate)
- Node.js 22+ (for the web console)
- Docker (only needed for the repository-integration tests, which use
  testcontainers to spin up a throwaway PostgreSQL)

```sh
git clone https://github.com/yi-nology/code-review-service.git
cd code-review-service

# Backend
make build
make test

# Frontend
cd frontend && npm install && npm run build
```

### Local run

```sh
cp .env.example .env      # then fill in REVIEW_TOKEN and DATABASE_URL
make run                  # listens on :8081
cd frontend && npm run dev  # Vite dev server on :3000, proxies /api → :8081
```

Business configuration (Git platform credentials, OCR engine settings, project
policies, rules) is **not** configured through environment variables. It is
persisted in the database and managed from the web console or the settings
API. Only infrastructure and deployment tuning live in `.env` and
`etc/config-{local,dev,prod}.yaml`.

## Before you open a pull request

Run the gate for the area you touched:

```sh
# Backend
make test lint fmt

# Frontend
npm run lint --prefix frontend && npm run build --prefix frontend
```

`make check` runs the full quality gate (backend test + lint + frontend lint +
frontend build) and is what CI enforces.

A few project-specific rules that are not obvious from the code layout:

- **Log with `github.com/oekit/logx`**, never with stdlib `log`, `logrus` or
  `zap` directly. The redacting wrapper in `init.go` already masks
  `Bearer`/`api_key`/`sk-`/`ghp_`-style secrets — rely on it and do not print
  raw credentials.
- **New identifiers** use `github.com/oklog/ulid/v2`.
- **Generated code is generated.** `biz/model/*` comes from `idl/*.thrift` via
  thriftgo and must not be hand-edited. Change the IDL, then regenerate.
  Never commit a `biz/model` change without the matching IDL change.
- **Do not build a second config-merge path.** `internal/ocr`'s `BuildConfig`
  is the single implementation (overrides > settings > fallback > defaults);
  route config changes through it.
- **Do not duplicate the phase table.** The legal phase order lives in
  `internal/domain/session/phase.go` and nowhere else.
- **Do not add a second status-transition table.** The legal step-status
  transitions are owned by the `agent-harness` kernel and reached through
  `internal/service/workflow/kernel.go`.

`docs/CODE_REVIEW_POLICY.md` describes the review and severity policy that this
project applies to itself. The areas it flags as sensitive — provider/VCS
clone and URL/SHA validation, webhook signature verification, OCR command
execution and environment variables, write-back flows, and anything touching
tokens or secrets — need a security-focused review. New or changed externally
reachable endpoints need authentication, input validation, and failure-path
tests.

## Commits and pull requests

- Keep commits focused; write the subject in the imperative mood.
- The project history is written in Chinese, with a structured body
  (`feat(scope): 摘要` followed by bullet points explaining what changed and
  why). Please follow that convention.
- Explain *why* in the body, not just *what* — the diff already shows what.
- If a change touches the OCR core, the workflow kernel, or the memory
  integration, say explicitly which invariant it preserves.

## Reporting bugs

Open an issue with:

- what you did, what you expected, and what happened instead;
- the version (git SHA or release tag) and deployment mode (Docker / binary);
- the relevant logs, with any token or credential redacted.

## Security issues

Please do not open a public issue for security problems. See
[SECURITY.md](SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the
Apache License 2.0, the same license as the project. See [LICENSE](LICENSE).

---

# 中文说明

## 开发环境

### 前置依赖

- Go 1.27+（以 `go.mod` 的 `go` 指令为准）
- PostgreSQL 14+（表结构由 GORM AutoMigrate 自动创建，无需手工建表）
- Node.js 22+（Web 控制台）
- Docker（仅仓库集成测试需要，用 testcontainers 拉起一次性 PostgreSQL）

```sh
git clone https://github.com/yi-nology/code-review-service.git
cd code-review-service

# 后端
make build
make test

# 前端
cd frontend && npm install && npm run build
```

### 本地运行

```sh
cp .env.example .env      # 填 REVIEW_TOKEN 与 DATABASE_URL
make run                  # 监听 :8081
cd frontend && npm run dev  # Vite 开发服务器 :3000，代理 /api → :8081
```

业务配置（Git 平台凭据、OCR 引擎设置、项目策略、规则）**不走环境变量**：它们
持久化在数据库里，由 Web 控制台或设置 API 管理。环境变量与
`etc/config-{local,dev,prod}.yaml` 只承载基础设施与部署调优。

## 提交前自检

按改动范围跑对应的门禁：

```sh
# 后端
make test lint fmt

# 前端
npm run lint --prefix frontend && npm run build --prefix frontend
```

`make check` 是完整门禁（后端测试 + lint + 前端 lint + 前端 build），CI 以它为准。

几条从代码结构上看不出来的项目约定：

- **日志用 `github.com/oekit/logx`**，不要直接用 stdlib `log`、`logrus`、`zap`。
  `init.go` 里的包装器已经会打码 `Bearer`/`api_key`/`sk-`/`ghp_` 等敏感串，
  依赖它即可，不要自己打印原始凭据。
- **新标识符**用 `github.com/oklog/ulid/v2`。
- **生成代码不手改**。`biz/model/*` 由 `idl/*.thrift` 经 thriftgo 生成，改动
  请求/响应结构必须先改 IDL 再重新生成；不要提交没有对应 IDL 改动的
  `biz/model` 变更。
- **不要另建一套配置合并逻辑**。`internal/ocr` 的 `BuildConfig` 是唯一实现
  （overrides > settings > fallback > defaults），配置变更一律走它。
- **不要复制阶段表**。合法的阶段顺序只存在于 `internal/domain/session/phase.go`。
- **不要复制状态迁移表**。合法的步骤状态迁移由 `agent-harness` 内核定义，
  本项目通过 `internal/service/workflow/kernel.go` 触达。

`docs/CODE_REVIEW_POLICY.md` 描述本项目对自己施加的评审与严重度策略。其中标记
为敏感的区域——provider/VCS 克隆与 URL/SHA 校验、webhook 签名校验、OCR 命令
执行与环境变量、回写链路，以及任何触及 token/密钥的地方——需要安全专项评审。
新增或修改对外可达的端点，需要补齐鉴权、输入校验与失败路径测试。

## 提交与 PR

- 提交聚焦，主题句用祈使语气。
- 本项目历史用中文书写，结构化正文（`feat(scope): 摘要` + 要点说明改了什么、
  为什么这么改）。请沿用这一约定。
- 正文写**为什么**，而不只是**做了什么**——diff 已经说明了做了什么。
- 改动涉及 OCR 核心、workflow 内核或记忆接入时，请明确说明保持的是哪条不变量。

## 报告缺陷

提 issue 时附上：复现步骤、期望行为与实际行为、版本（git SHA 或发布 tag）与
部署方式（Docker / 二进制）、相关日志（**先脱敏**任何 token 或凭据）。

## 安全问题

安全问题请勿公开提 issue，见 [SECURITY.md](SECURITY.md)。

## 许可协议

贡献即表示你同意以 Apache License 2.0 授权你的贡献，与本项目一致，见
[LICENSE](LICENSE)。
