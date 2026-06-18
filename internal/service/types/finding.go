package types

import "strings"

type Finding struct {
	ID         string   `json:"id"`
	Severity   string   `json:"severity"`
	Category   string   `json:"category"`
	Path       string   `json:"path"`
	Line       int      `json:"line"`
	EndLine    int      `json:"end_line"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Suggestion string   `json:"suggestion"`
	Confidence float64  `json:"confidence"`
	Agents     []string `json:"agents"`
}

func SeverityRank(severity string) int {
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
