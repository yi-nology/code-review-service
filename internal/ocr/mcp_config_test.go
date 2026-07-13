package ocr

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// withTempHome 把 HOME 重定向到临时目录，避免污染真实 ~/.opencodereview/config.json。
// 返回 config.json 的完整路径与 cleanup。
func withTempHome(t *testing.T) (configPath string, cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	prevHome, hadHome := os.LookupEnv("HOME")
	t.Setenv("HOME", dir)
	return filepath.Join(dir, ".opencodereview", "config.json"), func() {
		if hadHome {
			_ = os.Setenv("HOME", prevHome)
		} else {
			_ = os.Unsetenv("HOME")
		}
	}
}

func readConfig(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	return cfg
}

// TestSyncMCPConfig_NewFile 不存在 config.json 时创建并写入 mcp_servers。
func TestSyncMCPConfig_NewFile(t *testing.T) {
	path, cleanup := withTempHome(t)
	defer cleanup()

	mcp := `{"docs":{"command":"npx","args":["-y","@acme/docs"]}}`
	if err := syncMCPConfig(mcp); err != nil {
		t.Fatalf("syncMCPConfig: %v", err)
	}

	cfg := readConfig(t, path)
	raw, ok := cfg["mcp_servers"]
	if !ok {
		t.Fatal("expected mcp_servers key in config")
	}
	var servers map[string]MCPServerConfig
	if err := json.Unmarshal(raw, &servers); err != nil {
		t.Fatalf("unmarshal mcp_servers: %v", err)
	}
	if len(servers) != 1 || servers["docs"].Command != "npx" {
		t.Errorf("unexpected servers: %+v", servers)
	}
}

// TestSyncMCPConfig_PreservesOtherKeys 写入 mcp_servers 时保留 config.json 其它键。
func TestSyncMCPConfig_PreservesOtherKeys(t *testing.T) {
	path, cleanup := withTempHome(t)
	defer cleanup()

	// 预置一个已有 llm + language 的 config.json。
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"llm":{"url":"https://x","model":"gpt-4"},"language":"Chinese","mcp_servers":{"old":{"command":"old-cmd"}}}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	mcp := `{"docs":{"command":"npx"}}`
	if err := syncMCPConfig(mcp); err != nil {
		t.Fatalf("syncMCPConfig: %v", err)
	}

	cfg := readConfig(t, path)
	// 其它键保留。
	if string(cfg["language"]) != `"Chinese"` {
		t.Errorf("language not preserved: %s", cfg["language"])
	}
	var llm struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(cfg["llm"], &llm); err != nil || llm.URL != "https://x" {
		t.Errorf("llm not preserved: %s", cfg["llm"])
	}
	// mcp_servers 被整体替换（old 消失，docs 出现）。
	var servers map[string]MCPServerConfig
	if err := json.Unmarshal(cfg["mcp_servers"], &servers); err != nil {
		t.Fatalf("unmarshal mcp_servers: %v", err)
	}
	if _, hasOld := servers["old"]; hasOld {
		t.Error("old server should be replaced")
	}
	if _, hasDocs := servers["docs"]; !hasDocs {
		t.Error("new server docs should be present")
	}
}

// TestSyncMCPConfig_EmptyRemovesKey 空 JSON 删除 mcp_servers 键（禁用 MCP）。
func TestSyncMCPConfig_EmptyRemovesKey(t *testing.T) {
	path, cleanup := withTempHome(t)
	defer cleanup()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"language":"Chinese","mcp_servers":{"docs":{"command":"npx"}}}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncMCPConfig(""); err != nil {
		t.Fatalf("syncMCPConfig empty: %v", err)
	}

	cfg := readConfig(t, path)
	if _, ok := cfg["mcp_servers"]; ok {
		t.Error("mcp_servers should be removed when empty")
	}
	if string(cfg["language"]) != `"Chinese"` {
		t.Errorf("language should be preserved: %s", cfg["language"])
	}
}

// TestSyncMCPConfig_InvalidJSON 非法 JSON 返回 error，不写文件。
func TestSyncMCPConfig_InvalidJSON(t *testing.T) {
	path, cleanup := withTempHome(t)
	defer cleanup()

	err := syncMCPConfig("{not valid json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	// 不应创建文件。
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("config file should not be created on invalid JSON: %v", statErr)
	}
}

// TestSyncMCPConfigIfSet 空字符串是 no-op，不触发文件操作。
func TestSyncMCPConfigIfSet_EmptyNoOp(t *testing.T) {
	path, cleanup := withTempHome(t)
	defer cleanup()

	syncMCPConfigIfSet("")
	syncMCPConfigIfSet("   ")

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("empty mcp should be no-op, file should not exist: %v", err)
	}
}
