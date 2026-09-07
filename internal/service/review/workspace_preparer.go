package review

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository/repoerr"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/logx"
)

// WorkspacePreparer handles workspace preparation with project policy and rule resolution.
// Extracted from multiple locations to eliminate duplication.
type WorkspacePreparer struct {
	s *Service
}

// NewWorkspacePreparer creates a new WorkspacePreparer
func NewWorkspacePreparer(s *Service) *WorkspacePreparer {
	return &WorkspacePreparer{s: s}
}

// PrepareResult contains the prepared workspace and associated cleanup function
type PrepareResult struct {
	Workspace     domain.Workspace
	ProjectPolicy projectpolicy.ProjectPolicy
	Cleanup       func()
}

// PrepareWorkspace prepares a workspace with project policy and rule resolution.
// This is the single implementation for all workspace preparation needs.
func (p *WorkspacePreparer) PrepareWorkspace(ctx context.Context, req domain.Request, paths, exclude []string, includeAgentRules bool) (*PrepareResult, error) {
	if err := validate(req); err != nil {
		return nil, err
	}

	provider, ok := p.s.Providers.Get(req.Provider)
	if !ok {
		logx.Errorf("PrepareWorkspace: provider not found: requested=%q", req.Provider)
		return nil, domain.ErrUnsupportedProvider
	}

	workspace, err := provider.Prepare(ctx, req)
	if err != nil {
		return nil, err
	}

	result := &PrepareResult{
		Workspace: workspace,
		Cleanup:   func() {},
	}

	// Setup cleanup for workspace directory
	if workspace.Dir != "" {
		result.Cleanup = func() { _ = os.RemoveAll(workspace.Dir) }
	}

	// Resolve project policy
	projectPolicy := projectpolicy.NewDefault(req.Provider, req.Repo)
	if p.s.projectRepo != nil {
		if policyRecord, err := p.s.projectRepo.GetOrCreateDefault(ctx, req.Provider, req.Repo); err == nil {
			projectPolicy = policyRecord
		} else if !errors.Is(err, repoerr.ErrNotFound) {
			logx.Warn("failed to get/create project policy, using default", "error", err)
		}
	}
	result.ProjectPolicy = projectPolicy

	// Apply ignore paths from project policy
	var ignorePaths []string
	if len(projectPolicy.IgnorePaths) > 0 {
		if err := json.Unmarshal(projectPolicy.IgnorePaths, &ignorePaths); err != nil {
			logx.Warn("failed to decode ignore_paths", "error", err)
		}
	}
	if len(ignorePaths) > 0 {
		workspace, err = ApplyProjectScope(ctx, workspace, ignorePaths)
		if err != nil {
			result.Cleanup()
			return nil, err
		}
		result.Workspace = workspace
	}

	// Set scan paths and exclude patterns
	result.Workspace.ScanPaths = paths
	result.Workspace.ScanExclude = exclude

	// Resolve rule files
	if p.s.globalRulesRepo != nil {
		var agentRefs [][]byte
		if includeAgentRules {
			agentRefs = AgentRulePackageRefs(ctx, p.s.agentRepo)
		}
		rulePath, cleanup, ruleErr := ResolveRuleFileForAgents(ctx, projectPolicy, p.s.rulePackageRepo, p.s.globalRulesRepo, agentRefs...)
		if ruleErr != nil {
			logx.Warn("failed to resolve rule file", "error", ruleErr)
		} else if rulePath != "" {
			result.Workspace.RulePath = rulePath
			if cleanup != nil {
				// Chain cleanup functions
				originalCleanup := result.Cleanup
				result.Cleanup = func() {
					cleanup()
					originalCleanup()
				}
			}
		}
	}

	return result, nil
}

// PrepareWorkspaceWithOCR prepares workspace and applies OCR overrides from project policy
func (p *WorkspacePreparer) PrepareWorkspaceWithOCR(ctx context.Context, req domain.Request, background string) (*PrepareResult, error) {
	result, err := p.PrepareWorkspace(ctx, req, nil, nil, true)
	if err != nil {
		return nil, err
	}

	// Apply OCR overrides from project policy
	result.Workspace.OCR = ProjectPolicyToOCROverrides(result.ProjectPolicy)
	if background != "" {
		result.Workspace.OCR.Background = background
	}

	return result, nil
}
