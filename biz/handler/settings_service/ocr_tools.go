package settings_service

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"

	"github.com/oekit/pkg/ptrx"

	"github.com/cloudwego/hertz/pkg/app"
)

func (h *SettingsHandler) rulesCheck(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.RulesCheckReq](c)
	if !ok {
		return
	}
	if req.Path == "" {
		common.Fail(ctx, c, common.BadRequest("文件路径不能为空"))
		return
	}
	if !common.FeatureGuard(c, h.ocr.Checker(), "规则检查") {
		return
	}
	result, err := h.ocr.Checker().RulesCheck(ctx, req.Path)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := &modelSettings.RulesCheckResult{Applicable: result.Applicable}
	if result.Rule != "" {
		resp.Rule = common.StrPtr(result.Rule)
	}
	if result.Message != "" {
		resp.Message = common.StrPtr(result.Message)
	}
	common.Success(c, &modelSettings.RulesCheckResp{Result: resp})
}

// RulesCheck 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func RulesCheck(ctx context.Context, c *app.RequestContext) { handlerInst.rulesCheck(ctx, c) }

func (h *SettingsHandler) oCRVersion(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.ocr.Versioner(), "OCR 版本查询") {
		return
	}
	result, err := h.ocr.Versioner().OCRVersion(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := &modelSettings.OCRVersionResp{Version: result.Version}
	if result.Commit != "" {
		resp.Commit = common.StrPtr(result.Commit)
	}
	if result.Platform != "" {
		resp.Platform = common.StrPtr(result.Platform)
	}
	if result.BuildTime != "" {
		resp.BuildTime = common.StrPtr(result.BuildTime)
	}
	common.Success(c, resp)
}

// OCRVersion 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func OCRVersion(ctx context.Context, c *app.RequestContext) { handlerInst.oCRVersion(ctx, c) }

func (h *SettingsHandler) viewerStatus(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.ocr.Viewer(), "Viewer") {
		return
	}
	result, err := h.ocr.Viewer().ViewerStatus(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := &modelSettings.ViewerStatusResp{Running: result.Running}
	if result.Addr != "" {
		resp.Addr = common.StrPtr(result.Addr)
	}
	if result.Message != "" {
		resp.Message = common.StrPtr(result.Message)
	}
	common.Success(c, resp)
}

// ViewerStatus 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ViewerStatus(ctx context.Context, c *app.RequestContext) { handlerInst.viewerStatus(ctx, c) }

func (h *SettingsHandler) startViewer(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.StartViewerReq](c)
	if !ok {
		return
	}
	if !common.FeatureGuard(c, h.ocr.Viewer(), "Viewer") {
		return
	}
	addr := ptrx.Deref(req.Addr, "")
	result, err := h.ocr.Viewer().StartViewer(ctx, addr)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := &modelSettings.StartViewerResp{Running: result.Running}
	if result.Addr != "" {
		resp.Addr = common.StrPtr(result.Addr)
	}
	if result.Message != "" {
		resp.Message = common.StrPtr(result.Message)
	}
	common.Success(c, resp)
}

// StartViewer 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func StartViewer(ctx context.Context, c *app.RequestContext) { handlerInst.startViewer(ctx, c) }

func (h *SettingsHandler) stopViewer(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.ocr.Viewer(), "Viewer") {
		return
	}
	result, err := h.ocr.Viewer().StopViewer(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := &modelSettings.ViewerStatusResp{Running: result.Running}
	if result.Addr != "" {
		resp.Addr = common.StrPtr(result.Addr)
	}
	if result.Message != "" {
		resp.Message = common.StrPtr(result.Message)
	}
	common.Success(c, resp)
}

// StopViewer 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func StopViewer(ctx context.Context, c *app.RequestContext) { handlerInst.stopViewer(ctx, c) }

