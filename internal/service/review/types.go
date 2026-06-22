package review

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/stringx"
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
	// Requirements 是需求/spec/验收标准，透传给 workflow 做需求感知评审（阶段 4）。
	Requirements []string `json:"requirements,omitempty"`
	// EphemeralReviewers 是本次评审临时加入的 reviewer 描述（阶段 5），不入库。
	EphemeralReviewers []EphemeralReviewer `json:"ephemeral_reviewers,omitempty"`
}

// EphemeralReviewer 描述一个临时的、不入库的 reviewer（阶段 5）。
// 在 executor 层映射为 agent.EphemeralSpec。
type EphemeralReviewer struct {
	Name       string `json:"name,omitempty"`
	Background string `json:"background"`
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
	return Gate{Status: status, Passed: ptrx.Of(passed), Reason: reason}
}

func ErrorGate(reason string) Gate {
	return Gate{Status: GateError, Passed: ptrx.Of(false), Reason: reason}
}

type Record struct {
	ID                 string          `json:"id"`
	IdempotencyKey     string          `json:"idempotency_key,omitempty"`
	RequestFingerprint string          `json:"request_fingerprint,omitempty"`
	Request            Request         `json:"request"`
	Response           Response        `json:"response"`
	RawOCR             json.RawMessage `json:"raw_ocr,omitempty"`
	RawOCRInput        json.RawMessage `json:"raw_ocr_input,omitempty"`
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
	case stringx.ContainsAny(lower, "critical") || stringx.ContainsAny(content, "严重问题", "致命"):
		return SeverityCritical
	case stringx.ContainsAny(lower, "minor") || stringx.ContainsAny(content, "轻微问题"):
		return SeverityMinor
	case stringx.ContainsAny(lower, "info") || stringx.ContainsAny(content, "提示"):
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
	Thinking       string   `json:"thinking,omitempty"`
}
