package requirements

import "strings"

type Evaluator struct{}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

func (e *Evaluator) Evaluate(findings []Finding, requirements []Requirement) *CoverageAssessment {
	coverage := &CoverageAssessment{
		Requirements: make([]RequirementCoverage, len(requirements)),
	}
	for i, req := range requirements {
		rc := RequirementCoverage{
			Requirement: req.Text,
			Status:      "missing",
			Confidence:  0.0,
		}
		related := e.findRelatedFindings(findings, req)
		if len(related) > 0 {
			rc.Status = "partial"
			rc.Confidence = 0.5
			rc.Evidence = "Found related findings"
			rc.Findings = related
		}
		coverage.Requirements[i] = rc
	}
	coverage.Score = e.calculateScore(coverage.Requirements)
	coverage.Summary = e.generateSummary(coverage)
	return coverage
}

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
		case "unclear":
			total += 0.25
		}
	}
	return total / float64(len(coverage))
}

func (e *Evaluator) generateSummary(coverage *CoverageAssessment) string {
	var met, partial, missing, unclear int
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
		if met+partial > 0 {
			summary += ", "
		}
		summary += pluralize(missing, "requirement") + " missing"
	}
	if unclear > 0 {
		if met+partial+missing > 0 {
			summary += ", "
		}
		summary += pluralize(unclear, "requirement") + " unclear"
	}
	return summary
}

func pluralize(count int, word string) string {
	if count == 1 {
		return "1 " + word
	}
	if strings.HasSuffix(word, "y") {
		return strings.TrimSuffix(word, "y") + "ies"
	}
	if strings.HasSuffix(word, "s") || strings.HasSuffix(word, "sh") || strings.HasSuffix(word, "ch") || strings.HasSuffix(word, "x") || strings.HasSuffix(word, "z") {
		return word + "es"
	}
	return word + "s"
}
