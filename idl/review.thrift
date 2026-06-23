include "webhook.thrift"
include "platform.thrift"

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

service reviewService {
    CreateReviewResp CreateReview(1: CreateReviewReq req) (api.post="/api/v1/reviews")
    GetReviewResp GetReview(1: GetReviewReq req) (api.get="/api/v1/reviews/:id")
    ListReviewsResp ListReviews(1: ListReviewsReq req) (api.get="/api/v1/reviews")
    PreviewResp PreviewReview(1: PreviewReq req) (api.post="/api/v1/reviews/preview")
    CreateReviewResp RerunReview(1: ReviewActionReq req) (api.post="/api/v1/reviews/:id/rerun")
    CreateReviewResp CancelReview(1: ReviewActionReq req) (api.post="/api/v1/reviews/:id/cancel")

    webhook.HandleWebhookResp HandleWebhook(1: webhook.HandleWebhookReq req) (api.post="/api/v1/webhooks/:provider/:installation")

    platform.ListProviderReposResp ListProviderRepos(1: platform.ListProviderReposReq req) (api.get="/api/v1/platforms/:provider/repos")
    platform.ListProviderBranchesResp ListProviderBranches(1: platform.ListProviderBranchesReq req) (api.get="/api/v1/platforms/:provider/repos/:owner/:repo/branches")
    platform.GetBranchSHAResp GetBranchSHA(1: platform.GetBranchSHAReq req) (api.get="/api/v1/platforms/:provider/repos/:owner/:repo/branch-sha")
    platform.ListProviderPullsResp ListProviderPulls(1: platform.ListProviderPullsReq req) (api.get="/api/v1/platforms/:provider/repos/:owner/:repo/pulls")
}
