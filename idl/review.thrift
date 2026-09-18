struct CreateReviewReq {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: optional string project_id (api.body="project_id")
    4: optional string pull_request_id (api.body="pull_request_id")
    5: optional string merge_request_iid (api.body="merge_request_iid")
    6: optional string base_sha (api.body="base_sha")
    7: optional string head_sha (api.body="head_sha")
    8: optional string callback_mode (api.body="callback_mode")
    9: optional bool wait (api.body="wait")
    10: optional list<string> requirements (api.body="requirements")
    11: optional list<EphemeralReviewerInput> ephemeral_reviewers (api.body="ephemeral_reviewers")
    12: optional string commit_sha (api.body="commit_sha")
    // engine 审查引擎：'ocr'（默认，多智能体）| 'argus'（外部 Argus Bot，异步回报）。
    // 留空按项目策略 review_engine 解析，仍空则 ocr。
    13: optional string engine (api.body="engine")
    // task_kind engine=argus 时的任务类型：'code_review'（默认）|
    // 'requirement_compliance'（需求复核 R1-R5）| 'visual_check'（视觉检查）。
    14: optional string task_kind (api.body="task_kind")
}

struct EphemeralReviewerInput {
    1: optional string name (api.body="name")
    2: string background (api.body="background")
}

struct Gate {
    1: string status (api.body="status")
    2: optional bool passed (api.body="passed")
    3: optional string reason (api.body="reason")
}

struct Event {
    1: string stage (api.body="stage")
    2: string status (api.body="status")
    3: string message (api.body="message")
    4: optional map<string, string> details (api.body="details")
    5: optional string created_at (api.body="created_at")
    6: optional string how (api.body="how")
    7: optional string input (api.body="input")
    8: optional string output (api.body="output")
    9: optional i64 duration_ms (api.body="duration_ms")
}

struct SeverityCounts {
    1: i64 critical (api.body="critical")
    2: i64 major (api.body="major")
    3: i64 minor (api.body="minor")
    4: i64 info (api.body="info")
}

struct ScoreDeduction {
    1: string reason (api.body="reason")
    2: i64 points (api.body="points")
    3: optional i64 count (api.body="count")
}

struct Score {
    1: i64 value (api.body="value")
    2: string grade (api.body="grade")
    3: string version (api.body="version")
    4: optional list<ScoreDeduction> deductions (api.body="deductions")
    5: optional i64 changed_lines (api.body="changed_lines")
    6: optional double findings_density (api.body="findings_density")
}

struct OCRToolCalls {
    1: i64 total (api.body="total")
    2: optional map<string, i64> by_tool (api.body="by_tool")
}

struct OCRMetadata {
    1: optional string engine (api.body="engine")
    2: optional string status (api.body="status")
    3: optional i64 files_reviewed (api.body="files_reviewed")
    4: optional i64 comments (api.body="comments")
    5: optional i64 total_tokens (api.body="total_tokens")
    6: optional i64 input_tokens (api.body="input_tokens")
    7: optional i64 output_tokens (api.body="output_tokens")
    8: optional i64 cache_read_tokens (api.body="cache_read_tokens")
    9: optional i64 cache_write_tokens (api.body="cache_write_tokens")
    10: optional string elapsed (api.body="elapsed")
    11: optional OCRToolCalls tool_calls (api.body="tool_calls")
    12: optional string project_summary (api.body="project_summary")
    13: optional bool budget_exceeded (api.body="budget_exceeded")
}

struct ScopeMetadata {
    1: i64 changed_files (api.body="changed_files")
    2: i64 reviewed_files (api.body="reviewed_files")
    3: i64 excluded_files (api.body="excluded_files")
    4: optional list<string> ignore_paths (api.body="ignore_paths")
}

struct OCRWarning {
    1: optional string type (api.body="type")
    2: string message (api.body="message")
    3: optional string path (api.body="path")
}

struct Report {
    1: optional OCRMetadata ocr (api.body="ocr")
    2: optional ScopeMetadata scope (api.body="scope")
    3: optional Score score (api.body="score")
    4: optional SeverityCounts counts (api.body="counts")
    5: optional list<OCRWarning> warnings (api.body="warnings")
    6: optional bool complete (api.body="complete")
    7: optional bool generated (api.body="generated")
}

struct Analysis {
    1: optional string overview (api.body="overview")
    2: optional list<string> root_cause (api.body="root_cause")
    3: optional list<string> risks (api.body="risks")
    4: optional list<string> recommendations (api.body="recommendations")
}

