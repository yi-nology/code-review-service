package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

// reviewRunner defines the interface for running reviews
type reviewRunner interface {
	Run(ctx context.Context, workspace reviewWorkspace) (reviewResult, error)
}

// reviewWorkspace represents a review workspace
type reviewWorkspace struct {
	Dir      string
	BaseSHA  string
	HeadSHA  string
	Diff     string
	RulePath string
}

// reviewResult represents a review result.
// Findings 直接复用 types.Finding（= domain.Finding），不再维护独立的 reviewFinding 适配类型。
type reviewResult struct {
	Summary     string
	Findings    []Finding
	TotalTokens int64
}

// ocrAgent wraps the existing OpenCodeReviewRunner as a reviewAgent
type ocrAgent struct {
	runner reviewRunner
	config ocrAgentConfig
}

// ocrAgentConfig contains OCR agent configuration
type ocrAgentConfig struct {
	Name      string
	Persona   string
	Priority  int
	Focus     []string
	Timeout   time.Duration
	IsDefault bool
}

// newOCRAgent creates a new OCR agent
func newOCRAgent(runner reviewRunner, config ocrAgentConfig) *ocrAgent {
	return &ocrAgent{
		runner: runner,
		config: config,
	}
}

// Name returns the agent name
func (a *ocrAgent) Name() string {
	return a.config.Name
}

// Persona 返回 agent 的 persona 基名（冗余展开时多个实例共享同一 persona，路径 A4）
func (a *ocrAgent) Persona() string {
	if a.config.Persona != "" {
		return a.config.Persona
	}
	return a.config.Name
}

// Focus returns the agent's focus areas
func (a *ocrAgent) Focus() []string {
	return a.config.Focus
}

// Priority returns the agent priority
func (a *ocrAgent) Priority() int {
	return a.config.Priority
}

// IsDefault 返回该审查员是否属于缺省集（用于技术栈推断无匹配时的兜底）
func (a *ocrAgent) IsDefault() bool {
	return a.config.IsDefault
}

// Review performs the code review using OCR
func (a *ocrAgent) Review(ctx context.Context, workspace Workspace) (AgentResult, error) {
	reviewWs := reviewWorkspace{
		Dir:      workspace.Dir,
		BaseSHA:  workspace.BaseSHA,
		HeadSHA:  workspace.HeadSHA,
		Diff:     workspace.Diff,
		RulePath: workspace.RulePath,
	}

	startTime := time.Now()
	result, err := a.runner.Run(ctx, reviewWs)
	if err != nil {
		return AgentResult{}, fmt.Errorf("OCR review failed: %w", err)
	}
	duration := time.Since(startTime)

	// 富化 runner 产出的 findings：补充 ID/分类/置信度/归属 agent，
	// 同时保留原始的 ExistingCode/SuggestionCode/StartLine 等字段（无字段丢失）。
	findings := make([]Finding, len(result.Findings))
	for i, f := range result.Findings {
		f.ID = types.GenerateID()
		f.Category = inferCategoryFromReview(f)
		if f.Confidence == 0 {
			f.Confidence = 0.8
		}
		f.Agents = []string{a.config.Name}
		if f.Suggestion == "" {
			f.Suggestion = f.SuggestionCode
		}
		findings[i] = f
	}

	return AgentResult{
		AgentName:  a.config.Name,
		Persona:    a.Persona(),
		Findings:   findings,
		Summary:    result.Summary,
		Confidence: 0.8,
		Duration:   duration,
		TokensUsed: result.TotalTokens,
	}, nil
}

func inferCategoryFromReview(f Finding) string {
	title := f.Title
	body := f.Body
	if types.ContainsAny(title, body, "security", "vulnerability", "injection", "xss", "csrf", "auth") {
		return "security"
	}
	if types.ContainsAny(title, body, "performance", "slow", "memory", "leak", "optimize") {
		return "performance"
	}
	if types.ContainsAny(title, body, "bug", "error", "crash", "null", "nil", "undefined") {
		return "bug"
	}
	if types.ContainsAny(title, body, "style", "format", "lint", "convention") {
		return "style"
	}
	if types.ContainsAny(title, body, "doc", "comment", "readme", "documentation") {
		return "documentation"
	}
	return "code_quality"
}
