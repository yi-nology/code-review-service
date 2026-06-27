package domain

import (
	"encoding/json"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/scoring"
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
	Diff            string
	Dir             string
	Scope           ScopeMetadata
	RulePath        string
	OCR             OCROverrides
}

type OCROverrides struct {
	Enabled                 *bool
	Concurrency             int
	TimeoutMinutes          int
	MaxTools                int
	DefaultSeverity         string
	Background              string
	Audience                string
	Provider                string
	URL                     string
	Model                   string
	MaxGitProcs             int
	UseAnthropic            bool
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
