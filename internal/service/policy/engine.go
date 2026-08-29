package policy

import (
	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/oekit/pkg/pathx"
)

type Config struct {
	FailOn      string
	MaxComments int
	SummaryOnly bool
	IgnorePaths []string
}

type Engine struct {
	cfg Config
}

type Decision struct {
	Passed bool
	Reason string
}

type Finding struct {
	Severity string
	Path     string
}

func NewEngine(cfg Config) *Engine {
	return &Engine{cfg: cfg}
}

func (e *Engine) Evaluate(findings []Finding) Decision {
	threshold := types.SeverityRank(e.cfg.FailOn)
	if threshold == 0 {
		threshold = types.SeverityRank(DefaultFailOn)
	}

	for _, finding := range findings {
		if e.ignored(finding.Path) {
			continue
		}
		if types.SeverityRank(finding.Severity) >= threshold {
			return Decision{
				Passed: false,
				Reason: finding.Severity + " issue found",
			}
		}
	}

	return Decision{Passed: true}
}

func (e *Engine) ignored(path string) bool {
	return pathx.MatchesPath(e.cfg.IgnorePaths, path)
}

func MatchesPath(patterns []string, path string) bool {
	return pathx.MatchesPath(patterns, path)
}
