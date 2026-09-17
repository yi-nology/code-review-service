-- 0009_argus_finding_suppress.sql
-- Argus 意见的误报抑制回流状态：review-service 前端点「误报抑制」→ 服务端代理
-- 调 argus POST /findings/suppress（按 fp）→ argus 侧 suppressed 后下一轮审查
-- 起自动剔除，argus 学习环同步记账。
--
-- suppress_state：''（未处理）| 'suppressed'（argus 确认命中并已置抑制）。
-- matched=false / 调用失败不落状态，由接口响应直接反馈给前端。

ALTER TABLE argus_findings
  ADD COLUMN IF NOT EXISTS suppress_state TEXT NOT NULL DEFAULT '';
