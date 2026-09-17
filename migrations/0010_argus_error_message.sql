-- 0010_argus_error_message.sql
-- Argus 上报契约补充：
-- 1. error_message：verdict=error 时的审查失败原因（此前前端只能显示「审查失败」
--    四个字，无法定位原因）。AutoMigrate 已同步模型（models.ArgusReview.ErrorMessage）。
-- 2. 索引自愈：早期版本 GORM 模型曾用 uniqueIndex tag 建过 (review_id, fp) 的
--    全量唯一索引（与 0008 的部分索引同名）。全量索引会让一次上报多条空 fp
--    意见（上游旧数据形态）在第二条撞唯一约束、整组入库失败。全量索引下
--    不可能存在重复行，先删后建（放宽为部分索引）总是安全的。
--    说明：启用 AutoMigrate 的部署无需手工执行本迁移，db.go 启动时已自愈。

ALTER TABLE argus_reviews
  ADD COLUMN IF NOT EXISTS error_message TEXT NOT NULL DEFAULT '';

DROP INDEX IF EXISTS idx_argus_finding_review_fp;
CREATE UNIQUE INDEX IF NOT EXISTS idx_argus_finding_review_fp
  ON argus_findings (review_id, fp) WHERE fp <> '';
