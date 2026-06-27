package discussion

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/slicex"
)

// Orchestrator manages the discussion process.
type Orchestrator struct {
	config   DiscussionConfig
	provider llm.Provider // B1: LLM 驱动真讨论；nil 或不可用时降级到模板逻辑
}

// NewOrchestrator creates a new discussion orchestrator.
// provider 为 LLM 调用抽象；传 nil 或不可用时自动降级到模板逻辑，保证可用。
func NewOrchestrator(config DiscussionConfig, provider llm.Provider) *Orchestrator {
	return &Orchestrator{config: config, provider: provider}
}

// RunDiscussion runs a discussion round.
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

	logx.Infof("starting discussion with %d agents and %d findings", len(agents), len(findings))

	// B1: 优先用 LLM 驱动真讨论（每个 agent 一次调用，集中表态）
	entries, newFindings, err := o.runLLMDiscourse(ctx, findings, agents, requirements)
	if err != nil {
		logx.Warnf("LLM discourse unavailable, falling back to template logic: %v", err)
		entries = o.fallbackEntries(findings, agents, requirements)
		newFindings = nil
	}

	// Analyze discussion results
	result := o.analyzeDiscussion(findings, entries)
	result.NewFindings = newFindings

	logx.Infof("discussion completed: %d consensus, %d disagreements, %d new findings",
		len(result.Consensus), len(result.Disagreements), len(result.NewFindings))

	return result, nil
}

// runLLMDiscourse 用 LLM 驱动真讨论（B1）。每个 agent 一次调用，对该 agent 未提交的
// 所有 findings 集中表态（AGREE/CHALLENGE/CONNECT），并可 SURFACE 新发现。
func (o *Orchestrator) runLLMDiscourse(ctx context.Context, findings []Finding, agents []string, requirements []string) ([]discussionEntry, []Finding, error) {
	if o.provider == nil || !o.provider.Available(ctx) {
		return nil, nil, llm.ErrNotConfigured
	}

	var allEntries []discussionEntry
	var newFindings []Finding

	for _, agentName := range agents {
		systemPrompt := buildDiscourseSystemPrompt(agentName)
		userPrompt := buildDiscourseUserPrompt(findings, agentName, requirements)

		var items []llmDiscussionItem
		err := llm.CompleteJSON(ctx, o.provider, llm.CompleteRequest{
			SystemPrompt: systemPrompt,
			UserPrompt:   userPrompt,
			MaxTokens:    4096,
		}, &items)
		if err != nil {
			logx.Warnf("LLM discourse for agent %s failed: %v", agentName, err)
			continue // 单个 agent 失败不阻塞其他 agent
		}

		for _, item := range items {
			// SURFACE 新发现（B1 关键：之前 NewFindings 永远为空）
			if item.Response == string(responseSurface) || item.FindingID == "SURFACE" {
				body := item.Content
				if strings.TrimSpace(item.Evidence) != "" {
					body = fmt.Sprintf("%s\n\n证据: %s", item.Content, item.Evidence)
				}
				newFindings = append(newFindings, Finding{
					ID:         types.GenerateID(),
					Severity:   "major",
					Category:   "discussion_surface",
					Title:      truncate(item.Content, 120),
					Body:       body,
					Confidence: normalizeConfidence(item.Confidence),
					Agents:     []string{agentName},
				})
				continue
			}

			resp := responseType(item.Response)
			if resp != responseAgree && resp != responseChallenge && resp != responseConnect {
				continue
			}
			allEntries = append(allEntries, discussionEntry{
				ID:         types.GenerateID(),
				AgentName:  agentName,
				FindingID:  item.FindingID,
				Response:   resp,
				Content:    item.Content,
				Evidence:   item.Evidence,
				Confidence: normalizeConfidence(item.Confidence),
				CreatedAt:  time.Now(),
			})
		}
	}

	if len(allEntries) == 0 && len(newFindings) == 0 {
		return nil, nil, fmt.Errorf("llm discourse produced no usable entries")
	}
	return allEntries, newFindings, nil
}

// fallbackEntries 是降级路径：LLM 不可用时用模板逻辑生成讨论条目。
func (o *Orchestrator) fallbackEntries(findings []Finding, agents []string, requirements []string) []discussionEntry {
	prompts := generateDiscussionPrompts(findings, agents, requirements)
	if len(prompts) > o.config.MaxPrompts {
		prompts = prompts[:o.config.MaxPrompts]
	}

	entries := make([]discussionEntry, 0, len(prompts))
	for _, prompt := range prompts {
		entries = append(entries, discussionEntry{
			ID:         types.GenerateID(),
			AgentName:  prompt.AgentName,
			FindingID:  prompt.FindingID,
			Response:   prompt.ResponseType,
			Content:    generateResponseContent(prompt),
			Confidence: 0.8,
			CreatedAt:  time.Now(),
		})
	}
	return entries
}

// analyzeDiscussion analyzes the discussion entries and generates results
func (o *Orchestrator) analyzeDiscussion(findings []Finding, entries []discussionEntry) *DiscussionResult {
	// Group entries by finding
	entriesByFinding := slicex.GroupBy(entries, func(e discussionEntry) string { return e.FindingID })

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
		switch {
		case challengeCount == 0 && agreeCount > 0:
			// Consensus
			agents := make([]string, 0)
			confSum := 0.0
			for _, entry := range findingEntries {
				if entry.Response == responseAgree {
					agents = append(agents, entry.AgentName)
					confSum += entry.Confidence
				}
			}
			// B3: 置信度动态化 = AGREE 平均置信度 × 同意比率
			avgConf := 0.0
			if agreeCount > 0 {
				avgConf = confSum / float64(agreeCount)
			}
			agreementRatio := float64(agreeCount) / float64(len(findingEntries))
			finalConfidence := avgConf * agreementRatio

			consensus = append(consensus, consensusItem{
				FindingID:  finding.ID,
				Agents:     agents,
				Confidence: finalConfidence,
				Summary:    "All agents agree on this finding",
			})

			// Update finding confidence
			updatedFinding := finding
			updatedFinding.Confidence = finalConfidence
			updatedFindings = append(updatedFindings, updatedFinding)
		case challengeCount > 0:
			// Disagreement
			positions := make([]agentPosition, 0)
			for _, entry := range findingEntries {
				switch entry.Response {
				case responseChallenge:
					positions = append(positions, agentPosition{
						AgentName:  entry.AgentName,
						Position:   "challenge",
						Evidence:   entry.Content,
						Confidence: entry.Confidence,
					})
				case responseAgree:
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
		default:
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
		switch pos.Position {
		case "agree":
			agreeCount++
		case "challenge":
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
