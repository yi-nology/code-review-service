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
    3: optional string url (api.body="url")
    4: optional string auth_token (api.body="auth_token")
    5: optional string auth_header (api.body="auth_header")
    6: optional string model (api.body="model")
    7: optional bool use_anthropic (api.body="use_anthropic")
    8: optional string extra_body (api.body="extra_body")
    9: optional string language (api.body="language")
    10: optional i64 concurrency (api.body="concurrency")
    11: optional i64 max_git_procs (api.body="max_git_procs")
    12: optional i64 timeout_minutes (api.body="timeout_minutes")
    13: optional i64 max_tools (api.body="max_tools")
    14: optional string background (api.body="background")
    15: optional string binary (api.body="binary")
    16: optional string audience (api.body="audience")
    17: optional string default_severity (api.body="default_severity")
    18: optional string tools_path (api.body="tools_path")
    19: optional bool telemetry_enabled (api.body="telemetry_enabled")
    20: optional string telemetry_exporter (api.body="telemetry_exporter")
    21: optional string telemetry_otlp_endpoint (api.body="telemetry_otlp_endpoint")
    22: optional bool telemetry_content_logging (api.body="telemetry_content_logging")
    23: optional string reasoning_mode (api.body="reasoning_mode")
    24: optional i64 reasoning_budget (api.body="reasoning_budget")
    25: optional i64 context_window (api.body="context_window")
    26: optional i64 max_output_tokens (api.body="max_output_tokens")
    27: optional string extra_headers (api.body="extra_headers")
    28: optional string tools_config (api.body="tools_config")
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

struct LLMProvider {
    1: string name (api.body="name")
    2: string protocol (api.body="protocol")
    3: optional string base_url (api.body="base_url")
    4: optional list<string> default_model (api.body="default_model")
}

struct ListLLMProvidersResp {
    1: list<LLMProvider> built_in (api.body="built_in")
}

struct CustomProvider {
    1: i64 id (api.body="id")
    2: string name (api.body="name")
    3: string protocol (api.body="protocol")
    4: optional string url (api.body="url")
    5: optional string api_key (api.body="api_key")
    6: optional string model (api.body="model")
    7: optional string auth_header (api.body="auth_header")
    8: optional string extra_body (api.body="extra_body")
    9: bool enabled (api.body="enabled")
}

struct ListCustomProvidersResp {
    1: list<CustomProvider> items (api.body="items")
}

struct CreateCustomProviderReq {
    1: string name (api.body="name")
    2: string protocol (api.body="protocol")
    3: optional string url (api.body="url")
    4: optional string api_key (api.body="api_key")
    5: optional string model (api.body="model")
    6: optional string auth_header (api.body="auth_header")
    7: optional string extra_body (api.body="extra_body")
    8: optional bool enabled (api.body="enabled")
}

struct CreateCustomProviderResp {
    1: CustomProvider provider (api.body="provider")
}

struct UpdateCustomProviderReq {
    1: i64 id (api.path="id")
    2: optional string name (api.body="name")
    3: optional string protocol (api.body="protocol")
    4: optional string url (api.body="url")
    5: optional string api_key (api.body="api_key")
    6: optional string model (api.body="model")
    7: optional string auth_header (api.body="auth_header")
    8: optional string extra_body (api.body="extra_body")
    9: optional bool enabled (api.body="enabled")
}

struct UpdateCustomProviderResp {
    1: CustomProvider provider (api.body="provider")
}

struct DeleteCustomProviderReq {
    1: i64 id (api.path="id")
}

struct DeleteCustomProviderResp {}

struct RulesCheckReq {
    1: string path (api.body="path")
}

struct RulesCheckResult {
    1: bool applicable (api.body="applicable")
    2: optional string rule (api.body="rule")
    3: optional string message (api.body="message")
}

struct RulesCheckResp {
    1: RulesCheckResult result (api.body="result")
}

struct OCRVersionResp {
    1: string version (api.body="version")
    2: optional string commit (api.body="commit")
    3: optional string platform (api.body="platform")
    4: optional string build_time (api.body="build_time")
}

struct OCRUpdateInfoResp {
    1: string current_version (api.body="current_version")
    2: string latest_version (api.body="latest_version")
    3: bool update_available (api.body="update_available")
    4: optional string release_notes (api.body="release_notes")
    5: optional string release_url (api.body="release_url")
}

struct StartOCRUpgradeReq {
    1: optional string version (api.body="version")
}

struct OCRUpgradeStatusResp {
    1: string state (api.body="state")
    2: optional string message (api.body="message")
    3: optional string version (api.body="version")
    4: optional i32 progress (api.body="progress")
}

struct ViewerStatusResp {
    1: bool running (api.body="running")
    2: optional string addr (api.body="addr")
    3: optional string message (api.body="message")
}

struct StartViewerReq {
    1: optional string addr (api.body="addr")
}

struct StartViewerResp {
    1: bool running (api.body="running")
    2: optional string addr (api.body="addr")
    3: optional string message (api.body="message")
}

