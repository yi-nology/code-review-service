package runner_test

import (
	"context"
	"testing"

	"review-service/internal/service/review"
	"review-service/internal/service/runner"
)

func TestDeterministicRunnerReturnsEmptyFindings(t *testing.T) {
	r := runner.NewDeterministicRunner()
	workspace := review.Workspace{
		Repo:    "acme/app",
		BaseSHA: "a1b2c3d4e5f6a1b2",
		HeadSHA: "f6e5d4c3b2a1f6e5",
	}

	result, err := r.Run(context.Background(), workspace)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(result.Findings))
	}
	if result.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestDeterministicRunnerIncludesDiffInfo(t *testing.T) {
	r := runner.NewDeterministicRunner()
	workspace := review.Workspace{
		Repo:    "acme/app",
		BaseSHA: "a1b2c3d4e5f6a1b2",
		HeadSHA: "f6e5d4c3b2a1f6e5",
		Diff:    "diff --git a/main.go b/main.go\n...",
	}

	result, err := r.Run(context.Background(), workspace)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestParseOpenCodeReviewOutputBasic(t *testing.T) {
	input := []byte(`{
		"status": "success",
		"comments": [
			{
				"path": "main.go",
				"content": "critical: SQL injection vulnerability detected",
				"start_line": 10,
				"end_line": 15
			},
			{
				"path": "util.go",
				"content": "minor: naming convention issue",
				"start_line": 5,
				"end_line": 5
			}
		],
		"summary": {
			"files_reviewed": 3,
			"comments": 2,
			"total_tokens": 1000,
			"elapsed": "5.2s"
		}
	}`)

	result, err := runner.ParseOpenCodeReviewOutput(input, "major")
	if err != nil {
		t.Fatalf("ParseOpenCodeReviewOutput returned error: %v", err)
	}

	if len(result.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(result.Findings))
	}

	if result.Findings[0].Path != "main.go" {
		t.Fatalf("expected path main.go, got %s", result.Findings[0].Path)
	}
	if result.Findings[0].Severity != review.SeverityCritical {
		t.Fatalf("expected critical severity for SQL injection, got %s", result.Findings[0].Severity)
	}
	if result.Findings[0].Line != 10 {
		t.Fatalf("expected line 10, got %d", result.Findings[0].Line)
	}

	if result.Findings[1].Path != "util.go" {
		t.Fatalf("expected path util.go, got %s", result.Findings[1].Path)
	}
	if result.Findings[1].Severity != review.SeverityMinor {
		t.Fatalf("expected minor severity, got %s", result.Findings[1].Severity)
	}

	if result.OCR.Engine != "open-code-review" {
		t.Fatalf("expected engine open-code-review, got %s", result.OCR.Engine)
	}
	if result.OCR.Status != "success" {
		t.Fatalf("expected status success, got %s", result.OCR.Status)
	}
	if result.OCR.FilesReviewed != 3 {
		t.Fatalf("expected 3 files reviewed, got %d", result.OCR.FilesReviewed)
	}
	if result.OCR.TotalTokens != 1000 {
		t.Fatalf("expected 1000 total tokens, got %d", result.OCR.TotalTokens)
	}
	if result.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestParseOpenCodeReviewOutputEmpty(t *testing.T) {
	_, err := runner.ParseOpenCodeReviewOutput([]byte{}, "major")
	if err == nil {
		t.Fatal("expected error for empty output")
	}
}

func TestParseOpenCodeReviewOutputNoJSON(t *testing.T) {
	_, err := runner.ParseOpenCodeReviewOutput([]byte("no json here"), "major")
	if err == nil {
		t.Fatal("expected error for output without JSON")
	}
}

func TestParseOpenCodeReviewOutputWithPrefix(t *testing.T) {
	input := []byte(`some prefix text
{
	"status": "success",
	"comments": []
}`)

	result, err := runner.ParseOpenCodeReviewOutput(input, "major")
	if err != nil {
		t.Fatalf("ParseOpenCodeReviewOutput returned error: %v", err)
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning for prefix text, got %d", len(result.Warnings))
	}
	if result.Warnings[0].Type != "runner_output" {
		t.Fatalf("expected runner_output warning type, got %s", result.Warnings[0].Type)
	}
}

func TestParseOpenCodeReviewOutputSeverityInference(t *testing.T) {
	tests := []struct {
		content  string
		expected review.Severity
	}{
		{"critical security vulnerability", review.SeverityCritical},
		{"严重问题 in code", review.SeverityCritical},
		{"minor issue found", review.SeverityMinor},
		{"轻微问题 detected", review.SeverityMinor},
		{"info: something", review.SeverityInfo},
		{"提示 for improvement", review.SeverityInfo},
		{"regular issue", review.SeverityMajor},
	}

	for _, tt := range tests {
		input := []byte(`{"status":"success","comments":[{"path":"main.go","content":"` + tt.content + `","start_line":1,"end_line":1}]}`)
		result, err := runner.ParseOpenCodeReviewOutput(input, "major")
		if err != nil {
			t.Fatalf("unexpected error for content %q: %v", tt.content, err)
		}
		if len(result.Findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(result.Findings))
		}
		if result.Findings[0].Severity != tt.expected {
			t.Errorf("content %q: expected severity %s, got %s", tt.content, tt.expected, result.Findings[0].Severity)
		}
	}
}

func TestParseOpenCodeReviewOutputWithWarnings(t *testing.T) {
	input := []byte(`{
		"status": "success",
		"comments": [],
		"warnings": [
			{"type": "config", "message": "missing config file"},
			{"type": "model", "message": "rate limit hit"}
		]
	}`)

	result, err := runner.ParseOpenCodeReviewOutput(input, "major")
	if err != nil {
		t.Fatalf("ParseOpenCodeReviewOutput returned error: %v", err)
	}

	if len(result.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(result.Warnings))
	}
	if result.Warnings[0].Type != "config" {
		t.Fatalf("expected config warning type, got %s", result.Warnings[0].Type)
	}
	if result.Warnings[1].Type != "model" {
		t.Fatalf("expected model warning type, got %s", result.Warnings[1].Type)
	}
}

func TestParseOpenCodeReviewOutputWithSuggestion(t *testing.T) {
	input := []byte(`{
		"status": "success",
		"comments": [
			{
				"path": "main.go",
				"content": "use const instead of var",
				"suggestion_code": "const x = 1",
				"existing_code": "var x = 1",
				"start_line": 5,
				"end_line": 5
			}
		]
	}`)

	result, err := runner.ParseOpenCodeReviewOutput(input, "major")
	if err != nil {
		t.Fatalf("ParseOpenCodeReviewOutput returned error: %v", err)
	}

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].SuggestionCode != "const x = 1" {
		t.Fatalf("expected suggestion code, got %q", result.Findings[0].SuggestionCode)
	}
	if result.Findings[0].ExistingCode != "var x = 1" {
		t.Fatalf("expected existing code, got %q", result.Findings[0].ExistingCode)
	}
}