struct Finding {
    1: string severity (api.body="severity")
    2: optional string category (api.body="category")
    3: optional string path (api.body="path")
    4: optional i64 line (api.body="line")
    5: optional i64 start_line (api.body="start_line")
    6: optional i64 end_line (api.body="end_line")
    7: string title (api.body="title")
    8: string body (api.body="body")
    9: optional string existing_code (api.body="existing_code")
    10: optional string suggestion_code (api.body="suggestion_code")
    11: optional string fingerprint (api.body="fingerprint")
}

struct ReviewResponse {
    1: string review_id (api.body="review_id")
    2: string status (api.body="status")
    3: string summary (api.body="summary")
    4: optional Gate gate (api.body="gate")
    5: optional Report report (api.body="report")
    6: optional Analysis analysis (api.body="analysis")
    7: optional list<Event> history (api.body="history")
    8: optional list<Finding> findings (api.body="findings")
    9: optional string error (api.body="error")
}

struct CreateReviewResp {
    1: ReviewResponse review (api.body="review")
}

struct GetReviewReq {
    1: string id (api.path="id")
}

struct ReviewDetail {
    1: string review_id (api.body="review_id")
    2: string provider (api.body="provider")
    3: string repo (api.body="repo")
    4: optional string project_id (api.body="project_id")
    5: optional string pull_request_id (api.body="pull_request_id")
    6: optional string merge_request_iid (api.body="merge_request_iid")
    7: optional string base_sha (api.body="base_sha")
    8: optional string head_sha (api.body="head_sha")
    9: optional string callback_mode (api.body="callback_mode")
    10: string status (api.body="status")
    11: string summary (api.body="summary")
    12: optional Gate gate (api.body="gate")
    13: optional Analysis analysis (api.body="analysis")
    14: optional list<Event> history (api.body="history")
    15: optional list<Finding> findings (api.body="findings")
    16: optional Report report (api.body="report")
    17: optional string raw_ocr (api.body="raw_ocr")
    18: optional string error (api.body="error")
    19: optional string created_at (api.body="created_at")
    20: optional string updated_at (api.body="updated_at")
    21: optional string author (api.body="author")
    22: optional string author_name (api.body="author_name")
    23: optional string author_avatar (api.body="author_avatar")
    24: optional OCRInput raw_ocr_input (api.body="raw_ocr_input")
    25: optional i64 retry_count (api.body="retry_count")
    // engine 生效审查引擎：'ocr' | 'argus'（解析后随记录返回）。
    26: optional string engine (api.body="engine")
    // task_kind argus 任务类型：'code_review' | 'requirement_compliance' | 'visual_check'
    // （engine=argus 时有值；OCR 记录为空）。
    27: optional string task_kind (api.body="task_kind")
}

struct OCRInputWorkspace {
    1: optional string provider (api.body="provider")
    2: optional string repo (api.body="repo")
    3: optional string base_sha (api.body="base_sha")
    4: optional string head_sha (api.body="head_sha")
    5: optional string dir (api.body="dir")
}

struct OCRInput {
    1: optional string binary (api.body="binary")
    2: optional list<string> args (api.body="args")
    3: optional OCRInputWorkspace workspace (api.body="workspace")
    4: optional string rule_path (api.body="rule_path")
    5: optional string started_at (api.body="started_at")
}

struct GetReviewResp {
    1: ReviewDetail review (api.body="review")
}

struct ListReviewsReq {
    1: optional string status (api.query="status")
    2: optional string provider (api.query="provider")
    3: optional string repo (api.query="repo")
    4: optional i64 page (api.query="page")
    5: optional i64 page_size (api.query="page_size")
}

struct ListItem {
    1: string review_id (api.body="review_id")
    2: string provider (api.body="provider")
    3: string repo (api.body="repo")
    4: string status (api.body="status")
    5: string summary (api.body="summary")
    6: optional Gate gate (api.body="gate")
    7: i64 findings (api.body="findings")
    8: i64 score (api.body="score")
    9: optional string grade (api.body="grade")
    10: bool report_generated (api.body="report_generated")
    11: optional string created_at (api.body="created_at")
    12: optional string updated_at (api.body="updated_at")
    13: optional string author (api.body="author")
    14: optional string author_name (api.body="author_name")
    15: optional string author_avatar (api.body="author_avatar")
    16: optional i64 retry_count (api.body="retry_count")
}

struct ListStats {
    1: i64 total (api.body="total")
    2: i64 running (api.body="running")
    3: i64 gate_failed (api.body="gate_failed")
    4: i64 findings (api.body="findings")
    5: optional i64 total_tokens (api.body="total_tokens")
}

