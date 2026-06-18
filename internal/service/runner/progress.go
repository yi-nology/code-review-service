package runner

import (
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/oekit/pkg/convert"
)

func parseOCRProgress(line string, reporter progress.ProgressReporter) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if idx := strings.Index(line, "%"); idx > 0 {
		numStr := ""
		for i := idx - 1; i >= 0; i-- {
			c := line[i]
			if (c >= '0' && c <= '9') || c == '.' {
				numStr = string(c) + numStr
			} else if len(numStr) > 0 {
				break
			}
		}
		if numStr != "" {
			progressVal := convert.Str2Float64(numStr, -1)
			if progressVal >= 0 && progressVal <= 100 {
				reporter.ReportProgress(line, progressVal)
				return
			}
		}
	}

	if strings.Contains(line, "/") {
		parts := strings.Split(line, "/")
		if len(parts) >= 2 {
			for i, part := range parts {
				part = strings.TrimSpace(part)
				numStr := ""
				for _, c := range part {
					if c >= '0' && c <= '9' {
						numStr += string(c)
					}
				}
				if numStr != "" {
					current := convert.Str2Int(numStr, 0)
					if current > 0 && i < len(parts)-1 {
						totalStr := ""
						for _, c := range parts[i+1] {
							if c >= '0' && c <= '9' {
								totalStr += string(c)
							} else if len(totalStr) > 0 {
								break
							}
						}
						if totalStr != "" {
							total := convert.Str2Int(totalStr, 0)
							if total > 0 {
								reporter.ReportFileProgress(line, current, total)
								return
							}
						}
					}
				}
			}
		}
	}

	if strings.Contains(line, "Agent") || strings.Contains(line, "agent") {
		reporter.ReportAgentStatus("ocr", "active", line)
	}
}
