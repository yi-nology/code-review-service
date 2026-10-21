# Contributing to Review Service

[English](CONTRIBUTING.md) | [中文](CONTRIBUTING.zh-CN.md)

# Contributing to Review Service

Thanks for your interest in improving Review Service. This document explains
how to set up a development environment, what we expect from a patch, and how
the review pipeline for this project itself works.


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

`etc/config-{env}.yaml` is loaded in **strict mode**: an unknown key (a typo,
or a key that no longer exists) makes the service refuse to start, and the
error names the offending path, the legal keys of that level, and a
nearest-match suggestion. A missing config file is not an error — the service
falls back to defaults and `Validate` decides whether that is acceptable.

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
