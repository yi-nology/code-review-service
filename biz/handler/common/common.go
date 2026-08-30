package common

import (
	"errors"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/oekit/pkg/convert"
	bizerrors "github.com/oekit/pkg/errors"
	"github.com/oekit/pkg/pagination"
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
	if bizErr, ok := bizerrors.AsBizError(err); ok {
		// v0.17.0 起 BizError 拆为 BizCode（body code）+ HTTPCode（HTTP 状态，
		// 0 时经 StatusMapper 从 BizCode 推导）。ErrorRespBiz 即此语义，
		// 线上行为与旧版 ErrorResp(c, bizErr.Code, ...) 一致。
		mw.ErrorRespBiz(c, bizErr)
		return
	}
	if errors.Is(err, repoerr.ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		mw.ErrorResp(c, 404, "resource not found")
		return
	}
	// 未知错误一律脱敏：详细错误仅入日志，响应体只返回通用消息，
	// 避免向客户端泄露内部路径、SQL、堆栈等敏感信息。
	logx.Errorf("internal error: %v", err)
	mw.ErrorResp(c, 500, "internal server error")
}

// InternalErr 包装一个内部错误，返回 500 + 通用消息。
// 真实的错误细节由 Fail 统一记录到日志，不会进入 HTTP 响应体。
//
// 用于 handler 希望显式标记某次失败为内部错误（而非透传 service 层错误）的场景。
func InternalErr(err error) error {
	if err == nil {
		return nil
	}
	logx.Errorf("internal error: %v", err)
	return &AppError{Code: 500, Message: "internal server error"}
}

func Success(c *hertzApp.RequestContext, data any) {
	mw.SuccessResp(c, data)
}

func BadRequest(msg string) error {
	return bizerrors.NewBadRequestError(msg)
}

func NotFound(msg string) error {
	return bizerrors.NewNotFoundError(msg)
}

func BizErr(code int, msg string) error {
	return bizerrors.NewBizError(code, msg)
}

// PageParams 从查询字符串里解析 page / pageSize 并规范化（默认值 + 最大 pageSize 上限）。
// 委托给 ekit pagination 包，避免每个 handler 重复手写 clamp。
//
// 行为变化：maxPageSize 200 → 500（ekit pagination.MaxPageSize 默认值）。
// 历史项目上线后没见触达过 200 上限，放宽到 500 让前端做大列表请求更灵活。
func PageParams(pageStr, pageSizeStr string) (int, int) {
	pr := pagination.NewPageRequest(
		convert.Str2Int(pageStr, pagination.DefaultPage),
		convert.Str2Int(pageSizeStr, pagination.DefaultPageSize),
	)
	pr.Normalize()
	return pr.Page, pr.PageSize
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
