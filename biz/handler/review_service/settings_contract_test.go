package review_service

import (
	"encoding/json"
	"testing"

	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/pkg/ptrx"
)

func TestGetOCRSettingsRespJSONContract(t *testing.T) {
	apiSettings := &modelSettings.OCRSettings{
		Enabled:     true,
		URL:         strPtr("https://example.com/v1"),
		Model:       strPtr("test-model"),
		AuthToken:   strPtr("***"),
		Concurrency: ptrx.Of(int64(4)),
	}
	resp := modelSettings.GetOCRSettingsResp{Settings: apiSettings}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	settings, ok := raw["settings"].(map[string]any)
	if !ok {
		t.Fatal("response should have 'settings' wrapper key")
	}
	if settings["enabled"] != true {
		t.Error("settings.enabled should be true")
	}
	if settings["url"] != "https://example.com/v1" {
		t.Error("settings.url mismatch")
	}
	if settings["model"] != "test-model" {
		t.Error("settings.model mismatch")
	}
	if _, hasResult := raw["result"]; hasResult {
		t.Error("response should not have 'result' key")
	}
}

func TestUpdateOCRSettingsReqJSONContract(t *testing.T) {
	input := `{"settings":{"enabled":true,"url":"https://example.com","model":"m1","auth_token":"tok","language":"English","concurrency":2,"max_git_procs":4,"timeout_minutes":10}}`

	var req modelSettings.UpdateOCRSettingsReq
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("unmarshal UpdateOCRSettingsReq: %v", err)
	}
	if req.Settings == nil {
		t.Fatal("req.Settings should not be nil")
	}
	if !req.Settings.Enabled {
		t.Error("req.Settings.Enabled should be true")
	}
	if ptrx.Deref(req.Settings.URL, "") != "https://example.com" {
		t.Error("url mismatch")
	}
	if ptrx.Deref(req.Settings.Model, "") != "m1" {
		t.Error("model mismatch")
	}
}

func TestUpdateOCRSettingsReqFlatJSONRejected(t *testing.T) {
	input := `{"enabled":true,"url":"https://example.com","model":"m1"}`

	var req modelSettings.UpdateOCRSettingsReq
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("unmarshal should not error: %v", err)
	}
	if req.Settings != nil {
		t.Error("flat JSON should not bind Settings — expected nil, got non-nil. " +
			"Frontend must send {\"settings\":{...}} wrapper.")
	}
}

func TestPutProjectPolicyReqJSONContract(t *testing.T) {
	input := `{"policy":{"provider":"github","repo":"acme/app","fail_on":"major","ignore_paths":"[\"vendor/**\"]"}}`

	var req modelSettings.PutProjectPolicyReq
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("unmarshal PutProjectPolicyReq: %v", err)
	}
	if req.Policy == nil {
		t.Fatal("req.Policy should not be nil")
	}
	if req.Policy.Provider != "github" {
		t.Error("provider mismatch")
	}
	if req.Policy.Repo != "acme/app" {
		t.Error("repo mismatch")
	}
	if ptrx.Deref(req.Policy.IgnorePaths, "") != "[\"vendor/**\"]" {
		t.Error("ignore_paths mismatch")
	}
}

func TestPutProjectPolicyReqFlatJSONRejected(t *testing.T) {
	input := `{"provider":"github","repo":"acme/app","fail_on":"major","ignore_paths":["vendor/**"]}`

	var req modelSettings.PutProjectPolicyReq
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("unmarshal should not error: %v", err)
	}
	if req.Policy != nil {
		t.Error("flat JSON should not bind Policy — expected nil, got non-nil. " +
			"Frontend must send {\"policy\":{...}} wrapper.")
	}
}

func TestTestPlatformRespJSONContract(t *testing.T) {
	connected := true
	resp := modelSettings.TestPlatformResp{
		Result: &modelSettings.GitTestResult{
			Connected: connected,
			Platform:  "github",
			UserName:  strPtr("testuser"),
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	result, ok := raw["result"].(map[string]any)
	if !ok {
		t.Fatal("response should have 'result' wrapper key")
	}
	if result["connected"] != true {
		t.Error("result.connected should be true")
	}
}

func TestDbProjectPolicyToAPIIgnorePathsIsJSONString(t *testing.T) {
	dbPolicy := &models.ProjectPolicy{
		Provider:    "github",
		Repo:        "acme/app",
		FailOn:      "critical",
		IgnorePaths: []byte(`["vendor/**","generated/**"]`),
	}
	apiPolicy := dbProjectPolicyToAPI(dbPolicy)

	if apiPolicy.IgnorePaths == nil {
		t.Fatal("IgnorePaths should not be nil")
	}
	paths := *apiPolicy.IgnorePaths
	if paths != `["vendor/**","generated/**"]` {
		t.Fatalf("IgnorePaths should be a JSON string, got: %s", paths)
	}

	var parsed []string
	if err := json.Unmarshal([]byte(paths), &parsed); err != nil {
		t.Fatalf("IgnorePaths should be valid JSON array string: %v", err)
	}
	if len(parsed) != 2 || parsed[0] != "vendor/**" || parsed[1] != "generated/**" {
		t.Fatalf("parsed paths mismatch: %v", parsed)
	}
}
