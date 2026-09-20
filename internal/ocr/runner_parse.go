package ocr

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// runner_parse.go：OCR 输出解析（进度行/结果 JSON/标题推断/语言 prompt）。
// parseOCRProgress 从 stderr 一行文本中解析 OCR 进度并通过 reporter 上报。
// OCR CLI 输出形如 "progress: 50% phase=context"。
// 此前该函数是空桩（_ = reporter），导致 WS 进度 UI 拿不到 OCR 子步骤进度。
func parseOCRProgress(line string, reporter progress.ProgressReporter) {
	const prefix = "progress:"
	if !strings.HasPrefix(line, prefix) {
		return
	}
	rest := strings.TrimSpace(line[len(prefix):])
	fields := strings.Fields(rest)

	var pct float64
	var phaseName string
	for _, f := range fields {
		switch {
		case strings.HasSuffix(f, "%"):
			if v, err := strconv.ParseFloat(strings.TrimSuffix(f, "%"), 64); err == nil {
				pct = v
			}
		case strings.HasPrefix(f, "phase="):
			phaseName = strings.TrimPrefix(f, "phase=")
		}
	}

	// 归一化百分比，防止异常输入。
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}

	// 已知 phase 名与 progress.Phase 常量一致，直接映射以驱动前端阶段切换。
	if _, ok := knownOCRPhases[phaseName]; ok {
		reporter.UpdatePhase(progress.Phase(phaseName), "active", pct, "ocr: "+rest)
		return
	}
	reporter.ReportProgress("ocr: "+rest, pct)
}

// parseOpenCodeReviewOutput 解析 `ocr review --format json` 的 JSON 输出。
//
// OCR 输出结构：
//
//	{
//	  "status": "success",
//	  "summary": {
//	    "files_reviewed": 1, "comments": 3,
//	    "total_tokens": 23474, "input_tokens": 21694, "output_tokens": 1780,
//	    "elapsed": "35s"
//	  },
//	  "tool_calls": { "total": 5, "by_tool": { "code_search": 1, ... } },
//	  "comments": [
//	    { "path", "content", "suggestion_code", "existing_code", "start_line", "end_line" }
//	  ]
//	}
//
// comments 映射为 domain.Finding；summary 映射为 OCRMetadata。
// OCR 不输出 severity，统一用 defaultSeverity（上层归一化）。
func parseOpenCodeReviewOutput(data []byte, defaultSeverity string) domain.Result {
	if len(data) == 0 {
		return domain.Result{Summary: "ocr 评审未产生输出"}
	}

	var payload struct {
		Status  string `json:"status"`
		Summary struct {
			FilesReviewed  int    `json:"files_reviewed"`
			Comments       int    `json:"comments"`
			TotalTokens    int64  `json:"total_tokens"`
			InputTokens    int64  `json:"input_tokens"`
			OutputTokens   int64  `json:"output_tokens"`
			Elapsed        string `json:"elapsed"`
			BudgetExceeded bool   `json:"budget_exceeded"`
		} `json:"summary"`
		ToolCalls *domain.ToolCallsMetadata `json:"tool_calls"`
		// Session 是 OCR v1.7.6+ 引入的 review session 元数据；旧版本无此字段，零值即可。
		// resume 命中时 reused_files > 0；session_id 可用于 `ocr session show <id>` 排障。
		Session *struct {
			SessionID   string `json:"session_id"`
			ReusedFiles int    `json:"reused_files"`
		} `json:"session"`
		Comments []struct {
			Path           string `json:"path"`
			Content        string `json:"content"`
			SuggestionCode string `json:"suggestion_code"`
			ExistingCode   string `json:"existing_code"`
			StartLine      int    `json:"start_line"`
			EndLine        int    `json:"end_line"`
		} `json:"comments"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		// 非 JSON 输出（如纯文本错误）：保留原文当 summary，便于排障。
		return domain.Result{
			Summary: strings.TrimSpace(string(data)),
			OCR:     domain.OCRMetadata{Engine: "open-code-review", Status: "parse_error"},
		}
	}

	severity := strings.TrimSpace(defaultSeverity)
	if severity == "" {
		severity = DefaultSeverity
	}

	findings := make([]domain.Finding, 0, len(payload.Comments))
	for _, c := range payload.Comments {
		line := c.StartLine
		if line <= 0 {
			line = c.EndLine
		}
		findings = append(findings, domain.Finding{
			Severity:       severity,
			Path:           c.Path,
			Line:           line,
			StartLine:      c.StartLine,
			EndLine:        c.EndLine,
			Title:          findingTitle(c.Content),
			Body:           c.Content,
			ExistingCode:   c.ExistingCode,
			SuggestionCode: c.SuggestionCode,
			Suggestion:     c.SuggestionCode,
		})
	}

	commentCount := payload.Summary.Comments
	if commentCount == 0 {
		commentCount = len(findings)
	}
	summary := fmt.Sprintf("ocr 评审完成：%d 条评论，%d 个文件", commentCount, payload.Summary.FilesReviewed)

	result := domain.Result{
		Summary:  summary,
		Findings: findings,
		OCR: domain.OCRMetadata{
			Engine:        "open-code-review",
			Status:        payload.Status,
			FilesReviewed: int64(payload.Summary.FilesReviewed),
			Comments:      int64(commentCount),
			TotalTokens:   payload.Summary.TotalTokens,
			InputTokens:   payload.Summary.InputTokens,
			OutputTokens:  payload.Summary.OutputTokens,
			Elapsed:       payload.Summary.Elapsed,
			ToolCalls:     payload.ToolCalls,
		},
	}
	if payload.Session != nil {
		result.OCR.SessionID = payload.Session.SessionID
		result.OCR.ReusedFiles = payload.Session.ReusedFiles
	}
	result.OCR.BudgetExceeded = payload.Summary.BudgetExceeded
	return result
}

// findingTitle 从 OCR comment 的完整内容中提取一个简短标题。
func findingTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	title := content
	for _, sep := range []string{"\n", "。", "；", "：", ":", ".", ";", "，"} {
		if idx := strings.Index(content, sep); idx > 0 {
			title = strings.TrimSpace(content[:idx])
			break
		}
	}
	if r := []rune(title); len(r) > 80 {
		title = string(r[:80]) + "…"
	}
	return title
}

// languagePrompt 根据语言设置生成 background 提示，确保 LLM 用指定语言输出。
// 支持 "中文"/"Chinese"/"zh"(不区分大小写) 和 "English"/"en"。
func languagePrompt(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "中文", "chinese", "zh", "zh-cn":
		return "请用中文输出所有审查结果，包括问题描述、建议和总结。"
	case "english", "en":
		return "Please output all review results in English, including findings, suggestions, and summary."
	default:
		return ""
	}
}
