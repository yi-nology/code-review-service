package discussion

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/google/uuid"
)

// Orchestrator manages the discussion process
type Orchestrator struct {
	config DiscussionConfig
}

// NewOrchestrator creates a new discussion orchestrator
func NewOrchestrator(config DiscussionConfig) *Orchestrator {
	return &Orchestrator{config: config}
}

// RunDiscussion runs a discussion round
func (o *Orchestrator) RunDiscussion(ctx context.Context, findings []Finding, agents []string, requirements []string) (*DiscussionResult, error) {
	if !o.config.Enabled {
		return &DiscussionResult{
			Summary: "discussion disabled",
		}, nil
	}

	if len(agents) < o.config.MinAgents {
		return &DiscussionResult{
			Summary: fmt.Sprintf("Not enough agents for discussion (need %d, have %d)", o.config.MinAgents, len(agents)),
		}, nil
	}

	hlog.Infof("starting discussion with %d agents and %d findings", len(agents), len(findings))

	// Generate discussion prompts
	prompts := generateDiscussionPrompts(findings, agents, requirements)
	if len(prompts) > o.config.MaxPrompts {
		prompts = prompts[:o.config.MaxPrompts]
	}

	// Create discussion entries
	entries := make([]discussionEntry, 0, len(prompts))
	for _, prompt := range prompts {
		entry := discussionEntry{
			ID:         uuid.New().String(),
			AgentName:  prompt.AgentName,
			FindingID:  prompt.FindingID,
			Response:   prompt.ResponseType,
			Content:    generateResponseContent(prompt),
			Confidence: 0.8, // Default confidence
			CreatedAt:  time.Now(),
		}
		entries = append(entries, entry)
	}

	// Analyze discussion results
	result := o.analyzeDiscussion(findings, entries)

	hlog.Infof("discussion completed: %d consensus, %d disagreements, %d new findings",
		len(result.Consensus), len(result.Disagreements), len(result.NewFindings))

	return result, nil
}

// analyzeDiscussion analyzes the discussion entries and generates results
func (o *Orchestrator) analyzeDiscussion(findings []Finding, entries []discussionEntry) *DiscussionResult {
	// Group entries by finding
	entriesByFinding := make(map[string][]discussionEntry)
	for _, entry := range entries {
		entriesByFinding[entry.FindingID] = append(entriesByFinding[entry.FindingID], entry)
	}

	// Analyze each finding
	var consensus []consensusItem
	var disagreements []disagreementItem
	var updatedFindings []Finding

	for _, finding := range findings {
		findingEntries := entriesByFinding[finding.ID]
		if len(findingEntries) == 0 {
			updatedFindings = append(updatedFindings, finding)
			continue
		}

		// Count response types
		agreeCount := 0
		challengeCount := 0
		connectCount := 0
		for _, entry := range findingEntries {
			switch entry.Response {
			case responseAgree:
				agreeCount++
			case responseChallenge:
				challengeCount++
			case responseConnect:
				connectCount++
			}
		}

		// Determine consensus or disagreement
		if challengeCount == 0 && agreeCount > 0 {
			// Consensus
			agents := make([]string, 0)
			for _, entry := range findingEntries {
				if entry.Response == responseAgree {
					agents = append(agents, entry.AgentName)
				}
			}
			consensus = append(consensus, consensusItem{
				FindingID:  finding.ID,
				Agents:     agents,
				Confidence: float64(agreeCount) / float64(len(findingEntries)),
				Summary:    fmt.Sprintf("All agents agree on this finding"),
			})

			// Update finding confidence
			updatedFinding := finding
			updatedFinding.Confidence = float64(agreeCount) / float64(len(findingEntries))
			updatedFindings = append(updatedFindings, updatedFinding)
		} else if challengeCount > 0 {
			// Disagreement
			positions := make([]agentPosition, 0)
			for _, entry := range findingEntries {
				if entry.Response == responseChallenge {
					positions = append(positions, agentPosition{
						AgentName:  entry.AgentName,
						Position:   "challenge",
						Evidence:   entry.Content,
						Confidence: entry.Confidence,
					})
				} else if entry.Response == responseAgree {
					positions = append(positions, agentPosition{
						AgentName:  entry.AgentName,
						Position:   "agree",
						Evidence:   entry.Content,
						Confidence: entry.Confidence,
					})
				}
			}
			disagreements = append(disagreements, disagreementItem{
				FindingID:  finding.ID,
				Positions:  positions,
				Resolution: determineResolution(positions),
			})

			// Reduce confidence for challenged findings
			updatedFinding := finding
			updatedFinding.Confidence = finding.Confidence * 0.7
			updatedFindings = append(updatedFindings, updatedFinding)
		} else {
			updatedFindings = append(updatedFindings, finding)
		}
	}

	// Generate summary
	summary := generateDiscussionSummary(consensus, disagreements, updatedFindings)

	return &DiscussionResult{
		Consensus:       consensus,
		Disagreements:   disagreements,
		NewFindings:     []Finding{}, // No new findings in basic implementation
		UpdatedFindings: updatedFindings,
		Summary:         summary,
	}
}

