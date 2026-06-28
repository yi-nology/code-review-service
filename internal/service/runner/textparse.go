package runner

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/types"
	"github.com/oekit/pkg/convert"
)

type ocrScanState struct {
	totalBatches     int
	completedBatches int
	totalFiles       int
	toolCallCount    int
	toolByTool       map[string]int64
	summary          bool
	filesReviewed    int64
	comments         int64
	totalTokens      int64
	inputTokens      int64
	outputTokens     int64
	elapsed          string
}

func newOcrScanState() *ocrScanState {
	return &ocrScanState{toolByTool: make(map[string]int64)}
}

func (st *ocrScanState) parseLine(line string, reporter progress.ProgressReporter) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[ocr]") {
		return
	}
	content := strings.TrimSpace(strings.TrimPrefix(line, "[ocr]"))

	switch {
	case strings.HasPrefix(content, "full-scan:"):
		st.parseFullScan(content)
		if reporter != nil && st.totalFiles > 0 {
			reporter.UpdatePhase(progress.PhaseAnalysis, "active", 12,
				"发现 "+strconv.Itoa(st.totalFiles)+" 个待审查文件")
		}
	case strings.HasPrefix(content, "scan dispatch:"):
		st.parseDispatch(content)
		if reporter != nil && st.totalBatches > 0 {
			reporter.UpdatePhase(progress.PhaseAnalysis, "active", 15,
				"共 "+strconv.Itoa(st.totalBatches)+" 个批次，开始审查...")
		}
	case strings.HasPrefix(content, "scan dedup batch #"):
		st.parseDedup(content, reporter)
	case strings.HasPrefix(content, "▶"):
		st.parseToolStart(content)
		if reporter != nil && st.totalBatches > 0 {
			pct := 15.0 + float64(st.completedBatches)/float64(st.totalBatches)*60.0
			toolFile := extractToolFile(content)
			if toolFile != "" {
				reporter.UpdatePhase(progress.PhaseAnalysis, "active", pct,
					"正在审查 "+toolFile+" (批次 "+strconv.Itoa(st.completedBatches+1)+"/"+strconv.Itoa(st.totalBatches)+")")
			}
		}
	case strings.HasPrefix(content, "✔"):
		st.parseToolDone(content)
	case strings.HasPrefix(content, "Summary:"):
		st.summary = true
		st.parseSummary(content)
	}
}

func (st *ocrScanState) parseFullScan(content string) {
	re := regexp.MustCompile(`reviewing (\d+)`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		st.totalFiles = convert.Str2Int(m[1], 0)
	}
}

func (st *ocrScanState) parseDispatch(content string) {
	re := regexp.MustCompile(`(\d+) batch`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		st.totalBatches = convert.Str2Int(m[1], 0)
	}
}

func (st *ocrScanState) parseDedup(content string, reporter progress.ProgressReporter) {
	re := regexp.MustCompile(`batch #(\d+)`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		st.completedBatches = convert.Str2Int(m[1], 0)
		if reporter != nil && st.totalBatches > 0 {
			pct := 15.0 + float64(st.completedBatches)/float64(st.totalBatches)*60.0
			reporter.UpdatePhase(progress.PhaseAnalysis, "active", pct,
				"已完成批次 "+strconv.Itoa(st.completedBatches)+"/"+strconv.Itoa(st.totalBatches))
		}
	}
}

func (st *ocrScanState) parseToolStart(content string) {
	re := regexp.MustCompile(`▶\s*(\S+)`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		st.toolCallCount++
		st.toolByTool[m[1]]++
	}
}

func extractToolFile(content string) string {
	re := regexp.MustCompile(`"([^"]+)"`)
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		return m[1]
	}
	return ""
}

func (st *ocrScanState) parseToolDone(content string) {
}

func (st *ocrScanState) parseSummary(content string) {
	re := regexp.MustCompile(`(\d+) file\(s\) reviewed.*?(\d+) comment\(s\).*?~?([\d]+) token\(s\).*?input: ~?([\d]+).*?output: ~?([\d]+).*?(\d+m\d+s)`)
	if m := re.FindStringSubmatch(content); len(m) > 5 {
		st.filesReviewed = convert.Str2Int64(m[1], 0)
		st.comments = convert.Str2Int64(m[2], 0)
		st.totalTokens = convert.Str2Int64(m[3], 0)
		st.inputTokens = convert.Str2Int64(m[4], 0)
		st.outputTokens = convert.Str2Int64(m[5], 0)
		if len(m) > 6 {
			st.elapsed = m[6]
		}
	}
}

