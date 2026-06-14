package review_service

import (
	"fmt"

	modelReview "github.com/yi-nology/code-review-service/biz/model/review"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"
)

func convertGate(g review.Gate) *modelReview.Gate {
	return &modelReview.Gate{
		Status: g.Status,
		Passed: g.Passed,
		Reason: strPtr(g.Reason),
	}
}

func convertResponse(r review.Response) modelReview.ReviewResponse {
	return modelReview.ReviewResponse{
		ReviewID: r.ReviewID,
		Status:   r.Status,
		Summary:  r.Summary,
		Gate:     convertGate(r.Gate),
		Report:   convertReport(r.Report),
		Analysis: convertAnalysis(r.Analysis),
		History:  convertEvents(r.History),
		Findings: convertFindings(r.Findings),
		Error:    strPtr(r.Error),
	}
}

func convertDetail(r review.Record) modelReview.ReviewDetail {
	return modelReview.ReviewDetail{
		ReviewID:        r.ID,
		Provider:        r.Request.Provider,
		Repo:            r.Request.Repo,
		ProjectID:       strPtr(r.Request.ProjectID),
		PullRequestID:   strPtr(r.Request.PullRequestID),
		MergeRequestIid: strPtr(r.Request.MergeRequestIID),
		BaseSha:         strPtr(r.Request.BaseSHA),
		HeadSha:         strPtr(r.Request.HeadSHA),
		CallbackMode:    strPtr(r.Request.CallbackMode),
		Status:          r.Response.Status,
		Summary:         r.Response.Summary,
		Gate:            convertGate(r.Response.Gate),
		Analysis:        convertAnalysis(r.Response.Analysis),
		History:         convertEvents(r.Response.History),
		Findings:        convertFindings(r.Response.Findings),
		Report:          convertReport(r.Response.Report),
		RawOcr:          strPtr(string(r.RawOCR)),
		Error:           strPtr(r.Response.Error),
		Author:          strPtr(r.Request.Author),
		AuthorName:      strPtr(r.Request.AuthorName),
		AuthorAvatar:    strPtr(r.Request.AuthorAvatar),
		CreatedAt:       strPtr(r.CreatedAt.Format(timeFormat)),
		UpdatedAt:       strPtr(r.UpdatedAt.Format(timeFormat)),
	}
}

func convertReport(r review.Report) *modelReview.Report {
	return &modelReview.Report{
		Ocr:   convertOCRMetadata(r.OCR),
		Scope: convertScopeMetadata(r.Scope),
		Score: &modelReview.Score{
			Value:           int64(r.Score.Value),
			Grade:           r.Score.Grade,
			Version:         r.Score.Version,
			ChangedLines:    toInt64Ptr(int64(r.Score.ChangedLines)),
			FindingsDensity: toFloat64Ptr(r.Score.FindingsDensity),
		},
		Counts: &modelReview.SeverityCounts{
			Critical: int64(r.Counts.Critical),
			Major:    int64(r.Counts.Major),
			Minor:    int64(r.Counts.Minor),
			Info:     int64(r.Counts.Info),
		},
		Generated: ptrx.Of(r.Generated),
		Complete:  ptrx.Of(r.Complete),
	}
}

func convertOCRMetadata(o review.OCRMetadata) *modelReview.OCRMetadata {
	return &modelReview.OCRMetadata{
		Engine:           strPtr(o.Engine),
		Status:           strPtr(o.Status),
		FilesReviewed:    ptrx.Of(o.FilesReviewed),
		Comments:         ptrx.Of(o.Comments),
		TotalTokens:      ptrx.Of(o.TotalTokens),
		InputTokens:      ptrx.Of(o.InputTokens),
		OutputTokens:     ptrx.Of(o.OutputTokens),
		CacheReadTokens:  ptrx.Of(o.CacheReadTokens),
		CacheWriteTokens: ptrx.Of(o.CacheWriteTokens),
		Elapsed:          strPtr(o.Elapsed),
	}
}

func convertScopeMetadata(s review.ScopeMetadata) *modelReview.ScopeMetadata {
	return &modelReview.ScopeMetadata{
		ChangedFiles:  int64(s.ChangedFiles),
		ReviewedFiles: int64(s.ReviewedFiles),
		ExcludedFiles: int64(s.ExcludedFiles),
		IgnorePaths:   s.IgnorePaths,
	}
}

func convertAnalysis(a review.Analysis) *modelReview.Analysis {
	return &modelReview.Analysis{
		Overview:        strPtr(a.Overview),
		RootCause:       a.RootCause,
		Risks:           a.Risks,
		Recommendations: a.Recommendations,
	}
}

func convertEvents(events []review.Event) []*modelReview.Event {
	if len(events) == 0 {
		return nil
	}
	return slicex.Map(events, func(e review.Event) *modelReview.Event {
		details := make(map[string]string, len(e.Details))
		for k, v := range e.Details {
			details[k] = fmt.Sprintf("%v", v)
		}
		return &modelReview.Event{
			Stage:     e.Stage,
			Status:    e.Status,
			Message:   e.Message,
			Details:   details,
			CreatedAt: strPtr(e.CreatedAt.Format(timeFormat)),
		}
	})
}

func convertFindings(findings []review.Finding) []*modelReview.Finding {
	if len(findings) == 0 {
		return nil
	}
	return slicex.Map(findings, func(f review.Finding) *modelReview.Finding {
		return &modelReview.Finding{
			Severity:       string(f.Severity),
			Category:       strPtr(f.Category),
			Path:           strPtr(f.Path),
			Line:           ptrx.Of(int64(f.Line)),
			StartLine:      ptrx.Of(int64(f.StartLine)),
			EndLine:        ptrx.Of(int64(f.EndLine)),
			Title:          f.Title,
			Body:           f.Body,
			ExistingCode:   strPtr(f.ExistingCode),
			SuggestionCode: strPtr(f.SuggestionCode),
		}
	})
}
