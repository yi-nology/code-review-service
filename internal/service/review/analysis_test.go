package review

import (
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/review"
)

func TestBuildAnalysis_WithTechLeadAnalysis(t *testing.T) {
	result := domain.Result{
		Summary: "irrelevant",
		OCR:     domain.OCRMetadata{FilesReviewed: 5},
		Findings: []domain.Finding{
			{Path: "a.go", Title: "issue1", Severity: domain.SeverityMajor, Body: "body1"},
		},
		TechLeadAnalysis: &domain.TechLeadAnalysis{
			Summary:         "TL 总结：3 个核心问题",
			Risks:           []string{"并发安全"},
			Recommendations: []string{"使用 sync.Mutex"},
		},
	}
	got := BuildAnalysis(result)
	if got.Overview != "TL 总结：3 个核心问题" {
		t.Fatalf("expected TechLead.Summary to win, got %q", got.Overview)
	}
	if len(got.Risks) != 1 || got.Risks[0] != "并发安全" {
		t.Fatalf("expected TL risks to propagate, got %v", got.Risks)
	}
	if len(got.Recommendations) != 1 || got.Recommendations[0] != "使用 sync.Mutex" {
		t.Fatalf("expected TL recommendations to propagate, got %v", got.Recommendations)
	}
}

func TestBuildAnalysis_TechLeadEmptySummaryFallsBack(t *testing.T) {
	result := domain.Result{
		OCR:      domain.OCRMetadata{FilesReviewed: 3},
		Findings: []domain.Finding{{Path: "a.go", Title: "x", Severity: domain.SeverityMinor, Body: "y"}},
		TechLeadAnalysis: &domain.TechLeadAnalysis{
			Summary:         "", // 空 overview
			Risks:           []string{"r"},
			Recommendations: []string{"fix"},
		},
	}
	got := BuildAnalysis(result)
	if got.Overview == "" {
		t.Fatalf("expected fallback overview when TL.Summary empty")
	}
	if !contains(got.Risks, "r") {
		t.Fatalf("expected TL risks to still propagate, got %v", got.Risks)
	}
}

func TestBuildAnalysis_NoTechLeadUsesRawOCR(t *testing.T) {
	result := domain.Result{
		OCR:     domain.OCRMetadata{FilesReviewed: 1},
		Summary: "runner summary",
	}
	got := BuildAnalysis(result)
	if got.Overview != "代码评审未发现需要报告的代码问题。" {
		t.Fatalf("expected fallback for empty findings, got %q", got.Overview)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