func (st *ocrScanState) reportProgress(reporter progress.ProgressReporter) {
	if reporter == nil || st.totalBatches <= 0 {
		return
	}
	if st.completedBatches > 0 {
		pct := 15.0 + float64(st.completedBatches)/float64(st.totalBatches)*60.0
		reporter.UpdatePhase(progress.PhaseAnalysis, "active", pct,
			"已完成批次 "+strconv.Itoa(st.completedBatches)+"/"+strconv.Itoa(st.totalBatches))
	} else if st.totalFiles > 0 {
		reporter.UpdatePhase(progress.PhaseAnalysis, "active", 15,
			"正在审查 "+strconv.Itoa(st.totalFiles)+" 个文件...")
	}
}

var (
	separatorRe = regexp.MustCompile(`───\s*(.+?):(\d+)(?:-(\d+))?\s*───`)
	severityRe  = regexp.MustCompile(`\((Critical|High|Medium|Low|Info)\)`)
	categoryRe  = regexp.MustCompile(`【([^】-]+)-([^】]+)】`)
	fileRefRe   = regexp.MustCompile("`([^`]+)`")
)

func parseScanTextOutput(rawOutput string, defaultSeverity string) domain.Result {
	clean := stripANSI(rawOutput)
	lines := strings.Split(clean, "\n")

	var findings []types.Finding
	var ocrMeta domain.OCRMetadata
	ocrMeta.Engine = "open-code-review"
	ocrMeta.Status = "success"

	findingIdx := 0
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		if m := separatorRe.FindStringSubmatch(line); len(m) > 0 {
			findingIdx++
			file := m[1]
			startLine := convert.Str2Int(m[2], 0)
			endLine := startLine
			if m[3] != "" {
				endLine = convert.Str2Int(m[3], startLine)
			}

			var bodyParts []string
			j := i + 1
			for j < len(lines) {
				next := strings.TrimSpace(lines[j])
				if separatorRe.MatchString(next) || strings.HasPrefix(next, "[ocr]") {
					break
				}
				if next != "" {
					bodyParts = append(bodyParts, next)
				}
				j++
			}
			body := strings.Join(bodyParts, "\n")
			title := extractTitle(body, findingIdx)
			severity := extractSeverity(body, defaultSeverity)
			category := extractCategory(body)

			findings = append(findings, types.Finding{
				ID:       "f_" + strconv.Itoa(findingIdx),
				Severity: severity,
				Category: category,
				Path:     file,
				Line:     startLine,
				StartLine: startLine,
				EndLine:   endLine,
				Title:    title,
				Body:     body,
			})
			i = j
			continue
		}

		if strings.HasPrefix(line, "[ocr] Summary:") {
			parseSummaryLine(line, &ocrMeta)
		}

		i++
	}

	if ocrMeta.FilesReviewed == 0 && len(findings) > 0 {
		files := make(map[string]bool)
		for _, f := range findings {
			files[f.Path] = true
		}
		ocrMeta.FilesReviewed = int64(len(files))
	}
	if ocrMeta.Comments == 0 {
		ocrMeta.Comments = int64(len(findings))
	}

	return domain.Result{
		Summary:  "text-scan: " + strconv.Itoa(len(findings)) + " findings",
		OCR:      ocrMeta,
		Findings: findings,
		Scope: domain.ScopeMetadata{
			ReviewedFiles: int(ocrMeta.FilesReviewed),
			ChangedFiles:  int(ocrMeta.FilesReviewed),
		},
	}
}

func parseSummaryLine(line string, meta *domain.OCRMetadata) {
	content := strings.TrimSpace(strings.TrimPrefix(line, "[ocr] Summary:"))
	re := regexp.MustCompile(`(\d+) file\(s\) reviewed.*?(\d+) comment\(s\).*?~?([\d]+) token\(s\).*?input: ~?([\d]+).*?output: ~?([\d]+).*?(\d+m\d+s)`)
	if m := re.FindStringSubmatch(content); len(m) > 5 {
		meta.FilesReviewed = convert.Str2Int64(m[1], 0)
		meta.Comments = convert.Str2Int64(m[2], 0)
		meta.TotalTokens = convert.Str2Int64(m[3], 0)
		meta.InputTokens = convert.Str2Int64(m[4], 0)
		meta.OutputTokens = convert.Str2Int64(m[5], 0)
		if len(m) > 6 {
			meta.Elapsed = m[6]
		}
	}
}

func extractTitle(body string, idx int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "Finding #" + strconv.Itoa(idx)
	}
	if len(body) > 80 {
		return body[:80] + "..."
	}
	return body
}

func extractSeverity(body string, fallback string) string {
	if m := severityRe.FindStringSubmatch(body); len(m) > 1 {
		return mapSeverity(m[1])
	}
	return types.NormalizeSeverity(fallback)
}

func mapSeverity(s string) string {
	switch strings.ToLower(s) {
	case "critical":
		return "critical"
	case "high":
		return "major"
	case "medium":
		return "minor"
	case "low", "info":
		return "info"
	default:
		return "major"
	}
}

func extractCategory(body string) string {
	if m := categoryRe.FindStringSubmatch(body); len(m) > 1 {
		return m[1]
	}
	return "general"
}