func (h *SettingsHandler) locateViewerSession(ctx context.Context, c *app.RequestContext) {
	var req struct {
		WorkspaceDir string `json:"workspace_dir"`
		StartedAt    string `json:"started_at"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	workspaceDir := strings.TrimSpace(req.WorkspaceDir)
	startedAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(req.StartedAt))
	if workspaceDir == "" && startedAt.IsZero() {
		common.Fail(ctx, c, common.BadRequest("workspace_dir 和 started_at 不能同时为空"))
		return
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	location := locateViewerSession(homeDir, workspaceDir, startedAt)
	common.Success(c, location)
}

// LocateViewerSession 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func LocateViewerSession(ctx context.Context, c *app.RequestContext) {
	handlerInst.locateViewerSession(ctx, c)
}

func (h *SettingsHandler) listLLMProviders(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.ocr.Lister(), "LLM Provider 列表") {
		return
	}
	providers, err := h.ocr.Lister().ListLLMProviders(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	var items []*modelSettings.LLMProvider
	for _, p := range providers {
		item := &modelSettings.LLMProvider{Name: p.Name, Protocol: p.Protocol}
		if p.BaseURL != "" {
			item.BaseURL = common.StrPtr(p.BaseURL)
		}
		if len(p.DefaultModel) > 0 {
			item.DefaultModel = p.DefaultModel
		}
		items = append(items, item)
	}
	common.Success(c, &modelSettings.ListLLMProvidersResp{BuiltIn: items})
}

// ListLLMProviders 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListLLMProviders(ctx context.Context, c *app.RequestContext) {
	handlerInst.listLLMProviders(ctx, c)
}

// ListOCRSessions 返回 OCR review session 历史列表。
func (h *SettingsHandler) listOCRSessions(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.ocr.SessionLister(), "OCR Session 列表") {
		return
	}
	result, err := h.ocr.SessionLister().ListOCRSessions(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, result)
}

// ListOCRSessions 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListOCRSessions(ctx context.Context, c *app.RequestContext) { handlerInst.listOCRSessions(ctx, c) }

type viewerSessionLocation struct {
	RepoSlug  string `json:"repo_slug"`
	Path      string `json:"path"`
	SessionID string `json:"session_id,omitempty"`
	Matched   bool   `json:"matched"`
}

func locateViewerSession(homeDir, workspaceDir string, startedAt time.Time) viewerSessionLocation {
	if workspaceDir == "" {
		return findSessionGlobally(homeDir, startedAt)
	}
	slug := viewerRepoSlug(workspaceDir)
	location := viewerSessionLocation{RepoSlug: slug, Path: "/r/" + url.PathEscape(slug)}
	sessionID := findNearestViewerSession(filepath.Join(homeDir, ".opencodereview", "sessions", slug), workspaceDir, startedAt)
	if sessionID != "" {
		location.SessionID = sessionID
		location.Path += "/" + url.PathEscape(sessionID)
		location.Matched = true
	}
	return location
}

func findSessionGlobally(homeDir string, startedAt time.Time) viewerSessionLocation {
	sessionsRoot := filepath.Join(homeDir, ".opencodereview", "sessions")
	entries, err := os.ReadDir(sessionsRoot)
	if err != nil {
		return viewerSessionLocation{}
	}
	bestSlug := ""
	bestSessionID := ""
	bestDelta := math.MaxFloat64
	for _, dirEntry := range entries {
		if !dirEntry.IsDir() {
			continue
		}
		slug := dirEntry.Name()
		fileEntries, err := os.ReadDir(filepath.Join(sessionsRoot, slug))
		if err != nil {
			continue
		}
		for _, fileEntry := range fileEntries {
			if fileEntry.IsDir() || filepath.Ext(fileEntry.Name()) != ".jsonl" {
				continue
			}
			meta, ok := readViewerSessionStart(filepath.Join(sessionsRoot, slug, fileEntry.Name()))
			if !ok {
				continue
			}
			delta := meta.Timestamp.Sub(startedAt).Seconds()
			if delta < -60 || delta > 60*60 {
				continue
			}
			if delta < bestDelta {
				bestSlug = slug
				bestSessionID = meta.SessionID
				bestDelta = delta
			}
		}
	}
	if bestSessionID == "" {
		return viewerSessionLocation{}
	}
	return viewerSessionLocation{
		RepoSlug:  bestSlug,
		Path:      "/r/" + url.PathEscape(bestSlug) + "/" + url.PathEscape(bestSessionID),
		SessionID: bestSessionID,
		Matched:   true,
	}
}

func viewerRepoSlug(workspaceDir string) string {
	value := strings.TrimLeft(strings.TrimSpace(workspaceDir), `/\`)
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	return strings.Join(parts, "-")
}

func findNearestViewerSession(sessionDir, workspaceDir string, startedAt time.Time) string {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return ""
	}
	bestSession := ""
	bestDelta := math.MaxFloat64
	var latestAt time.Time
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		meta, ok := readViewerSessionStart(filepath.Join(sessionDir, entry.Name()))
		if !ok {
			continue
		}
		if meta.CWD != "" && workspaceDir != "" && filepath.Clean(meta.CWD) != filepath.Clean(workspaceDir) {
			continue
		}
		if startedAt.IsZero() {
			if meta.Timestamp.After(latestAt) {
				bestSession = meta.SessionID
				latestAt = meta.Timestamp
			}
			continue
		}
		delta := math.Abs(meta.Timestamp.Sub(startedAt).Seconds())
		if delta < bestDelta {
			bestSession = meta.SessionID
			bestDelta = delta
		}
	}
	if !startedAt.IsZero() && bestDelta > 30*60 {
		return ""
	}
	return bestSession
}

type viewerSessionStart struct {
	CWD       string    `json:"cwd"`
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`
}

func readViewerSessionStart(path string) (viewerSessionStart, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return viewerSessionStart{}, false
	}
	if idx := strings.IndexByte(string(data), '\n'); idx >= 0 {
		data = data[:idx]
	}
	var meta viewerSessionStart
	if err := json.Unmarshal(data, &meta); err != nil {
		return viewerSessionStart{}, false
	}
	if strings.TrimSpace(meta.SessionID) == "" {
		meta.SessionID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return meta, meta.SessionID != ""
}
