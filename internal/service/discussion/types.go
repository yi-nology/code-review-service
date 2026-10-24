package discussion

import (
	"time"

	"github.com/oekit/pkg/slicex"

	types "github.com/yi-nology/code-review-service/internal/domain/finding"
)

type responseType string

const (
	responseAgree     responseType = "AGREE"
	responseChallenge responseType = "CHALLENGE"
	responseConnect   responseType = "CONNECT"
	responseSurface   responseType = "SURFACE"
)

type discussionEntry struct {
	ID         string       `json:"id"`
	AgentName  string       `json:"agent_name"`
	FindingID  string       `json:"finding_id"`
	Response   responseType `json:"response"`
	Content    string       `json:"content"`
	Evidence   string       `json:"evidence"`
	Confidence float64      `json:"confidence"`
	References []string     `json:"references"`
	CreatedAt  time.Time    `json:"created_at"`
}

type DiscussionResult struct {
	Consensus       []consensusItem    `json:"consensus"`
	Disagreements   []disagreementItem `json:"disagreements"`
	NewFindings     []Finding          `json:"new_findings"`
	UpdatedFindings []Finding          `json:"updated_findings"`
	Summary         string             `json:"summary"`
}

type consensusItem struct {
	FindingID  string   `json:"finding_id"`
	Agents     []string `json:"agents"`
	Confidence float64  `json:"confidence"`
	Summary    string   `json:"summary"`
}

type disagreementItem struct {
	FindingID  string          `json:"finding_id"`
	Positions  []agentPosition `json:"positions"`
	Resolution string          `json:"resolution"`
}

type agentPosition struct {
	AgentName  string  `json:"agent_name"`
	Position   string  `json:"position"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
}

type Finding = types.Finding

type discussionPrompt struct {
	AgentName     string       `json:"agent_name"`
	FindingID     string       `json:"finding_id"`
	Finding       Finding      `json:"finding"`
	OtherFindings []Finding    `json:"other_findings"`
	Requirements  []string     `json:"requirements,omitempty"`
	ResponseType  responseType `json:"response_type"`
}

func generateDiscussionPrompts(findings []Finding, agents []string, requirements []string) []discussionPrompt {
	var prompts []discussionPrompt
	for _, finding := range findings {
		for _, agent := range agents {
			if containsAgent(finding.Agents, agent) {
				continue
			}
			responseTypes := []responseType{responseAgree, responseChallenge, responseConnect}
			for _, responseType := range responseTypes {
				prompts = append(prompts, discussionPrompt{
					AgentName:     agent,
					FindingID:     finding.ID,
					Finding:       finding,
					OtherFindings: filterFindings(findings, finding.ID),
					Requirements:  requirements,
					ResponseType:  responseType,
				})
			}
		}
	}
	return prompts
}

func containsAgent(agents []string, agent string) bool {
	return slicex.Contains(agents, agent)
}

func filterFindings(findings []Finding, excludeID string) []Finding {
	var filtered []Finding
	for _, finding := range findings {
		if finding.ID != excludeID {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

type DiscussionConfig struct {
	Enabled    bool `yaml:"enabled"`
	MaxRounds  int  `yaml:"max_rounds"`
	MaxPrompts int  `yaml:"max_prompts"`
	MinAgents  int  `yaml:"min_agents"`
}

func DefaultDiscussionConfig() DiscussionConfig {
	return DiscussionConfig{
		Enabled:    true,
		MaxRounds:  3,
		MaxPrompts: 50,
		MinAgents:  2,
	}
}
