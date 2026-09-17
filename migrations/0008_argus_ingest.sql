-- 0008_argus_ingest.sql
-- Argus（PR 自动审查 Bot）审查结果承接链路：任务 + 意见两张表。
--
-- 背景：Argus 以 push 模型在每次审查完成后上报（POST /api/v1/argus/reviews），
-- review-service 落库并经前端「Argus 审查」页展示任务清单与意见明细。
-- 独立建表（不混入生产 reviews/findings 语义），前端可并排展示。
--
-- 幂等：任务业务键 (platform, owner, repo, number, head_sha) 唯一——同 PR 同 SHA
-- 重审覆盖（upsert，argus_task_id 更新为最新任务）；意见按 (review_id, fp) 唯一，
-- fp 为 Argus 规范指纹（与 finding_feedback.fingerprint 的防重思路同源），
-- 上报侧整组替换，指纹唯一约束仅作兜底。
-- 表结构同步定义于 internal/repository/postgres/models/argus_review.go（GORM 标签）。

CREATE TABLE IF NOT EXISTS argus_reviews (
  id              BIGSERIAL PRIMARY KEY,
  argus_task_id   TEXT        NOT NULL DEFAULT '',
  platform        TEXT        NOT NULL,
  owner           TEXT        NOT NULL,
  repo            TEXT        NOT NULL,
  number          TEXT        NOT NULL,
  head_sha        TEXT        NOT NULL,
  title           TEXT        NOT NULL DEFAULT '',
  base_branch     TEXT        NOT NULL DEFAULT '',
  verdict         TEXT        NOT NULL DEFAULT '',
  findings_total  INTEGER     NOT NULL DEFAULT 0,
  findings_high   INTEGER     NOT NULL DEFAULT 0,
  findings_medium INTEGER     NOT NULL DEFAULT 0,
  findings_low    INTEGER     NOT NULL DEFAULT 0,
  plugins         TEXT        NOT NULL DEFAULT '',
  duration_ms     BIGINT      NOT NULL DEFAULT 0,
  token_usage     INTEGER     NOT NULL DEFAULT 0,
  occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT idx_argus_review_biz UNIQUE (platform, owner, repo, number, head_sha)
);
CREATE INDEX IF NOT EXISTS idx_argus_reviews_occurred ON argus_reviews (occurred_at);
CREATE INDEX IF NOT EXISTS idx_argus_reviews_verdict ON argus_reviews (verdict);

CREATE TABLE IF NOT EXISTS argus_findings (
  id            BIGSERIAL PRIMARY KEY,
  review_id     BIGINT      NOT NULL REFERENCES argus_reviews(id) ON DELETE CASCADE,
  fp            TEXT        NOT NULL DEFAULT '',
  file          TEXT        NOT NULL,
  line          INTEGER     NOT NULL DEFAULT 0,
  severity      TEXT        NOT NULL DEFAULT '',
  comment       TEXT        NOT NULL DEFAULT '',
  suggested_fix TEXT        NOT NULL DEFAULT '',
  by            TEXT        NOT NULL DEFAULT '',
  suppress_state TEXT       NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- (review, fp) 唯一只约束非空 fp：fp 为空的行（上游旧数据）允许多条共存，
-- 否则一次上报多条无指纹意见会在第二条撞唯一约束、整组入库失败。
CREATE UNIQUE INDEX IF NOT EXISTS idx_argus_finding_review_fp
  ON argus_findings (review_id, fp) WHERE fp <> '';
CREATE INDEX IF NOT EXISTS idx_argus_finding_review
  ON argus_findings (review_id);
