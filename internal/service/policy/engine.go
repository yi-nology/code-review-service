package policy

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/types"
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
		threshold = types.SeverityRank("critical")
	}

	for _, finding := range findings {
		if e.ignored(finding.Path) {
			continue
		}
		if types.SeverityRank(finding.Severity) >= threshold {
			return Decision{
				Passed: false,
				Reason: fmt.Sprintf("%s issue found", finding.Severity),
			}
		}
	}

	return Decision{Passed: true}
}

func (e *Engine) ignored(path string) bool {
	return MatchesPath(e.cfg.IgnorePaths, path)
}

func MatchesPath(patterns []string, path string) bool {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return false
	}

	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
		if strings.HasSuffix(pattern, "/**") && strings.HasPrefix(path, strings.TrimSuffix(pattern, "**")) {
			return true
		}
	}
	return false
}
