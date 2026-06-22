package review_map_service

import (
	"context"
	"encoding/json"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelReviewMap "github.com/yi-nology/code-review-service/biz/model/review_map"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/reviewmap"
	"github.com/yi-nology/code-review-service/internal/service/session"

	"github.com/oekit/pkg/slicex"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/datatypes"
)

var handlerInst *ReviewMapHandler

type ReviewMapHandler struct {
	sessionService session.Service
	reviewMapRepo  repository.ReviewMapRepository
	mapGenerator   *reviewmap.Generator
}

func InitHandler(sessionService session.Service, reviewMapRepo repository.ReviewMapRepository) {
	handlerInst = &ReviewMapHandler{
		sessionService: sessionService,
		reviewMapRepo:  reviewMapRepo,
		mapGenerator:   reviewmap.NewGenerator(),
	}
}

func GenerateMap(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReviewMap.GenerateMapReq](c)
	if !ok {
		return
	}

	reviewID := req.ID
	if req.ReviewID != nil && *req.ReviewID != "" {
		reviewID = *req.ReviewID
	}

	sess, err := handlerInst.sessionService.GetSession(ctx, req.SessionID)
	if err != nil {
		hlog.Errorf("get session failed: %v", err)
		common.Fail(c, common.NotFound("session not found"))
		return
	}

	var findings []reviewmap.Finding
	if sess.AgentResults != nil {
		findings = convertSessionFindings(sess)
	}

	reviewMap, err := handlerInst.mapGenerator.Generate(req.SessionID, req.Diff, findings)
	if err != nil {
		hlog.Errorf("generate review map failed: %v", err)
		common.Fail(c, err)
		return
	}

	dbMap := &models.ReviewMap{
		MapID:          reviewMap.ID,
		ReviewID:       reviewID,
		SessionID:      req.SessionID,
		Title:          reviewMap.Title,
		Description:    reviewMap.Description,
		Sections:       marshalJSONToDatatypes(reviewMap.Sections),
		Dependencies:   marshalJSONToDatatypes(reviewMap.Dependencies),
		Summary:        reviewMap.Summary,
		FileCount:      reviewMap.FileCount,
		SectionCount:   reviewMap.SectionCount,
		MermaidDiagram: reviewMap.ToMermaid(),
	}

	if err := handlerInst.reviewMapRepo.Create(ctx, dbMap); err != nil {
		hlog.Errorf("save review map failed: %v", err)
	}

	common.Success(c, reviewMap)
}

func GetMaps(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReviewMap.GetMapsReq](c)
	if !ok {
		return
	}

	maps, err := handlerInst.reviewMapRepo.GetByReviewID(ctx, req.ID)
	if err != nil {
		hlog.Errorf("get review maps failed: %v", err)
		common.Fail(c, err)
		return
	}

	result := slicex.Map(maps, func(m models.ReviewMap) *modelReviewMap.ReviewMapSummary {
		return &modelReviewMap.ReviewMapSummary{
			ID:           m.MapID,
			ReviewID:     m.ReviewID,
			SessionID:    m.SessionID,
			Title:        m.Title,
			Description:  m.Description,
			Summary:      m.Summary,
			FileCount:    int64(m.FileCount),
			SectionCount: int64(m.SectionCount),
			CreatedAt:    common.FormatTime(m.CreatedAt),
		}
	})

	common.Success(c, &modelReviewMap.GetMapsResp{
		Maps:  result,
		Total: int64(len(result)),
	})
}

func GetMap(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReviewMap.GetMapReq](c)
	if !ok {
		return
	}

	m, err := handlerInst.reviewMapRepo.GetByMapID(ctx, req.MapID)
	if err != nil {
		hlog.Errorf("get review map failed: %v", err)
		common.Fail(c, err)
		return
	}
	if m == nil {
		common.Fail(c, common.NotFound("map not found"))
		return
	}

	var sections []interface{}
	var dependencies []interface{}
	_ = json.Unmarshal(m.Sections, &sections)
	_ = json.Unmarshal(m.Dependencies, &dependencies)

	common.Success(c, map[string]interface{}{
		"id":            m.MapID,
		"review_id":     m.ReviewID,
		"session_id":    m.SessionID,
		"title":         m.Title,
		"description":   m.Description,
		"sections":      sections,
		"dependencies":  dependencies,
		"summary":       m.Summary,
		"file_count":    m.FileCount,
		"section_count": m.SectionCount,
		"mermaid":       m.MermaidDiagram,
		"created_at":    m.CreatedAt,
	})
}

