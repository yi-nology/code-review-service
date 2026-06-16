# Review Service

Minimal service-shaped code review gateway for GitHub, GitLab, and Gitea CI.

## Run

```sh
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
REVIEW_TOKEN=dev-token \
go run .
```

Enable Alibaba Open Code Review:

```sh
OCR_ENABLED=true \
OCR_BINARY=ocr \
REVIEW_TOKEN=dev-token \
DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
go run .
```

The `ocr` binary must be available in `PATH` when `OCR_ENABLED=true`. The Docker image installs it from the official Alibaba Open Code Review GitHub release.

Health check:

```sh
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

`/healthz` is a liveness check. `/readyz` verifies that the configured review store is reachable.

## Web Console

Run the frontend dev server alongside the Go service:

```sh
cd frontend && npm install && npm run dev
```

The frontend dev proxy targets `http://localhost:8081` by default, matching `etc/config-local.yaml`. If you start the Go service on another port, override it before running Vite:

```sh
VITE_API_PROXY_TARGET=http://localhost:8080 npm run dev
```

Open `http://localhost:3000/` and connect with the value of `REVIEW_TOKEN`.

The console uses a Chinese interface and provides:

- Paginated access to all stored review records
- Review status, gate, and finding summaries across the full filtered result
- Provider, status, and repository filters
- Complete review reports with request metadata, SHAs, timestamps, gate reasons, findings, and errors
- Versioned code quality scoring with visible deductions and severity totals
- Open Code Review file, duration, token, cache, warning, and completion metrics
- Side-by-side existing and suggested code for actionable improvements
- Manual review creation for GitHub, GitLab, and Gitea

The token is kept in browser session storage and is sent as a Bearer token to the same-origin API. In production, build the frontend (`npm run build`) and serve the `dist/` directory via the Go service or a reverse proxy.

Create a synchronous review:

```sh
curl -sS -X POST http://localhost:8080/api/v1/reviews \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "gitlab",
    "repo": "group/project",
    "project_id": "123",
    "merge_request_iid": "15",
    "base_sha": "abc123",
    "head_sha": "def456",
    "callback_mode": "comment",
    "wait": true
  }'
```

Set `"wait": false` to receive `202 Accepted` immediately. Poll the returned ID:

```sh
curl -sS "$REVIEW_SERVICE_URL/api/v1/reviews/$REVIEW_ID" \
  -H "Authorization: Bearer $REVIEW_TOKEN"
```

The status progresses through `queued`, `running`, and then `success` or `failed`.

List stored reviews with server-side pagination:

```sh
curl -sS "http://localhost:8080/api/v1/reviews?provider=gitlab&status=success&page=1&page_size=50" \
  -H "Authorization: Bearer dev-token"
```

The response includes `total`, `page`, `page_size`, `total_pages`, aggregate `stats`, and the current page of `items`.

## CI Trigger

```sh
payload=$(jq -n \
  --arg provider "gitlab" \
  --arg repo "$CI_PROJECT_PATH" \
  --arg project_id "$CI_PROJECT_ID" \
  --arg merge_request_iid "$CI_MERGE_REQUEST_IID" \
  --arg base_sha "$CI_MERGE_REQUEST_TARGET_BRANCH_SHA" \
  --arg head_sha "$CI_COMMIT_SHA" \
  '{
    provider: $provider,
    repo: $repo,
    project_id: $project_id,
    merge_request_iid: $merge_request_iid,
    base_sha: $base_sha,
    head_sha: $head_sha,
    callback_mode: "comment",
    wait: true
  }')

resp=$(curl -sS -X POST "$REVIEW_SERVICE_URL/api/v1/reviews" \
  -H "Authorization: Bearer $REVIEW_TOKEN" \
  -H "Idempotency-Key: $CI_PIPELINE_ID:$CI_JOB_ID:$CI_COMMIT_SHA" \
  -H "Content-Type: application/json" \
  -d "$payload")

gate_status=$(echo "$resp" | jq -r '.gate.status')
if [ "$gate_status" != "passed" ]; then
  echo "$resp"
  exit 1
fi
```

## Idempotent Requests

CI callers should send an `Idempotency-Key` header. Repeating the same key with the same JSON payload returns the existing review instead of creating another model run:

```sh
Idempotency-Key: gitlab:123:15:def456
```

Keys are limited to 200 characters. Reusing a key with a different payload returns `409 Conflict`. PostgreSQL enforces uniqueness across service replicas.

## Configuration

