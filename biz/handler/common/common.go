package common

import (
	"errors"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/mathx"
	"github.com/oekit/pkg/ptrx"

	mw "github.com/oekit/middleware"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

func Fail(c *hertzApp.RequestContext, err error) {
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

func Success(c *hertzApp.RequestContext, data any) {
	mw.SuccessResp(c, data)
}

func BadRequest(msg string) error {
	return mw.NewBadRequestError(msg)
}

func NotFound(msg string) error {
	return mw.NewNotFoundError(msg)
}

func BizErr(code int, msg string) error {
	return mw.NewBizError(code, msg)
}

func PageParams(pageStr, pageSizeStr string) (int, int) {
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

func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

func MaskWebhookSecret(secret string) *string {
	if secret == "" {
		return nil
	}
	return ptrx.Of("***")
}

func StrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

const TimeFormat = "2006-01-02T15:04:05Z"
