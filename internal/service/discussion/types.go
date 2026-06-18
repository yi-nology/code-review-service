package discussion

import (
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

// ResponseType represents the type of discussion response
type ResponseType string

const (
	ResponseAgree     ResponseType = "AGREE"
	ResponseChallenge ResponseType = "CHALLENGE"
	ResponseConnect   ResponseType = "CONNECT"
)

// Discussion represents a discussion round
type Discussion struct {
	ID        string           `json:"id"`
	SessionID string           `json:"session_id"`
	Round     int              `json:"round"`
	Entries   []DiscussionEntry `json:"entries"`
	Summary   string           `json:"summary"`
	CreatedAt time.Time        `json:"created_at"`
}

// DiscussionEntry represents a single discussion entry
type DiscussionEntry struct {
	ID         string       `json:"id"`
	AgentName  string       `json:"agent_name"`
	FindingID  string       `json:"finding_id"`
	Response   ResponseType `json:"response"`
	Content    string       `json:"content"`
	Evidence   string       `json:"evidence"`
	Confidence float64      `json:"confidence"`
	References []string     `json:"references"`
	CreatedAt  time.Time    `json:"created_at"`
}

// DiscussionResult represents the aggregated discussion result
type DiscussionResult struct {
	Consensus       []ConsensusItem    `json:"consensus"`
	Disagreements   []DisagreementItem `json:"disagreements"`
	NewFindings     []Finding          `json:"new_findings"`
	UpdatedFindings []Finding          `json:"updated_findings"`
	Summary         string             `json:"summary"`
}

// ConsensusItem represents a consensus finding
type ConsensusItem struct {
	FindingID  string   `json:"finding_id"`
	Agents     []string `json:"agents"`
	Confidence float64  `json:"confidence"`
	Summary    string   `json:"summary"`
}

// DisagreementItem represents a disagreement
type DisagreementItem struct {
	FindingID  string          `json:"finding_id"`
	Positions  []AgentPosition `json:"positions"`
	Resolution string          `json:"resolution"`
}

// AgentPosition represents an agent's position
type AgentPosition struct {
	AgentName  string  `json:"agent_name"`
	Position   string  `json:"position"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
}

// Finding is an alias for the canonical types.Finding
type Finding = types.Finding

// DiscussionPrompt represents a prompt for discussion
type DiscussionPrompt struct {
	AgentName   string   `json:"agent_name"`
	FindingID   string   `json:"finding_id"`
	Finding     Finding  `json:"finding"`
	OtherFindings []Finding `json:"other_findings"`
	Requirements  []string `json:"requirements,omitempty"`
	ResponseType  ResponseType `json:"response_type"`
}

// GenerateDiscussionPrompts generates prompts for discussion
func GenerateDiscussionPrompts(findings []Finding, agents []string, requirements []string) []DiscussionPrompt {
	var prompts []DiscussionPrompt

	// For each finding, ask other agents to respond
	for _, finding := range findings {
		for _, agent := range agents {
			// Skip if agent already found this
			if containsAgent(finding.Agents, agent) {
				continue
			}

			// Generate prompts for each response type
			responseTypes := []ResponseType{ResponseAgree, ResponseChallenge, ResponseConnect}
			for _, responseType := range responseTypes {
				prompt := DiscussionPrompt{
					AgentName:     agent,
					FindingID:     finding.ID,
					Finding:       finding,
					OtherFindings: filterFindings(findings, finding.ID),
					Requirements:  requirements,
					ResponseType:  responseType,
				}
				prompts = append(prompts, prompt)
			}
		}
	}

	return prompts
}

// containsAgent checks if an agent is in the list
func containsAgent(agents []string, agent string) bool {
	for _, a := range agents {
		if a == agent {
			return true
		}
	}
	return false
}

// filterFindings filters out a finding by ID
func filterFindings(findings []Finding, excludeID string) []Finding {
	var filtered []Finding
	for _, finding := range findings {
		if finding.ID != excludeID {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

// DiscussionConfig contains discussion configuration
type DiscussionConfig struct {
	Enabled     bool `yaml:"enabled"`
	MaxRounds   int  `yaml:"max_rounds"`
	MaxPrompts  int  `yaml:"max_prompts"`
	MinAgents   int  `yaml:"min_agents"`
}

// DefaultDiscussionConfig returns default discussion configuration
func DefaultDiscussionConfig() DiscussionConfig {
	return DiscussionConfig{
		Enabled:    true,
		MaxRounds:  3,
		MaxPrompts: 50,
		MinAgents:  2,
	}
}
