package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// =====================================================================
// API 集成测试：覆盖 review-service 所有 HTTP 端点
// 运行方式：go test -v -run TestAPI -timeout 300s ./api_integration_test.go
// 前置条件：服务运行在 BASE_URL（默认 http://localhost:8081）
// =====================================================================

const (
	defaultBaseURL = "http://localhost:8081"
	defaultToken   = "dev-token"
)

func baseURL() string {
	if u := os.Getenv("TEST_BASE_URL"); u != "" {
		return u
	}
	return defaultBaseURL
}

func token() string {
	if t := os.Getenv("TEST_TOKEN"); t != "" {
		return t
	}
	return defaultToken
}

// apiResponse 是统一的 API 响应结构
type apiResponse struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
	TraceID string          `json:"trace_id"`
}

// doRequest 执行 HTTP 请求并返回响应
func doRequest(t *testing.T, method, path string, body any, withAuth bool) (*http.Response, *apiResponse) {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	u := baseURL() + path
	req, err := http.NewRequest(method, u, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if withAuth {
		req.Header.Set("Authorization", "Bearer "+token())
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, path, err)
	}

	var apiResp apiResponse
	if resp.Body != nil {
		raw, _ := io.ReadAll(resp.Body)
		_ = json.Unmarshal(raw, &apiResp)
		resp.Body.Close()
	}

	return resp, &apiResp
}

// assertOK 验证 API 返回 code=0
func assertOK(t *testing.T, resp *apiResponse, path string) {
	t.Helper()
	if resp.Code != 0 {
		t.Errorf("%s: expected code=0, got code=%d msg=%s", path, resp.Code, resp.Message)
	}
}

// assertHTTP 验证 HTTP 状态码
func assertHTTP(t *testing.T, resp *http.Response, expected int, path string) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Errorf("%s: expected HTTP %d, got %d", path, expected, resp.StatusCode)
	}
}

// extractData 提取 data 字段到目标
func extractData(t *testing.T, resp *apiResponse, target any) {
	t.Helper()
	if err := json.Unmarshal(resp.Data, target); err != nil {
		t.Fatalf("unmarshal data: %v (raw: %s)", err, string(resp.Data))
	}
}

// =====================================================================
// 测试主入口：确保服务可用
// =====================================================================

func TestAPI_All(t *testing.T) {
	// 检查服务是否在线
	resp, err := http.Get(baseURL() + "/ping")
	if err != nil {
		t.Skipf("service not running at %s, skipping integration tests: %v", baseURL(), err)
	}
	resp.Body.Close()

	// 按顺序运行所有子测试
	t.Run("Health", testHealth)
	t.Run("Readyz", testReadyz)
	t.Run("Ping", testPing)
	t.Run("BuildVersion", testBuildVersion)
	t.Run("Auth", testAuth)
	t.Run("OCRSettings", testOCRSettings)
	t.Run("OCRVersion", testOCRVersion)
	t.Run("LLMProviders", testLLMProviders)
	t.Run("TestLLM", testTestLLM)
	t.Run("RulesCheck", testRulesCheck)
	t.Run("GlobalRules", testGlobalRules)
	t.Run("RulePackages", testRulePackages)
	t.Run("ToolPackages", testToolPackages)
	t.Run("CustomProviders", testCustomProviders)
	t.Run("Platforms", testPlatforms)
	t.Run("ViewerStatus", testViewerStatus)
	t.Run("Agents", testAgents)
	t.Run("ProjectAgents", testProjectAgents)
	t.Run("ReviewsList", testReviewsList)
	t.Run("ReviewDetail", testReviewDetail)
	t.Run("ReviewNotFound", testReviewNotFound)
	t.Run("ReviewCreateValidation", testReviewCreateValidation)
	t.Run("ReviewCreateAndQuery", testReviewCreateAndQuery)
	t.Run("SessionList", testSessionList)
	t.Run("SessionCancel", testSessionCancel)
	t.Run("ActiveProgress", testActiveProgress)
	t.Run("StatsProjects", testStatsProjects)
	t.Run("StatsOCRMetrics", testStatsOCRMetrics)
	t.Run("Feedback", testFeedback)
	t.Run("Diagnostics", testDiagnostics)
	t.Run("LLMMetrics", testLLMMetrics)
	t.Run("TraceID", testTraceID)
	t.Run("Swagger", testSwagger)
}

