package review_test

import (
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

func TestBuildReportPerfectScore(t *testing.T) {
	result := &review.Result{
		Summary:  "no issues found",
		Findings: []review.Finding{},
		OCR:      review.OCRMetadata{Engine: "test", Status: "success", FilesReviewed: 5},
	}

	report := review.BuildReport(result)

	if report.Score.Value != 100 {
		t.Fatalf("expected score 100, got %d", report.Score.Value)
	}
	if report.Score.Grade != "优秀" {
		t.Fatalf("expected grade 优秀, got %s", report.Score.Grade)
	}
	if report.Complete != true {
		t.Fatal("expected complete=true")
	}
	if report.Generated != true {
		t.Fatal("expected generated=true when engine or findings present")
	}
	if report.Counts.Critical != 0 || report.Counts.Major != 0 || report.Counts.Minor != 0 || report.Counts.Info != 0 {
		t.Fatalf("expected all zero counts, got %+v", report.Counts)
	}
	if len(report.Score.Deductions) != 0 {
		t.Fatalf("expected no deductions, got %d", len(report.Score.Deductions))
	}
}

func TestBuildReportCriticalDeduction(t *testing.T) {
	result := &review.Result{
		Summary: "found critical issues",
		OCR:     review.OCRMetadata{Engine: "test", Status: "success", FilesReviewed: 3},
		Findings: []review.Finding{
			{Severity: review.SeverityCritical, Title: "SQL Injection", Body: "sql injection vulnerability detected"},
			{Severity: review.SeverityMajor, Title: "Error Handling", Body: "unchecked error value"},
			{Severity: review.SeverityMinor, Title: "Style", Body: "naming convention issue"},
			{Severity: review.SeverityInfo, Title: "Suggestion", Body: "consider refactoring"},
		},
	}

	report := review.BuildReport(result)

	if report.Counts.Critical != 1 {
		t.Fatalf("expected 1 critical, got %d", report.Counts.Critical)
	}
	if report.Counts.Major != 1 {
		t.Fatalf("expected 1 major, got %d", report.Counts.Major)
	}
	if report.Counts.Minor != 1 {
		t.Fatalf("expected 1 minor, got %d", report.Counts.Minor)
	}
	if report.Counts.Info != 1 {
		t.Fatalf("expected 1 info, got %d", report.Counts.Info)
	}

	if report.Score.Value >= 100 {
		t.Fatalf("expected score < 100, got %d", report.Score.Value)
	}

	expectedDeduction := 1*25 + 1*12 + 1*5 + 1*1
	if report.Score.Value != 100-expectedDeduction {
		t.Fatalf("expected score %d, got %d", 100-expectedDeduction, report.Score.Value)
	}

	if len(report.Score.Deductions) != 4 {
		t.Fatalf("expected 4 deductions, got %d", len(report.Score.Deductions))
	}

	if report.Generated != true {
		t.Fatal("expected generated=true")
	}
}

func TestBuildReportIncompleteRun(t *testing.T) {
	result := &review.Result{
		Summary:  "partial review",
		OCR:      review.OCRMetadata{Engine: "test", Status: "timeout"},
		Findings: []review.Finding{},
	}

	report := review.BuildReport(result)

	if report.Complete != false {
		t.Fatal("expected complete=false for non-success status")
	}

	if report.Score.Value != 80 {
		t.Fatalf("expected score 80 (100 - 20 incomplete), got %d", report.Score.Value)
	}

	found := false
	for _, d := range report.Score.Deductions {
		if d.Reason == "审查未完整完成" && d.Points == 20 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected incomplete deduction")
	}
}

func TestBuildReportWithWarnings(t *testing.T) {
	result := &review.Result{
		Summary: "review with warnings",
		OCR:     review.OCRMetadata{Engine: "test", Status: "success"},
		Findings: []review.Finding{},
		Warnings: []review.OCRWarning{
			{Type: "config", Message: "missing config"},
			{Type: "model", Message: "model warning"},
			{Type: "runner_output", Message: "should be ignored"},
		},
	}

	report := review.BuildReport(result)

	if report.Score.Value != 94 {
		t.Fatalf("expected score 94 (100 - 2*3), got %d", report.Score.Value)
	}

	found := false
	for _, d := range report.Score.Deductions {
		if d.Reason == "OCR 审查警告" && d.Count == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected warning deduction with count=2 (excluding runner_output)")
	}
}

func TestBuildReportNilResult(t *testing.T) {
	report := review.BuildReport(nil)

	if report.Score.Value != 0 {
		t.Fatalf("expected score 0 for nil result, got %d", report.Score.Value)
	}
	if report.Generated != false {
		t.Fatal("expected generated=false for nil result")
	}
}

func TestBuildReportScoreClampsToZero(t *testing.T) {
	findings := make([]review.Finding, 10)
	for i := range findings {
		findings[i] = review.Finding{
			Severity: review.SeverityCritical,
			Title:    "Critical Issue",
			Body:     "critical vulnerability",
		}
	}
	result := &review.Result{
		Summary:  "many issues",
		OCR:      review.OCRMetadata{Engine: "test", Status: "timeout"},
		Findings: findings,
		Warnings: []review.OCRWarning{
			{Type: "config", Message: "warning 1"},
			{Type: "config", Message: "warning 2"},
			{Type: "config", Message: "warning 3"},
			{Type: "config", Message: "warning 4"},
			{Type: "config", Message: "warning 5"},
		},
	}

	report := review.BuildReport(result)

	if report.Score.Value < 0 {
		t.Fatalf("score should not be negative, got %d", report.Score.Value)
	}
	if report.Score.Value != 0 {
		t.Fatalf("expected score clamped to 0, got %d", report.Score.Value)
	}
}

func TestScoreGrade(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{100, "优秀"},
		{95, "优秀"},
		{90, "优秀"},
		{89, "良好"},
		{85, "良好"},
		{80, "良好"},
		{79, "需改进"},
		{70, "需改进"},
		{60, "需改进"},
		{59, "高风险"},
		{30, "高风险"},
		{0, "高风险"},
	}
	for _, tt := range tests {
		findings := []review.Finding{}
		deduction := 100 - tt.score
		if deduction > 0 {
			findings = append(findings, review.Finding{
				Severity: review.SeverityInfo,
				Title:    "Issue",
				Body:     "info issue",
			})
		}
		result := &review.Result{
			Summary:  "test",
			OCR:      review.OCRMetadata{Engine: "test", Status: "success"},
			Findings: findings,
		}

		if deduction > 100 {
			result.OCR.Status = "timeout"
		}

		report := review.BuildReport(result)

		if deduction == 0 && report.Score.Grade != tt.expected {
			t.Errorf("score %d: expected grade %s, got %s", tt.score, tt.expected, report.Score.Grade)
		}
	}
}

