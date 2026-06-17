package requirements

import (
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

type Requirements struct {
	ID        string            `json:"id"`
	Inline    []string          `json:"inline"`
	Files     []string          `json:"files"`
	URLs      []string          `json:"urls"`
	Context   map[string]string `json:"context"`
	CreatedAt time.Time         `json:"created_at"`
}

type Requirement struct {
	ID         string   `json:"id"`
	Text       string   `json:"text"`
	Category   string   `json:"category"`
	Priority   string   `json:"priority"`
	Acceptance []string `json:"acceptance"`
	Source     string   `json:"source"`
}

type CoverageAssessment struct {
	Requirements []RequirementCoverage `json:"requirements"`
	Summary      string                `json:"summary"`
	Score        float64               `json:"score"`
}

type RequirementCoverage struct {
	Requirement string   `json:"requirement"`
	Status      string   `json:"status"`
	Confidence  float64  `json:"confidence"`
	Evidence    string   `json:"evidence"`
	Findings    []string `json:"findings"`
}

type Finding = types.Finding
