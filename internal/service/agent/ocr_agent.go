package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/google/uuid"
)

// ReviewRunner defines the interface for running reviews
type ReviewRunner interface {
	Run(ctx context.Context, workspace ReviewWorkspace) (ReviewResult, error)
}

// ReviewWorkspace represents a review workspace
type ReviewWorkspace struct {
	Dir     string
	BaseSHA string
	HeadSHA string
	Diff    string
}

// ReviewResult represents a review result
type ReviewResult struct {
	Summary  string
	Findings []ReviewFinding
}

// ReviewFinding represents a review finding
type ReviewFinding struct {
	Severity       string
	Path           string
	Line           int
	StartLine      int
	EndLine        int
	Title          string
	Body           string
	ExistingCode   string
	SuggestionCode string
}

// OCRAgent wraps the existing OpenCodeReviewRunner as a ReviewAgent
type OCRAgent struct {
	runner ReviewRunner
	config OCRAgentConfig
}

// OCRAgentConfig contains OCR agent configuration
type OCRAgentConfig struct {
	Name     string
	Priority int
	Focus    []string
	Timeout  time.Duration
}

// DefaultOCRAgentConfig returns default OCR agent configuration
func DefaultOCRAgentConfig() OCRAgentConfig {
	return OCRAgentConfig{
		Name:     "ocr",
		Priority: 10,
		Focus:    []string{"code_quality", "best_practices", "potential_bugs"},
		Timeout:  10 * time.Minute,
	}
}

// NewOCRAgent creates a new OCR agent
func NewOCRAgent(runner ReviewRunner, config OCRAgentConfig) *OCRAgent {
	return &OCRAgent{
		runner: runner,
		config: config,
	}
}

// Name returns the agent name
func (a *OCRAgent) Name() string {
	return a.config.Name
}

// Focus returns the agent's focus areas
func (a *OCRAgent) Focus() []string {
	return a.config.Focus
}

// Priority returns the agent priority
func (a *OCRAgent) Priority() int {
	return a.config.Priority
}

// Review performs the code review using OCR
func (a *OCRAgent) Review(ctx context.Context, workspace Workspace) (AgentResult, error) {
	// Convert agent workspace to review workspace
	reviewWorkspace := ReviewWorkspace{
		Dir:     workspace.Dir,
		BaseSHA: workspace.BaseSHA,
		HeadSHA: workspace.HeadSHA,
		Diff:    workspace.Diff,
	}

	// Run OCR
	startTime := time.Now()
	result, err := a.runner.Run(ctx, reviewWorkspace)
	if err != nil {
		return AgentResult{}, fmt.Errorf("OCR review failed: %w", err)
	}
	duration := time.Since(startTime)

	// Convert review findings to agent findings
	findings := make([]Finding, len(result.Findings))
	for i, f := range result.Findings {
		findings[i] = Finding{
			ID:         uuid.New().String(),
			Severity:   f.Severity,
			Category:   inferCategoryFromReview(f),
			Path:       f.Path,
			Line:       f.Line,
			EndLine:    f.EndLine,
			Title:      f.Title,
			Body:       f.Body,
			Suggestion: f.SuggestionCode,
			Confidence: 0.8, // Default confidence for OCR
			Agents:     []string{a.config.Name},
		}
	}

	return AgentResult{
		AgentName:  a.config.Name,
		Findings:   findings,
		Summary:    result.Summary,
		Confidence: 0.8,
		Duration:   duration,
	}, nil
}

// inferCategoryFromReview infers the category of a finding
func inferCategoryFromReview(f ReviewFinding) string {
	// Simple category inference based on content
	title := f.Title
	body := f.Body

	// Security related
	if types.ContainsAny(title, body, "security", "vulnerability", "injection", "xss", "csrf", "auth") {
		return "security"
	}

	// Performance related
	if types.ContainsAny(title, body, "performance", "slow", "memory", "leak", "optimize") {
		return "performance"
	}

	// Bug related
	if types.ContainsAny(title, body, "bug", "error", "crash", "null", "nil", "undefined") {
		return "bug"
	}

	// Style related
	if types.ContainsAny(title, body, "style", "format", "lint", "convention") {
		return "style"
	}

	// Documentation related
	if types.ContainsAny(title, body, "doc", "comment", "readme", "documentation") {
		return "documentation"
	}

	// Default category
	return "code_quality"
}

// MultiOCRAgent creates multiple OCR agents with different configurations
type MultiOCRAgent struct {
	agents []*OCRAgent
}

// NewMultiOCRAgent creates multiple OCR agents
func NewMultiOCRAgent(runner ReviewRunner, count int) *MultiOCRAgent {
	agents := make([]*OCRAgent, count)
	for i := 0; i < count; i++ {
		config := DefaultOCRAgentConfig()
		config.Name = fmt.Sprintf("ocr-%d", i+1)
		config.Priority = 10 + i
		agents[i] = NewOCRAgent(runner, config)
	}
	return &MultiOCRAgent{agents: agents}
}

// GetAgents returns all OCR agents
func (m *MultiOCRAgent) GetAgents() []ReviewAgent {
	agents := make([]ReviewAgent, len(m.agents))
	for i, agent := range m.agents {
		agents[i] = agent
	}
	return agents
}
