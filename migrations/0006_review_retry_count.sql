-- 0006_review_retry_count.sql
-- 新增 reviews.retry_count 列，用于 RequeueStale 雪崩防护。
--
-- 背景：服务重启或 worker 崩溃后，所有 status='running' 的评审会变成孤儿，
-- RequeueStale 会把它们重新标记为 queued 以便重新处理。但此前没有任何重试次数上限，
-- 一个持续失败的评审（如 LLM 端点长时间不可用）会在每次重启时被无限重新入队，
-- 重启瞬间大量孤儿任务同时涌入 worker，造成雪崩。
--
-- 本迁移新增 retry_count 列（默认 0）。RequeueStale 会：
--   1. WHERE 增加 retry_count < 上限 过滤，超过上限的直接标记 failed
--   2. UPDATE 时 retry_count = retry_count + 1
-- 这样持续失败的任务在达到上限后被永久标记失败，不再无谓重排，保护重启时的队列稳定。
ALTER TABLE reviews
  ADD COLUMN IF NOT EXISTS retry_count INTEGER NOT NULL DEFAULT 0;
