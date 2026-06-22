package agent

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/oekit/pkg/slicex"
)

// DefaultAggregator implements ResultAggregator with deduplication and confidence scoring
type DefaultAggregator struct {
	config AggregatorConfig
}

// AggregatorConfig contains aggregator configuration
type AggregatorConfig struct {
	// ConfidenceWeights defines weights for confidence calculation
	ConfidenceWeights ConfidenceWeights
}

// ConfidenceWeights defines weights for confidence calculation
type ConfidenceWeights struct {
	// AgentCount weight for number of agents that found the issue
	AgentCount float64

	// Severity weight for severity level
	Severity float64

	// Category weight for category consistency
	Category float64
}

// DefaultAggregatorConfig returns default aggregator configuration
func DefaultAggregatorConfig() AggregatorConfig {
	return AggregatorConfig{
		ConfidenceWeights: ConfidenceWeights{
			AgentCount: 0.5,
			Severity:   0.3,
			Category:   0.2,
		},
	}
}

// NewDefaultAggregator creates a new default aggregator
func NewDefaultAggregator(config AggregatorConfig) *DefaultAggregator {
	return &DefaultAggregator{config: config}
}

// Aggregate combines results from multiple agents
func (a *DefaultAggregator) Aggregate(results []AgentResult) AggregatedResult {
	if len(results) == 0 {
		return AggregatedResult{}
	}

	if len(results) == 1 {
		return AggregatedResult{
			Findings:     results[0].Findings,
			Summary:      results[0].Summary,
			AgentResults: results,
			Confidence:   results[0].Confidence,
			Duration:     results[0].Duration,
		}
	}

	// Collect all findings with agent attribution
	var allFindings []FindingWithAgent
	for _, result := range results {
		for _, finding := range result.Findings {
			allFindings = append(allFindings, FindingWithAgent{
				Finding:   finding,
				AgentName: result.AgentName,
				Persona:   result.Persona,
			})
		}
	}

	// 统计每个 persona 实际运行的实例数（= 配置的冗余数，路径 A4）
	personaTotals := make(map[string]int)
	for _, result := range results {
		personaTotals[result.Persona]++
	}

	// Deduplicate findings
	deduplicated := a.deduplicateFindings(allFindings, personaTotals)

	// Calculate confidence for each finding
	for i := range deduplicated {
		deduplicated[i].Confidence = a.calculateConfidence(deduplicated[i], len(results))
	}

	// Sort by severity and confidence
	sort.Slice(deduplicated, func(i, j int) bool {
		if deduplicated[i].Severity != deduplicated[j].Severity {
			return types.SeverityRank(deduplicated[i].Severity) > types.SeverityRank(deduplicated[j].Severity)
		}
		return deduplicated[i].Confidence > deduplicated[j].Confidence
	})

	// Convert to final findings
	findings := slicex.Map(deduplicated, func(f FindingWithAgent) Finding {
		return f.Finding
	})

	// Calculate overall confidence
	overallConfidence := a.calculateOverallConfidence(findings, len(results))

	// Generate summary
	summary := a.generateSummary(findings, results)

	// Calculate total duration
	var totalDuration time.Duration
	for _, r := range results {
		totalDuration += r.Duration
	}

	return AggregatedResult{
		Findings:     findings,
		Summary:      summary,
		AgentResults: results,
		Confidence:   overallConfidence,
		Duration:     totalDuration,
	}
}

// FindingWithAgent represents a finding with agent attribution
type FindingWithAgent struct {
	Finding
	AgentName string
	Persona   string
}

// deduplicateFindings removes duplicate findings from different agents
func (a *DefaultAggregator) deduplicateFindings(findings []FindingWithAgent, personaTotals map[string]int) []FindingWithAgent {
	if len(findings) == 0 {
		return findings
	}

	// Group findings by path and line
	groups := make(map[string][]FindingWithAgent)
	for _, f := range findings {
		key := fmt.Sprintf("%s:%d:%d", f.Path, f.Line, f.EndLine)
		groups[key] = append(groups[key], f)
	}

	var deduplicated []FindingWithAgent
	for _, group := range groups {
		if len(group) == 1 {
			// 单个 agent 发现：仍需判断是否属于冗余 persona 的"单次观察"
			group[0].RedundancyStatus = computeRedundancyStatus(group, personaTotals)
			deduplicated = append(deduplicated, group[0])
			continue
		}

		// Merge findings in the same location
		merged := a.mergeFindings(group)
		merged.RedundancyStatus = computeRedundancyStatus(group, personaTotals)
		deduplicated = append(deduplicated, merged)
	}

	return deduplicated
}

