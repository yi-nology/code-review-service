-- 0004_ocr_audience_default_agent.sql
-- 修正：OCR audience 默认值从 "human" 改为 "agent"。
--
-- 背景：review-service 是无人值守服务，调用方始终是程序，必须传 --audience agent
-- 才能拿到纯净的 JSON 输出（OCR 的 human 模式会在 stdout 混进度 UI，污染
-- --format json 的可解析输出）。此前默认 human 是 bug：进程化调用与官方契约相悖。
--
-- GORM 模型 models.OCRSettings.Audience 的列默认已改为 default:'agent'，
-- 但旧数据行仍是 "human"（或为空继承旧默认）。本迁移把历史行修正为 agent，
-- 让未显式配置 audience 的存量用户也立即受益。
--
-- 仅更新 audience 为空或显式等于 'human' 的行；用户若已显式设过其它值则不动。
UPDATE ocr_settings
   SET audience = 'agent'
 WHERE audience IS NULL OR audience = '' OR audience = 'human';
