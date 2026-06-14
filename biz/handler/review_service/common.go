package review_service

import (
	"errors"
	"strings"

	"github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/mathx"
	"github.com/oekit/pkg/ptrx"

	mw "github.com/oekit/middleware"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

type ReviewHandler struct {
	reviewSvc   app.ReviewService
	ocrRepo     repository.OCRSettingsRepository
	project     repository.ProjectPolicyRepository
	platform    repository.GitPlatformRepository
	globalRules repository.GlobalRulesRepository
	agentRepo   repository.ReviewerAgentRepository
	gitMgr      app.GitProviderManager
	llmTester   review.LLMTester
}

func NewReviewHandler(
	svc app.ReviewService,
	ocrRepo repository.OCRSettingsRepository,
	project repository.ProjectPolicyRepository,
	platform repository.GitPlatformRepository,
	globalRules repository.GlobalRulesRepository,
	agentRepo repository.ReviewerAgentRepository,
	gitMgr app.GitProviderManager,
	llmTester review.LLMTester,
) *ReviewHandler {
	return &ReviewHandler{
		reviewSvc:   svc,
		ocrRepo:     ocrRepo,
		project:     project,
		platform:    platform,
		globalRules: globalRules,
		agentRepo:   agentRepo,
		gitMgr:      gitMgr,
		llmTester:   llmTester,
	}
}

func fail(c *hertzApp.RequestContext, err error) {
	var bizErr *mw.BizError
	if errors.As(err, &bizErr) {
		mw.ErrorResp(c, bizErr.Code, bizErr.Message)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		mw.ErrorResp(c, 404, "resource not found")
		return
	}
	hlog.Errorf("internal error: %v", err)
	mw.ErrorResp(c, 500, err.Error())
}

func badRequest(msg string) error {
	return mw.NewBadRequestError(msg)
}

func notFound(msg string) error {
	return mw.NewNotFoundError(msg)
}

func bizErr(code int, msg string) error {
	return mw.NewBizError(code, msg)
}

func pageParams(pageStr, pageSizeStr string) (int, int) {
	const (
		defaultPage     = 1
		defaultPageSize = 20
		maxPageSize     = 200
	)
	page := convert.Str2Int(pageStr, defaultPage)
	if page <= 0 {
		page = defaultPage
	}
	pageSize := convert.Str2Int(pageSizeStr, defaultPageSize)
	pageSize = mathx.Clamp(pageSize, 1, maxPageSize)
	return page, pageSize
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

func maskWebhookSecret(secret string) *string {
	if secret == "" {
		return nil
	}
	return ptrx.Of("***")
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func cleanQuery(s string) string {
	return strings.TrimSpace(s)
}

func toInt64Ptr(v int64) *int64   { return ptrx.Of(v) }
func toFloat64Ptr(v float64) *float64 { return ptrx.Of(v) }

const timeFormat = "2006-01-02T15:04:05Z"
