package review_service

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelReview "github.com/yi-nology/code-review-service/biz/model/review"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/ptrx"
	"github.com/oekit/pkg/slicex"
)

// convert.go：domain ↔ IDL 转换与错误映射（纯函数，端点层的序列化边界）。
var handlerInst *ReviewHandler

var webhookInst *WebhookHandler

func mapReviewError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnsupportedProvider):
		return common.BizErr(422, err.Error())
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return common.BizErr(409, err.Error())
	case ocr.IsBinaryMissing(err):
		return common.BizErr(400, err.Error())
	case ocr.IsTimeout(err):
		return common.BizErr(504, err.Error())
	case ocr.IsExit(err):
		return common.BizErr(502, err.Error())
	default:
		return err
	}
}

func toServiceRequest(req *modelReview.CreateReviewReq) domain.Request {
	svcReq := domain.Request{
		Provider:        req.Provider,
		Repo:            req.Repo,
		ProjectID:       ptrx.Deref(req.ProjectID, ""),
		PullRequestID:   ptrx.Deref(req.PullRequestID, ""),
		MergeRequestIID: ptrx.Deref(req.MergeRequestIid, ""),
		BaseSHA:         ptrx.Deref(req.BaseSha, ""),
		HeadSHA:         ptrx.Deref(req.HeadSha, ""),
		CommitSHA:       ptrx.Deref(req.CommitSha, ""),
		CallbackMode:    ptrx.Deref(req.CallbackMode, ""),
		Wait:            ptrx.Deref(req.Wait, false),
		Requirements:    req.Requirements,
		Engine:          ptrx.Deref(req.Engine, ""),
		TaskKind:        ptrx.Deref(req.TaskKind, ""),
	}
	if len(req.EphemeralReviewers) > 0 {
		svcReq.EphemeralReviewers = make([]domain.EphemeralReviewer, len(req.EphemeralReviewers))
		for i, e := range req.EphemeralReviewers {
			svcReq.EphemeralReviewers[i] = domain.EphemeralReviewer{Name: ptrx.Deref(e.Name, ""), Background: e.Background}
		}
	}
	return svcReq
}

func convertGate(g domain.Gate) *modelReview.Gate {
	return &modelReview.Gate{Status: g.Status, Passed: g.Passed, Reason: common.StrPtr(g.Reason)}
}

func convertResponse(r domain.Response) modelReview.ReviewResponse {
	return modelReview.ReviewResponse{
		ReviewID: r.ReviewID, Status: r.Status, Summary: r.Summary,
		Gate: convertGate(r.Gate), Report: convertReport(r.Report),
		Analysis: convertAnalysis(r.Analysis), History: convertEvents(r.History),
		Findings: convertFindings(r.Findings), Error: common.StrPtr(r.Error),
	}
}

func convertDetail(r domain.Record) modelReview.ReviewDetail {
	return modelReview.ReviewDetail{
		ReviewID: r.ID, Provider: r.Request.Provider, Repo: r.Request.Repo,
		ProjectID: common.StrPtr(r.Request.ProjectID), PullRequestID: common.StrPtr(r.Request.PullRequestID),
		MergeRequestIid: common.StrPtr(r.Request.MergeRequestIID), BaseSha: common.StrPtr(r.Request.BaseSHA),
		HeadSha: common.StrPtr(r.Request.HeadSHA), CallbackMode: common.StrPtr(r.Request.CallbackMode),
		Status: r.Response.Status, Summary: r.Response.Summary,
		Gate: convertGate(r.Response.Gate), Analysis: convertAnalysis(r.Response.Analysis),
		History: convertEvents(r.Response.History), Findings: convertFindings(r.Response.Findings),
		Report: convertReport(r.Response.Report), RawOcr: common.StrPtr(string(r.RawOCR)),
		RawOcrInput: convertOCRInput(r.RawOCRInput),

		Error: common.StrPtr(r.Response.Error), Author: common.StrPtr(r.Request.Author),
		AuthorName: common.StrPtr(r.Request.AuthorName), AuthorAvatar: common.StrPtr(r.Request.AuthorAvatar),
		RetryCount: ptrx.Of(int64(r.RetryCount)),
		// 生效引擎（创建时已解析并随 Request 持久化；空值视为 ocr 以兼容历史记录）。
		Engine:    common.StrPtr(r.Request.Engine),
		TaskKind:  common.StrPtr(r.Request.TaskKind),
		CreatedAt: common.StrPtr(common.FormatTime(r.CreatedAt)),
		UpdatedAt: common.StrPtr(common.FormatTime(r.UpdatedAt)),
	}
}

func convertOCRInput(raw json.RawMessage) *modelReview.OCRInput {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	item := &modelReview.OCRInput{}
	if err := json.Unmarshal(raw, item); err != nil {
		return nil
	}
	return item
}