// determineResolution determines the resolution for a disagreement
func determineResolution(positions []agentPosition) string {
	agreeCount := 0
	challengeCount := 0
	for _, pos := range positions {
		if pos.Position == "agree" {
			agreeCount++
		} else if pos.Position == "challenge" {
			challengeCount++
		}
	}

	if challengeCount > agreeCount {
		return "majority_challenges"
	} else if agreeCount > challengeCount {
		return "majority_agrees"
	}
	return "split_opinion"
}

// generateResponseContent generates content for a discussion response
func generateResponseContent(prompt discussionPrompt) string {
	switch prompt.ResponseType {
	case responseAgree:
		return fmt.Sprintf("I agree with this finding. %s", generateAgreeReason(prompt.Finding))
	case responseChallenge:
		return fmt.Sprintf("I challenge this finding. %s", generateChallengeReason(prompt.Finding))
	case responseConnect:
		return fmt.Sprintf("This connects to other findings. %s", generateConnectReason(prompt.Finding, prompt.OtherFindings))
	default:
		return "No comment"
	}
}

// generateAgreeReason generates a reason for agreeing
func generateAgreeReason(finding Finding) string {
	switch finding.Severity {
	case "critical":
		return "This is a critical issue that needs immediate attention."
	case "major":
		return "This is an important issue that should be addressed."
	case "minor":
		return "This is a valid minor improvement."
	default:
		return "This finding is valid."
	}
}

// generateChallengeReason generates a reason for challenging
func generateChallengeReason(finding Finding) string {
	switch finding.Category {
	case "security":
		return "The security concern may not be applicable in this context."
	case "performance":
		return "The performance impact is negligible."
	case "bug":
		return "This may not actually be a bug."
	default:
		return "I have concerns about this finding."
	}
}

// generateConnectReason generates a reason for connecting
func generateConnectReason(finding Finding, otherFindings []Finding) string {
	if len(otherFindings) == 0 {
		return "This finding stands alone."
	}

	// Find related findings
	related := make([]Finding, 0)
	for _, other := range otherFindings {
		if other.Path == finding.Path || other.Category == finding.Category {
			related = append(related, other)
		}
	}

	if len(related) == 0 {
		return "This finding is unique."
	}

	return fmt.Sprintf("This finding relates to %d other findings in the same file or category.", len(related))
}

// generateDiscussionSummary generates a summary of the discussion
func generateDiscussionSummary(consensus []consensusItem, disagreements []disagreementItem, findings []Finding) string {
	summary := fmt.Sprintf("discussion completed with %d findings. ", len(findings))

	if len(consensus) > 0 {
		summary += fmt.Sprintf("%d findings have consensus. ", len(consensus))
	}

	if len(disagreements) > 0 {
		summary += fmt.Sprintf("%d findings have disagreements. ", len(disagreements))
	}

	return summary
}
