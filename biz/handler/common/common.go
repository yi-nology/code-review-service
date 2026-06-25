package common

import (
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/oekit/pkg/convert"
	"github.com/oekit/pkg/mathx"
	"github.com/oekit/pkg/ptrx"

	mw "github.com/oekit/middleware"

	"github.com/oekit/logx"
	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

type AppError struct {
	Code    int
	Message string
}

func (e *AppError) Error() string {
	return e.Message
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
	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		mw.ErrorResp(c, 404, "resource not found")
		return
	}
	logx.Errorf("internal error: %v", err)
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

// Bind 绑定并验证请求参数到泛型类型 T。
// 失败时自动返回 400 错误响应，调用方无需处理 error。
//
// 用法:
//
//	req, ok := common.Bind[model.CreateReq](c)
//	if !ok { return }
func Bind[T any](c *hertzApp.RequestContext) (*T, bool) {
	var req T
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, BadRequest(err.Error()))
		return nil, false
	}
	return &req, true
}

// FeatureGuard 检查可选功能是否已配置。未配置时返回 400。
//
// 用法:
//
//	if !common.FeatureGuard(c, handlerInst.viewerMgr, "Viewer") { return }
func FeatureGuard(c *hertzApp.RequestContext, feature any, name string) bool {
	if feature == nil {
		Fail(c, BadRequest(name+" 功能未配置"))
		return false
	}
	return true
}
