# Quality Review Report

Date: 2026-06-17

## Executive Summary

The project is buildable and the basic gate now passes:

- `go test` across repository packages passes.
- `go vet` across repository packages passes.
- `npm run lint --prefix frontend` passes as an error-only frontend gate.
- `npm run build --prefix frontend` passes.
- `make check` passes and is suitable as the first required local/CI gate.

The strict lint profile is intentionally separated as `make lint-strict`. It currently reports historical debt and should be paid down in focused PRs before becoming a hard merge gate.

## Configured Gates

- Added `Makefile` with `check`, `lint`, `lint-strict`, `test`, `test-race`, `vuln`, `frontend-lint`, and `frontend-build` targets.
- Added `.github/workflows/quality.yml` for backend and frontend CI.
- Added `.golangci.yml` using golangci-lint v2 config format.
- Added `frontend/eslint.config.js` and package scripts for Vue/TypeScript linting.
- Added `docs/CODE_REVIEW_POLICY.md` with severity rules and project-specific review watch areas.

## Key Findings

### P0/P1

- No immediate test/build blocker in the required gate.
- `npm audit --audit-level=high --omit=dev` reports 0 production vulnerabilities after `npm audit fix`.
- `npm audit --audit-level=high` still reports a dev dependency advisory through `vite`/`esbuild`; the available fix requires a breaking Vite major upgrade.

### P2

- `make lint-strict` reports 91 issues in the current codebase after the first remediation pass:
  - 12 unchecked error returns.
  - 11 direct error comparisons that should use `errors.Is`.
  - 42 gofmt issues.
  - 9 staticcheck issues.
  - 6 integer conversion warnings from gosec.
  - 4 unused symbols.
- The first remediation pass fixed the strict-lint `nilerr` findings, removed the possible nil dereference in `internal/service/workflow/workflow.go`, checked ignored git close paths, wrapped the Gitea review/fallback-note double failure with `errors.Join`, and removed unused writeback formatter helpers.
- Installing frontend dependencies creates `frontend/node_modules` with a Go package inside a dependency. The Makefile now excludes `frontend/node_modules` from Go package discovery so local checks do not accidentally test dependency internals.

### P3

- Frontend strict lint currently has many historical warnings, mostly Vue style rules and `any` usage. The first hard gate uses `eslint --quiet` so real errors are blocked without turning style migration into this PR.
- Frontend build emits Rollup warnings about PURE annotations from a dependency. These are noisy but not currently build-breaking.

## Recommended Remediation Order

1. Handle the breaking Vite/esbuild advisory in a dedicated frontend dependency upgrade.
2. Run `make fmt` in a dedicated formatting PR, then make gofmt a hard CI check.
3. Fix unchecked database/websocket/close errors where the failure affects correctness or observability.
4. Replace direct `gorm.ErrRecordNotFound` comparisons with `errors.Is`.
5. Remove or wire unused handlers/helpers.
6. Address gosec integer conversion warnings at API boundaries.
7. Gradually move frontend from error-only lint to `npm run lint:strict`.

## Commands

Required gate:

```sh
make check
```

Strict review gate:

```sh
make lint-strict
make vuln
make test-race
```
