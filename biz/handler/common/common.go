package common

import (
	"errors"
	"fmt"
	"time"

	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/mathx"
	"github.com/oekit/pkg/ptrx"

	mw "github.com/oekit/middleware"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

type AppError struct {
	Code    int
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(code int, msg string) *AppError {
	return &AppError{Code: code, Message: msg}
}

func Fail(c *hertzApp.RequestContext, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		mw.ErrorResp(c, appErr.Code, appErr.Message)
		return
	}
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

// MaskToken 脱敏 token，保留首尾各 4 字符。
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// MaskSecret 通用脱敏，保留首尾各 visible 字符。
func MaskSecret(val string, visible int) *string {
	if val == "" {
		return nil
	}
	if len(val) <= visible*2 {
		return ptrx.Of("***")
	}
	return ptrx.Of(val[:visible] + "****" + val[len(val)-visible:])
}

// MaskWebhookSecret 脱敏 webhook secret。
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

func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(TimeFormat)
}

// ParseUintID 将字符串解析为 uint ID。无效时返回 error。
func ParseUintID(s string) (uint, error) {
	id := convert.Str2Int(s, -1)
	if id < 0 {
		return 0, fmt.Errorf("invalid id: %s", s)
	}
	return uint(id), nil
}

// SafeInt64 将 uint 安全转换为 int64，溢出时返回 MaxInt64。
func SafeInt64(v uint) int64 {
	const maxInt64 = ^uint(0) >> 1
	if v > maxInt64 {
		return int64(maxInt64)
	}
	return int64(v)
}

// SafeUint 将 int64 安全转换为 uint，负数时返回 0。
func SafeUint(v int64) uint {
	if v < 0 {
		return 0
	}
	return uint(v)
}
