package review_service

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/reviewmap"
	"github.com/yi-nology/code-review-service/internal/service/session"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ReviewMapHandler handles review map related HTTP requests
type ReviewMapHandler struct {
	sessionService session.Service
	reviewMapRepo  repository.ReviewMapRepository
	mapGenerator   *reviewmap.Generator
}

// NewReviewMapHandler creates a new review map handler
func NewReviewMapHandler(sessionService session.Service, reviewMapRepo repository.ReviewMapRepository) *ReviewMapHandler {
	return &ReviewMapHandler{
		sessionService: sessionService,
		reviewMapRepo:  reviewMapRepo,
		mapGenerator:   reviewmap.NewGenerator(),
	}
}

// GenerateMapRequest represents the request to generate a review map
type GenerateMapRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Diff      string `json:"diff" binding:"required"`
	ReviewID  string `json:"review_id"`
}

// GenerateMap generates a review map for a session
func (h *ReviewMapHandler) GenerateMap(ctx context.Context, c *app.RequestContext) {
	reviewID := c.Param("id")
	if reviewID == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "review ID is required",
		})
		return
	}

	var req GenerateMapRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	// Use review ID from URL if not in request
	if req.ReviewID == "" {
		req.ReviewID = reviewID
	}

	// Get session to retrieve findings
	sess, err := h.sessionService.GetSession(ctx, req.SessionID)
	if err != nil {
		hlog.Errorf("get session failed: %v", err)
		c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "session not found",
		})
		return
	}

	// Convert session findings to reviewmap format
	var findings []reviewmap.Finding
	if sess.AgentResults != nil {
		findings = h.convertSessionFindings(sess)
	}

	// Generate review map
	reviewMap, err := h.mapGenerator.Generate(req.SessionID, req.Diff, findings)
	if err != nil {
		hlog.Errorf("generate review map failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to generate review map",
		})
		return
	}

	// Save to database
	dbMap := &models.ReviewMap{
		MapID:          reviewMap.ID,
		ReviewID:       req.ReviewID,
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

	if err := h.reviewMapRepo.Create(ctx, dbMap); err != nil {
		hlog.Errorf("save review map failed: %v", err)
		// Still return the map even if save fails
	}

	c.JSON(http.StatusOK, reviewMap)
}

// GetMaps returns review maps for a review
func (h *ReviewMapHandler) GetMaps(ctx context.Context, c *app.RequestContext) {
	reviewID := c.Param("id")
	if reviewID == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "review ID is required",
		})
		return
	}

	// Get maps from database
	maps, err := h.reviewMapRepo.GetByReviewID(ctx, reviewID)
	if err != nil {
		hlog.Errorf("get review maps failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to get review maps",
		})
		return
	}

	// Convert to response format
	result := make([]map[string]interface{}, 0, len(maps))
	for _, m := range maps {
		result = append(result, map[string]interface{}{
			"id":            m.MapID,
			"review_id":     m.ReviewID,
			"session_id":    m.SessionID,
			"title":         m.Title,
			"description":   m.Description,
			"summary":       m.Summary,
			"file_count":    m.FileCount,
			"section_count": m.SectionCount,
			"created_at":    m.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"maps":  result,
		"total": len(result),
	})
}

// GetMap returns a specific review map
func (h *ReviewMapHandler) GetMap(ctx context.Context, c *app.RequestContext) {
	mapID := c.Param("mapId")
	if mapID == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "map ID is required",
		})
		return
	}

	// Get map from database
	m, err := h.reviewMapRepo.GetByMapID(ctx, mapID)
	if err != nil {
		hlog.Errorf("get review map failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to get review map",
		})
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "map not found",
		})
		return
	}

	// Convert to response format
	var sections []interface{}
	var dependencies []interface{}
	json.Unmarshal(m.Sections, &sections)
	json.Unmarshal(m.Dependencies, &dependencies)

	c.JSON(http.StatusOK, map[string]interface{}{
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

// GetSessionMap returns the review map for a session
func (h *ReviewMapHandler) GetSessionMap(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "session ID is required",
		})
		return
	}

	// Get map from database
	m, err := h.reviewMapRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		hlog.Errorf("get session map failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to get session map",
		})
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "map not found for this session",
		})
		return
	}

	// Convert to response format
	var sections []interface{}
	var dependencies []interface{}
	json.Unmarshal(m.Sections, &sections)
	json.Unmarshal(m.Dependencies, &dependencies)

	c.JSON(http.StatusOK, map[string]interface{}{
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

// PreviewMapRequest represents the request to preview a review map
type PreviewMapRequest struct {
	Diff string `json:"diff" binding:"required"`
}

// PreviewMap generates a preview of a review map without saving
func (h *ReviewMapHandler) PreviewMap(ctx context.Context, c *app.RequestContext) {
	var req PreviewMapRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	// Generate preview map
	reviewMap, err := h.mapGenerator.Generate("preview", req.Diff, nil)
	if err != nil {
		hlog.Errorf("generate preview map failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to generate preview map",
		})
		return
	}

	c.JSON(http.StatusOK, reviewMap)
}

// convertSessionFindings converts session agent results to reviewmap findings
func (h *ReviewMapHandler) convertSessionFindings(sess *models.ReviewSession) []reviewmap.Finding {
	var findings []reviewmap.Finding

	if sess.AgentResults == nil {
		return findings
	}

	// Parse agent results
	var agentResults []models.AgentResultData
	if err := json.Unmarshal(sess.AgentResults, &agentResults); err != nil {
		hlog.Warnf("failed to parse agent results: %v", err)
		return findings
	}

	// Convert each agent's findings
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

// marshalJSON marshals data to JSON, returns nil on error
func marshalJSON(data interface{}) json.RawMessage {
	result, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	return result
}

// marshalJSONToDatatypes marshals data to datatypes.JSON
func marshalJSONToDatatypes(data interface{}) datatypes.JSON {
	result, err := json.Marshal(data)
	if err != nil {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(result)
}

// generateMapID generates a unique map ID
func generateMapID() string {
	return "map-" + uuid.New().String()
}
