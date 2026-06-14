package review_service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// ProjectAgentHandler handles project agent configuration requests
type ProjectAgentHandler struct {
	projectAgentRepo repository.ProjectAgentRepository
	reviewerAgentRepo repository.ReviewerAgentRepository
}

// NewProjectAgentHandler creates a new project agent handler
func NewProjectAgentHandler(
	projectAgentRepo repository.ProjectAgentRepository,
	reviewerAgentRepo repository.ReviewerAgentRepository,
) *ProjectAgentHandler {
	return &ProjectAgentHandler{
		projectAgentRepo:  projectAgentRepo,
		reviewerAgentRepo: reviewerAgentRepo,
	}
}

// GetProjectAgents returns agent configuration for a project
func (h *ProjectAgentHandler) GetProjectAgents(ctx context.Context, c *app.RequestContext) {
	provider := c.Param("provider")
	repo := c.Param("repo")

	if provider == "" || repo == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "provider and repo are required",
		})
		return
	}

	// Get project agents
	projectAgents, err := h.projectAgentRepo.GetByProject(ctx, provider, repo)
	if err != nil {
		hlog.Errorf("get project agents failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to get project agents",
		})
		return
	}

	// Get all available agents
	allAgents, err := h.reviewerAgentRepo.List(ctx)
	if err != nil {
		hlog.Errorf("get all agents failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to get agents",
		})
		return
	}

	// Build response
	agentMap := make(map[uint]models.ReviewerAgent)
	for _, a := range allAgents {
		agentMap[a.ID] = a
	}

	// Mark which agents are enabled for this project
	enabledMap := make(map[uint]models.ProjectAgent)
	for _, pa := range projectAgents {
		enabledMap[pa.AgentID] = pa
	}

	// Build agent list with project-specific settings
	type AgentWithProjectConfig struct {
		AgentID         uint   `json:"agent_id"`
		Name            string `json:"name"`
		DisplayName     string `json:"display_name"`
		Description     string `json:"description"`
		Background      string `json:"background"`
		Enabled         bool   `json:"enabled"`
		Priority        int    `json:"priority"`
		ProjectEnabled  bool   `json:"project_enabled"`
		ProjectPriority int    `json:"project_priority"`
	}

	var agents []AgentWithProjectConfig
	for _, a := range allAgents {
		agent := AgentWithProjectConfig{
			AgentID:         a.ID,
			Name:            a.Name,
			DisplayName:     a.DisplayName,
			Description:     a.Description,
			Background:      a.Background,
			Enabled:         a.Enabled,
			Priority:        a.Priority,
			ProjectEnabled:  false,
			ProjectPriority: a.Priority,
		}
		if pa, ok := enabledMap[a.ID]; ok {
			agent.ProjectEnabled = pa.Enabled
			agent.ProjectPriority = pa.Priority
		}
		agents = append(agents, agent)
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"provider": provider,
		"repo":     repo,
		"agents":   agents,
	})
}

// UpdateProjectAgentRequest represents the request to update project agent config
type UpdateProjectAgentRequest struct {
	AgentID  uint `json:"agent_id" binding:"required"`
	Enabled  bool `json:"enabled"`
	Priority int  `json:"priority"`
}

// UpdateProjectAgent updates agent configuration for a project
func (h *ProjectAgentHandler) UpdateProjectAgent(ctx context.Context, c *app.RequestContext) {
	provider := c.Param("provider")
	repo := c.Param("repo")

	if provider == "" || repo == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "provider and repo are required",
		})
		return
	}

	var req UpdateProjectAgentRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	// Verify agent exists
	_, err := h.reviewerAgentRepo.GetByID(ctx, req.AgentID)
	if err != nil {
		c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "agent not found",
		})
		return
	}

	// Upsert project agent
	pa := &models.ProjectAgent{
		Provider: provider,
		Repo:     repo,
		AgentID:  req.AgentID,
		Enabled:  req.Enabled,
		Priority: req.Priority,
	}

	if err := h.projectAgentRepo.Upsert(ctx, pa); err != nil {
		hlog.Errorf("upsert project agent failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to update project agent",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"message": "project agent updated",
	})
}

// BatchUpdateProjectAgentsRequest represents the request to batch update project agents
type BatchUpdateProjectAgentsRequest struct {
	Agents []UpdateProjectAgentRequest `json:"agents" binding:"required"`
}

// BatchUpdateProjectAgents batch updates agent configuration for a project
func (h *ProjectAgentHandler) BatchUpdateProjectAgents(ctx context.Context, c *app.RequestContext) {
	provider := c.Param("provider")
	repo := c.Param("repo")

	if provider == "" || repo == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "provider and repo are required",
		})
		return
	}

	var req BatchUpdateProjectAgentsRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request body",
		})
		return
	}

	// Delete existing associations
	if err := h.projectAgentRepo.DeleteByProject(ctx, provider, repo); err != nil {
		hlog.Errorf("delete project agents failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to update project agents",
		})
		return
	}

	// Create new associations
	for _, agentReq := range req.Agents {
		pa := &models.ProjectAgent{
			Provider: provider,
			Repo:     repo,
			AgentID:  agentReq.AgentID,
			Enabled:  agentReq.Enabled,
			Priority: agentReq.Priority,
		}
		if err := h.projectAgentRepo.Upsert(ctx, pa); err != nil {
			hlog.Errorf("upsert project agent failed: %v", err)
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"message": "project agents updated",
		"count":   len(req.Agents),
	})
}

// DeleteProjectAgent removes an agent from a project
func (h *ProjectAgentHandler) DeleteProjectAgent(ctx context.Context, c *app.RequestContext) {
	provider := c.Param("provider")
	repo := c.Param("repo")
	agentID := c.Param("agentId")

	if provider == "" || repo == "" || agentID == "" {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "provider, repo, and agentId are required",
		})
		return
	}

	var id uint
	if _, err := fmt.Sscanf(agentID, "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "invalid agent id",
		})
		return
	}

	if err := h.projectAgentRepo.Delete(ctx, provider, repo, id); err != nil {
		hlog.Errorf("delete project agent failed: %v", err)
		c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to delete project agent",
		})
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"message": "project agent deleted",
	})
}

// GetProjectAgentConfig returns the full agent configuration for a project (used internally)
func (h *ProjectAgentHandler) GetProjectAgentConfig(ctx context.Context, provider, repo string) ([]models.ProjectAgent, error) {
	return h.projectAgentRepo.GetEnabledByProject(ctx, provider, repo)
}