struct ProjectPolicy {
    1: string provider (api.body="provider")
    2: string repo (api.body="repo")
    3: string fail_on (api.body="fail_on")
    4: optional string ignore_paths (api.body="ignore_paths")
    5: optional string rules (api.body="rules")
    6: optional bool ocr_enabled (api.body="ocr_enabled")
    7: optional i64 ocr_concurrency (api.body="ocr_concurrency")
    8: optional i64 ocr_timeout_minutes (api.body="ocr_timeout_minutes")
    9: optional i64 ocr_max_tools (api.body="ocr_max_tools")
    10: optional string ocr_default_severity (api.body="ocr_default_severity")
    11: optional string ocr_background (api.body="ocr_background")
    12: optional string ocr_audience (api.body="ocr_audience")
    13: optional string ocr_provider (api.body="ocr_provider")
    14: optional string ocr_url (api.body="ocr_url")
    15: optional string ocr_model (api.body="ocr_model")
    16: optional i64 ocr_max_git_procs (api.body="ocr_max_git_procs")
    17: optional bool ocr_use_anthropic (api.body="ocr_use_anthropic")
    18: optional string ocr_extra_body (api.body="ocr_extra_body")
    19: optional string ocr_language (api.body="ocr_language")
    20: optional string ocr_binary (api.body="ocr_binary")
    21: optional string ocr_tools_path (api.body="ocr_tools_path")
    22: optional bool ocr_telemetry_enabled (api.body="ocr_telemetry_enabled")
    23: optional string ocr_telemetry_exporter (api.body="ocr_telemetry_exporter")
    24: optional string ocr_telemetry_otlp_endpoint (api.body="ocr_telemetry_otlp_endpoint")
    25: optional bool ocr_telemetry_content_logging (api.body="ocr_telemetry_content_logging")
    26: optional list<i64> rule_package_ids (api.body="rule_package_ids")
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
    7: optional bool skip_tls (api.body="skip_tls")
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
    7: optional bool skip_tls (api.body="skip_tls")
}

struct PutPlatformResp {
    1: GitPlatformInfo platform (api.body="platform")
}

struct DeletePlatformReq {
    1: string provider (api.query="provider")
    2: optional string installation_id (api.query="installation_id")
}

struct DeletePlatformResp {}

struct TestPlatformReq {
    1: string provider (api.body="provider")
    2: optional string base_url (api.body="base_url")
    3: string token (api.body="token")
    4: optional bool skip_tls (api.body="skip_tls")
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
    9: optional string rule_file (api.body="rule_file")
    10: i64 redundancy (api.body="redundancy")
    11: bool is_default (api.body="is_default")
    12: optional list<i64> rule_package_ids (api.body="rule_package_ids")
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
    8: optional string rule_file (api.body="rule_file")
    9: optional i64 redundancy (api.body="redundancy")
    10: optional bool is_default (api.body="is_default")
    11: optional list<i64> rule_package_ids (api.body="rule_package_ids")
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
    8: optional string rule_file (api.body="rule_file")
    9: optional i64 redundancy (api.body="redundancy")
    10: optional bool is_default (api.body="is_default")
    11: optional list<i64> rule_package_ids (api.body="rule_package_ids")
}

struct UpdateReviewerAgentResp {
    1: ReviewerAgentInfo agent (api.body="agent")
}

struct DeleteReviewerAgentReq {
    1: string id (api.path="id")
}

struct DeleteReviewerAgentResp {}

struct RulePackage {
    1: optional i64 id (api.body="id")
    2: string name (api.body="name")
    3: optional string description (api.body="description")
    4: optional list<string> languages (api.body="languages")
    5: optional list<string> tags (api.body="tags")
    6: optional list<string> includes (api.body="includes")
    7: optional list<string> excludes (api.body="excludes")
    8: list<map<string,string>> rules (api.body="rules")
    9: bool enabled (api.body="enabled")
    10: optional list<string> project_refs (api.body="project_refs")
    11: optional bool built_in (api.body="built_in")
    12: optional string scope (api.body="scope")
}

struct ListRulePackagesReq {}

struct ListRulePackagesResp {
    1: list<RulePackage> items (api.body="items")
}

struct UpsertRulePackageReq {
    1: optional i64 id (api.path="id")
    2: RulePackage package (api.body="package")
}

struct UpsertRulePackageResp {
    1: RulePackage item (api.body="item")
}

struct DeleteRulePackageReq {
    1: i64 id (api.path="id")
}

struct DeleteRulePackageResp {}

struct ListToolPackagesReq {}

struct ListToolPackagesResp {}

struct UpsertToolPackageReq {}

struct UpsertToolPackageResp {}

struct UpdateToolPackageReq {
    1: optional i64 id (api.path="id")
}

struct UpdateToolPackageResp {}

struct DeleteToolPackageReq {
    1: i64 id (api.path="id")
}

struct DeleteToolPackageResp {}

struct ValidateToolPackageReq {}

struct ValidateToolPackageResp {}
