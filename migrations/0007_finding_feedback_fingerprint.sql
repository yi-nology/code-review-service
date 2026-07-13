-- 0007_finding_feedback_fingerprint.sql
-- 新增 finding_feedback.fingerprint 列，支撑 finding 生命周期与误报抑制。
--
-- 背景：feedback 原本仅靠 (review_id, finding_index) 关联 finding，
-- finding_index 是 Response.Findings 数组下标，一旦 findings 顺序变化反馈就指向错误 finding，
-- 且无法跨多次评审识别"同一问题"。
--
-- 现引入跨 review 稳定指纹（path+category+title签名+行桶），feedback 记录指纹后：
--   1. 同一指纹曾被标记 false_positive → 后续评审可自动降级/过滤该 finding
--   2. 跨 review 统计"新增/存量/已修复"生命周期
--
-- 历史反馈行无指纹（字段为空），仅新写入的反馈携带。唯一键仍为 (review_id, finding_index)，
-- 指纹加普通索引供按指纹查询历史反馈。
ALTER TABLE finding_feedback
  ADD COLUMN IF NOT EXISTS fingerprint TEXT DEFAULT '';

-- 按指纹查历史反馈的索引（误报抑制查询用）。
CREATE INDEX IF NOT EXISTS idx_finding_feedback_fingerprint
  ON finding_feedback (fingerprint)
  WHERE fingerprint <> '';
