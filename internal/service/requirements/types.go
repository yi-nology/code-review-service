package requirements

import (
	"bufio"
	"os"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

// Requirements represents review requirements
type Requirements struct {
	ID        string            `json:"id"`
	Inline    []string          `json:"inline"`
	Files     []string          `json:"files"`
	URLs      []string          `json:"urls"`
	Context   map[string]string `json:"context"`
	CreatedAt time.Time         `json:"created_at"`
}

// Requirement represents a single requirement
type Requirement struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	Category    string   `json:"category"`
	Priority    string   `json:"priority"`
	Acceptance  []string `json:"acceptance"`
	Source      string   `json:"source"`
}

// CoverageAssessment represents the coverage assessment of requirements
type CoverageAssessment struct {
	Requirements []RequirementCoverage `json:"requirements"`
	Summary      string                `json:"summary"`
	Score        float64               `json:"score"`
}

// RequirementCoverage represents the coverage of a single requirement
type RequirementCoverage struct {
	Requirement string  `json:"requirement"`
	Status      string  `json:"status"` // met, partial, missing, unclear
	Confidence  float64 `json:"confidence"`
	Evidence    string  `json:"evidence"`
	Findings    []string `json:"findings"`
}

// Parser parses requirements from various sources
type Parser struct{}

// NewParser creates a new requirements parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse parses requirements from inline text
func (p *Parser) Parse(inline []string) (*Requirements, error) {
	requirements := &Requirements{
		ID:        types.GenerateID(),
		Inline:    inline,
		Context:   make(map[string]string),
		CreatedAt: time.Now(),
	}

	return requirements, nil
}

// ParseFromFile parses requirements from a file
func (p *Parser) ParseFromFile(filePath string) (*Requirements, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return p.Parse(lines)
}

// ParseRequirements parses individual requirements from text
func (p *Parser) ParseRequirements(text string) []Requirement {
	var requirements []Requirement

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if line is a requirement (starts with - or *)
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			req := Requirement{
				ID:       types.GenerateID(),
				Text:     strings.TrimLeft(line, "-* "),
				Category: inferCategory(line),
				Priority: inferPriority(line),
				Source:   "inline",
			}
			requirements = append(requirements, req)
		}
	}

	return requirements
}

// inferCategory infers the category of a requirement
func inferCategory(text string) string {
	text = strings.ToLower(text)

	if types.ContainsAny(text, "security", "auth", "permission", "access") {
		return "security"
	}
	if types.ContainsAny(text, "performance", "speed", "fast", "optimize") {
		return "performance"
	}
	if types.ContainsAny(text, "test", "coverage", "quality") {
		return "testing"
	}
	if types.ContainsAny(text, "document", "readme", "comment") {
		return "documentation"
	}
	if types.ContainsAny(text, "ui", "interface", "user", "frontend") {
		return "ui"
	}
	if types.ContainsAny(text, "api", "endpoint", "request", "response") {
		return "api"
	}

	return "functional"
}

// inferPriority infers the priority of a requirement
func inferPriority(text string) string {
	text = strings.ToLower(text)

	if types.ContainsAny(text, "must", "critical", "important", "required") {
		return "high"
	}
	if types.ContainsAny(text, "should", "recommended") {
		return "medium"
	}
	if types.ContainsAny(text, "could", "optional", "nice") {
		return "low"
	}

	return "medium"
}

// Evaluator evaluates findings against requirements
type Evaluator struct{}

// NewEvaluator creates a new requirements evaluator
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate evaluates findings against requirements
func (e *Evaluator) Evaluate(findings []Finding, requirements []Requirement) *CoverageAssessment {
	coverage := &CoverageAssessment{
		Requirements: make([]RequirementCoverage, len(requirements)),
	}

	for i, req := range requirements {
		reqCoverage := RequirementCoverage{
			Requirement: req.Text,
			Status:      "missing",
			Confidence:  0.0,
		}

		// Find related findings
		relatedFindings := e.findRelatedFindings(findings, req)
		if len(relatedFindings) > 0 {
			reqCoverage.Status = "partial"
			reqCoverage.Confidence = 0.5
			reqCoverage.Evidence = "Found related findings"
			reqCoverage.Findings = relatedFindings
		}

		coverage.Requirements[i] = reqCoverage
	}

	// Calculate overall score
	coverage.Score = e.calculateScore(coverage.Requirements)
	coverage.Summary = e.generateSummary(coverage)

	return coverage
}

// findRelatedFindings finds findings related to a requirement
func (e *Evaluator) findRelatedFindings(findings []Finding, req Requirement) []string {
	var related []string

	reqWords := strings.Fields(strings.ToLower(req.Text))
	for _, finding := range findings {
		findingText := strings.ToLower(finding.Title + " " + finding.Body)
		for _, word := range reqWords {
			if len(word) > 3 && strings.Contains(findingText, word) {
				related = append(related, finding.ID)
				break
			}
		}
	}

	return related
}

// calculateScore calculates the overall coverage score
func (e *Evaluator) calculateScore(coverage []RequirementCoverage) float64 {
	if len(coverage) == 0 {
		return 0.0
	}

	total := 0.0
	for _, c := range coverage {
		switch c.Status {
		case "met":
			total += 1.0
		case "partial":
			total += 0.5
		case "missing":
			total += 0.0
		case "unclear":
			total += 0.25
		}
	}

	return total / float64(len(coverage))
}

// generateSummary generates a summary of the coverage assessment
func (e *Evaluator) generateSummary(coverage *CoverageAssessment) string {
	met := 0
	partial := 0
	missing := 0
	unclear := 0

	for _, c := range coverage.Requirements {
		switch c.Status {
		case "met":
			met++
		case "partial":
			partial++
		case "missing":
			missing++
		case "unclear":
			unclear++
		}
	}

	total := len(coverage.Requirements)
	if total == 0 {
		return "No requirements to evaluate"
	}

	summary := "Requirements coverage: "
	if met > 0 {
		summary += pluralize(met, "requirement") + " met"
	}
	if partial > 0 {
		if met > 0 {
			summary += ", "
		}
		summary += pluralize(partial, "requirement") + " partially met"
	}
	if missing > 0 {
		if met > 0 || partial > 0 {
			summary += ", "
		}
		summary += pluralize(missing, "requirement") + " missing"
	}
	if unclear > 0 {
		if met > 0 || partial > 0 || missing > 0 {
			summary += ", "
		}
		summary += pluralize(unclear, "requirement") + " unclear"
	}

	return summary
}

// pluralize returns plural form if count > 1
func pluralize(count int, word string) string {
	if count == 1 {
		return "1 " + word
	}
	return pluralizeWord(count, word)
}

// pluralizeWord returns the plural form of a word
func pluralizeWord(count int, word string) string {
	// Simple pluralization rules
	if strings.HasSuffix(word, "y") {
		return strings.TrimSuffix(word, "y") + "ies"
	}
	if strings.HasSuffix(word, "s") || strings.HasSuffix(word, "sh") || strings.HasSuffix(word, "ch") || strings.HasSuffix(word, "x") || strings.HasSuffix(word, "z") {
		return word + "es"
	}
	return word + "s"
}

// Finding is an alias for the canonical types.Finding
type Finding = types.Finding
