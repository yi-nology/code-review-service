package domain

import (
	"encoding/json"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/scoring"
)

// Score / SeverityCounts / ScoreDeduction 通过类型别名委托给 scoring 包，
// 消费方无需改 import 即可继续使用 domain.Score 等名称。
type (
	Score          = scoring.Result
	ScoreDeduction = scoring.Deduction
	SeverityCounts = scoring.Counts
)

type Result struct {
	Summary  string          `json:"summary"`
	OCR      OCRMetadata     `json:"ocr"`
	Scope    ScopeMetadata   `json:"scope"`
	Warnings []OCRWarning    `json:"warnings,omitempty"`
	Findings []Finding       `json:"findings"`
	RawOCR   json.RawMessage `json:"-"`
	// TechLeadAnalysis 是多 agent 评审中 TechLead 步骤的输出。
	// 存在时优先于 findings 驱动的 Analysis 推导。PR-10 引入。
	TechLeadAnalysis *TechLeadAnalysis `json:"tech_lead_analysis,omitempty"`
}

// TechLeadAnalysis 描述多 agent 评审中 Tech Lead 步骤的总结。
// 字段直接映射到 domain.Analysis 字段。
type TechLeadAnalysis struct {
	Summary         string   `json:"summary,omitempty"`
	Risks           []string `json:"risks,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
}

type ScopeMetadata struct {
	ChangedFiles  int      `json:"changed_files"`
	ReviewedFiles int      `json:"reviewed_files"`
	ExcludedFiles int      `json:"excluded_files"`
	ChangedLines  int      `json:"changed_lines,omitempty"`
	IgnorePaths   []string `json:"ignore_paths,omitempty"`
}

type OCRMetadata struct {
	Engine           string             `json:"engine,omitempty"`
	Model            string             `json:"model,omitempty"`
	Status           string             `json:"status,omitempty"`
	FilesReviewed    int64              `json:"files_reviewed,omitempty"`
	Comments         int64              `json:"comments,omitempty"`
	TotalTokens      int64              `json:"total_tokens,omitempty"`
	InputTokens      int64              `json:"input_tokens,omitempty"`
	OutputTokens     int64              `json:"output_tokens,omitempty"`
	CacheReadTokens  int64              `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int64              `json:"cache_write_tokens,omitempty"`
	Elapsed          string             `json:"elapsed,omitempty"`
	ToolCalls        *ToolCallsMetadata `json:"tool_calls,omitempty"`
	ProjectSummary   string             `json:"project_summary,omitempty"`
	// SessionID 是 OCR v1.7.6+ review session 的标识；可用于 ocr session show 排障，留空表示旧版本或未启用 session。
	SessionID string `json:"session_id,omitempty"`
	// ReusedFiles 是 resume 命中后复用的文件 checkpoint 数量（OCR v1.7.6+）。
	ReusedFiles int `json:"reused_files,omitempty"`
	// BudgetExceeded 表示 scan 时 token 预算已耗尽（OCR v1.9.5+）。
	BudgetExceeded bool `json:"budget_exceeded,omitempty"`
}

type ToolCallsMetadata struct {
	Total  int64            `json:"total"`
	ByTool map[string]int64 `json:"by_tool,omitempty"`
}

type OCRWarning struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type Report struct {
	OCR       OCRMetadata    `json:"ocr"`
	Scope     ScopeMetadata  `json:"scope"`
	Score     Score          `json:"score"`
	Counts    SeverityCounts `json:"counts"`
	Warnings  []OCRWarning   `json:"warnings,omitempty"`
	Complete  bool           `json:"complete"`
	Generated bool           `json:"generated"`
}

type Workspace struct {
	Provider        string
	Repo            string
	BaseSHA         string
	HeadSHA         string
	OriginalHeadSHA string
	// CommitSHA 用于单 commit 评审模式（OCR v1.9.5+ --commit）。
	// 非空时 Runner 使用 `--commit <sha>` 替代 `--from --to`。
	CommitSHA string
	Diff      string
	Dir       string
	Scope     ScopeMetadata
	RulePath  string
	OCR       OCROverrides

	// ScanPaths / ScanExclude 仅供 scan preview 使用（传给 ocr scan --preview --path/--exclude）。
	// 普通 review preview 留空 = 扫描整个仓库。
	ScanPaths   []string
	ScanExclude []string

	// ResumeSessionID 是上一次 OCR review 的 session_id（OCR v1.7.6+），非空时 runner 会传 `--resume <id>`。
	// 仅在 retry 同一 review 时填入：复用已完成文件的 checkpoint，跳过重复 LLM 调用。
	// 注意：OCR session 持久化在 $HOME/.opencodereview/sessions/，跨进程重启后失效；
	// PR 模式下 HeadSHA 可能是合成提交，resume 命中率不确定，调用方应自行评估后开启。
	ResumeSessionID string
}

