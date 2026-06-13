package review_service

import (
	"errors"
	"strconv"
	"strings"

	"github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/pkg/ptr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/review"

	mw "github.com/oekit/middleware"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

type ReviewHandler struct {
	reviewSvc       app.ReviewService
	ocrRepo         repository.OCRSettingsRepository
	project         repository.ProjectPolicyRepository
	platform        repository.GitPlatformRepository
	globalRules     repository.GlobalRulesRepository
	gitMgr          app.GitProviderManager
	llmTester       review.LLMTester
}

func NewReviewHandler(
	svc app.ReviewService,
	ocrRepo repository.OCRSettingsRepository,
	project repository.ProjectPolicyRepository,
	platform repository.GitPlatformRepository,
	globalRules repository.GlobalRulesRepository,
	gitMgr app.GitProviderManager,
	llmTester review.LLMTester,
) *ReviewHandler {
	return &ReviewHandler{
		reviewSvc:   svc,
		ocrRepo:    ocrRepo,
		project:    project,
		platform:   platform,
		globalRules: globalRules,
		gitMgr:     gitMgr,
		llmTester:  llmTester,
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
	return &mw.BizError{Code: 400, Message: msg}
}

func notFound(msg string) error {
	return &mw.BizError{Code: 404, Message: msg}
}

func bizErr(code int, msg string) error {
	return &mw.BizError{Code: code, Message: msg}
}

func pageParams(pageStr, pageSizeStr string) (int, int) {
	const maxPageSize = 200
	page, _ := strconv.Atoi(pageStr)
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
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
	return strPtr("***")
}

func derefStr(s *string) string      { return ptr.DerefStr(s) }
func derefBool(b *bool) bool         { return ptr.DerefBool(b) }
func derefInt64(i *int64) int64      { return ptr.DerefInt64(i) }
func strPtr(s string) *string        { if s == "" { return nil }; return &s }
func boolPtr(b bool) *bool           { return &b }
func int64Ptr(i int64) *int64        { return &i }

func cleanQuery(s string) string {
	return strings.TrimSpace(s)
}

const timeFormat = "2006-01-02T15:04:05Z"