// computeRedundancyStatus 计算冗余状态（路径 A4）：
// 同一 persona 的多个实例都发现 → confirmed（高置信）
// 同一 persona 配置了冗余但仅部分实例发现 → single（低置信）
// 未配置冗余 → 留空
func computeRedundancyStatus(group []FindingWithAgent, personaTotals map[string]int) string {
	personaHits := make(map[string]map[string]bool) // persona -> set of instance names
	for _, fwa := range group {
		if personaHits[fwa.Persona] == nil {
			personaHits[fwa.Persona] = make(map[string]bool)
		}
		personaHits[fwa.Persona][fwa.AgentName] = true
	}

	status := ""
	for persona, instances := range personaHits {
		total := personaTotals[persona]
		if total < 2 {
			continue
		}
		hits := len(instances)
		if hits >= 2 && hits == total {
			return types.RedundancyConfirmed
		}
		if status == "" && hits < total {
			status = types.RedundancySingle
		}
	}
	return status
}

// mergeFindings merges multiple findings from the same location
func (a *DefaultAggregator) mergeFindings(findings []FindingWithAgent) FindingWithAgent {
	if len(findings) == 0 {
		return FindingWithAgent{}
	}

	// Use the first finding as base
	merged := findings[0]

	// Collect all agent names
	agentSet := make(map[string]bool)
	for _, f := range findings {
		agentSet[f.AgentName] = true
	}

	// Update agents list
	agents := make([]string, 0, len(agentSet))
	for agent := range agentSet {
		agents = append(agents, agent)
	}
	sort.Strings(agents)
	merged.Agents = agents

	// Use the highest severity
	for _, f := range findings {
		if types.SeverityRank(f.Severity) > types.SeverityRank(merged.Severity) {
			merged.Severity = f.Severity
		}
	}

	// Combine bodies if different
	bodies := make(map[string]bool)
	for _, f := range findings {
		bodies[f.Body] = true
	}
	if len(bodies) > 1 {
		var combined []string
		for _, f := range findings {
			if f.Body != "" {
				combined = append(combined, fmt.Sprintf("[%s] %s", f.AgentName, f.Body))
			}
		}
		merged.Body = strings.Join(combined, "\n\n")
	}

	// Use the best suggestion
	for _, f := range findings {
		if f.Suggestion != "" && (merged.Suggestion == "" || len(f.Suggestion) > len(merged.Suggestion)) {
			merged.Suggestion = f.Suggestion
		}
	}

	return merged
}

// calculateConfidence calculates confidence for a finding
func (a *DefaultAggregator) calculateConfidence(finding FindingWithAgent, totalAgents int) float64 {
	// Agent count factor
	agentCount := len(finding.Agents)
	agentFactor := float64(agentCount) / float64(totalAgents)

	// Severity factor
	severityFactor := 1.0
	switch finding.Severity {
	case "critical":
		severityFactor = 1.0
	case "major":
		severityFactor = 0.8
	case "minor":
		severityFactor = 0.6
	case "info":
		severityFactor = 0.4
	}

	// Category factor
	categoryFactor := 1.0
	if finding.Category == "" {
		categoryFactor = 0.8
	}

	// Calculate weighted confidence
	weights := a.config.ConfidenceWeights
	confidence := (agentFactor * weights.AgentCount) +
		(severityFactor * weights.Severity) +
		(categoryFactor * weights.Category)

	// Normalize to 0-1
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// calculateOverallConfidence calculates overall confidence for the review
func (a *DefaultAggregator) calculateOverallConfidence(findings []Finding, totalAgents int) float64 {
	if len(findings) == 0 {
		return 0.0
	}

	// Calculate average confidence
	var totalConfidence float64
	for _, f := range findings {
		totalConfidence += f.Confidence
	}
	avgConfidence := totalConfidence / float64(len(findings))

	// Factor in agent coverage
	agentCoverage := float64(totalAgents) / 4.0 // Assuming 4 is ideal
	if agentCoverage > 1.0 {
		agentCoverage = 1.0
	}

	return avgConfidence * agentCoverage
}

// generateSummary generates a summary of the review
func (a *DefaultAggregator) generateSummary(findings []Finding, results []AgentResult) string {
	// Count findings by severity
	severityCounts := make(map[string]int)
	for _, f := range findings {
		severityCounts[f.Severity]++
	}

	// Build summary
	var parts []string

	// Total findings
	parts = append(parts, fmt.Sprintf("发现 %d 个问题", len(findings)))

	// Severity breakdown
	if count, ok := severityCounts["critical"]; ok && count > 0 {
		parts = append(parts, fmt.Sprintf("%d 个严重问题", count))
	}
	if count, ok := severityCounts["major"]; ok && count > 0 {
		parts = append(parts, fmt.Sprintf("%d 个重要问题", count))
	}
	if count, ok := severityCounts["minor"]; ok && count > 0 {
		parts = append(parts, fmt.Sprintf("%d 个次要问题", count))
	}

	// Agent info
	parts = append(parts, fmt.Sprintf("由 %d 个代理审查", len(results)))

	return strings.Join(parts, "，")
}
