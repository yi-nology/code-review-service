# Security Policy

[English](SECURITY.md) | [中文](SECURITY.zh-CN.md)

# Security Policy


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
