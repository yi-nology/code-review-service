// Package argusclient 是 review-service 对 Argus（PR 自动审查 Bot）出站调用的唯一收口。
//
// 与 internal/ocr 作为 OCR 单一入口同范式：触发审查任务（trigger 模式）、
// 查询任务状态（stale 兜底）、误报抑制回流共用同一 base URL / 凭证 / 超时，
// 凭证只在服务端（argus.bot_url + argus.bot_token，即 argus 侧 ARGUS_APITOKEN）。
//
// 契约来源：lacus/argus idl/argus.thrift + internal/service/service.go
//   - POST /tasks  {"kind":1(CODE_REVIEW),"platform","owner","repo","number"}
//     → {code:0,data:{taskId,status,duplicated?}}；同 PR 在飞任务幂等返回既有任务
//   - GET  /tasks/:taskId → {code:0,data:{taskId,status,verdict?,failReason?}}
//   - POST /findings/suppress {"platform","owner","repo","number","fingerprint"}
//     → {code:0,data:{matched,fingerprint}}
//
// 响应为 ekit 信封 {code,data,message,trace_id}；number 必须纯十进制（argus 侧校验）。
package argusclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// taskKindCodeReview 对应 argus TaskKind 枚举 CODE_REVIEW = 1（IDL 序列化为整数）。
const taskKindCodeReview = 1

// Client Argus HTTP 客户端。baseURL 为空表示未启用（Enabled=false）。
type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

// New 构造客户端；baseURL/token 为空时 Enabled()=false，调用方法将返回 ErrDisabled。
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled 是否已配置 argus 对接（bot_url 非空）。
func (c *Client) Enabled() bool {
	return c != nil && c.baseURL != ""
}

// ErrDisabled 未配置 argus 对接时的错误（创建期 fail-fast 用）。
var ErrDisabled = fmt.Errorf("argus 对接未配置（argus.bot_url 为空）")

// TriggerResult 触发结果。
type TriggerResult struct {
	TaskID     string
	Status     string
	Duplicated bool // true = 同 PR 已有在飞任务，argus 幂等返回既有任务
}

// TriggerReview 创建 CODE_REVIEW 审查任务（异步：argus 完成后经 sink push 回报）。
func (c *Client) TriggerReview(ctx context.Context, platform, owner, repo, number string) (*TriggerResult, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner/repo 必填")
	}
	if !isDigits(number) {
		return nil, fmt.Errorf("number 必须为纯十进制 PR 序号: %q", number)
	}
	payload, _ := json.Marshal(map[string]any{
		"kind":     taskKindCodeReview,
		"platform": strings.ToLower(strings.TrimSpace(platform)),
		"owner":    strings.TrimSpace(owner),
		"repo":     strings.TrimSpace(repo),
		"number":   strings.TrimSpace(number),
	})
	var out struct {
		TaskID     string `json:"taskId"`
		Status     string `json:"status"`
		Duplicated bool   `json:"duplicated"`
	}
	if err := c.do(ctx, http.MethodPost, "/tasks", payload, &out); err != nil {
		return nil, err
	}
	if out.TaskID == "" {
		return nil, fmt.Errorf("argus 返回空 taskId")
	}
	return &TriggerResult{TaskID: out.TaskID, Status: out.Status, Duplicated: out.Duplicated}, nil
}

// TaskState 任务状态快照（轮询兜底用）。
type TaskState struct {
	TaskID     string
	Status     string // pending|running|done|failed|stale
	Verdict    string
	FailReason string
}

// GetTask 查询任务状态（GET /tasks/:taskId）。
func (c *Client) GetTask(ctx context.Context, taskID string) (*TaskState, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("taskID 必填")
	}
	var out struct {
		TaskID     string `json:"taskId"`
		Status     string `json:"status"`
		Verdict    string `json:"verdict"`
		FailReason string `json:"failReason"`
	}
	if err := c.do(ctx, http.MethodGet, "/tasks/"+strings.TrimSpace(taskID), nil, &out); err != nil {
		return nil, err
	}
	return &TaskState{TaskID: out.TaskID, Status: out.Status, Verdict: out.Verdict, FailReason: out.FailReason}, nil
}

// SuppressFinding 误报抑制回流：按指纹（≥4 位前缀，全值直传）抑制意见，
// 命中返回 matched=true。matched=false 表示 argus 侧未命中（措辞漂移/已被覆盖）。
func (c *Client) SuppressFinding(ctx context.Context, platform, owner, repo, number, fingerprint string) (bool, error) {
	if !c.Enabled() {
		return false, ErrDisabled
	}
	payload, _ := json.Marshal(map[string]string{
		"platform":    strings.ToLower(strings.TrimSpace(platform)),
		"owner":       strings.TrimSpace(owner),
		"repo":        strings.TrimSpace(repo),
		"number":      strings.TrimSpace(number),
		"fingerprint": fingerprint,
	})
	var out struct {
		Matched     bool   `json:"matched"`
		Fingerprint string `json:"fingerprint"`
	}
	if err := c.do(ctx, http.MethodPost, "/findings/suppress", payload, &out); err != nil {
		return false, err
	}
	return out.Matched, nil
}

// ---------- 内部 ----------

// do 发请求并解 ekit 信封 {code,data,message}：code!=0 或 HTTP 非 2xx 均为错误。
func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("调用 argus 失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("argus 返回 %s: %.200s", resp.Status, string(raw))
	}
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("argus 响应解析失败: %w", err)
	}
	if envelope.Code != 0 {
		return fmt.Errorf("argus 业务错误 code=%d: %s", envelope.Code, envelope.Message)
	}
	if out != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("argus 响应解析失败: %w", err)
		}
	}
	return nil
}

// isDigits 纯十进制校验（与 argus 侧 isDigits 同语义，创建期即拒绝非数字）。
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
