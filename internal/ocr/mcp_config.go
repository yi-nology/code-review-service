package ocr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/oekit/logx"
)

// 本文件把 MCP server 配置同步到 OCR CLI 的全局配置文件
// ~/.opencodereview/config.json 的 mcp_servers 键。
//
// 为什么写全局文件:上游 OCR CLI 的 review/scan 路径只从该文件读 MCP 配置,
// 刻意忽略 OCR_CONFIG_PATH 环境变量,也没有 --config/--mcp flag。
//
// 为什么是并发安全的:与已删除的 ensureOCRLanguage 不同,MCP 配置是全局慢变配置
// (所有并发评审用同一套 MCP server),不是 per-review 配置。last-writer-wins
// 在这里无害——所有评审要的就是同一份 MCP 配置。进程级 Mutex 串行化文件读写,
// 防止并发写损坏文件。跨实例部署时全局文件不共享,这是上游 OCR 的已知限制。

// MCPServerConfig 描述单个 MCP server(与上游 OCR 的 MCPServerConfig 对齐)。
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	Tools   []string `json:"tools,omitempty"`
	Setup   string   `json:"setup,omitempty"`
}

// mcpConfigMu 串行化对 ~/.opencodereview/config.json 的读-改-写。
var mcpConfigMu sync.Mutex

// syncMCPConfig 把 mcpServersJSON(一个 map[name]MCPServerConfig 的 JSON 字符串)
// 同步到 ~/.opencodereview/config.json 的 mcp_servers 键。
//
// 行为:
//   - mcpServersJSON 为空/空白 → 从 config.json 删除 mcp_servers 键(禁用 MCP)
//   - 否则 → 解析后整体替换 mcp_servers 键(保留 config.json 其它键如 llm/language)
//   - 原子写:写临时文件 → os.Rename,防止并发读撕裂
//
// 容错:读/解析失败时记 warn 并返回 error,调用方应降级继续(评审优先,只是没 MCP 工具)。
func syncMCPConfig(mcpServersJSON string) error {
	mcpConfigMu.Lock()
	defer mcpConfigMu.Unlock()

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	configPath := filepath.Join(home, ".opencodereview", "config.json")

	// 读现有 config.json(不存在则视为空对象)。
	raw := []byte("{}")
	if data, readErr := os.ReadFile(configPath); readErr == nil {
		raw = data
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read config %q: %w", configPath, readErr)
	}

	// config.json 顶层是任意 JSON 对象,用 map[string]json.RawMessage 保留未知键原样。
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("parse config %q: %w", configPath, err)
	}
	if cfg == nil {
		cfg = make(map[string]json.RawMessage)
	}

	// 解析并替换 mcp_servers 键。
	trimmed := strings.TrimSpace(mcpServersJSON)
	if trimmed == "" {
		delete(cfg, "mcp_servers")
	} else {
		// 校验是合法的 MCP server map(必须是对象),非法 JSON 直接返回 error。
		var check map[string]MCPServerConfig
		if err := json.Unmarshal([]byte(trimmed), &check); err != nil {
			return fmt.Errorf("invalid mcp_servers JSON: %w", err)
		}
		cfg["mcp_servers"] = json.RawMessage(trimmed)
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// 确保目录存在(config.json 可能还不存在)。
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// 原子写:临时文件 → rename。rename 在同一文件系统(同目录)下是原子的。
	tmp, err := os.CreateTemp(filepath.Dir(configPath), ".config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpName, configPath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename config: %w", err)
	}

	return nil
}

// syncMCPConfigIfSet 在 mcpServersJSON 非空时同步 MCP 配置。
// 失败时记 warn 并返回 nil(降级:评审继续,只是 OCR 跑时没 MCP 工具)。
// 评审优先,不因 MCP 配置同步失败而阻断整个评审。
func syncMCPConfigIfSet(mcpServersJSON string) {
	if strings.TrimSpace(mcpServersJSON) == "" {
		return
	}
	if err := syncMCPConfig(mcpServersJSON); err != nil {
		logx.Warnf("sync mcp config failed (review continues without mcp): %v", err)
	}
}
