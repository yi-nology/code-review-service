package requirements

import (
	"strings"
	"time"

	types "github.com/yi-nology/code-review-service/internal/domain/finding"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(inline []string) (*Requirements, error) {
	return &Requirements{
		ID:        types.GenerateID(),
		Inline:    inline,
		Context:   make(map[string]string),
		CreatedAt: time.Now(),
	}, nil
}

func (p *Parser) ParseRequirements(text string) []Requirement {
	var requirements []Requirement
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			requirements = append(requirements, Requirement{
				ID:       types.GenerateID(),
				Text:     strings.TrimLeft(line, "-* "),
				Category: inferCategory(line),
				Priority: inferPriority(line),
				Source:   "inline",
			})
		}
	}
	return requirements
}

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
