-- AI Code Review 演示数据
--
-- 用于本地起控制台、截产品图、录宣传片。数据是**合成**的：
-- 仓库名用 acme/*，提交者用 ci-bot 等机器人账号，不含任何真实代码或客户信息。
--
-- 用法：
--   docker exec difftest-pg psql -U postgres -c "CREATE DATABASE ai_code_review_demo"
--   docker exec -i difftest-pg psql -U postgres -d ai_code_review_demo < scripts/seed-demo.sql
--
-- 幂等：先清空再写入，可反复执行。

BEGIN;

TRUNCATE reviews, review_sessions, git_platforms, project_policies, global_rules,
         reviewer_agents, ocr_settings, webhook_deliveries, writeback_items,
         finding_feedback, notifications, project_agents, audit_logs RESTART IDENTITY CASCADE;

-- ---------------------------------------------------------------- Git 平台（8 个）
-- 覆盖 SDK 的 7 个 + Gerrit（Gerrit 不在 SDK backend 列表，走独立 go-gerrit 客户端）。
INSERT INTO git_platforms (provider, base_url, token, enabled, skip_tls, created_at, updated_at) VALUES
  ('github',       'https://github.com',                    'ghp_***',      true,  false, now(), now()),
  ('gitlab',       'https://gitlab.example.com',            'glpat-***',    true,  false, now(), now()),
  ('gitea',        'https://gitea.example.com',             'gitea_***',    true,  false, now(), now()),
  ('forgejo',      'https://forgejo.example.com',           'forgejo_***',  true,  false, now(), now()),
  ('gitcode',      'https://gitcode.example.com',           'gitcode_***',  true,  false, now(), now()),
  ('gitee',        'https://gitee.example.com',             'gitee_***',    true,  false, now(), now()),
  ('tencent_code', 'https://git.code.tencent.example.com',  'tc_***',       true,  false, now(), now()),
  ('gerrit',       'https://gerrit.example.com',            'admin:http_pw',true,  false, now(), now());

-- ---------------------------------------------------------------- 全局规则
INSERT INTO global_rules (rules, created_at, updated_at) VALUES
  ('[
    {"id":"r-sec-hardcode","severity":"critical","title":"禁止硬编码密钥","rule":"代码中不得出现明文 API Key / Token / 密码常量","enabled":true},
    {"id":"r-sec-sql","severity":"critical","title":"SQL 拼接禁止","rule":"禁止把用户输入直接拼进 SQL 语句，必须参数化","enabled":true},
    {"id":"r-err-swallow","severity":"major","title":"禁止吞掉错误","rule":"禁止空的 catch 块或 _ = err 形式的错误忽略","enabled":true},
    {"id":"r-perf-nplus1","severity":"major","title":"N+1 查询","rule":"循环体内不得逐条查询数据库，应改为批量查询","enabled":true},
    {"id":"r-arch-layer","severity":"minor","title":"分层越界","rule":"service 层不得直接 import 仓储实现包","enabled":true},
    {"id":"r-doc-export","severity":"info","title":"导出符号需注释","rule":"对包外暴露的导出符号应有 doc 注释","enabled":false}
  ]'::jsonb, now(), now());

-- ---------------------------------------------------------------- 审查代理（审查员池）
-- tags 即能力标签：stack:* 用于按技术栈匹配，always 表示所有项目自动选中。
INSERT INTO reviewer_agents (name, display_name, description, background, rule_file, rule_package_ids, redundancy, enabled, priority, is_default, tags, created_at, updated_at) VALUES
  ('security',   '安全审查员',   '关注权限、输入校验、密钥管理与注入风险',   '攻防视角，逐条核对外部输入的信任边界', NULL, '[]'::jsonb, 0, true, 10, true,  '["always","security","memory-safety"]'::jsonb, now(), now()),
  ('backend',    '后端审查员',   '关注接口契约、错误处理、并发与数据一致性', '服务端视角，关注契约与边界',           NULL, '[]'::jsonb, 0, true, 20, true,  '["always","stack:go","stack:python"]'::jsonb, now(), now()),
  ('performance','性能审查员',   '关注复杂度、热点路径、缓存与 IO 放大',     '从流量峰值视角估算成本',               NULL, '[]'::jsonb, 0, true, 30, true,  '["always","performance"]'::jsonb, now(), now()),
  ('rust',       'Rust 审查员',  '关注所有权、生命周期与 unsafe 块',       'Rust 特有的内存安全与并发模型',        NULL, '[]'::jsonb, 0, true, 40, false, '["stack:rust"]'::jsonb, now(), now()),
  ('c_lang',     'C 语言审查员', '关注越界、UAF、整数溢出与未初始化内存',  'C 家族的内存安全',                     NULL, '[]'::jsonb, 0, true, 50, false, '["c-lang"]'::jsonb, now(), now()),
  ('frontend',   '前端审查员',   '关注渲染性能、可访问性与状态管理',       '浏览器端视角',                         NULL, '[]'::jsonb, 0, true, 60, false, '["stack:node"]'::jsonb, now(), now());

-- ---------------------------------------------------------------- 项目策略
-- fail_on = 门禁档位：critical / major / minor / all
-- gate_mode = strong（可挂分支保护必需检查）/ weak（仅留痕）
INSERT INTO project_policies (provider, repo, fail_on, review_engine, gate_mode, ignore_paths, rules, ocr_enabled, ocr_concurrency, ocr_timeout_minutes, ocr_max_tools, ocr_default_severity, ocr_background, ocr_audience, ocr_provider, ocr_url, ocr_model, ocr_max_git_procs, ocr_use_anthropic, ocr_protocol, ocr_language, created_at, updated_at) VALUES
  ('github','acme/service-template','major','ocr','strong',
   '["vendor/**","dist/**","node_modules/**","*.lock"]'::jsonb,
   '["r-sec-hardcode","r-err-swallow"]'::jsonb,
   true, 4, 30, 24, 'major', 'standard', 'default', 'anthropic', 'https://api.anthropic.com', 'claude-sonnet-4-20250514', 4, true, 'anthropic', 'zh', now(), now()),
  ('github','acme/console-web','critical','ocr','strong',
   '["dist/**","node_modules/**"]'::jsonb,
   '["r-sec-sql"]'::jsonb,
   true, 4, 30, 24, 'critical', 'standard', 'default', 'anthropic', 'https://api.anthropic.com', 'claude-sonnet-4-20250514', 4, true, 'anthropic', 'zh', now(), now()),
  ('gitlab','acme/payments-api','major','ocr','strong',
   '["vendor/**","*.pb.go"]'::jsonb,
   '[]'::jsonb,
   true, 6, 45, 32, 'major', 'thorough', 'default', 'anthropic', 'https://api.anthropic.com', 'claude-sonnet-4-20250514', 6, true, 'anthropic', 'zh', now(), now()),
  ('gerrit','acme/legacy-portal','minor','ocr','weak',
   '["third_party/**"]'::jsonb,
   '[]'::jsonb,
   true, 2, 20, 16, 'minor', 'standard', 'default', 'anthropic', 'https://api.anthropic.com', 'claude-sonnet-4-20250514', 2, true, 'anthropic', 'zh', now(), now());

-- ---------------------------------------------------------------- OCR 引擎设置
INSERT INTO ocr_settings (enabled, "binary", audience, default_severity, provider, url, auth_token, auth_header, model, use_anthropic, protocol, language, concurrency, max_git_procs, timeout_minutes, max_tools, context_window, max_output_tokens, policy_fail_on, policy_max_comments, policy_summary_only, policy_ignore_paths, telemetry_enabled, created_at, updated_at) VALUES
  (true, '/usr/local/bin/open-code-review', 'default', 'major', 'anthropic', 'https://api.anthropic.com', 'sk-***', 'x-api-key',
   'claude-sonnet-4-20250514', true, 'anthropic', 'zh', 4, 4, 30, 24, 200000, 32000,
   'major', 20, false, '["dist/**","node_modules/**","vendor/**"]'::jsonb, false, now(), now());

-- ---------------------------------------------------------------- 评审记录
-- ★ 号是宣传片 A3 的主角：门禁未通过 + 80/100 + 5 条问题（1 严重 + 4 高风险）
INSERT INTO reviews
(review_id, provider, repo, status, author, author_name, sender, pull_request_id, merge_request_iid,
 base_sha, head_sha, score, grade, summary, request, response, created_at, updated_at) VALUES
('01K8A1Q0M3N7PQR8V2X4YB6Z0', 'github', 'acme/service-template', 'success', 'ci-bot', 'CI Bot', 'ci-bot', '482', NULL,
 'a3f9c1e7b2d8456f9e0c3a7b5d2e1f4c8a9b0d3e', '7c1e9b4d2a8f6035c7e1b9d4a2f8036c5e7b1d904', 80, 'B',
 '评审完成，发现 5 个问题（严重 1 · 高风险 4）',
 '{"provider":"github","repo":"acme/service-template","pull_request_id":"482","callback_mode":"comment","engine":"ocr"}'::jsonb,
 '{"status":"success",
   "gate":{"passed":false,"status":"failed","reason":"门禁策略触发：5 should-fix items found"},
   "analysis":{
     "overview":"本次变更集中在鉴权中间件与订单接口，存在一处严重鉴权缺陷与四处高风险实现问题。",
     "root_cause":["token 有效期判断只在前端做了一次，服务端未二次校验","错误分支缺少统一兜底，导致失败被静默吞掉","部分查询在循环内逐条发起，量级上去后必然拖垮接口"],
     "risks":["越权访问风险","订单金额口径不一致","接口 P99 延迟随数据量线性上升"],
     "recommendations":["在网关层统一校验 token 有效期","错误分支补统一兜底并落日志","批量查询改造并加缓存"]},
   "findings":[
     {"severity":"critical","category":"security","path":"src/api/auth.ts","line":42,"start_line":38,"end_line":51,
      "title":"服务端未校验 token 过期时间",
      "body":"请求进入业务逻辑前只解析 token 内容，未比对 exp 字段。攻击者可用过期 token 继续访问受保护接口。",
      "existing_code":"const payload = decode(token)\nif (!payload) return res.status(401).end()\nreq.user = payload\nreturn next()",
      "suggestion_code":"const payload = decode(token)\nif (!payload) return res.status(401).end()\nif (payload.exp && payload.exp * 1000 < Date.now()) {\n  throw new UnauthorizedError(\"token expired\")\n}\nreq.user = payload\nreturn next()"},
     {"severity":"major","category":"error-handling","path":"src/service/order.ts","line":118,"start_line":112,"end_line":126,
      "title":"错误被静默吞掉，调用方无法感知失败",
      "body":"catch 块内只写了注释没有处理错误，上层会认为订单已成功落库，导致对账缺口。",
      "existing_code":"try {\n  await db.insert(order)\n} catch (e) {\n  // ignore\n}",
      "suggestion_code":"try {\n  await db.insert(order)\n} catch (e) {\n  logger.error({ err: e, orderId: order.id }, \"insert order failed\")\n  throw new OrderError(e)\n}"},
     {"severity":"major","category":"performance","path":"src/repo/order-repo.ts","line":77,"start_line":70,"end_line":88,
      "title":"循环内逐条查询，N+1 问题",
      "body":"遍历订单列表时对每条记录单独查询详情，数据量上来后请求数会线性放大。",
      "existing_code":"for (const id of ids) {\n  rows.push(await db.findById(id))\n}",
      "suggestion_code":"const rows = await db.findByIds(ids)\n// 一次批量查询，或按 id 建 Map 索引"},
     {"severity":"major","category":"correctness","path":"src/service/price.ts","line":203,"start_line":196,"end_line":212,
      "title":"金额用了浮点累加，存在精度丢失",
      "body":"连续累加浮点金额会出现尾数误差，订单总价与明细求和对不上。",
      "existing_code":"let total = 0\nfor (const i of items) total += i.price * i.qty\nreturn total",
      "suggestion_code":"const total = items.reduce((s, i) => s + Math.round(i.price * 100) * i.qty, 0)\nreturn total / 100"},
     {"severity":"major","category":"reliability","path":"src/repo/user-repo.ts","line":154,"start_line":150,"end_line":162,
      "title":"外部调用未设超时，依赖方卡死会拖垮整个 worker",
      "body":"调用用户中心接口没有设置超时，下游抖动时当前协程会一直挂住。",
      "existing_code":"const res = await usercenter.get(id)\nreturn res.data",
      "suggestion_code":"const res = await usercenter.get(id, { timeout: 2000 })\nreturn res.data"}
   ],
   "report":{
     "ocr":{"model":"claude-sonnet-4-20250514","engine":"ocr","status":"success","elapsed":"4m12s","comments":5,
            "session_id":"sess-01K8A1Q0M3N7PQR8V2X4Y","tool_calls":{"total":57,"by_tool":{"grep":18,"list_dir":8,"read_file":31}},
            "input_tokens":150000,"total_tokens":184320,"output_tokens":34320,"files_reviewed":14,"cache_read_tokens":96000},
     "scope":{"ignore_paths":["vendor/**","dist/**"],"changed_files":14,"changed_lines":486,"excluded_files":2,"reviewed_files":14},
     "score":{"grade":"B","value":80,"version":"severity-v2",
              "deductions":[{"count":1,"points":7,"reason":"严重问题 -7 分"},{"count":4,"points":13,"reason":"高风险问题 -13 分"}],
              "changed_lines":486,"findings_density":0.0103},
     "counts":{"critical":1,"major":4,"minor":0,"info":0},
     "complete":true,"generated":true},
   "history":[
     {"stage":"queued","status":"success","message":"任务入队","created_at":"2026-10-21T09:12:03Z"},
     {"stage":"worker","status":"success","message":"worker 领取任务并开始执行","created_at":"2026-10-21T09:12:04Z"},
     {"stage":"notify","status":"success","message":"review start notification sent","created_at":"2026-10-21T09:12:05Z"},
     {"stage":"repository","status":"success","message":"仓库准备完成","how":"go-git-platform clone","duration_ms":771,
      "input":"{\"author\":\"ci-bot\",\"base_sha\":\"a3f9c1e7b2d8456f9e0c3a7b5d2e1f4c8a9b0d3e\",\"engine\":\"ocr\",\"gate_mode\":\"strong\"}",
      "output":"{\"head_sha\":\"7c1e9b4d2a8f6035c7e1b9d4a2f8036c5e7b1d904\",\"changed_files\":14}"},
     {"stage":"policy","status":"success","message":"项目策略已加载","duration_ms":1,"output":"{\"fail_on\":\"major\",\"gate_mode\":\"strong\",\"ignore_paths\":4}"},
     {"stage":"rules","status":"success","message":"审查规则已合并","duration_ms":2,"output":"{\"global\":6,\"project\":2,\"package\":0,\"total\":8}"},
     {"stage":"agent","status":"success","message":"已按技术栈选择审查员","duration_ms":3,"output":"{\"selected\":[\"security\",\"backend\",\"performance\"]}"},
     {"stage":"scope","status":"success","message":"文件过滤完成","duration_ms":4,"output":"{\"changed_files\":14,\"excluded\":2,\"ignore_paths\":4}"},
     {"stage":"memory","status":"success","message":"长期记忆已召回","duration_ms":12,"output":"{\"atoms\":7,\"hits\":2}"},
     {"stage":"ocr_config","status":"success","message":"OCR 参数已合并","duration_ms":1,"output":"{\"model\":\"claude-sonnet-4-20250514\",\"concurrency\":4,\"max_tools\":24}"},
     {"stage":"workflow","status":"success","message":"多 Agent 工作流执行完成","how":"context → parallel review → aggregate → discourse → synthesis",
      "duration_ms":251840,"output":"{\"rounds\":2,\"agents\":3,\"findings\":5}"},
     {"stage":"judge","status":"success","message":"AI 判断与综合裁决完成","duration_ms":18400,"output":"{\"verdict\":\"should-fix\",\"findings\":5}"},
     {"stage":"writeback","status":"success","message":"结果已写回 PR","how":"inline comments + summary","duration_ms":3140,
      "output":"{\"published\":5,\"degraded\":0}"},
     {"stage":"gate","status":"success","message":"报告与门禁结论已生成","duration_ms":58,"output":"{\"score\":80,\"gate\":\"failed\"}"}
   ]}'::jsonb,
 now() - interval '2 hours', now() - interval '2 hours'),

('01K8A1Q0M3N7PQR8V2X4YB6Z1', 'github', 'acme/console-web', 'success', 'ci-bot', 'CI Bot', 'ci-bot', '481', NULL,
 'b4e2d1a9c7f0365e8b1a4d2c9e7f035a6c8b2d1e', '3d5c8b1e7a9f2046c8b3e1d5a7f9026b4c8d1e5a', 92, 'A',
 '评审完成，未发现阻断性问题',
 '{"provider":"github","repo":"acme/console-web","pull_request_id":"481","callback_mode":"comment","engine":"ocr"}'::jsonb,
 '{"status":"success","gate":{"passed":true,"status":"passed","reason":"无 critical/major 级未解决问题"},
   "analysis":{"overview":"变更以组件拆分与样式收敛为主，实现方式与既有约定一致。","root_cause":[],"risks":[],"recommendations":[]},
   "findings":[{"severity":"minor","category":"readability","path":"src/components/Table.tsx","line":88,
     "title":"组件缺少 props 类型注释","body":"新增的 props 未补充 TSDoc，后续维护成本偏高。","existing_code":"export default function Table({ rows, onPick }: Props) {"}],
   "report":{"ocr":{"model":"claude-sonnet-4-20250514","engine":"ocr","status":"success","elapsed":"2m41s","comments":1,
              "session_id":"sess-01K8A1Q0M3N7PQR8V2X4Z","input_tokens":82000,"total_tokens":101440,"output_tokens":19440,
              "files_reviewed":9,"cache_read_tokens":51200},
             "scope":{"ignore_paths":["dist/**"],"changed_files":9,"changed_lines":212,"excluded_files":1,"reviewed_files":9},
             "score":{"grade":"A","value":92,"version":"severity-v2","deductions":[{"count":1,"points":8,"reason":"中风险问题 -8 分"}],
                      "changed_lines":212,"findings_density":0.0377},
             "counts":{"critical":0,"major":0,"minor":1,"info":0},"complete":true,"generated":true},
   "history":[{"stage":"queued","status":"success","message":"任务入队"},
              {"stage":"workflow","status":"success","message":"多 Agent 工作流执行完成","duration_ms":161400},
              {"stage":"writeback","status":"success","message":"结果已写回 PR","duration_ms":2140},
              {"stage":"gate","status":"success","message":"报告与门禁结论已生成","duration_ms":41}]}'::jsonb,
 now() - interval '1 day', now() - interval '1 day'),

('01K8A1Q0M3N7PQR8V2X4YB6Z2', 'gitlab', 'acme/payments-api', 'success', 'ci-bot', 'CI Bot', 'ci-bot', NULL, '216',
 'c5f3a2b8d1e9407f6a3b5c2d9e8f1407b3c6d1a', '9e2d4c1b7a3f8056d2b4e8c7a1f3950d6b8e2c4a', 88, 'A',
 '评审完成，发现 1 个高风险问题',
 '{"provider":"gitlab","repo":"acme/payments-api","merge_request_iid":"216","callback_mode":"comment","engine":"ocr"}'::jsonb,
 '{"status":"success","gate":{"passed":true,"status":"passed","reason":"无 critical 级未解决问题"},
   "analysis":{"overview":"对账逻辑调整方向正确，但缺少幂等键保护。","root_cause":["回调未带幂等键"],"risks":["重复入账"],"recommendations":["补幂等键"]},
   "findings":[{"severity":"major","category":"reliability","path":"internal/settle/apply.go","line":141,
     "title":"回调缺少幂等键，重复投递会重复入账","body":"支付回调未做幂等处理，重复投递会重复入账。",
     "existing_code":"func apply(cb Callback) error {\n  return db.Create(tx).Error\n}"}],
   "report":{"ocr":{"model":"claude-sonnet-4-20250514","engine":"ocr","status":"success","elapsed":"3m58s","comments":1,
              "session_id":"sess-01K8A1Q0M3N7PQR8V2X50","input_tokens":96000,"total_tokens":128640,"output_tokens":32640,
              "files_reviewed":11,"cache_read_tokens":64000},
             "scope":{"ignore_paths":["vendor/**","*.pb.go"],"changed_files":11,"changed_lines":318,"excluded_files":3,"reviewed_files":11},
             "score":{"grade":"A","value":88,"version":"severity-v2","deductions":[{"count":1,"points":12,"reason":"高风险问题 -12 分"}],
                      "changed_lines":318,"findings_density":0.0377},
             "counts":{"critical":0,"major":1,"minor":0,"info":0},"complete":true,"generated":true},
   "history":[{"stage":"queued","status":"success","message":"任务入队"},
              {"stage":"workflow","status":"success","message":"多 Agent 工作流执行完成","duration_ms":238800},
              {"stage":"writeback","status":"success","message":"结果已写回 MR","duration_ms":2860},
              {"stage":"gate","status":"success","message":"报告与门禁结论已生成","duration_ms":52}]}'::jsonb,
 now() - interval '2 days', now() - interval '2 days'),

('01K8A1Q0M3N7PQR8V2X4YB6Z3', 'gerrit', 'acme/legacy-portal', 'success', 'ci-bot', 'CI Bot', 'ci-bot', NULL, NULL,
 'd6a4b3c9e2f1508a7b4c6d3f9a2e1508c4b7d2e', '1b6d5e4a3c2f9087b6a5d4c3e2f1098a7b6c5d4',
 72, 'B', '评审完成，发现 2 个问题',
 '{"provider":"gerrit","repo":"acme/legacy-portal","callback_mode":"comment","engine":"ocr"}'::jsonb,
 '{"status":"success","gate":{"passed":true,"status":"passed","reason":"弱门禁：结果仅留痕，不阻塞合并"},
   "analysis":{"overview":"老旧门户的表单改造，逻辑基本可用。","root_cause":[],"risks":[],"recommendations":[]},
   "findings":[{"severity":"minor","category":"style","path":"web/form/validate.js","line":67,
     "title":"表单校验未做空值保护","body":"部分输入为空时直接取属性会抛异常。","existing_code":"if (!form.name.length) show(form.title)"}],
   "report":{"ocr":{"model":"claude-sonnet-4-20250514","engine":"ocr","status":"success","elapsed":"1m52s","comments":1,
              "session_id":"sess-01K8A1Q0M3N7PQR8V2X60","input_tokens":41000,"total_tokens":52800,"output_tokens":11800,
              "files_reviewed":6,"cache_read_tokens":24000},
             "scope":{"ignore_paths":["third_party/**"],"changed_files":6,"changed_lines":94,"excluded_files":2,"reviewed_files":6},
             "score":{"grade":"B","value":72,"version":"severity-v2","deductions":[{"count":1,"points":8,"reason":"中风险问题 -8 分"}],
                      "changed_lines":94,"findings_density":0.0851},
             "counts":{"critical":0,"major":0,"minor":1,"info":0},"complete":true,"generated":true},
   "history":[{"stage":"queued","status":"success","message":"任务入队"},
              {"stage":"repository","status":"success","message":"仓库准备完成","duration_ms":640},
              {"stage":"workflow","status":"success","message":"多 Agent 工作流执行完成","duration_ms":112000},
              {"stage":"writeback","status":"success","message":"结果已写回 Gerrit","duration_ms":1860},
              {"stage":"gate","status":"success","message":"报告与门禁结论已生成","duration_ms":33}]}'::jsonb,
 now() - interval '3 days', now() - interval '3 days'),

('01K8A1Q0M3N7PQR8V2X4YB6Z4', 'gitea', 'acme/mobile-app', 'success_with_publish_failed', 'ci-bot', 'CI Bot', 'ci-bot', '96', NULL,
 'e7b5c4d0a3f2619b8c5d7e4f0a3b2619d5e8c4b', '2c7e6f5a4b3d0928c1b0a9f8e7d6c5b4a3f2e1d0',
 61, 'C', '评审完成，但评论写回降级为仅总结',
 '{"provider":"gitea","repo":"acme/mobile-app","pull_request_id":"96","callback_mode":"comment","engine":"ocr"}'::jsonb,
 '{"status":"success_with_publish_failed",
   "gate":{"passed":false,"status":"failed","reason":"门禁策略触发：2 should-fix items found"},
   "analysis":{"overview":"离线缓存逻辑存在一致性问题。","root_cause":["本地缓存与服务端未做版本对齐"],"risks":["用户看到旧数据"],"recommendations":["引入缓存版本号"]},
   "findings":[{"severity":"major","category":"consistency","path":"src/store/cache.ts","line":93,
     "title":"本地缓存与服务端未做版本对齐","body":"缓存没有版本号，服务端更新后客户端仍读旧值。",
     "existing_code":"const cached = localStorage.getItem(key)\nreturn cached ? JSON.parse(cached) : null"}],
   "report":{"ocr":{"model":"claude-sonnet-4-20250514","engine":"ocr","status":"success","elapsed":"2m20s","comments":1,
              "session_id":"sess-01K8A1Q0M3N7PQR8V2X70","input_tokens":52000,"total_tokens":67200,"output_tokens":15200,
              "files_reviewed":7,"cache_read_tokens":32000},
             "scope":{"ignore_paths":["dist/**"],"changed_files":7,"changed_lines":141,"excluded_files":1,"reviewed_files":7},
             "score":{"grade":"C","value":61,"version":"severity-v2","deductions":[{"count":1,"points":13,"reason":"高风险问题 -13 分"}],
                      "changed_lines":141,"findings_density":0.0922},
             "counts":{"critical":0,"major":1,"minor":0,"info":0},"complete":true,"generated":true},
   "history":[{"stage":"queued","status":"success","message":"任务入队"},
              {"stage":"workflow","status":"success","message":"多 Agent 工作流执行完成","duration_ms":140000},
              {"stage":"writeback","status":"degraded","message":"writeback degraded (stale): head 2c7e6f5a -> 9f1a2b3c",
               "how":"inline comments + summary","duration_ms":2200,"output":"{\"published\":0,\"degraded\":1}"},
              {"stage":"gate","status":"success","message":"报告与门禁结论已生成","duration_ms":37}]}'::jsonb,
 now() - interval '5 days', now() - interval '5 days'),

('01K8A1Q0M3N7PQR8V2X4YB6Z5', 'gitcode', 'acme/data-pipeline', 'success', 'ci-bot', 'CI Bot', 'ci-bot', '73', NULL,
 'f8c6d5e1b4a3720c9d6e5f4a3b2c1908d7e6f5a', '3d8f7e6c5b4a1928d0c1b2a3f4e5d6c7b8a9f0e1',
 100, 'A', '评审完成，未发现问题',
 '{"provider":"gitcode","repo":"acme/data-pipeline","pull_request_id":"73","callback_mode":"comment","engine":"ocr"}'::jsonb,
 '{"status":"success","gate":{"passed":true,"status":"passed","reason":"无 critical/major 级未解决问题"},
   "analysis":{"overview":"数据管道调度改动清晰，无明显风险。","root_cause":[],"risks":[],"recommendations":[]},
   "findings":[],
   "report":{"ocr":{"model":"claude-sonnet-4-20250514","engine":"ocr","status":"success","elapsed":"1m36s","comments":0,
              "session_id":"sess-01K8A1Q0M3N7PQR8V2X80","input_tokens":33000,"total_tokens":41200,"output_tokens":8200,
              "files_reviewed":5,"cache_read_tokens":18000},
             "scope":{"ignore_paths":["vendor/**"],"changed_files":5,"changed_lines":68,"excluded_files":1,"reviewed_files":5},
             "score":{"grade":"A","value":100,"version":"severity-v2","deductions":[],"changed_lines":68,"findings_density":0},
             "counts":{"critical":0,"major":0,"minor":0,"info":0},"complete":true,"generated":true},
   "history":[{"stage":"queued","status":"success","message":"任务入队"},
              {"stage":"workflow","status":"success","message":"多 Agent 工作流执行完成","duration_ms":96000},
              {"stage":"gate","status":"success","message":"报告与门禁结论已生成","duration_ms":22}]}'::jsonb,
 now() - interval '6 days', now() - interval '6 days'),

('01K8A1Q0M3N7PQR8V2X4YB6Z6', 'gitee', 'acme/gateway', 'failed', 'ci-bot', 'CI Bot', 'ci-bot', '51', NULL,
 'a9d7e6f5c4b38210e9f8a7b6c5d4e3f2a1b0c9d8', '4e9f8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0',
 0, NULL, '评审执行失败：OCR 引擎返回 401',
 '{"provider":"gitee","repo":"acme/gateway","pull_request_id":"51","callback_mode":"comment","engine":"ocr"}'::jsonb,
 '{"status":"failed","gate":{"passed":false,"status":"error","reason":"审查执行失败，流程未完整完成"},
   "error":"OCR 引擎鉴权失败：HTTP 401，请检查模型服务的 API Key",
   "findings":[],
   "report":{"ocr":{"model":"claude-sonnet-4-20250514","engine":"ocr","status":"failed","elapsed":"12s","comments":0,
              "session_id":"sess-01K8A1Q0M3N7PQR8V2X90","input_tokens":0,"total_tokens":0,"output_tokens":0,
              "files_reviewed":0},
             "scope":{"changed_files":8,"changed_lines":203,"excluded_files":0,"reviewed_files":0},
             "counts":{"critical":0,"major":0,"minor":0,"info":0},"complete":false,"generated":false},
   "history":[{"stage":"queued","status":"success","message":"任务入队"},
              {"stage":"repository","status":"success","message":"仓库准备完成","duration_ms":702},
              {"stage":"workflow","status":"failed","message":"OCR 引擎鉴权失败：HTTP 401","duration_ms":12000,
               "output":"{\"error\":\"authentication failed: invalid api key\"}"},
              {"stage":"failed","status":"failed","message":"review execution failed"}]}'::jsonb,
 now() - interval '8 days', now() - interval '8 days'),

('01K8A1Q0M3N7PQR8V2X4YB6Z7', 'forgejo', 'acme/api-gateway', 'success', 'ci-bot', 'CI Bot', 'ci-bot', '44', NULL,
 'b8e7f6a5c4d39201e9f8a7b6c5d4e3f2a1b0c9', '5f0a9b8c7d6e5f4a3b2c1d0e9f8a7b6c5d4e3f2a1',
 95, 'A', '评审完成，未发现阻断性问题',
 '{"provider":"forgejo","repo":"acme/api-gateway","pull_request_id":"44","callback_mode":"comment","engine":"ocr"}'::jsonb,
 '{"status":"success","gate":{"passed":true,"status":"passed","reason":"无 critical/major 级未解决问题"},
   "analysis":{"overview":"路由配置调整，结构清晰。","root_cause":[],"risks":[],"recommendations":[]},
   "findings":[],
   "report":{"ocr":{"model":"claude-sonnet-4-20250514","engine":"ocr","status":"success","elapsed":"1m28s","comments":0,
              "session_id":"sess-01K8A1Q0M3N7PQR8V2XA0","input_tokens":28000,"total_tokens":35200,"output_tokens":7200,
              "files_reviewed":4,"cache_read_tokens":16000},
             "scope":{"changed_files":4,"changed_lines":52,"excluded_files":0,"reviewed_files":4},
             "score":{"grade":"A","value":95,"version":"severity-v2","deductions":[],"changed_lines":52,"findings_density":0},
             "counts":{"critical":0,"major":0,"minor":0,"info":0},"complete":true,"generated":true},
   "history":[{"stage":"queued","status":"success","message":"任务入队"},
              {"stage":"workflow","status":"success","message":"多 Agent 工作流执行完成","duration_ms":88000},
              {"stage":"gate","status":"success","message":"报告与门禁结论已生成","duration_ms":19}]}'::jsonb,
 now() - interval '9 days', now() - interval '9 days'),

('01K8A1Q0M3N7PQR8V2X4YB6Z8', 'tencent_code', 'acme/notifications', 'success', 'ci-bot', 'CI Bot', 'ci-bot', '28', NULL,
 'c9f8a7b6c5d4e3021f9a8b7c6d5e4f3a2b1c0d', '6a1b0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b',
 100, 'A', '评审完成，未发现问题',
 '{"provider":"tencent_code","repo":"acme/notifications","pull_request_id":"28","callback_mode":"comment","engine":"ocr"}'::jsonb,
 '{"status":"success","gate":{"passed":true,"status":"passed","reason":"无 critical/major 级未解决问题"},
   "analysis":{"overview":"通知模板改动，无明显风险。","root_cause":[],"risks":[],"recommendations":[]},
   "findings":[],
   "report":{"ocr":{"model":"claude-sonnet-4-20250514","engine":"ocr","status":"success","elapsed":"1m12s","comments":0,
              "session_id":"sess-01K8A1Q0M3N7PQR8V2XB0","input_tokens":21000,"total_tokens":26800,"output_tokens":5800,
              "files_reviewed":3,"cache_read_tokens":12000},
             "scope":{"changed_files":3,"changed_lines":31,"excluded_files":0,"reviewed_files":3},
             "score":{"grade":"A","value":100,"version":"severity-v2","deductions":[],"changed_lines":31,"findings_density":0},
             "counts":{"critical":0,"major":0,"minor":0,"info":0},"complete":true,"generated":true},
   "history":[{"stage":"queued","status":"success","message":"任务入队"},
              {"stage":"workflow","status":"success","message":"多 Agent 工作流执行完成","duration_ms":72000},
              {"stage":"gate","status":"success","message":"报告与门禁结论已生成","duration_ms":15}]}'::jsonb,
 now() - interval '11 days', now() - interval '11 days');

-- ---------------------------------------------------------------- 会话
INSERT INTO review_sessions (session_id, review_id, round, phase, status, progress, agent_results, discussion, final_report, completed_at, created_at, updated_at) VALUES
('sess-01K8A1Q0M3N7PQR8V2X4Y', '01K8A1Q0M3N7PQR8V2X4YB6Z0', 1, 'complete', 'succeeded', 1.0,
 '[{"agent":"security","status":"completed","findings":2},{"agent":"backend","status":"completed","findings":2},{"agent":"performance","status":"completed","findings":1}]'::jsonb,
 '[{"round":1,"topic":"token 校验缺失是否属于阻断级","conclusion":"一致认为阻断"}]'::jsonb,
 '{"findings":5,"score":80}'::jsonb, now() - interval '2 hours', now() - interval '2 hours', now() - interval '2 hours'),
('sess-01K8A1Q0M3N7PQR8V2X4Z', '01K8A1Q0M3N7PQR8V2X4YB6Z1', 1, 'complete', 'succeeded', 1.0,
 '[{"agent":"security","status":"completed","findings":0},{"agent":"backend","status":"completed","findings":1}]'::jsonb,
 '[]'::jsonb, '{"findings":1,"score":92}'::jsonb, now() - interval '1 day', now() - interval '1 day', now() - interval '1 day'),
('sess-01K8A1Q0M3N7PQR8V2X50', '01K8A1Q0M3N7PQR8V2X4YB6Z2', 1, 'complete', 'succeeded', 1.0,
 '[{"agent":"backend","status":"completed","findings":1},{"agent":"performance","status":"completed","findings":0}]'::jsonb,
 '[]'::jsonb, '{"findings":1,"score":88}'::jsonb, now() - interval '2 days', now() - interval '2 days', now() - interval '2 days');

-- ---------------------------------------------------------------- 诊断中心：webhook 投递
INSERT INTO webhook_deliveries (provider, installation_id, delivery_id, event_type, action, payload_sha256, status, received_at, processed_at) VALUES
('github','install-1','d-1001','push','created','a1b2c3d4e5f60718293a4b5c6d7e8f90','success', now() - interval '12 minutes', now() - interval '12 minutes'),
('github','install-1','d-1002','pull_request','opened','b2c3d4e5f60718293a4b5c6d7e8f9001','success', now() - interval '35 minutes', now() - interval '35 minutes'),
('gitlab','install-2','d-1003','push','updated','c3d4e5f60718293a4b5c6d7e8f900112','success', now() - interval '1 hour', now() - interval '1 hour'),
('gitlab','install-2','d-1004','merge_request','opened','d4e5f60718293a4b5c6d7e8f90011223','success', now() - interval '2 hours', now() - interval '2 hours'),
('gitea','install-3','d-1005','push','created','e5f60718293a4b5c6d7e8f9001122334','success', now() - interval '3 hours', now() - interval '3 hours'),
('gerrit','install-4','d-1006','patchset-created','created','f60718293a4b5c6d7e8f900112233445','success', now() - interval '4 hours', now() - interval '4 hours'),
('gitea','install-3','d-1007','push','created','0718293a4b5c6d7e8f90011223344556','failed', now() - interval '5 hours', now() - interval '5 hours'),
('gitcode','install-5','d-1008','pull_request','opened','18293a4b5c6d7e8f9001122334455667','success', now() - interval '7 hours', now() - interval '7 hours'),
('github','install-1','d-1009','pull_request','synchronize','293a4b5c6d7e8f900112233445566778','success', now() - interval '9 hours', now() - interval '9 hours'),
('gitee','install-6','d-1010','push','created','3a4b5c6d7e8f90011223344556677889','success', now() - interval '11 hours', now() - interval '11 hours'),
('tencent_code','install-7','d-1011','pull_request','opened','4b5c6d7e8f9001122334455667788990','success', now() - interval '13 hours', now() - interval '13 hours'),
('forgejo','install-8','d-1012','push','created','5c6d7e8f9001122334455667788990011','success', now() - interval '20 hours', now() - interval '20 hours');

-- ---------------------------------------------------------------- 诊断中心：写回记录
INSERT INTO writeback_items (review_id, finding_key, kind, provider_external_id, provider_url, status, attempts, published_at) VALUES
('01K8A1Q0M3N7PQR8V2X4YB6Z0','critical:src/api/auth.ts:42','inline','c-8801','https://github.com/acme/service-template/pull/482#discussion_r8801','published',1, now() - interval '2 hours'),
('01K8A1Q0M3N7PQR8V2X4YB6Z0','major:src/service/order.ts:118','inline','c-8802','https://github.com/acme/service-template/pull/482#discussion_r8802','published',1, now() - interval '2 hours'),
('01K8A1Q0M3N7PQR8V2X4YB6Z0','major:src/repo/order-repo.ts:77','inline','c-8803','https://github.com/acme/service-template/pull/482#discussion_r8803','published',1, now() - interval '2 hours'),
('01K8A1Q0M3N7PQR8V2X4YB6Z0','major:src/service/price.ts:203','inline','c-8804','https://github.com/acme/service-template/pull/482#discussion_r8804','published',1, now() - interval '2 hours'),
('01K8A1Q0M3N7PQR8V2X4YB6Z0','major:src/repo/user-repo.ts:154','inline','c-8805','https://github.com/acme/service-template/pull/482#discussion_r8805','published',1, now() - interval '2 hours'),
('01K8A1Q0M3N7PQR8V2X4YB6Z1','minor:src/components/Table.tsx:88','inline','c-8806','https://github.com/acme/console-web/pull/481#discussion_r8806','published',1, now() - interval '1 day'),
('01K8A1Q0M3N7PQR8V2X4YB6Z4','major:src/store/cache.ts:93','summary','s-2201','https://gitea.example.com/acme/mobile-app/pulls/96','failed',2, NULL),
('01K8A1Q0M3N7PQR8V2X4YB6Z2','major:internal/settle/apply.go:141','inline','m-4401','https://gitlab.example.com/acme/payments-api/-/merge_requests/216#note_4401','published',1, now() - interval '2 days'),
('01K8A1Q0M3N7PQR8V2X4YB6Z3','minor:web/form/validate.js:67','inline','g-3301','https://gerrit.example.com/c/acme/legacy-portal/+/7/1','published',1, now() - interval '3 days'),
('01K8A1Q0M3N7PQR8V2X4YB6Z8','summary','summary','t-1101','https://git.code.tencent.example.com/acme/notifications/pulls/28','published',1, now() - interval '11 days');

COMMIT;
