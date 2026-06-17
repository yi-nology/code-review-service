# Review Service Usability Upgrade Plan

## Current Assessment

The service already has a meaningful foundation: review records, quality gates, project policy, reviewer agents, progress tracking, writeback, and a Chinese web console. The biggest gap is not a missing core feature, but the amount of friction around first-run setup, navigation confidence, and operational feedback when something goes wrong.

## Issues Found

1. The web console had competing rule-setting URLs. The current navigation should use one canonical route and keep old links working through redirects.
2. Unknown frontend routes had no explicit fallback, so mistyped or stale links could leave users on an unhelpful route.
3. The README still used `go run ./cmd/review-service`, but the current application entrypoint is the repository root.
4. Local frontend proxy expectations were implicit. `etc/config-local.yaml` uses port `8081`, while compose and many README examples use `8080`.
5. The production frontend bundle had a large single vendor chunk. That slows first load and makes cache behavior worse as the console grows.
6. API error messages were inconsistent across backend response shapes. Some useful backend messages could be hidden behind a generic Axios message.

## Changes Applied

1. Pointed the sidebar global-rules item to `/settings/rules`.
2. Kept `/settings/global-rules` as a compatibility redirect.
3. Added a catch-all frontend route that returns authenticated users to the reviews workspace.
4. Made the Vite API proxy configurable through `VITE_API_PROXY_TARGET`.
5. Added Vite manual chunks for Vue, Element Plus, and Axios.
6. Improved frontend request error-message extraction.
7. Updated README local run commands and proxy notes.

## Recommended Upgrade Path

### Phase 1: First-Run Experience

- Add a guided checklist for model settings, Git platform token, reviewer agents, and first project policy.
- Add a visible backend connectivity badge that checks `/healthz` or `/readyz` instead of always showing "服务运行中".
- Add clear empty states with primary actions: create review, configure platform, add project policy.

### Phase 2: Review Workflow Ergonomics

- Add saved filters for common review queues: failed gate, running, my repositories, high-risk findings.
- Add bulk actions for reviewed files and sections in review maps.
- Add direct links from findings to source platform files when provider metadata is available.
- Add "why did this fail" explanations beside gate status, with the exact policy and threshold that triggered it.

### Phase 3: Configuration Safety

- Add inline validation and test buttons for all platform/model settings before saving.
- Show masked secret state instead of empty token fields, so users can tell whether a token is configured.
- Add dry-run previews for project rules and ignore-path patterns.
- Keep an audit trail for settings changes.

### Phase 4: Operability

- Add structured frontend error pages for 401, 404, and backend unavailable states.
- Add server-side metrics for review duration, queue time, OCR failures, writeback failures, and provider API latency.
- Add a troubleshooting page that surfaces recent webhook deliveries and writeback attempts.
- Add smoke tests for core frontend routes and API contract compatibility.

### Phase 5: Code Health

- Replace broad frontend `any` use in high-traffic review/detail flows with typed DTOs.
- Add API client normalization tests for wrapped and unwrapped backend response shapes.
- Split large views into focused components where state and rendering are currently coupled.
- Add integration tests around settings persistence and route compatibility.
