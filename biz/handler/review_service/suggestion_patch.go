package review_service

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/cloudwego/hertz/pkg/app"
)

func PreviewSuggestionPatch(ctx context.Context, c *app.RequestContext) {
	preview, ok := buildSuggestionPatchPreview(ctx, c)
	if !ok {
		return
	}
	common.Success(c, preview)
}

func DownloadSuggestionPatch(ctx context.Context, c *app.RequestContext) {
	preview, ok := buildSuggestionPatchPreview(ctx, c)
	if !ok {
		return
	}
	filename := fmt.Sprintf("%s-suggestion.patch", preview.ReviewID)
	c.Response.Header.Set("Content-Type", "text/x-diff; charset=utf-8")
	c.Response.Header.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.SetStatusCode(http.StatusOK)
	_, _ = c.WriteString(preview.Patch)
}

func CheckSuggestionPatch(ctx context.Context, c *app.RequestContext) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.Fail(c, common.BadRequest("review id 不能为空"))
		return
	}
	record, found, err := handlerInst.reviewSvc.Get(ctx, id)
	if err != nil {
		common.Fail(c, err)
		return
	}
	if !found {
		common.Fail(c, common.NotFound("review not found"))
		return
	}
	indexes, err := parseFindingIndexes(c.Query("finding_indexes"), len(record.Response.Findings))
	if err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	result, found, err := handlerInst.reviewSvc.CheckSuggestionPatch(ctx, id, indexes)
	if err != nil {
		common.Fail(c, err)
		return
	}
	if !found {
		common.Fail(c, common.NotFound("review not found"))
		return
	}
	common.Success(c, result)
}

type applySuggestionPatchReq struct {
	FindingIndexes []int  `json:"finding_indexes"`
	Branch         string `json:"branch"`
	Push           bool   `json:"push"`
}

func ApplySuggestionPatch(ctx context.Context, c *app.RequestContext) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.Fail(c, common.BadRequest("review id 不能为空"))
		return
	}
	req, ok := common.Bind[applySuggestionPatchReq](c)
	if !ok {
		return
	}
	record, found, err := handlerInst.reviewSvc.Get(ctx, id)
	if err != nil {
		common.Fail(c, err)
		return
	}
	if !found {
		common.Fail(c, common.NotFound("review not found"))
		return
	}
	if err := validateFindingIndexes(req.FindingIndexes, len(record.Response.Findings)); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	result, found, err := handlerInst.reviewSvc.ApplySuggestionPatch(ctx, id, review.SuggestionPatchApplyOptions{
		Indexes: req.FindingIndexes,
		Branch:  req.Branch,
		Push:    req.Push,
	})
	if err != nil {
		common.Fail(c, err)
		return
	}
	if !found {
		common.Fail(c, common.NotFound("review not found"))
		return
	}
	common.Success(c, result)
}

func buildSuggestionPatchPreview(ctx context.Context, c *app.RequestContext) (review.SuggestionPatchPreview, bool) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.Fail(c, common.BadRequest("review id 不能为空"))
		return review.SuggestionPatchPreview{}, false
	}
	record, found, err := handlerInst.reviewSvc.Get(ctx, id)
	if err != nil {
		common.Fail(c, err)
		return review.SuggestionPatchPreview{}, false
	}
	if !found {
		common.Fail(c, common.NotFound("review not found"))
		return review.SuggestionPatchPreview{}, false
	}
	indexes, err := parseFindingIndexes(c.Query("finding_indexes"), len(record.Response.Findings))
	if err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return review.SuggestionPatchPreview{}, false
	}
	return review.BuildSuggestionPatch(record.ID, record.Response.Findings, indexes), true
}

func validateFindingIndexes(indexes []int, total int) error {
	seen := map[int]struct{}{}
	for _, idx := range indexes {
		if idx < 0 || idx >= total {
			return fmt.Errorf("finding index %d out of range", idx)
		}
		if _, ok := seen[idx]; ok {
			return fmt.Errorf("finding index %d duplicated", idx)
		}
		seen[idx] = struct{}{}
	}
	return nil
}

func parseFindingIndexes(raw string, total int) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	indexes := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("finding_indexes 包含空值")
		}
		idx, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("finding_indexes 只能包含数字")
		}
		if idx < 0 || idx >= total {
			return nil, fmt.Errorf("finding index %d out of range", idx)
		}
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		indexes = append(indexes, idx)
	}
	return indexes, nil
}
