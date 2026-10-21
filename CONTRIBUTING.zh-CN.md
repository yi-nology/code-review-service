# 贡献指南 / Contributing to Review Service

[English](CONTRIBUTING.md) | [中文](CONTRIBUTING.zh-CN.md)

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

`etc/config-{env}.yaml` 以**严格模式**加载：出现未知键（拼错了，或该键已废弃）时
服务**拒绝启动**，错误信息会指出出错路径、该层的合法键清单与近似拼写建议。
配置文件**不存在**不算错误——回落到默认值，由 `Validate` 决定这份默认配置是否可接受。

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