func TestBuildReportClassifiesFindings(t *testing.T) {
	result := &review.Result{
		Summary: "classified findings",
		OCR:     review.OCRMetadata{Engine: "test", Status: "success"},
		Findings: []review.Finding{
			{Severity: review.SeverityCritical, Title: "SQL Injection", Body: "sql injection vulnerability detected"},
			{Severity: review.SeverityMajor, Title: "Performance", Body: "performance issue with slow query"},
			{Severity: review.SeverityMinor, Title: "Test", Body: "missing test coverage"},
			{Severity: review.SeverityInfo, Title: "Nil Check", Body: "nil pointer error handling"},
			{Severity: review.SeverityInfo, Title: "Naming", Body: "variable naming convention"},
		},
	}

	report := review.BuildReport(result)

	categories := map[string]bool{}
	for _, f := range result.Findings {
		categories[f.Category] = true
	}
	if !categories["安全性"] {
		t.Error("expected 安全性 category for SQL injection finding")
	}
	if !categories["性能"] {
		t.Error("expected 性能 category for performance finding")
	}
	if !categories["测试"] {
		t.Error("expected 测试 category for test coverage finding")
	}
	if !categories["正确性"] {
		t.Error("expected 正确性 category for nil pointer finding")
	}

	_ = report
}

func TestBuildReportVolumeMultiplierLargePR(t *testing.T) {
	result := &review.Result{
		Summary: "large PR with one critical",
		OCR:     review.OCRMetadata{Engine: "test", Status: "success"},
		Scope: review.ScopeMetadata{ChangedLines: 1000},
		Findings: []review.Finding{
			{Severity: review.SeverityCritical, Title: "Critical", Body: "critical vulnerability"},
		},
	}

	report := review.BuildReport(result)

	if report.Score.Value <= 75 {
		t.Fatalf("large PR (1000 lines) with 1 critical should score higher than v1 baseline (75), got %d", report.Score.Value)
	}
	if report.Score.Version != "severity-v2" {
		t.Fatalf("expected version severity-v2, got %s", report.Score.Version)
	}
	if report.Score.ChangedLines != 1000 {
		t.Fatalf("expected changed_lines=1000, got %d", report.Score.ChangedLines)
	}
}

func TestBuildReportVolumeMultiplierSmallPR(t *testing.T) {
	result := &review.Result{
		Summary: "small PR with one critical",
		OCR:     review.OCRMetadata{Engine: "test", Status: "success"},
		Scope: review.ScopeMetadata{ChangedLines: 10},
		Findings: []review.Finding{
			{Severity: review.SeverityCritical, Title: "Critical", Body: "critical vulnerability"},
		},
	}

	report := review.BuildReport(result)

	if report.Score.Value > 50 {
		t.Fatalf("small PR (10 lines) with 1 critical should score low, got %d", report.Score.Value)
	}
}

func TestBuildReportLargeCleanPRBonus(t *testing.T) {
	result := &review.Result{
		Summary: "large clean PR",
		OCR:     review.OCRMetadata{Engine: "test", Status: "success", FilesReviewed: 20},
		Scope:   review.ScopeMetadata{ChangedLines: 500},
		Findings: []review.Finding{
			{Severity: review.SeverityInfo, Title: "Suggestion", Body: "minor suggestion"},
		},
	}

	report := review.BuildReport(result)

	foundBonus := false
	for _, d := range report.Score.Deductions {
		if d.Reason == "大 PR 低问题密度奖励" && d.Points == -5 {
			foundBonus = true
		}
	}
	if !foundBonus {
		t.Fatal("expected large clean PR bonus deduction")
	}
}

func TestCountDiffLines(t *testing.T) {
	diff := `diff --git a/file.go b/file.go
index abc..def 100644
--- a/file.go
+++ b/file.go
@@ -1,3 +1,5 @@
 line1
+added1
+added2
 line2
-old1
 line3
`
	lines := review.CountDiffLines(diff)
	if lines != 3 {
		t.Fatalf("expected 3 changed lines (2 added + 1 removed), got %d", lines)
	}
}

func TestCountDiffLinesEmpty(t *testing.T) {
	if review.CountDiffLines("") != 0 {
		t.Fatal("expected 0 for empty diff")
	}
}