func convertReport(r domain.Report) *modelReview.Report {
	return &modelReview.Report{
		Ocr: convertOCRMetadata(r.OCR), Scope: convertScopeMetadata(r.Scope),
		Score: &modelReview.Score{
			Value: int64(r.Score.Value), Grade: r.Score.Grade, Version: r.Score.Version,
			Deductions:   convertDeductions(r.Score.Deductions),
			ChangedLines: ptrx.Of(int64(r.Score.ChangedLines)), FindingsDensity: ptrx.Of(r.Score.FindingsDensity),
		},
		Counts: &modelReview.SeverityCounts{
			Critical: int64(r.Counts.Critical), Major: int64(r.Counts.Major),
			Minor: int64(r.Counts.Minor), Info: int64(r.Counts.Info),
		},
		Generated: ptrx.Of(r.Generated), Complete: ptrx.Of(r.Complete),
	}
}

func convertOCRMetadata(o domain.OCRMetadata) *modelReview.OCRMetadata {
	item := &modelReview.OCRMetadata{
		Engine: common.StrPtr(o.Engine), Status: common.StrPtr(o.Status),
		FilesReviewed: ptrx.Of(o.FilesReviewed), Comments: ptrx.Of(o.Comments),
		TotalTokens: ptrx.Of(o.TotalTokens), InputTokens: ptrx.Of(o.InputTokens),
		OutputTokens: ptrx.Of(o.OutputTokens), CacheReadTokens: ptrx.Of(o.CacheReadTokens),
		CacheWriteTokens: ptrx.Of(o.CacheWriteTokens), Elapsed: common.StrPtr(o.Elapsed),
	}
	if o.ToolCalls != nil {
		item.ToolCalls = &modelReview.OCRToolCalls{
			Total:  o.ToolCalls.Total,
			ByTool: o.ToolCalls.ByTool,
		}
	}
	if strings.TrimSpace(o.ProjectSummary) != "" {
		item.ProjectSummary = common.StrPtr(o.ProjectSummary)
	}
	if o.BudgetExceeded {
		item.BudgetExceeded = ptrx.Of(true)
	}
	return item
}

func convertDeductions(ds []domain.ScoreDeduction) []*modelReview.ScoreDeduction {
	if len(ds) == 0 {
		return nil
	}
	return slicex.Map(ds, func(d domain.ScoreDeduction) *modelReview.ScoreDeduction {
		return &modelReview.ScoreDeduction{Reason: d.Reason, Points: int64(d.Points), Count: ptrx.Of(int64(d.Count))}
	})
}

func convertScopeMetadata(s domain.ScopeMetadata) *modelReview.ScopeMetadata {
	return &modelReview.ScopeMetadata{
		ChangedFiles: int64(s.ChangedFiles), ReviewedFiles: int64(s.ReviewedFiles),
		ExcludedFiles: int64(s.ExcludedFiles), IgnorePaths: s.IgnorePaths,
	}
}

func convertAnalysis(a domain.Analysis) *modelReview.Analysis {
	return &modelReview.Analysis{
		Overview: common.StrPtr(a.Overview), RootCause: a.RootCause,
		Risks: a.Risks, Recommendations: a.Recommendations,
	}
}

func convertEvents(events []domain.Event) []*modelReview.Event {
	if len(events) == 0 {
		return nil
	}
	return slicex.Map(events, func(e domain.Event) *modelReview.Event {
		details := make(map[string]string, len(e.Details))
		for k, v := range e.Details {
			details[k] = convert.AnyToStr(v)
		}
		ev := &modelReview.Event{
			Stage: e.Stage, Status: e.Status, Message: e.Message, Details: details,
			CreatedAt: common.StrPtr(common.FormatTime(e.CreatedAt)),
		}
		if e.How != "" {
			ev.How = common.StrPtr(e.How)
		}
		if e.DurationMs > 0 {
			ev.DurationMs = &e.DurationMs
		}
		if e.Input != nil {
			if data, err := json.Marshal(e.Input); err == nil {
				ev.Input = common.StrPtr(string(data))
			}
		}
		if e.Output != nil {
			if data, err := json.Marshal(e.Output); err == nil {
				ev.Output = common.StrPtr(string(data))
			}
		}
		return ev
	})
}

func convertFindings(findings []domain.Finding) []*modelReview.Finding {
	if len(findings) == 0 {
		return nil
	}
	return slicex.Map(findings, func(f domain.Finding) *modelReview.Finding {
		return &modelReview.Finding{
			Severity: f.Severity, Category: common.StrPtr(f.Category), Path: common.StrPtr(f.Path),
			Line: ptrx.Of(int64(f.Line)), StartLine: ptrx.Of(int64(f.StartLine)), EndLine: ptrx.Of(int64(f.EndLine)),
			Title: f.Title, Body: f.Body, ExistingCode: common.StrPtr(f.ExistingCode),
			SuggestionCode: common.StrPtr(f.SuggestionCode),
			Fingerprint:    common.StrPtr(f.Fingerprint),
		}
	})
}
