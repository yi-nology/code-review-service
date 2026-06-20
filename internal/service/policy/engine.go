package policy

import (
	"strings"

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
	threshold := severityRank(e.cfg.FailOn)
	if threshold == 0 {
		threshold = severityRank("critical")
	}

	for _, finding := range findings {
		if e.ignored(finding.Path) {
			continue
		}
		if severityRank(finding.Severity) >= threshold {
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

// MatchesPath 检查路径是否匹配模式列表。
// 委托给 ekit 的 pathx.MatchesPath。
func MatchesPath(patterns []string, path string) bool {
	return pathx.MatchesPath(patterns, path)
}

func severityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "critical":
		return 4
	case "major":
		return 3
	case "minor":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}
