package review

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/service/reviewmap"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"fmt"
)

func (s *Service) recordFailure(ctx context.Context, id string, req Request, runErr error) {
	response := Response{
		ReviewID: id,
		Status:   "failed",
		Summary:  "review failed",
		Gate:     ErrorGate("review execution failed"),
		Error:    runErr.Error(),
	}
	if err := s.updateWithRawOCR(ctx, id, req, response, nil); err != nil {
		hlog.Error("failed to persist review failure", "review_id", id, "error", err)
	}
}

func (s *Service) generateAndSaveReviewMap(ctx context.Context, id string, req Request, workspace Workspace, result Result) {
	var findings []reviewmap.Finding
	for i, f := range result.Findings {
		findings = append(findings, reviewmap.Finding{
			ID:         fmt.Sprintf("finding-%d", i),
			Severity:   string(f.Severity),
			Category:   f.Category,
			Path:       f.Path,
			Line:       f.Line,
			EndLine:    f.EndLine,
			Title:      f.Title,
			Body:       f.Body,
			Suggestion: f.SuggestionCode,
			Confidence: 0.8,
			Agents:     []string{"default"},
		})
	}

	generator := reviewmap.NewGenerator()
	reviewMap, err := generator.Generate(id, workspace.Diff, findings)
	if err != nil {
		hlog.Warn("failed to generate review map", "review_id", id, "error", err)
		return
	}

	dbMap := &models.ReviewMap{
		MapID:          reviewMap.ID,
		ReviewID:       id,
		SessionID:      id,
		Title:          reviewMap.Title,
		Description:    reviewMap.Description,
		Sections:       mustMarshal(reviewMap.Sections),
		Dependencies:   mustMarshal(reviewMap.Dependencies),
		Summary:        reviewMap.Summary,
		FileCount:      reviewMap.FileCount,
		SectionCount:   reviewMap.SectionCount,
		MermaidDiagram: reviewMap.ToMermaid(),
	}

	if err := s.reviewMapRepo.Create(ctx, dbMap); err != nil {
		hlog.Warn("failed to save review map", "review_id", id, "error", err)
		return
	}

	hlog.Info("review map generated and saved", "review_id", id, "map_id", reviewMap.ID, "sections", reviewMap.SectionCount, "files", reviewMap.FileCount)
}