| Env | Default | Purpose |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | HTTP listen address |
| `REVIEW_TOKEN` | `dev-token` | Bearer token expected from CI |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `POLICY_FAIL_ON` | `critical` | Severity threshold that fails the gate |
| `POLICY_MAX_COMMENTS` | `20` | Reserved for comment publisher limiting |
| `POLICY_SUMMARY_ONLY` | `false` | Reserved for summary-only publishing |
| `POLICY_IGNORE_PATHS` | `vendor/**,dist/**,*.pb.go` | Comma-separated ignore patterns |
| `PROJECT_SETTINGS_PATH` | `data/project-settings.json` | Per-project gate and ignored-path policy file |
| `OCR_COMMAND` | empty | Optional external reviewer command returning JSON |
| `OCR_ENABLED` | `false` | Enable the Alibaba Open Code Review runner; when `false`, reviews return empty results |
| `OCR_BINARY` | `ocr` | Path to the Alibaba OCR executable |
| `OCR_RULE` | empty | Optional OCR custom rule JSON path |
| `OCR_DEFAULT_SEVERITY` | `major` | Severity assigned to OCR comments |
| `DATABASE_URL` | empty | PostgreSQL connection URL (required) |
| `DATABASE_MAX_OPEN` | `10` | Maximum open PostgreSQL connections |
| `DATABASE_MAX_IDLE` | `5` | Maximum idle PostgreSQL connections |
| `WORKER_CONCURRENCY` | `2` | Number of in-process review workers; `0` disables workers |
| `WORKER_POLL_INTERVAL` | `1s` | Delay when no queued review is available |
| `WORKER_STALE_AFTER` | `15m` | Requeue running reviews older than this on startup |
| `GIT_ALLOW_REPO_URLS` | `false` | Allow request payloads to contain direct HTTP(S) or SSH repository URLs |
| `GIT_ALLOW_LOCAL_REPOS` | `false` | Allow absolute paths and `file://` repositories; development only |
| `GIT_ALLOW_INSECURE_HTTP` | `false` | Allow direct repository URLs using plain HTTP |
| `GIT_FETCH_TIMEOUT` | `2m` | Maximum time for repository preparation and diff generation |
| `REVIEW_MAX_DIFF_BYTES` | `5242880` | Maximum generated diff size before review is rejected |
| `GITHUB_BASE_URL` | `https://github.com` | GitHub Enterprise base URL |
| `GITHUB_API_URL` | auto | GitHub REST API URL |
| `GITHUB_TOKEN` | empty | Token used when fetching GitHub repos |
| `GITLAB_BASE_URL` | `https://gitlab.com` | GitLab base URL |
| `GITLAB_API_URL` | auto | GitLab v4 API URL |
| `GITLAB_TOKEN` | empty | Token used when fetching GitLab repos |
| `GITEA_BASE_URL` | empty | Gitea base URL, required for path-style Gitea repos |
| `GITEA_API_URL` | auto | Gitea v1 API URL |
| `GITEA_TOKEN` | empty | Token used when fetching Gitea repos |

## Comment Publishing

Set `POLICY_MAX_COMMENTS` to a positive value to publish one Markdown summary comment (default `20`). Set `POLICY_MAX_COMMENTS=0` to disable publishing.

- GitHub requires `pull_request_id` and `GITHUB_TOKEN`.
- GitLab requires `merge_request_iid`, plus `project_id` or `repo`, and `GITLAB_TOKEN`.
- Gitea requires `pull_request_id`, `GITEA_BASE_URL`, and `GITEA_TOKEN`.

`POLICY_MAX_COMMENTS` limits the findings included in the summary. Set `POLICY_SUMMARY_ONLY=true` to publish counts without individual finding details.

## PostgreSQL

Set `DATABASE_URL` in production:

```sh
DATABASE_URL='postgres://review_service:secret@postgres:5432/review_service?sslmode=require'
```

The service checks the connection and applies the embedded schema migration during startup. Review requests, responses, findings, statuses, and timestamps survive service restarts.

Asynchronous requests are claimed from PostgreSQL with `FOR UPDATE SKIP LOCKED`, so multiple service replicas can process the queue without claiming the same review. Stale `running` reviews are returned to `queued` during worker startup.

Set `WORKER_CONCURRENCY=0` on API-only replicas, and use a positive value on worker-enabled replicas.

## Repository Security

By default, request payloads must use a repository slug such as `group/project`. The service builds the clone URL from the configured provider base URL.

Direct URLs, SSH URLs, local paths, and `file://` repositories are rejected by default. `base_sha` and `head_sha` must be 7-64 hexadecimal characters, preventing Git option injection.

For local-only testing with filesystem repositories:

```sh
GIT_ALLOW_LOCAL_REPOS=true go run .
```

Do not enable local repositories on a shared or internet-accessible service.

For local development:

```sh
docker compose up --build
```

To run the PostgreSQL integration test:

```sh
TEST_DATABASE_URL='postgres://review_service:review_service@localhost:5432/review_service?sslmode=disable' \
go test ./internal/store -run TestPostgresStoreRoundTrip
```

When `OCR_ENABLED=true`, the service runs:

```sh
ocr review \
  --repo "$REVIEW_WORKSPACE_DIR" \
  --from "$REVIEW_BASE_SHA" \
  --to "$REVIEW_HEAD_SHA" \
  --format json
```

OCR comments are converted from its native `path`, `content`, `start_line`, `end_line`, `existing_code`, and `suggestion_code` fields into service findings. OCR does not currently emit severity, so comments use `OCR_DEFAULT_SEVERITY`.

The service stores the original OCR JSON for audit purposes but does not expose model `thinking` fields in the API, web console, or PR comments. A deterministic `severity-v1` score starts at 100 and records every deduction in the report: critical `25`, major `12`, minor `5`, info `1`, OCR warnings up to `15`, and incomplete OCR execution `20`. This score is a service policy signal, not a score emitted by Alibaba Open Code Review.

The legacy `OCR_COMMAND` integration remains available when `OCR_ENABLED=false`. It should print a JSON object compatible with:

```json
{
  "summary": "发现 1 个问题",
  "findings": [
    {
      "severity": "critical",
      "path": "internal/api/server.go",
      "line": 42,
      "title": "nil dereference",
      "body": "..."
    }
  ]
}
```

The command receives `REVIEW_PROVIDER`, `REVIEW_REPO`, `REVIEW_BASE_SHA`, `REVIEW_HEAD_SHA`, and `REVIEW_WORKSPACE_DIR` in its environment. The computed diff is passed on stdin.

// test writeback at Sun Jun 14 06:38:16 CST 2026
