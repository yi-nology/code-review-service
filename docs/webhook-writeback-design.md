# Webhook and Inline Review Design

## Decision

Keep the existing review engine, but split platform integration into four explicit
boundaries:

1. `WebhookAdapter`: verifies signatures and normalizes provider payloads.
2. `ChangeLoader`: resolves the current PR/MR revision and unified diff.
3. `ReviewEngine`: produces provider-neutral findings.
4. `WritebackAdapter`: publishes summary, inline comments, and commit status.

The current `publisher.Publisher` only creates one general note. It should become
the compatibility fallback behind `WritebackAdapter`, not the long-term inline
comment implementation.

## Inbound Endpoint

Use one endpoint per configured platform:

```text
POST /api/v1/webhooks/:provider/:installation
```

`installation` identifies a stored connection, not a repository. This allows
multiple GitHub Enterprise, GitLab, or Gitea instances without putting secrets
in URLs.

The handler must:

1. Read the raw body with a strict size limit.
2. Look up the installation and webhook secret.
3. Verify the provider signature before JSON decoding.
4. Normalize only pull/merge request events.
5. Return `202` after durably inserting an inbox record.
6. Let a worker create or supersede the review job.

Do not run clone or model work in the webhook request.

## Normalized Event

```go
type ChangeEvent struct {
    Provider       string
    InstallationID string
    DeliveryID     string
    EventType      string
    Action         string
    RepositoryID   string
    Repository     string
    Number         int
    BaseSHA        string
    StartSHA       string
    HeadSHA        string
    Sender         string
    ReceivedAt     time.Time
}
```

Supported actions for the first release:

| Provider | Event | Actions |
| --- | --- | --- |
| GitHub | `pull_request` | `opened`, `reopened`, `synchronize`, `ready_for_review` |
| GitLab | Merge Request Hook | `open`, `reopen`, `update` when SHA changed |
| Gitea | `pull_request` | `opened`, `reopened`, `synchronized` |

Ignore drafts unless project policy enables them. Ignore bot-authored events
from this service to prevent feedback loops.

## Persistence and Idempotency

Add these tables:

### `webhook_deliveries`

```text
provider, installation_id, delivery_id, event_type, action,
payload_sha256, payload, status, error, received_at, processed_at
```

Unique key: `(provider, installation_id, delivery_id)`.

### `change_revisions`

```text
provider, installation_id, repository_id, repository, change_number,
base_sha, start_sha, head_sha, latest_review_id, created_at
```

Unique key:
`(provider, installation_id, repository_id, change_number, head_sha)`.

### `writeback_items`

```text
review_id, finding_key, kind, provider_external_id, provider_url,
status, attempts, last_error, published_at
```

Unique key: `(review_id, finding_key, kind)`.

The review idempotency key should be derived from the immutable revision:

```text
webhook:{provider}:{installation}:{repository_id}:{change_number}:{head_sha}
```

A newer head SHA supersedes queued work for the same change. A running review
may finish, but it must check the current head before publishing. Stale results
are stored for audit and not written back.

## Finding Location

The model's path and line numbers are untrusted hints. Before publishing:

1. Normalize the path and reject traversal or absolute paths.
2. Parse the exact unified diff used by the model.
3. Confirm every inline range exists on the requested old or new side.
4. Prefer the new side for added or context lines and old side for deletions.
5. Downgrade an invalid or outdated location to the summary.

`internal/service/writeback.DiffIndex` is the initial validation layer. A later
`AnchorResolver` can use surrounding code hashes to relocate a finding when the
model reports a nearby line.

Use a stable finding key:

```text
sha256(rule/category + path + side + start_line + normalized body)
```

This prevents duplicate comments across retries.

## Provider Writeback

### GitHub

Publish one review with:

```text
POST /repos/{owner}/{repo}/pulls/{number}/reviews
```

Use `commit_id=head_sha`, a summary `body`, `event=COMMENT` (or
`REQUEST_CHANGES` by policy), and batched comments with `path`, `line`, `side`,
and optional `start_line`/`start_side`.

Use GitHub App installation tokens in production. The installation needs Pull
Requests write permission; commit status or checks need their corresponding
permission. Re-read the PR head immediately before publishing.

### GitLab

Create each inline thread with:

```text
POST /projects/:id/merge_requests/:iid/discussions
```

Send `position[position_type]=text`, the latest diff version's `base_sha`,
`start_sha`, and `head_sha`, plus `old_path`, `new_path`, and the applicable
`old_line` or `new_line`. Fetch the latest MR version before publication.

Publish the overall summary as a regular MR note and set a commit status on
`head_sha`. GitLab discussions are separate calls, so partial success and retry
must be tracked per finding.

### Gitea

For Gitea 1.12 or newer, create a pull review:

```text
POST /repos/{owner}/{repo}/pulls/{index}/reviews
```

Send `commit_id=head_sha`, `event=COMMENT`, summary body, and comments containing
`path`, `body`, and `new_position` or `old_position`. Treat server version and
capability detection as runtime data because self-hosted instances vary.

If inline review is unavailable, publish the same findings in a general PR
comment and record the downgrade reason.

## Formatting

Inline comment:

````markdown
**[MAJOR] Possible nil dereference**

The value can be nil when ...

```suggestion
replacement code
```

<!-- review-service:finding:{finding_key} -->
````

Only emit suggestion blocks when the replacement exactly matches the selected
new-side range. Otherwise use a normal fenced block.

Summary comments should contain a stable marker and be updated where the
platform supports it. Inline comments should not be deleted on every rerun;
new revisions create new findings while old comments naturally become outdated.

## Failure Policy

- Signature failure: `401`, never persist the raw payload as accepted work.
- Duplicate delivery: return `202` with the existing delivery id.
- Unsupported event: return `204`.
- Inline location rejected by provider: retry once after refreshing revision,
  then downgrade that finding to summary.
- Rate limit or 5xx: exponential backoff with jitter.
- Auth or permission error: fail writeback without rerunning the model.
- Summary succeeds but some inline comments fail: review remains successful,
  writeback is `partial`, and failed items remain retryable.

## Migration

1. Add webhook installation/secret storage and inbox tables.
2. Add webhook routes that create existing `review.Request` jobs.
3. Change publisher input from `(Request, Result)` to `writeback.Review`.
4. Implement GitHub adapter first, then GitLab, then Gitea.
5. Keep the current general-note publisher as fallback until all configured
   instances report inline capability.
6. Expose writeback state and provider URLs in the review detail API.

## References

- GitHub REST API: pull request reviews and review comments
  - https://docs.github.com/en/rest/pulls/reviews
  - https://docs.github.com/en/rest/pulls/comments
- GitHub webhook signature validation
  - https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
- GitLab Discussions API
  - https://docs.gitlab.com/api/discussions/
- GitLab webhook security
  - https://docs.gitlab.com/user/project/integrations/webhooks/
- Gitea API and official Go SDK pull review model
  - https://docs.gitea.com/api/1.24/
  - https://gitea.com/gitea/go-sdk/src/branch/main/gitea/pull_review.go
