// Package argusclient 是 review-service 对 Argus（PR 自动审查 Bot）出站调用的唯一收口。
//
// 与 internal/ocr 作为 OCR 单一入口同范式：触发审查任务（trigger 模式）、
// 查询任务状态（stale 兜底）、误报抑制回流共用同一 base URL / 凭证 / 超时，
// 凭证只在服务端（argus.bot_url + argus.bot_token，即 argus 侧 ARGUS_APITOKEN）。
//
// 契约来源：lacus/argus idl/argus.thrift + internal/service/service.go
//   - POST /tasks  {"kind","platform","owner","repo","number","artifacts?"}
//     → {code:0,data:{taskId,status,duplicated?}}；同 PR 在飞任务幂等返回既有任务
//   - GET  /tasks/:taskId → {code:0,data:{taskId,status,verdict?,findings?,report?,failReason?}}
//   - GET  /tasks?status=&limit= → {code:0,data:{tasks:[TaskSummary]}}
//   - GET  /usage/monthly → {code:0,data:{month,total,budget,repos:[...]}}
//   - GET  /reports/health → {code:0,data:{generated_at,repos[],empty}}（手工注册路由，IDL 外）
//   - POST /findings/suppress {"platform","owner","repo","number","fingerprint"|"file"+"comment"}
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

// TaskKind argus 任务类型（IDL TaskKind 枚举，线上传输为整数）。
const (
	// TaskKindRequirementCompliance 需求复核（R1→R5 workflow：需求解析→变更理解→
	// 完成度判定→插件评审→真值表裁决）。
	TaskKindRequirementCompliance = 0
	// TaskKindCodeReview 代码评审（R4 插件评审 + R5 裁决，不看需求完成度）。
	TaskKindCodeReview = 1
	// TaskKindVisualCheck 视觉检查（路由到 skills=[visual-check] 插件，argus 侧默认 disabled）。
	TaskKindVisualCheck = 2
)

// supportedPlatforms argus 侧支持的平台白名单（argus 自身 webhook 只接
// gitea/github/gitlab，TaskAPI 的 platform 同源校验）。review-service 侧
// resolveEngine 据此对 gerrit/gitcode/tencent_code + argus 策略 fail-fast。
var supportedPlatforms = map[string]bool{
	"gitea":  true,
	"github": true,
	"gitlab": true,
}

// SupportsPlatform 平台是否在 argus 支持面内（小写归一后判断）。
func SupportsPlatform(platform string) bool {
	return supportedPlatforms[strings.ToLower(strings.TrimSpace(platform))]
}

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

// TriggerOptions 创建 argus 审查任务的入参。
type TriggerOptions struct {
	Platform string
	Owner    string
	Repo     string
	// Number 纯十进制 PR/MR 序号（argus 只按 PR 维度审查）。
	Number string
	// Kind 任务类型，必须显式设置（0 是合法值 TaskKindRequirementCompliance，
	// 不能依赖零值当默认）：TaskKindCodeReview / TaskKindRequirementCompliance /
	// TaskKindVisualCheck。其余值在客户端即拒绝。
	Kind int
	// Artifacts 自定义产物注入（如需求清单）；可为 nil。
	Artifacts map[string]string
}

// TriggerTask 创建 argus 审查任务（异步：argus 完成后经 sink push 回报）。
func (c *Client) TriggerTask(ctx context.Context, opts TriggerOptions) (*TriggerResult, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	if opts.Owner == "" || opts.Repo == "" {
		return nil, fmt.Errorf("owner/repo 必填")
	}
	if !isDigits(opts.Number) {
		return nil, fmt.Errorf("number 必须为纯十进制 PR 序号: %q", opts.Number)
	}
	kind := opts.Kind
	if kind != TaskKindRequirementCompliance && kind != TaskKindCodeReview && kind != TaskKindVisualCheck {
		return nil, fmt.Errorf("非法任务类型 kind=%d", kind)
	}
	body := map[string]any{
		"kind":     kind,
		"platform": strings.ToLower(strings.TrimSpace(opts.Platform)),
		"owner":    strings.TrimSpace(opts.Owner),
		"repo":     strings.TrimSpace(opts.Repo),
		"number":   strings.TrimSpace(opts.Number),
	}
	if len(opts.Artifacts) > 0 {
		body["artifacts"] = opts.Artifacts
	}
	payload, _ := json.Marshal(body)
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

// GetTask 查询任务状态（GET /tasks/:taskId，轮询兜底用轻量三件套）。
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

// TaskFinding argus 侧意见（GET /tasks/:id 的 Finding）。
// 注意：pull 通道不含 fp 与 suggested_fix（sink push 回报才有完整指纹/建议），
// 由本通道恢复的意见无法参与抑制回流，属降级恢复。
type TaskFinding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Comment  string `json:"comment"`
	By       string `json:"by"`
}

