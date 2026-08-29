package auth_service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/domain/auth"

	"github.com/oekit/pkg/convert"

	"github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *Handler

type Handler struct {
	repo authRepo
}

// authRepo 是 handler 依赖的最小接口（避免直接 import repository 包）。
type authRepo interface {
	CreateToken(ctx context.Context, t *auth.Token) error
	ListTokens(ctx context.Context) ([]auth.Token, error)
	DeleteToken(ctx context.Context, id uint) error
	ListAudit(ctx context.Context, limit, offset int) ([]auth.AuditEntry, int64, error)
}

func InitHandler(repo authRepo) {
	handlerInst = &Handler{repo: repo}
}

type createTokenReq struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	CreatedBy string   `json:"created_by"`
}

// CreateToken 创建一个具名 API Token，返回明文 token（仅此一次）。
// 明文 token 格式：rs_<32 hex>，DB 只存 SHA256 哈希。
func CreateToken(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil {
		common.Fail(c, common.BizErr(500, "auth handler not initialized"))
		return
	}
	req, ok := common.Bind[createTokenReq](c)
	if !ok {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		common.Fail(c, common.BadRequest("name 不能为空"))
		return
	}

	// 生成明文 token：rs_ + 32 字节随机 hex（64 字符）。
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		common.Fail(c, common.BizErr(500, "生成 token 失败"))
		return
	}
	plaintext := "rs_" + hex.EncodeToString(raw)
	hash := hashToken(plaintext)

	t := &auth.Token{
		Name:      name,
		TokenHash: hash,
		Scopes:    req.Scopes,
		CreatedBy: req.CreatedBy,
	}
	if err := handlerInst.repo.CreateToken(ctx, t); err != nil {
		common.Fail(c, err)
		return
	}
	// 明文只在创建时返回一次，后续无法再获取。
	common.Success(c, map[string]any{
		"id":         t.ID,
		"name":       t.Name,
		"token":      plaintext,
		"scopes":     t.Scopes,
		"created_by": t.CreatedBy,
		"created_at": t.CreatedAt,
	})
}

func ListTokens(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil {
		common.Fail(c, common.BizErr(500, "auth handler not initialized"))
		return
	}
	tokens, err := handlerInst.repo.ListTokens(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}
	// 不返回 token_hash（敏感），只返回 name/scopes/时间。
	common.Success(c, map[string]any{"items": tokens})
}

func DeleteToken(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil {
		common.Fail(c, common.BizErr(500, "auth handler not initialized"))
		return
	}
	idStr := c.Param("id")
	id, err := common.ParseUintID(idStr)
	if err != nil {
		common.Fail(c, common.BadRequest("invalid token id"))
		return
	}
	if err := handlerInst.repo.DeleteToken(ctx, uint(id)); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"ok": true})
}

func ListAudit(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil {
		common.Fail(c, common.BizErr(500, "auth handler not initialized"))
		return
	}
	limit := convert.Str2Int(string(c.QueryArgs().Peek("limit")), 0)
	offset := convert.Str2Int(string(c.QueryArgs().Peek("offset")), 0)
	entries, total, err := handlerInst.repo.ListAudit(ctx, limit, offset)
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"items": entries, "total": total})
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
