package review

import (
	"encoding/json"
	"time"
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
	Engine           string `json:"engine,omitempty"`
	Status           string `json:"status,omitempty"`
	FilesReviewed    int64  `json:"files_reviewed,omitempty"`
	Comments         int64  `json:"comments,omitempty"`
	TotalTokens      int64  `json:"total_tokens,omitempty"`
	InputTokens      int64  `json:"input_tokens,omitempty"`
	OutputTokens     int64  `json:"output_tokens,omitempty"`
	CacheReadTokens  int64  `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int64  `json:"cache_write_tokens,omitempty"`
	Elapsed          string `json:"elapsed,omitempty"`
}

type OCRWarning struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

type ScoreDeduction struct {
	Reason string `json:"reason"`
	Points int    `json:"points"`
	Count  int    `json:"count,omitempty"`
}

type Score struct {
	Value           int              `json:"value"`
	Grade           string           `json:"grade"`
	Version         string           `json:"version"`
	Deductions      []ScoreDeduction `json:"deductions"`
	ChangedLines    int              `json:"changed_lines,omitempty"`
	FindingsDensity float64          `json:"findings_density,omitempty"`
}

type SeverityCounts struct {
	Critical int `json:"critical"`
	Major    int `json:"major"`
	Minor    int `json:"minor"`
	Info     int `json:"info"`
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