struct ListReviewsResp {
    1: list<ListItem> items (api.body="items")
    2: i64 total (api.body="total")
    3: i64 page (api.body="page")
    4: i64 page_size (api.body="page_size")
    5: i64 total_pages (api.body="total_pages")
    6: ListStats stats (api.body="stats")
}

struct PreviewReq {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: optional string project_id (api.body="project_id")
    4: optional string pull_request_id (api.body="pull_request_id")
    5: optional string merge_request_iid (api.body="merge_request_iid")
    6: optional string base_sha (api.body="base_sha")
    7: optional string head_sha (api.body="head_sha")
}

struct PreviewResp {
    1: list<string> files (api.body="files")
}

struct ReviewActionReq {
    1: string id (api.path="id")
}

struct SuggestionPatchItem {
    1: i64 index (api.body="index")
    2: string path (api.body="path")
    3: i64 start_line (api.body="start_line")
    4: i64 end_line (api.body="end_line")
    5: optional string title (api.body="title")
}

struct SuggestionPatchSkipped {
    1: i64 index (api.body="index")
    2: optional string path (api.body="path")
    3: optional string title (api.body="title")
    4: string reason (api.body="reason")
}

struct SuggestionPatchPreview {
    1: string review_id (api.body="review_id")
    2: string patch (api.body="patch")
    3: list<SuggestionPatchItem> items (api.body="items")
    4: list<SuggestionPatchSkipped> skipped (api.body="skipped")
    5: i64 generated_count (api.body="generated_count")
    6: i64 skipped_count (api.body="skipped_count")
}

struct PreviewSuggestionPatchReq {
    1: string id (api.path="id")
    2: optional string finding_indexes (api.query="finding_indexes")
}

struct SuggestionPatchCheck {
    1: string review_id (api.body="review_id")
    2: string patch (api.body="patch")
    3: list<SuggestionPatchItem> items (api.body="items")
    4: list<SuggestionPatchSkipped> skipped (api.body="skipped")
    5: i64 generated_count (api.body="generated_count")
    6: i64 skipped_count (api.body="skipped_count")
    7: bool applicable (api.body="applicable")
    8: optional string error (api.body="error")
    9: optional string output (api.body="output")
}

struct CheckSuggestionPatchReq {
    1: string id (api.path="id")
    2: optional string finding_indexes (api.query="finding_indexes")
}

// ---------- Argus 承接（push 上报 + 反馈回流） ----------
// 契约：argus 审查完成后 POST /argus/reviews 落库（幂等键 platform+owner+repo+
// number+head_sha，重审覆盖；意见整组替换、(review,fp) 唯一兜底）。
// 前端「Argus 审查」页读列表/详情；误报抑制由服务端代理调 argus（凭证不出服务端）。

struct ArgusFindingItem {
    1: optional string fp (api.body="fp")
    2: string file (api.body="file")
    3: i32 line (api.body="line")
    4: string severity (api.body="severity")
    5: string comment (api.body="comment")
    6: optional string suggested_fix (api.body="suggested_fix")
    7: optional string by (api.body="by")
    8: optional string suppress_state (api.body="suppress_state")
}

struct ArgusIngestReq {
    1: optional string argus_task_id (api.body="argus_task_id")
    2: string platform (api.body="platform")
    3: string owner (api.body="owner")
    4: string repo (api.body="repo")
    5: string number (api.body="number")
    6: optional string title (api.body="title")
    7: string head_sha (api.body="head_sha")
    8: optional string base_branch (api.body="base_branch")
    9: string verdict (api.body="verdict")
    10: optional list<ArgusFindingItem> findings (api.body="findings")
    11: optional list<string> plugins (api.body="plugins")
    12: optional i64 duration_ms (api.body="duration_ms")
    13: optional i32 token_usage (api.body="token_usage")
    14: optional string occurred_at (api.body="occurred_at")
    // error_message 仅 verdict=error 时有意义（argus 侧审查失败原因）。
    15: optional string error_message (api.body="error_message")
}

struct ArgusReviewItem {
    1: i64 id (api.body="id")
    2: string argus_task_id (api.body="argus_task_id")
    3: string platform (api.body="platform")
    4: string owner (api.body="owner")
    5: string repo (api.body="repo")
    6: string number (api.body="number")
    7: string title (api.body="title")
    8: string head_sha (api.body="head_sha")
    9: optional string base_branch (api.body="base_branch")
    10: string verdict (api.body="verdict")
    11: i32 findings_total (api.body="findings_total")
    12: i32 findings_high (api.body="findings_high")
    13: i32 findings_medium (api.body="findings_medium")
    14: i32 findings_low (api.body="findings_low")
    15: optional list<string> plugins (api.body="plugins")
    16: optional i64 duration_ms (api.body="duration_ms")
    17: optional i32 token_usage (api.body="token_usage")
    18: string occurred_at (api.body="occurred_at")
    19: optional list<ArgusFindingItem> findings (api.body="findings")
    20: optional string error_message (api.body="error_message")
}