// TaskDetail 任务全量详情（GET /tasks/:taskId）。
type TaskDetail struct {
	TaskID     string
	Status     string // pending|running|done|failed|stale
	Verdict    string
	FailReason string
	ReportRef  string
	// Report 完整 markdown 报告（argus 侧落库后才非空）。
	Report   string
	Findings []TaskFinding
}

// GetTaskDetail 拉取任务全量详情（状态 + verdict + findings + report 全文）。
// sink 回推丢失时的恢复通道，也供「查看完整报告」代理端点使用。
func (c *Client) GetTaskDetail(ctx context.Context, taskID string) (*TaskDetail, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("taskID 必填")
	}
	var out struct {
		TaskID     string        `json:"taskId"`
		Status     string        `json:"status"`
		Verdict    string        `json:"verdict"`
		FailReason string        `json:"failReason"`
		ReportRef  string        `json:"reportRef"`
		Report     string        `json:"report"`
		Findings   []TaskFinding `json:"findings"`
	}
	if err := c.do(ctx, http.MethodGet, "/tasks/"+strings.TrimSpace(taskID), nil, &out); err != nil {
		return nil, err
	}
	return &TaskDetail{
		TaskID: out.TaskID, Status: out.Status, Verdict: out.Verdict,
		FailReason: out.FailReason, ReportRef: out.ReportRef, Report: out.Report,
		Findings: out.Findings,
	}, nil
}

// TaskSummary argus 侧任务概要（GET /tasks 列表项）。
type TaskSummary struct {
	TaskID    string `json:"taskId"`
	Status    string `json:"status"`
	Kind      string `json:"kind"` // 列表里 kind 是字符串（requirement_compliance|code_review|visual_check）
	Platform  string `json:"platform"`
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	Number    string `json:"number"`
	HeadSHA   string `json:"headSha"`
	Verdict   string `json:"verdict"`
	CreatedAt int64  `json:"createdAt"` // Unix 秒
}

// ListTasks 拉取 argus 侧任务列表（status 空=全部；limit 上限 100）。对账扫描用。
func (c *Client) ListTasks(ctx context.Context, status string, limit int) ([]TaskSummary, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	q := ""
	params := make([]string, 0, 2)
	if s := strings.TrimSpace(status); s != "" {
		params = append(params, "status="+s)
	}
	if limit > 0 {
		params = append(params, fmt.Sprintf("limit=%d", limit))
	}
	if len(params) > 0 {
		q = "?" + strings.Join(params, "&")
	}
	var out struct {
		Tasks []TaskSummary `json:"tasks"`
	}
	if err := c.do(ctx, http.MethodGet, "/tasks"+q, nil, &out); err != nil {
		return nil, err
	}
	return out.Tasks, nil
}

// RepoTokenUsage 单仓库本月 token 用量（GET /usage/monthly 的 repos 项）。
type RepoTokenUsage struct {
	Platform string `json:"platform"`
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	Tokens   int64  `json:"tokens"`
	Tasks    int64  `json:"tasks"`
}

// UsageMonthly 本月各仓库 token 用量（GET /usage/monthly）。
type UsageMonthly struct {
	Month  string           `json:"month"`
	Total  int64            `json:"total"`
	Budget int64            `json:"budget"` // 0 = argus 侧未设预算
	Repos  []RepoTokenUsage `json:"repos"`
}

// GetUsageMonthly 拉取 argus 本月各仓库 token 用量。
func (c *Client) GetUsageMonthly(ctx context.Context) (*UsageMonthly, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	var out UsageMonthly
	if err := c.do(ctx, http.MethodGet, "/usage/monthly", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetReportHealth 拉取 argus 审查健康报告（GET /reports/health，手工注册路由）。
// 学习环飞轮数据，结构跨版本可能扩展，此处透传原始 JSON 由前端按需渲染。
func (c *Client) GetReportHealth(ctx context.Context) (json.RawMessage, error) {
	if !c.Enabled() {
		return nil, ErrDisabled
	}
	var out json.RawMessage
	if err := c.do(ctx, http.MethodGet, "/reports/health", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
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
	return c.suppress(ctx, payload)
}

// SuppressFindingByComment 误报抑制回流的 file+comment 兜底模式：fp 缺失
// （旧数据）或指纹未命中（措辞漂移）时，按 file+意见原文匹配。
func (c *Client) SuppressFindingByComment(ctx context.Context, platform, owner, repo, number, file, comment string) (bool, error) {
	if !c.Enabled() {
		return false, ErrDisabled
	}
	payload, _ := json.Marshal(map[string]string{
		"platform": strings.ToLower(strings.TrimSpace(platform)),
		"owner":    strings.TrimSpace(owner),
		"repo":     strings.TrimSpace(repo),
		"number":   strings.TrimSpace(number),
		"file":     file,
		"comment":  comment,
	})
	return c.suppress(ctx, payload)
}

// suppress 发抑制请求并解析 matched。
func (c *Client) suppress(ctx context.Context, payload []byte) (bool, error) {
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
