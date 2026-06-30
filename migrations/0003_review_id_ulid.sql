-- 0003_review_id_ulid.sql
-- PR-14 修复：review ID 从 "rv_YYYYMMDD_<32-bit hex>" 改为 ULID (26 字符 Base32)。
-- ULID 提供 80 bit 熵 + 字典序 = 时间序，外部链接更短，便于按 ID 排序做 range scan。
--
-- ALTER TYPE：reviews.id 列当前是 text/默认长度 32，ULID 长度 26 兼容，无需扩大；
-- 旧 id 长度 < 32 也不需要截断，varchar 长度限制是上限。
-- 若生产中 id 实际超过 26 字符（历史 rv_20260630_xxxxxxxx 形式 = 24 字符），
-- 这次 ALTER 是 no-op。
ALTER TABLE reviews
  ALTER COLUMN id TYPE varchar(32);

-- 同时把 review_id 列也确认下长度（避免历史 schema 不一致）
ALTER TABLE reviews
  ALTER COLUMN review_id TYPE varchar(32);

-- 说明：本迁移只在 schema 上放宽上限，不变更现有数据；
-- 新生成的 review ID 长度 26，旧的 rv_YYYYMMDD_xxx (24 字符) 继续可用。
SELECT 1;
