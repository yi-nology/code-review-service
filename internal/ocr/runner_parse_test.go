package ocr

import (
	"encoding/json"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// TestParseOpenCodeReviewOutput_RealSchema 用真实 `ocr review --format json` 输出验证解析。
func TestParseOpenCodeReviewOutput_RealSchema(t *testing.T) {
	// 来自真实 ocr review 的输出（结构守卫，字段精简）。
	raw := `{
	  "status": "success",
	  "summary": {
	    "files_reviewed": 1,
	    "comments": 3,
	    "total_tokens": 23474,
	    "input_tokens": 21694,
	    "output_tokens": 1780,
	    "elapsed": "35s"
	  },
	  "tool_calls": {
	    "total": 5,
	    "by_tool": {"code_comment": 1, "code_search": 1, "file_find": 2, "file_read": 1}
	  },
	  "comments": [
	    {
	      "path": "calc.go",
	      "content": "安全问题：硬编码的敏感信息（密码和API密钥）应该从代码中移除。",
	      "suggestion_code": "password := os.Getenv(\"APP_PASSWORD\")",
	      "existing_code": "password := \"hardcoded-secret-12345\"",
	      "start_line": 14,
	      "end_line": 16
	    },
	    {
	      "path": "calc.go",
	      "content": "运行时错误：第18行的 divide(10, 0) 会导致运行时panic。",
	      "suggestion_code": "if b == 0 { return 0 }",
	      "existing_code": "return a / b",
	      "start_line": 10,
	      "end_line": 12
	    }
	  ]
	}`

	got := parseOpenCodeReviewOutput([]byte(raw), "major")

	if len(got.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(got.Findings))
	}
	first := got.Findings[0]
	if first.Path != "calc.go" || first.StartLine != 14 || first.EndLine != 16 {
		t.Errorf("unexpected first finding: %+v", first)
	}
	if first.Body == "" || first.Title == "" {
		t.Errorf("expected non-empty title/body, got title=%q body=%q", first.Title, first.Body)
	}
	if first.Severity != "major" {
		t.Errorf("expected severity=major, got %q", first.Severity)
	}
	if first.Suggestion == "" || first.Suggestion != first.SuggestionCode {
		t.Errorf("expected Suggestion=SuggestionCode=%q, got %q", first.SuggestionCode, first.Suggestion)
	}
	if first.ExistingCode == "" {
		t.Errorf("expected non-empty existing code")
	}

	if got.OCR.Status != "success" {
		t.Errorf("expected OCR status=success, got %q", got.OCR.Status)
	}
	if got.OCR.FilesReviewed != 1 || got.OCR.Comments != 3 {
		t.Errorf("unexpected OCR metadata: files=%d comments=%d", got.OCR.FilesReviewed, got.OCR.Comments)
	}
	if got.OCR.TotalTokens != 23474 || got.OCR.InputTokens != 21694 || got.OCR.OutputTokens != 1780 {
		t.Errorf("unexpected token counts: total=%d in=%d out=%d", got.OCR.TotalTokens, got.OCR.InputTokens, got.OCR.OutputTokens)
	}
	if got.OCR.ToolCalls == nil || got.OCR.ToolCalls.Total != 5 {
		t.Errorf("unexpected tool_calls: %+v", got.OCR.ToolCalls)
	}
	if got.OCR.Elapsed != "35s" {
		t.Errorf("expected elapsed=35s, got %q", got.OCR.Elapsed)
	}
	if got.Summary == "" {
		t.Errorf("expected non-empty summary")
	}
}

// TestParseOpenCodeReviewOutput_EmptyAndNonJSON 覆盖空输出与非 JSON 输出路径。
func TestParseOpenCodeReviewOutput_EmptyAndNonJSON(t *testing.T) {
	if got := parseOpenCodeReviewOutput(nil, "major"); got.Summary == "" {
		t.Errorf("expected non-empty summary for empty output")
	}

	got := parseOpenCodeReviewOutput([]byte("not a json, just an error message"), "major")
	if got.OCR.Status != "parse_error" {
		t.Errorf("expected parse_error status, got %q", got.OCR.Status)
	}
	if got.Summary == "" {
		t.Errorf("expected raw text preserved as summary")
	}
}

// TestParseOpenCodeReviewOutput_NoComments 确保 0 评论时仍正常返回元数据。
func TestParseOpenCodeReviewOutput_NoComments(t *testing.T) {
	raw := `{"status":"success","summary":{"files_reviewed":2,"comments":0,"total_tokens":100,"elapsed":"5s"},"comments":[]}`
	got := parseOpenCodeReviewOutput([]byte(raw), "major")
	if len(got.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(got.Findings))
	}
	if got.OCR.FilesReviewed != 2 {
		t.Errorf("expected 2 files reviewed, got %d", got.OCR.FilesReviewed)
	}
}

// 守卫：确保 domain.Result 仍可 JSON 序列化（防止字段类型回归）。
func TestDomainResultSerializes(t *testing.T) {
	r := domain.Result{
		Summary:  "x",
		Findings: []domain.Finding{{Path: "a.go", Line: 1}},
	}
	if _, err := json.Marshal(r); err != nil {
		t.Fatalf("marshal Result: %v", err)
	}
}