struct ArgusIngestResp {
    1: i64 id (api.body="id")
    2: string status (api.body="status")
}

struct ArgusListReviewsReq {
    1: optional string repo (api.query="repo")
    2: optional string severity (api.query="severity")
    3: optional string since (api.query="since")
    4: optional i64 page (api.query="page")
    5: optional i64 page_size (api.query="page_size")
    6: optional string platform (api.query="platform")
}

struct ArgusListReviewsResp {
    1: list<ArgusReviewItem> items (api.body="items")
    2: i64 total (api.body="total")
}

struct ArgusGetReviewReq {
    1: string id (api.path="id")
}

struct ArgusGetReviewResp {
    1: ArgusReviewItem review (api.body="review")
}

struct ArgusSuppressReq {
    // fp 指纹模式（≥4 位前缀全值直传）；与 file+comment 二选一，fp 优先。
    1: optional string fp (api.body="fp")
    // file+comment 兜底模式：旧数据无 fp 或指纹未命中（措辞漂移）时按原文匹配。
    2: optional string file (api.body="file")
    3: optional string comment (api.body="comment")
}

struct ArgusSuppressResp {
    1: bool matched (api.body="matched")
    2: optional string state (api.body="state")
    3: optional string message (api.body="message")
}

// ---------- Argus 治理面（观测代理 + 报告 + 问答） ----------
// 契约：argus 侧 GET /tasks、GET /usage/monthly、GET /reports/health、
// GET /tasks/:id（report 内联）、MCP /mcp ask_argus；本服务做带鉴权与
// 凭证收敛的代理（argus token 不出服务端）。

struct ArgusTaskSummary {
    1: string task_id (api.body="task_id")
    2: string status (api.body="status")
    // kind argus 侧字符串任务类型：requirement_compliance|code_review|visual_check。
    3: string kind (api.body="kind")
    4: string platform (api.body="platform")
    5: string owner (api.body="owner")
    6: string repo (api.body="repo")
    7: string number (api.body="number")
    8: optional string head_sha (api.body="head_sha")
    9: optional string verdict (api.body="verdict")
    10: i64 created_at (api.body="created_at")
}

struct ArgusListTasksReq {
    // status 空=全部；pending|running|done|failed|stale。
    1: optional string status (api.query="status")
    // limit 默认 20，上限 100（argus 侧约束）。
    2: optional i32 limit (api.query="limit")
}

struct ArgusListTasksResp {
    1: list<ArgusTaskSummary> tasks (api.body="tasks")
}

struct ArgusUsageReq {
}

struct ArgusUsageRepo {
    1: string platform (api.body="platform")
    2: string owner (api.body="owner")
    3: string repo (api.body="repo")
    4: i64 tokens (api.body="tokens")
    5: i64 tasks (api.body="tasks")
}

struct ArgusUsageResp {
    1: string month (api.body="month")
    2: i64 total (api.body="total")
    // budget argus 侧仓库月度 token 预算；0 = 未设置。
    3: i64 budget (api.body="budget")
    4: list<ArgusUsageRepo> repos (api.body="repos")
}

struct ArgusHealthReq {
}

struct ArgusHealthResp {
    // payload argus 健康报告原始 JSON（{generated_at,repos[],empty}），
    // 透传字符串由前端解析，规避 argus 侧结构演进破坏契约。
    1: string payload (api.body="payload")
}

struct ArgusReviewReportReq {
    1: string id (api.path="id")
}

struct ArgusReviewReportResp {
    1: string argus_task_id (api.body="argus_task_id")
    2: string status (api.body="status")
    3: optional string verdict (api.body="verdict")
    // report argus 侧 markdown 报告全文（懒拉取代理，不落本站库）。
    4: optional string report (api.body="report")
    5: optional i32 findings_count (api.body="findings_count")
}

struct ArgusAskReq {
    1: string platform (api.body="platform")
    2: string owner (api.body="owner")
    3: string repo (api.body="repo")
    4: string number (api.body="number")
    // question 追问；空串 = 返回该 PR 最近一次审查结论摘要。
    5: string question (api.body="question")
}

struct ArgusAskResp {
    1: string answer (api.body="answer")
}
