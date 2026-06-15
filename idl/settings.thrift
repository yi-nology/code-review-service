struct ReviewRule {
    1: string path (api.body="path")
    2: string rule (api.body="rule")
}

struct GlobalRules {
    1: optional list<ReviewRule> rules (api.body="rules")
}

struct GetGlobalRulesResp {
    1: GlobalRules rules (api.body="rules")
}

struct UpdateGlobalRulesReq {
    1: GlobalRules rules (api.body="rules")
}

struct UpdateGlobalRulesResp {
    1: GlobalRules rules (api.body="rules")
}

struct TestLLMResp {
    1: TestLLMResult result (api.body="result")
}

struct TestLLMResult {
    1: bool success (api.body="success")
    2: optional string message (api.body="message")
}

struct OCRSettings {
    1: bool enabled (api.body="enabled")
    2: optional string provider (api.body="provider")
    3: optional string plan (api.body="plan")
    4: optional string url (api.body="url")
    5: optional string auth_token (api.body="auth_token")
    6: optional string auth_header (api.body="auth_header")
    7: optional string model (api.body="model")
    8: optional bool use_anthropic (api.body="use_anthropic")
    9: optional string extra_body (api.body="extra_body")
    10: optional string language (api.body="language")
    11: optional i64 concurrency (api.body="concurrency")
    12: optional i64 max_git_procs (api.body="max_git_procs")
    13: optional i64 timeout_minutes (api.body="timeout_minutes")
    14: optional i64 max_tools (api.body="max_tools")
    15: optional string background (api.body="background")
}

struct GetOCRSettingsResp {
    1: OCRSettings settings (api.body="settings")
}

struct UpdateOCRSettingsReq {
    1: OCRSettings settings (api.body="settings")
}

struct UpdateOCRSettingsResp {
    1: OCRSettings settings (api.body="settings")
}

struct ProjectPolicy {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: string fail_on (api.body="fail_on")
    4: optional string ignore_paths (api.body="ignore_paths")
    5: optional string rules (api.body="rules")
}

struct ListProjectPoliciesResp {
    1: list<ProjectPolicy> items (api.body="items")
}

struct PutProjectPolicyReq {
    1: ProjectPolicy policy (api.body="policy")
}

struct PutProjectPolicyResp {
    1: ProjectPolicy policy (api.body="policy")
}

struct DeleteProjectPolicyReq {
    1: string provider (api.query="provider")
    2: string repo (api.query="repo")
}

struct DeleteProjectPolicyResp {}

struct GitPlatformInfo {
    1: string provider (api.body="provider")
    2: string base_url (api.body="base_url")
    3: string token (api.body="token")
    4: bool enabled (api.body="enabled")
    5: optional string webhook_secret (api.body="webhook_secret")
    6: optional string installation_id (api.body="installation_id")
}

struct ListPlatformsResp {
    1: list<GitPlatformInfo> items (api.body="items")
}

struct PutPlatformReq {
    1: string provider (api.body="provider")
    2: optional string base_url (api.body="base_url")
    3: optional string token (api.body="token")
    4: optional bool enabled (api.body="enabled")
    5: optional string webhook_secret (api.body="webhook_secret")
    6: optional string installation_id (api.body="installation_id")
}

struct PutPlatformResp {
    1: GitPlatformInfo platform (api.body="platform")
}

struct DeletePlatformReq {
    1: string provider (api.query="provider")
}

struct DeletePlatformResp {}

struct TestPlatformReq {
    1: string provider (api.body="provider")
    2: optional string base_url (api.body="base_url")
    3: string token (api.body="token")
}

struct GitTestResult {
    1: bool connected (api.body="connected")
    2: string platform (api.body="platform")
    3: optional string user_name (api.body="user_name")
    4: optional string message (api.body="message")
}

struct TestPlatformResp {
    1: GitTestResult result (api.body="result")
}

struct ReviewerAgentInfo {
    1: i64 id (api.body="id")
    2: string name (api.body="name")
    3: string display_name (api.body="display_name")
    4: optional string description (api.body="description")
    5: optional string background (api.body="background")
    6: bool enabled (api.body="enabled")
    7: i64 priority (api.body="priority")
    8: optional string tags (api.body="tags")
}

struct ListReviewerAgentsResp {
    1: list<ReviewerAgentInfo> agents (api.body="agents")
}

struct CreateReviewerAgentReq {
    1: string name (api.body="name")
    2: string display_name (api.body="display_name")
    3: optional string description (api.body="description")
    4: string background (api.body="background")
    5: optional bool enabled (api.body="enabled")
    6: optional i64 priority (api.body="priority")
    7: optional list<string> tags (api.body="tags")
}

struct CreateReviewerAgentResp {
    1: ReviewerAgentInfo agent (api.body="agent")
}

struct UpdateReviewerAgentReq {
    1: string id (api.path="id")
    2: optional string display_name (api.body="display_name")
    3: optional string description (api.body="description")
    4: optional string background (api.body="background")
    5: optional bool enabled (api.body="enabled")
    6: optional i64 priority (api.body="priority")
    7: optional list<string> tags (api.body="tags")
}

struct UpdateReviewerAgentResp {
    1: ReviewerAgentInfo agent (api.body="agent")
}

struct DeleteReviewerAgentReq {
    1: string id (api.path="id")
}

struct DeleteReviewerAgentResp {}