type OCROverrides struct {
	Enabled         *bool
	Concurrency     int
	TimeoutMinutes  int
	MaxTools        int
	DefaultSeverity string
	Background      string
	// BackgroundFile 是项目级 --background-file 路径覆盖（OCR v1.7.6+）；留空=回退到全局 settings。
	BackgroundFile string
	Audience       string
	Provider       string
	URL            string
	Model          string
	AuthToken      string
	AuthHeader     string
	ExtraHeaders   string
	MaxGitProcs    int
	UseAnthropic   bool
	// Protocol 项目级 LLM 协议覆盖；留空=回退到全局 Protocol 或 UseAnthropic。
	Protocol                string
	ExtraBody               string
	Language                string
	Binary                  string
	ToolsPath               string
	TelemetryEnabled        bool
	TelemetryExporter       string
	TelemetryOTLPEndpoint   string
	TelemetryContentLogging bool
}

type OCRInput struct {
	Binary    string            `json:"binary"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env,omitempty"`
	Workspace WorkspaceInfo     `json:"workspace"`
	RulePath  string            `json:"rule_path,omitempty"`
	StartedAt time.Time         `json:"started_at"`
}

type WorkspaceInfo struct {
	Provider string `json:"provider"`
	Repo     string `json:"repo"`
	BaseSHA  string `json:"base_sha"`
	HeadSHA  string `json:"head_sha"`
	Dir      string `json:"dir,omitempty"`
}

type PreviewResult struct {
	Files []string `json:"files"`
}

type ScanRequest struct {
	Provider        string   `json:"provider"`
	Repo            string   `json:"repo"`
	ProjectID       string   `json:"project_id,omitempty"`
	HeadSHA         string   `json:"head_sha"`
	Paths           []string `json:"paths,omitempty"`
	Exclude         []string `json:"exclude,omitempty"`
	Background      string   `json:"background,omitempty"`
	MaxTokensBudget int      `json:"max_tokens_budget,omitempty"`
	Batch           string   `json:"batch,omitempty"`
	NoPlan          bool     `json:"no_plan,omitempty"`
	NoDedup         bool     `json:"no_dedup,omitempty"`
	NoSummary       bool     `json:"no_summary,omitempty"`
	Preview         bool     `json:"preview,omitempty"`
	TimeoutMinutes  int      `json:"timeout_minutes,omitempty"`
	// ResumeSessionID 是上一次 OCR scan 的 session_id（OCR v1.11.0+），非空时 scanner 会传 `--resume <id>`。
	ResumeSessionID string `json:"resume_session_id,omitempty"`
}

type ScanResult struct {
	ReviewID     string       `json:"review_id,omitempty"`
	PreviewFiles []string     `json:"preview_files,omitempty"`
	Result       *Result      `json:"result,omitempty"`
	Report       *Report      `json:"report,omitempty"`
	Analysis     *Analysis    `json:"analysis,omitempty"`
	Warnings     []OCRWarning `json:"warnings,omitempty"`
}

type LLMTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type LLMProvider struct {
	Name         string   `json:"name"`
	Protocol     string   `json:"protocol"`
	BaseURL      string   `json:"base_url,omitempty"`
	DefaultModel []string `json:"default_model,omitempty"`
}

type RulesCheckResult struct {
	Applicable bool   `json:"applicable"`
	Rule       string `json:"rule,omitempty"`
	Message    string `json:"message,omitempty"`
}

type OCRVersion struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	Platform  string `json:"platform,omitempty"`
	BuildTime string `json:"build_time,omitempty"`
}

type ViewerStatus struct {
	Running bool   `json:"running"`
	Addr    string `json:"addr,omitempty"`
	Message string `json:"message,omitempty"`
}

// UpgradeState 表示 ocr 二进制升级流程的状态机。
//
// 状态流转：idle → checking → downloading → verifying → installing → done / failed。
type UpgradeState string

const (
	UpgradeIdle        UpgradeState = "idle"
	UpgradeChecking    UpgradeState = "checking"
	UpgradeDownloading UpgradeState = "downloading"
	UpgradeVerifying   UpgradeState = "verifying"
	UpgradeInstalling  UpgradeState = "installing"
	UpgradeDone        UpgradeState = "done"
	UpgradeFailed      UpgradeState = "failed"
)

// OCRUpdateInfo 是检查更新接口的返回，对比当前版本与 GitHub 最新 Release。
type OCRUpdateInfo struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	ReleaseURL      string `json:"release_url,omitempty"`
}

// OCRUpgradeStatus 是升级状态轮询接口的返回。
type OCRUpgradeStatus struct {
	State    UpgradeState `json:"state"`
	Message  string       `json:"message,omitempty"`
	Version  string       `json:"version,omitempty"`
	Progress int          `json:"progress,omitempty"`
}