func GetSessionMap(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReviewMap.GetSessionMapReq](c)
	if !ok {
		return
	}

	m, err := handlerInst.reviewMapRepo.GetBySessionID(ctx, req.SessionID)
	if err != nil {
		hlog.Errorf("get session map failed: %v", err)
		common.Fail(c, err)
		return
	}
	if m == nil {
		common.Fail(c, common.NotFound("map not found for this session"))
		return
	}

	var sections []interface{}
	var dependencies []interface{}
	_ = json.Unmarshal(m.Sections, &sections)
	_ = json.Unmarshal(m.Dependencies, &dependencies)

	common.Success(c, map[string]interface{}{
		"id":            m.MapID,
		"review_id":     m.ReviewID,
		"session_id":    m.SessionID,
		"title":         m.Title,
		"description":   m.Description,
		"sections":      sections,
		"dependencies":  dependencies,
		"summary":       m.Summary,
		"file_count":    m.FileCount,
		"section_count": m.SectionCount,
		"mermaid":       m.MermaidDiagram,
		"created_at":    m.CreatedAt,
	})
}

func PreviewMap(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelReviewMap.PreviewMapReq](c)
	if !ok {
		return
	}

	reviewMap, err := handlerInst.mapGenerator.Generate("preview", req.Diff, nil)
	if err != nil {
		hlog.Errorf("generate preview map failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, reviewMap)
}

func convertSessionFindings(sess *models.ReviewSession) []reviewmap.Finding {
	var findings []reviewmap.Finding

	if sess.AgentResults == nil {
		return findings
	}

	var agentResults []models.AgentResultData
	if err := json.Unmarshal(sess.AgentResults, &agentResults); err != nil {
		hlog.Warnf("failed to parse agent results: %v", err)
		return findings
	}

	for _, agentResult := range agentResults {
		for _, f := range agentResult.Findings {
			findings = append(findings, reviewmap.Finding{
				ID:         f.ID,
				Severity:   f.Severity,
				Category:   f.Category,
				Path:       f.Path,
				Line:       f.Line,
				EndLine:    f.EndLine,
				Title:      f.Title,
				Body:       f.Body,
				Suggestion: f.Suggestion,
				Confidence: f.Confidence,
				Agents:     f.Agents,
			})
		}
	}

	return findings
}

func marshalJSONToDatatypes(data interface{}) datatypes.JSON {
	result, err := json.Marshal(data)
	if err != nil {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(result)
}

type UpdateFileReviewedRequest struct {
	Reviewed bool `json:"reviewed"`
}

func UpdateFileReviewed(ctx context.Context, c *app.RequestContext) {
	reviewID := c.Param("id")
	mapID := c.Param("mapId")
	fileID := c.Param("fileId")

	// Decode file path (may contain encoded slashes)
	filePath := fileID

	req, ok := common.Bind[UpdateFileReviewedRequest](c)
	if !ok {
		return
	}

	// Verify map exists and belongs to review
	m, err := handlerInst.reviewMapRepo.GetByMapID(ctx, mapID)
	if err != nil || m == nil {
		common.Fail(c, common.NotFound("map not found"))
		return
	}
	if m.ReviewID != reviewID {
		common.Fail(c, common.BadRequest("map does not belong to this review"))
		return
	}

	if err := handlerInst.reviewMapRepo.UpdateFileReviewed(ctx, mapID, filePath, req.Reviewed); err != nil {
		hlog.Errorf("update file reviewed failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, map[string]string{"status": "ok"})
}

type UpdateSectionProgressRequest struct {
	Reviewed bool `json:"reviewed"`
}

func UpdateSectionProgress(ctx context.Context, c *app.RequestContext) {
	reviewID := c.Param("id")
	mapID := c.Param("mapId")
	sectionID := c.Param("sectionId")

	req, ok := common.Bind[UpdateSectionProgressRequest](c)
	if !ok {
		return
	}

	// Verify map exists and belongs to review
	m, err := handlerInst.reviewMapRepo.GetByMapID(ctx, mapID)
	if err != nil || m == nil {
		common.Fail(c, common.NotFound("map not found"))
		return
	}
	if m.ReviewID != reviewID {
		common.Fail(c, common.BadRequest("map does not belong to this review"))
		return
	}

	if err := handlerInst.reviewMapRepo.UpdateSectionFilesReviewed(ctx, mapID, sectionID, req.Reviewed); err != nil {
		hlog.Errorf("update section progress failed: %v", err)
		common.Fail(c, err)
		return
	}

	common.Success(c, map[string]string{"status": "ok"})
}
