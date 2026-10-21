# Code Review Policy

This project should treat code review as a quality gate, not only a comment pass.

## Required Gates

- Backend must pass `go test ./...`.
- Backend must pass `go vet ./...`.
- Backend Go files must be `gofmt` formatted.
- Backend should keep `make lint-strict` clean for changed code. The current repository has a historical lint baseline, so strict lint is advisory until that baseline is paid down.
- Frontend must pass `npm run lint --prefix frontend`.
- Frontend must pass `npm run build --prefix frontend`.
- New externally reachable endpoints must include auth, validation, and failure-path tests.
- Changes touching git clone, webhook, token, OCR command execution, or writeback flows require security-focused review.

## Severity Rules

- Critical: credential exposure, command injection, repository path escape, unauthenticated state changes, broken webhook signature verification, or data loss. Do not merge.
- High: missing authorization, unbounded external calls, unsafe defaults in production config, SQL or shell injection risk, or persistent background worker failure. Do not merge without a targeted fix or explicit risk acceptance.
- Medium: missing tests for changed behavior, weak validation, fragile concurrency, confusing operational behavior, or incomplete error handling. Fix before merge unless the PR is explicitly experimental.
- Low: readability, naming, dead code, minor duplication, docs drift, or polish. May merge with follow-up only when tracked.

## Review Checklist

- API contracts: request validation, response shape, pagination, error code, and backward compatibility are clear.
- Security: secrets are masked, webhook signatures are verified, repository URLs are constrained, and command arguments are not built from unchecked user input.
- Reliability: context cancellation, timeouts, retry boundaries, worker idempotency, and database transaction behavior are intentional.
- Observability: important failures include actionable logs or persisted errors without leaking tokens.
- Frontend: state updates, loading/error/empty states, route guards, and API type definitions match backend behavior.
- Tests: changed domain logic has focused unit tests; bug fixes include regression tests; generated code is not hand-edited without source IDL changes.

## Local Commands

Run the default local gate:

```sh
make check
```

Run heavier checks before risky changes:

```sh
make lint-strict
make test-race
make vuln
```

## Project-Specific Watch Areas

- `internal/service/provider` and `internal/vcs`: clone URL validation, SHA validation, and cleanup safety.
- `internal/service/webhook`: signature verification and provider-specific payload normalization.
- `internal/ocr`: external OCR command execution, environment variables, and timeout behavior.
  The unified exec entry is `internal/ocr/internal/command.go` (`Run`); `command.go` / `env.go` /
  `failure.go` in the same package cover command building, env merging, and failure classification.
- `internal/service/writeback`: comment deduplication, platform API failures, and partial publish behavior.
- `biz/model` and `biz/handler`: generated code drift from `idl/*.thrift`.
- `frontend/src/api` and `frontend/src/types`: API contract drift from backend models and thrift definitions.

> Historical note: this list previously pointed at `internal/service/runner` for OCR command
> execution. That package was folded into `internal/ocr` during the OCR core consolidation
> (see `docs/OCR_CORE_DESIGN.md`); the concern moved with the code.
