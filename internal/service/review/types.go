package review

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type Request struct {
	Provider        string `json:"provider"`
	Repo            string `json:"repo"`
	ProjectID       string `json:"project_id,omitempty"`
	PullRequestID   string `json:"pull_request_id,omitempty"`
	MergeRequestIID string `json:"merge_request_iid,omitempty"`
	BaseSHA         string `json:"base_sha"`
	HeadSHA         string `json:"head_sha"`
	CallbackMode    string `json:"callback_mode,omitempty"`
	Wait            bool   `json:"wait"`
	Author          string `json:"author,omitempty"`
	AuthorName      string `json:"author_name,omitempty"`
	AuthorAvatar    string `json:"author_avatar,omitempty"`
	Sender          string `json:"sender,omitempty"`
}

type Response struct {
	ReviewID string    `json:"review_id"`
	Status   string    `json:"status"`
	Summary  string    `json:"summary"`
	Gate     Gate      `json:"gate"`
	Report   Report    `json:"report"`
	Analysis Analysis  `json:"analysis"`
	History  []Event   `json:"history,omitempty"`
	Findings []Finding `json:"findings,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type Event struct {
	Stage     string         `json:"stage"`
	Status    string         `json:"status"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Analysis struct {
	Overview        string   `json:"overview,omitempty"`
	RootCause       []string `json:"root_cause,omitempty"`
	Risks           []string `json:"risks,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
}

type Gate struct {
	Status string `json:"status"`
	Passed *bool  `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

const (
	GatePending = "pending"
	GatePassed  = "passed"
	GateFailed  = "failed"
	GateError   = "error"
)

func PendingGate() Gate {
	return Gate{Status: GatePending}
}

func CompletedGate(passed bool, reason string) Gate {
	status := GateFailed
	if passed {
		status = GatePassed
	}
	return Gate{Status: status, Passed: Ptr(passed), Reason: reason}
}

func ErrorGate(reason string) Gate {
	return Gate{Status: GateError, Passed: Ptr(false), Reason: reason}
}


type Record struct {
	ID                 string          `json:"id"`
	IdempotencyKey     string          `json:"idempotency_key,omitempty"`
	RequestFingerprint string          `json:"request_fingerprint,omitempty"`
	Request            Request         `json:"request"`
	Response           Response        `json:"response"`
	RawOCR             json.RawMessage `json:"raw_ocr,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityMinor    Severity = "minor"
	SeverityMajor    Severity = "major"
	SeverityCritical Severity = "critical"
)

func NormalizeSeverity(value string) Severity {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(SeverityInfo):
		return SeverityInfo
	case string(SeverityMinor):
		return SeverityMinor
	case string(SeverityCritical):
		return SeverityCritical
	default:
		return SeverityMajor
	}
}

func InferSeverity(content, fallback string) Severity {
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "critical"),
		strings.Contains(content, "严重问题"),
		strings.Contains(content, "致命"):
		return SeverityCritical
	case strings.Contains(lower, "minor"),
		strings.Contains(content, "轻微问题"):
		return SeverityMinor
	case strings.Contains(lower, "info"),
		strings.Contains(content, "提示"):
		return SeverityInfo
	default:
		return NormalizeSeverity(fallback)
	}
}

type Finding struct {
	Severity       Severity `json:"severity"`
	Category       string   `json:"category,omitempty"`
	Path           string   `json:"path,omitempty"`
	Line           int      `json:"line,omitempty"`
	StartLine      int      `json:"start_line,omitempty"`
	EndLine        int      `json:"end_line,omitempty"`
	Title          string   `json:"title"`
	Body           string   `json:"body"`
	ExistingCode   string   `json:"existing_code,omitempty"`
	SuggestionCode string   `json:"suggestion_code,omitempty"`
}

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
	Value      int              `json:"value"`
	Grade      string           `json:"grade"`
	Version    string           `json:"version"`
	Deductions []ScoreDeduction `json:"deductions"`
	ChangedLines      int     `json:"changed_lines,omitempty"`
	FindingsDensity   float64 `json:"findings_density,omitempty"`
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

type ListOptions struct {
	Status   string
	Provider string
	Repo     string
	Limit    int
	Offset   int
}

type ListItem struct {
	ReviewID        string    `json:"review_id"`
	Provider        string    `json:"provider"`
	Repo            string    `json:"repo"`
	Status          string    `json:"status"`
	Summary         string    `json:"summary"`
	Gate            Gate      `json:"gate"`
	Findings        int       `json:"findings"`
	Score           int       `json:"score"`
	Grade           string    `json:"grade,omitempty"`
	ReportGenerated bool      `json:"report_generated"`
	Author          string    `json:"author,omitempty"`
	AuthorName      string    `json:"author_name,omitempty"`
	AuthorAvatar    string    `json:"author_avatar,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ListStats struct {
	Total      int `json:"total"`
	Running    int `json:"running"`
	GateFailed int `json:"gate_failed"`
	Findings   int `json:"findings"`
}

type ListResult struct {
	Items      []ListItem `json:"items"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
	Stats      ListStats  `json:"stats"`
}

type Detail struct {
	ReviewID       string          `json:"review_id"`
	Provider       string          `json:"provider"`
	Repo           string          `json:"repo"`
	ProjectID      string          `json:"project_id,omitempty"`
	PullRequestID  string          `json:"pull_request_id,omitempty"`
	MergeRequestID string          `json:"merge_request_iid,omitempty"`
	BaseSHA        string          `json:"base_sha"`
	HeadSHA        string          `json:"head_sha"`
	CallbackMode   string          `json:"callback_mode,omitempty"`
	Status         string          `json:"status"`
	Summary        string          `json:"summary"`
	Gate           Gate            `json:"gate"`
	Analysis       Analysis        `json:"analysis"`
	History        []Event         `json:"history"`
	Findings       []Finding       `json:"findings"`
	Report         Report          `json:"report"`
	Author         string          `json:"author,omitempty"`
	AuthorName     string          `json:"author_name,omitempty"`
	AuthorAvatar   string          `json:"author_avatar,omitempty"`
	RawOCR         json.RawMessage `json:"raw_ocr,omitempty"`
	Error          string          `json:"error,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Workspace struct {
	Provider string
	Repo     string
	BaseSHA  string
	HeadSHA  string
	Diff     string
	Dir      string
	Scope    ScopeMetadata
	RulePath string
}

type Providers interface {
	Get(name string) (Provider, bool)
}

type Provider interface {
	Prepare(context.Context, Request) (Workspace, error)
}

type GitProvider interface {
	GetCRDiff(ctx context.Context, platform, owner, repo string, number int) (string, error)
	CreateComment(ctx context.Context, platform, owner, repo string, number int, body string) (string, error)
	HasProvider(platform string) bool
}

type Runner interface {
	Run(context.Context, Workspace) (Result, error)
}

type Publisher interface {
	Publish(context.Context, Request, Result) error
}

type WritebackPublisher interface {
	PublishWithDiff(ctx context.Context, req Request, result Result, diff string) error
}

type PreviewRunner interface {
	Preview(ctx context.Context, workspace Workspace) (*PreviewResult, error)
}

type LLMTester interface {
	TestLLM(ctx context.Context) (*LLMTestResult, error)
}

type PreviewResult struct {
	Files []string `json:"files"`
}

type LLMTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type WebhookReviewCreator interface {
	CreateFromWebhook(ctx context.Context, req Request, idempotencyKey string) (Response, error)
}