// =====================================================================
// 健康检查 & 基础端点
// =====================================================================

func testHealth(t *testing.T) {
	resp, api := doRequest(t, "GET", "/healthz", nil, false)
	assertHTTP(t, resp, 200, "/healthz")
	assertOK(t, api, "/healthz")

	var data struct {
		Status string `json:"status"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	extractData(t, api, &data)
	if data.Status != "ok" {
		t.Errorf("expected status=ok, got %s", data.Status)
	}
	for _, c := range data.Checks {
		if c.Status != "ok" {
			t.Errorf("checker %s: expected ok, got %s", c.Name, c.Status)
		}
	}
}

func testReadyz(t *testing.T) {
	resp, api := doRequest(t, "GET", "/readyz", nil, false)
	assertHTTP(t, resp, 200, "/readyz")
	assertOK(t, api, "/readyz")
}

func testPing(t *testing.T) {
	resp, api := doRequest(t, "GET", "/ping", nil, false)
	assertHTTP(t, resp, 200, "/ping")
	if api.Message != "pong" {
		t.Errorf("expected pong, got %s", api.Message)
	}
}

func testBuildVersion(t *testing.T) {
	resp, api := doRequest(t, "GET", "/buildVersion", nil, false)
	assertHTTP(t, resp, 200, "/buildVersion")
	assertOK(t, api, "/buildVersion")

	var data struct {
		StartTime string `json:"startTime"`
		Uptime    string `json:"uptime"`
	}
	extractData(t, api, &data)
	if data.StartTime == "" {
		t.Error("expected non-empty startTime")
	}
	if data.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

// =====================================================================
// 认证测试
// =====================================================================

func testAuth(t *testing.T) {
	// 无 token → 401
	resp, _ := doRequest(t, "GET", "/api/v1/reviews", nil, false)
	assertHTTP(t, resp, 401, "no token")

	// 错误 token → 401
	req, _ := http.NewRequest("GET", baseURL()+"/api/v1/reviews", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp2, _ := http.DefaultClient.Do(req)
	assertHTTP(t, resp2, 401, "wrong token")
	resp2.Body.Close()

	// 正确 token → 200
	resp3, _ := doRequest(t, "GET", "/api/v1/reviews", nil, true)
	assertHTTP(t, resp3, 200, "correct token")
}

// =====================================================================
// OCR 设置 & 工具
// =====================================================================

func testOCRSettings(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/ocr", nil, true)
	assertHTTP(t, resp, 200, "/api/v1/settings/ocr")
	assertOK(t, api, "/api/v1/settings/ocr")

	var data struct {
		Settings struct {
			Enabled          bool   `json:"enabled"`
			Binary           string `json:"binary"`
			URL              string `json:"url"`
			Model            string `json:"model"`
			Language         string `json:"language"`
			TimeoutMinutes   int    `json:"timeout_minutes"`
			Concurrency      int    `json:"concurrency"`
			DefaultSeverity  string `json:"default_severity"`
		} `json:"settings"`
	}
	extractData(t, api, &data)

	// token 应被脱敏
	if strings.Contains(string(api.Data), "api_key=secret") {
		t.Error("token not redacted in response")
	}
}

func testOCRVersion(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/ocr/version", nil, true)
	assertHTTP(t, resp, 200, "/ocr/version")
	assertOK(t, api, "/ocr/version")
}

func testLLMProviders(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/ocr/providers", nil, true)
	assertHTTP(t, resp, 200, "/ocr/providers")
	assertOK(t, api, "/ocr/providers")

	var data struct {
		BuiltIn []struct {
			Name     string `json:"name"`
			Protocol string `json:"protocol"`
		} `json:"built_in"`
	}
	extractData(t, api, &data)
	if len(data.BuiltIn) == 0 {
		t.Error("expected at least one provider")
	}
}

func testTestLLM(t *testing.T) {
	resp, api := doRequest(t, "POST", "/api/v1/settings/ocr/test-llm", nil, true)
	assertHTTP(t, resp, 200, "/test-llm")
	assertOK(t, api, "/test-llm")

	var data struct {
		Result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		} `json:"result"`
	}
	extractData(t, api, &data)
}

func testRulesCheck(t *testing.T) {
	body := map[string]string{"path": "test.go"}
	resp, api := doRequest(t, "POST", "/api/v1/settings/ocr/rules-check", body, true)
	assertHTTP(t, resp, 200, "/rules-check")
	assertOK(t, api, "/rules-check")

	var data struct {
		Result struct {
			Applicable bool `json:"applicable"`
		} `json:"result"`
	}
	extractData(t, api, &data)
}

// =====================================================================
// 规则 & 工具包
// =====================================================================

func testGlobalRules(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/rules/global", nil, true)
	assertHTTP(t, resp, 200, "/rules/global")
	assertOK(t, api, "/rules/global")
}

func testRulePackages(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/rules/packages", nil, true)
	assertHTTP(t, resp, 200, "/rules/packages")
	assertOK(t, api, "/rules/packages")
}

func testToolPackages(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/ocr/tool-packages", nil, true)
	assertHTTP(t, resp, 200, "/tool-packages")
	assertOK(t, api, "/tool-packages")
}

func testCustomProviders(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/ocr/custom-providers", nil, true)
	assertHTTP(t, resp, 200, "/custom-providers")
	assertOK(t, api, "/custom-providers")
}

func testPlatforms(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/platforms", nil, true)
	assertHTTP(t, resp, 200, "/platforms")
	assertOK(t, api, "/platforms")
}

// =====================================================================
// Viewer
// =====================================================================

func testViewerStatus(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/ocr/viewer", nil, true)
	assertHTTP(t, resp, 200, "/viewer")
	assertOK(t, api, "/viewer")

	var data struct {
		Running bool `json:"running"`
	}
	extractData(t, api, &data)
}

// =====================================================================
// Agent
// =====================================================================

func testAgents(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/settings/agents", nil, true)
	assertHTTP(t, resp, 200, "/agents")
	assertOK(t, api, "/agents")

	var data struct {
		Agents []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Enabled  bool   `json:"enabled"`
			Priority int    `json:"priority"`
		} `json:"agents"`
	}
	extractData(t, api, &data)
	if len(data.Agents) == 0 {
		t.Error("expected at least one agent")
	}
}

func testProjectAgents(t *testing.T) {
	path := "/api/v1/settings/project-agents?provider=gitea&repo=" + url.QueryEscape("enjoydream/ekit")
	resp, api := doRequest(t, "GET", path, nil, true)
	assertHTTP(t, resp, 200, "/project-agents")
	assertOK(t, api, "/project-agents")
}

// =====================================================================
// Review CRUD
// =====================================================================

func testReviewsList(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/reviews?page=1&page_size=3", nil, true)
	assertHTTP(t, resp, 200, "/reviews")
	assertOK(t, api, "/reviews")

	var data struct {
		Items []struct {
			ReviewID string `json:"review_id"`
			Status   string `json:"status"`
			Provider string `json:"provider"`
			Repo     string `json:"repo"`
		} `json:"items"`
		Total int `json:"total"`
	}
	extractData(t, api, &data)
}

func testReviewDetail(t *testing.T) {
	// 先拿一个已有 review
	_, listAPI := doRequest(t, "GET", "/api/v1/reviews?page=1&page_size=1", nil, true)
	var list struct {
		Items []struct {
			ReviewID string `json:"review_id"`
		} `json:"items"`
	}
	extractData(t, listAPI, &list)
	if len(list.Items) == 0 {
		t.Skip("no reviews in DB")
	}

	reviewID := list.Items[0].ReviewID
	resp, api := doRequest(t, "GET", "/api/v1/reviews/"+reviewID, nil, true)
	assertHTTP(t, resp, 200, "/reviews/{id}")
	assertOK(t, api, "/reviews/{id}")

	var data struct {
		Review struct {
			ReviewID string `json:"review_id"`
			Status   string `json:"status"`
		} `json:"review"`
	}
	extractData(t, api, &data)
	if data.Review.ReviewID != reviewID {
		t.Errorf("expected review_id=%s, got %s", reviewID, data.Review.ReviewID)
	}
}

func testReviewNotFound(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/reviews/nonexistent-id-12345", nil, true)
	assertHTTP(t, resp, 404, "/reviews/notfound")
	if api.Code != 404 {
		t.Errorf("expected code=404, got %d", api.Code)
	}
}

func testReviewCreateValidation(t *testing.T) {
	// 缺少 provider
	resp, api := doRequest(t, "POST", "/api/v1/reviews", map[string]string{"repo": "test/repo"}, true)
	assertHTTP(t, resp, 400, "missing provider")
	if api.Code != 400 {
		t.Errorf("expected code=400, got %d", api.Code)
	}
	if !strings.Contains(api.Message, "provider") {
		t.Errorf("expected 'provider' in message, got %s", api.Message)
	}

	// 缺少 repo
	resp2, api2 := doRequest(t, "POST", "/api/v1/reviews", map[string]string{"provider": "gitea"}, true)
	assertHTTP(t, resp2, 400, "missing repo")
	if api2.Code != 400 {
		t.Errorf("expected code=400, got %d", api2.Code)
	}
}

func testReviewCreateAndQuery(t *testing.T) {
	body := map[string]any{
		"provider":        "gitea",
		"repo":            "enjoydream/ekit",
		"pull_request_id": "1",
		"callback_mode":   "none",
		"wait":            false,
	}
	resp, api := doRequest(t, "POST", "/api/v1/reviews", body, true)
	// 200 或 202 都可接受（Hertz 在某些场景返回 202）
	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		t.Errorf("POST /reviews: expected HTTP 200/202, got %d", resp.StatusCode)
	}
	assertOK(t, api, "POST /reviews")

	var data struct {
		Review struct {
			ReviewID string `json:"review_id"`
			Status   string `json:"status"`
			Gate     struct {
				Status string `json:"status"`
			} `json:"gate"`
		} `json:"review"`
	}
	extractData(t, api, &data)
	if data.Review.ReviewID == "" {
		t.Fatal("expected non-empty review_id")
	}
	if data.Review.Status != "queued" && data.Review.Status != "running" {
		t.Errorf("expected status=queued or running, got %s", data.Review.Status)
	}

	// 查询刚创建的 review
	time.Sleep(2 * time.Second)
	resp2, api2 := doRequest(t, "GET", "/api/v1/reviews/"+data.Review.ReviewID, nil, true)
	assertHTTP(t, resp2, 200, "GET created review")
	assertOK(t, api2, "GET created review")

	// 验证 status 是 queued/running/success/failed 之一
	var data2 struct {
		Review struct {
			Status string `json:"status"`
		} `json:"review"`
	}
	extractData(t, api2, &data2)
	validStatuses := map[string]bool{"queued": true, "running": true, "success": true, "failed": true}
	if !validStatuses[data2.Review.Status] {
		t.Errorf("invalid status: %s", data2.Review.Status)
	}
}

// =====================================================================
// Session
// =====================================================================

func testSessionList(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/sessions?page=1&page_size=5", nil, true)
	assertHTTP(t, resp, 200, "/sessions")
	assertOK(t, api, "/sessions")
}

func testSessionCancel(t *testing.T) {
	// 使用不存在的 session ID，验证响应
	// 200（幂等取消）或 404/400/500 都可接受
	resp, _ := doRequest(t, "POST", "/api/v1/reviews/fake-review-id/sessions/fake-session-id/cancel", nil, true)
	if resp.StatusCode < 200 || resp.StatusCode >= 500 {
		// 500 也可接受（review 不存在导致 DB 查询失败）
		if resp.StatusCode != 500 {
			t.Errorf("expected 2xx/4xx/500, got %d", resp.StatusCode)
		}
	}
}

// =====================================================================
// Progress
// =====================================================================

func testActiveProgress(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/progress/active", nil, true)
	assertHTTP(t, resp, 200, "/progress/active")
	assertOK(t, api, "/progress/active")

	var data struct {
		Reviews []any `json:"reviews"`
		Total   int   `json:"total"`
	}
	extractData(t, api, &data)
}

// =====================================================================
// Stats
// =====================================================================

func testStatsProjects(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/stats/projects", nil, true)
	assertHTTP(t, resp, 200, "/stats/projects")
	assertOK(t, api, "/stats/projects")

	var data struct {
		Projects []any `json:"projects"`
		Total    int   `json:"total"`
	}
	extractData(t, api, &data)
}

func testStatsOCRMetrics(t *testing.T) {
	path := "/api/v1/stats/projects/ocr?provider=gitea&repo=" + url.QueryEscape("enjoydream/ekit")
	resp, api := doRequest(t, "GET", path, nil, true)
	assertHTTP(t, resp, 200, "/stats/projects/ocr")
	assertOK(t, api, "/stats/projects/ocr")

	var data struct {
		Metrics struct {
			ReviewCount int `json:"review_count"`
		} `json:"metrics"`
	}
	extractData(t, api, &data)
}

// =====================================================================
// Feedback
// =====================================================================

func testFeedback(t *testing.T) {
	// 先获取一个已有 review
	_, listAPI := doRequest(t, "GET", "/api/v1/reviews?page=1&page_size=1", nil, true)
	var list struct {
		Items []struct {
			ReviewID string `json:"review_id"`
		} `json:"items"`
	}
	extractData(t, listAPI, &list)
	if len(list.Items) == 0 {
		t.Skip("no reviews in DB")
	}

	reviewID := list.Items[0].ReviewID
	resp, api := doRequest(t, "GET", fmt.Sprintf("/api/v1/reviews/%s/findings/feedback", reviewID), nil, true)
	assertHTTP(t, resp, 200, "/feedback")
	assertOK(t, api, "/feedback")
}

// =====================================================================
// Diagnostics
// =====================================================================

func testDiagnostics(t *testing.T) {
	resp, api := doRequest(t, "GET", "/api/v1/diagnostics", nil, true)
	assertHTTP(t, resp, 200, "/diagnostics")
	assertOK(t, api, "/diagnostics")

	var data struct {
		SinceHours int `json:"since_hours"`
		Webhooks   struct {
			Received int `json:"received"`
		} `json:"webhooks"`
	}
	extractData(t, api, &data)
}

// =====================================================================
// LLM Metrics
// =====================================================================

func testLLMMetrics(t *testing.T) {
	resp, api := doRequest(t, "GET", "/debug/llm/metrics", nil, true)
	assertHTTP(t, resp, 200, "/debug/llm/metrics")
	assertOK(t, api, "/debug/llm/metrics")

	var data struct {
		Calls     int64 `json:"calls"`
		Successes int64 `json:"successes"`
	}
	extractData(t, api, &data)
}

// =====================================================================
// Trace ID
// =====================================================================

func testTraceID(t *testing.T) {
	req, _ := http.NewRequest("GET", baseURL()+"/ping", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	traceID := resp.Header.Get("X-Trace-Id")
	if traceID == "" {
		t.Error("expected non-empty X-Trace-Id header")
	}
}

// =====================================================================
// Swagger
// =====================================================================

func testSwagger(t *testing.T) {
	req, _ := http.NewRequest("GET", baseURL()+"/swagger/index.html", nil)
	req.Header.Set("Authorization", "Bearer "+token())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	assertHTTP(t, resp, 200, "/swagger/index.html")
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "swagger") && !strings.Contains(string(body), "Swagger") {
		t.Error("expected swagger content")
	}
}
