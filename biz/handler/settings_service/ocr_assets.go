package settings_service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/domain/ocrasset"
	"github.com/yi-nology/code-review-service/internal/ocr"

	"github.com/cloudwego/hertz/pkg/app"
)

type rulePackageDTO struct {
	ID          uint                `json:"id,omitempty"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Languages   []string            `json:"languages,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Includes    []string            `json:"includes,omitempty"`
	Excludes    []string            `json:"excludes,omitempty"`
	Rules       []map[string]string `json:"rules"`
	Enabled     bool                `json:"enabled"`
	ProjectRefs []string            `json:"project_refs,omitempty"`
	BuiltIn     bool                `json:"built_in,omitempty"`
	// Scope 区分规则集来源：global=全局始终合并；project=需引用或 project_refs 匹配。空视为 project。
	Scope string                 `json:"scope,omitempty"`
	Meta  map[string]interface{} `json:"meta,omitempty"`
}

type toolPackageDTO struct {
	ID          uint                   `json:"id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	ScanType    string                 `json:"scan_type,omitempty"`
	Config      json.RawMessage        `json:"config"`
	Enabled     bool                   `json:"enabled"`
	ProjectRefs []string               `json:"project_refs,omitempty"`
	BuiltIn     bool                   `json:"built_in,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

type assetVersionDTO struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Snapshot  any       `json:"snapshot"`
}

func (h *SettingsHandler) listRulePackages(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.rulePackages, "规则包") {
		return
	}
	if err := ensureDefaultRulePackages(ctx); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	items, err := h.rulePackages.List(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := make([]rulePackageDTO, 0, len(items))
	for i := range items {
		resp = append(resp, dbRulePackageToDTO(&items[i]))
	}
	common.Success(c, map[string]any{"items": resp})
}

// ListRulePackages 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListRulePackages(ctx context.Context, c *app.RequestContext) {
	handlerInst.listRulePackages(ctx, c)
}

func (h *SettingsHandler) upsertRulePackage(ctx context.Context, c *app.RequestContext) {
	upsertRulePackage(ctx, c)
}

// UpsertRulePackage 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func UpsertRulePackage(ctx context.Context, c *app.RequestContext) {
	handlerInst.upsertRulePackage(ctx, c)
}

func (h *SettingsHandler) updateRulePackage(ctx context.Context, c *app.RequestContext) {
	upsertRulePackage(ctx, c)
}

// UpdateRulePackage 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func UpdateRulePackage(ctx context.Context, c *app.RequestContext) {
	handlerInst.updateRulePackage(ctx, c)
}

func upsertRulePackage(ctx context.Context, c *app.RequestContext) {
	var req rulePackageDTO
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	if req.ID == 0 && c.Param("id") != "" {
		id, err := common.ParseUintID(c.Param("id"))
		if err != nil {
			common.Fail(ctx, c, common.BadRequest("规则包 ID 无效"))
			return
		}
		req.ID = id
	}
	pkg, err := dtoToRulePackage(req)
	if err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	if err := handlerInst.rulePackages.Save(ctx, pkg); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"item": dbRulePackageToDTO(pkg)})
}

func (h *SettingsHandler) deleteRulePackage(ctx context.Context, c *app.RequestContext) {
	id, err := common.ParseUintID(c.Param("id"))
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("规则包 ID 无效"))
		return
	}
	existing, err := h.rulePackages.GetByID(ctx, id)
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("规则包不存在"))
		return
	}
	if existing.BuiltIn {
		common.Fail(ctx, c, common.BadRequest("内置规则包不能删除"))
		return
	}
	if err := h.rulePackages.Delete(ctx, id); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{})
}

// DeleteRulePackage 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func DeleteRulePackage(ctx context.Context, c *app.RequestContext) {
	handlerInst.deleteRulePackage(ctx, c)
}

func (h *SettingsHandler) listRulePackageVersions(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.rulePackages, "规则包") {
		return
	}
	id, err := common.ParseUintID(c.Param("id"))
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("规则包 ID 无效"))
		return
	}
	items, err := h.rulePackages.ListVersions(ctx, id)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := make([]assetVersionDTO, 0, len(items))
	for i := range items {
		var snapshot ocrasset.RulePackage
		_ = json.Unmarshal(items[i].Snapshot, &snapshot)
		resp = append(resp, assetVersionDTO{Version: items[i].Version, CreatedAt: items[i].CreatedAt, Snapshot: dbRulePackageToDTO(&snapshot)})
	}
	common.Success(c, map[string]any{"items": resp})
}

// ListRulePackageVersions 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListRulePackageVersions(ctx context.Context, c *app.RequestContext) {
	handlerInst.listRulePackageVersions(ctx, c)
}

func (h *SettingsHandler) restoreRulePackageVersion(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.rulePackages, "规则包") {
		return
	}
	id, err := common.ParseUintID(c.Param("id"))
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("规则包 ID 无效"))
		return
	}
	version, err := strconv.Atoi(strings.TrimSpace(c.Param("version")))
	if err != nil || version <= 0 {
		common.Fail(ctx, c, common.BadRequest("版本号无效"))
		return
	}
	pkg, err := h.rulePackages.RestoreVersion(ctx, id, version)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"item": dbRulePackageToDTO(pkg)})
}

// RestoreRulePackageVersion 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func RestoreRulePackageVersion(ctx context.Context, c *app.RequestContext) {
	handlerInst.restoreRulePackageVersion(ctx, c)
}

func (h *SettingsHandler) listToolPackages(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.toolPackages, "工具包") {
		return
	}
	if err := ensureDefaultToolPackages(ctx); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	items, err := h.toolPackages.List(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := make([]toolPackageDTO, 0, len(items))
	for i := range items {
		resp = append(resp, dbToolPackageToDTO(&items[i]))
	}
	common.Success(c, map[string]any{"items": resp})
}

// ListToolPackages 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListToolPackages(ctx context.Context, c *app.RequestContext) {
	handlerInst.listToolPackages(ctx, c)
}

func (h *SettingsHandler) upsertToolPackage(ctx context.Context, c *app.RequestContext) {
	upsertToolPackage(ctx, c)
}

// UpsertToolPackage 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func UpsertToolPackage(ctx context.Context, c *app.RequestContext) {
	handlerInst.upsertToolPackage(ctx, c)
}

func (h *SettingsHandler) updateToolPackage(ctx context.Context, c *app.RequestContext) {
	upsertToolPackage(ctx, c)
}

// UpdateToolPackage 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func UpdateToolPackage(ctx context.Context, c *app.RequestContext) {
	handlerInst.updateToolPackage(ctx, c)
}

func upsertToolPackage(ctx context.Context, c *app.RequestContext) {
	var req toolPackageDTO
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	if req.ID == 0 && c.Param("id") != "" {
		id, err := common.ParseUintID(c.Param("id"))
		if err != nil {
			common.Fail(ctx, c, common.BadRequest("工具包 ID 无效"))
			return
		}
		req.ID = id
	}
	pkg, err := dtoToToolPackage(req)
	if err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	if err := handlerInst.toolPackages.Save(ctx, pkg); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"item": dbToolPackageToDTO(pkg)})
}

func (h *SettingsHandler) deleteToolPackage(ctx context.Context, c *app.RequestContext) {
	id, err := common.ParseUintID(c.Param("id"))
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("工具包 ID 无效"))
		return
	}
	existing, err := h.toolPackages.GetByID(ctx, id)
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("工具包不存在"))
		return
	}
	if existing.BuiltIn {
		common.Fail(ctx, c, common.BadRequest("内置工具包不能删除"))
		return
	}
	if err := h.toolPackages.Delete(ctx, id); err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{})
}

// DeleteToolPackage 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func DeleteToolPackage(ctx context.Context, c *app.RequestContext) {
	handlerInst.deleteToolPackage(ctx, c)
}

func (h *SettingsHandler) listToolPackageVersions(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.toolPackages, "工具包") {
		return
	}
	id, err := common.ParseUintID(c.Param("id"))
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("工具包 ID 无效"))
		return
	}
	items, err := h.toolPackages.ListVersions(ctx, id)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	resp := make([]assetVersionDTO, 0, len(items))
	for i := range items {
		var snapshot ocrasset.ToolPackage
		_ = json.Unmarshal(items[i].Snapshot, &snapshot)
		resp = append(resp, assetVersionDTO{Version: items[i].Version, CreatedAt: items[i].CreatedAt, Snapshot: dbToolPackageToDTO(&snapshot)})
	}
	common.Success(c, map[string]any{"items": resp})
}

// ListToolPackageVersions 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ListToolPackageVersions(ctx context.Context, c *app.RequestContext) {
	handlerInst.listToolPackageVersions(ctx, c)
}

func (h *SettingsHandler) restoreToolPackageVersion(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, h.toolPackages, "工具包") {
		return
	}
	id, err := common.ParseUintID(c.Param("id"))
	if err != nil {
		common.Fail(ctx, c, common.BadRequest("工具包 ID 无效"))
		return
	}
	version, err := strconv.Atoi(strings.TrimSpace(c.Param("version")))
	if err != nil || version <= 0 {
		common.Fail(ctx, c, common.BadRequest("版本号无效"))
		return
	}
	pkg, err := h.toolPackages.RestoreVersion(ctx, id, version)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, map[string]any{"item": dbToolPackageToDTO(pkg)})
}

// RestoreToolPackageVersion 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func RestoreToolPackageVersion(ctx context.Context, c *app.RequestContext) {
	handlerInst.restoreToolPackageVersion(ctx, c)
}

func (h *SettingsHandler) validateToolPackage(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Config json.RawMessage `json:"config"`
		DryRun bool            `json:"dry_run"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	if err := validateToolConfig(req.Config); err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	resp := map[string]any{"valid": true, "message": "工具配置 JSON 可用"}
	if req.DryRun {
		output, err := dryRunToolConfig(ctx, req.Config)
		if err != nil {
			common.Fail(ctx, c, common.BadRequest(err.Error()))
			return
		}
		resp["message"] = "工具配置已通过 OCR dry-run"
		resp["dry_run"] = true
		resp["output"] = output
	}
	common.Success(c, resp)
}

// ValidateToolPackage 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func ValidateToolPackage(ctx context.Context, c *app.RequestContext) {
	handlerInst.validateToolPackage(ctx, c)
}

func dtoToRulePackage(req rulePackageDTO) (*ocrasset.RulePackage, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("规则包名称不能为空")
	}
	if len(req.Rules) == 0 {
		return nil, fmt.Errorf("规则包至少需要一条规则")
	}
	for i, rule := range req.Rules {
		if strings.TrimSpace(rule["path"]) == "" || strings.TrimSpace(rule["rule"]) == "" {
			return nil, fmt.Errorf("第 %d 条规则缺少 path 或 rule", i+1)
		}
	}
	return &ocrasset.RulePackage{
		ID:          req.ID,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Languages:   marshalJSON(req.Languages),
		Tags:        marshalJSON(req.Tags),
		Includes:    marshalJSON(req.Includes),
		Excludes:    marshalJSON(req.Excludes),
		Rules:       marshalJSON(req.Rules),
		Enabled:     req.Enabled,
		ProjectRefs: marshalJSON(req.ProjectRefs),
		BuiltIn:     req.BuiltIn,
		Scope:       strings.TrimSpace(req.Scope),
	}, nil
}

func dtoToToolPackage(req toolPackageDTO) (*ocrasset.ToolPackage, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("工具包名称不能为空")
	}
	if err := validateToolConfig(req.Config); err != nil {
		return nil, err
	}
	scanType := strings.TrimSpace(req.ScanType)
	if scanType == "" {
		scanType = "all"
	}
	switch scanType {
	case "all", "diff", "scan":
	default:
		return nil, fmt.Errorf("scan_type 仅支持 all、diff、scan")
	}
	return &ocrasset.ToolPackage{
		ID:          req.ID,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		ScanType:    scanType,
		Config:      json.RawMessage(req.Config),
		Enabled:     req.Enabled,
		ProjectRefs: marshalJSON(req.ProjectRefs),
		BuiltIn:     req.BuiltIn,
	}, nil
}

func validateToolConfig(raw json.RawMessage) error {
	if len(raw) == 0 || !json.Valid(raw) {
		return fmt.Errorf("工具配置必须是合法 JSON")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("工具配置必须是合法 JSON")
	}
	switch typed := value.(type) {
	case []any:
		for i, item := range typed {
			obj, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("第 %d 个工具配置必须是对象", i+1)
			}
			if strings.TrimSpace(fmt.Sprint(obj["name"])) == "" {
				return fmt.Errorf("第 %d 个工具缺少 name", i+1)
			}
			if _, ok := obj["definition"]; !ok {
				return fmt.Errorf("第 %d 个工具缺少 definition", i+1)
			}
		}
	case map[string]any:
		if _, ok := typed["tools"]; !ok {
			return fmt.Errorf("对象格式工具配置需要包含 tools 字段")
		}
	default:
		return fmt.Errorf("工具配置必须是数组或包含 tools 的对象")
	}
	return nil
}

func dryRunToolConfig(ctx context.Context, raw json.RawMessage) (string, error) {
	binary := "ocr"
	if handlerInst != nil && handlerInst.ocrRepo != nil {
		if settings, err := handlerInst.ocrRepo.Get(ctx); err == nil && strings.TrimSpace(settings.Binary) != "" {
			binary = strings.TrimSpace(settings.Binary)
		}
	}

	tmpDir, err := os.MkdirTemp("", "ocr-tools-dry-run-*")
	if err != nil {
		return "", fmt.Errorf("创建 dry-run 临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		return "", fmt.Errorf("写入 dry-run 文件失败: %w", err)
	}
	toolsPath := filepath.Join(tmpDir, "tools.json")
	if err := os.WriteFile(toolsPath, raw, 0o600); err != nil {
		return "", fmt.Errorf("写入 tools 配置失败: %w", err)
	}
	if _, err := runDryRunCommand(ctx, tmpDir, "git", "-C", tmpDir, "init", "-q"); err != nil {
		return "", err
	}
	if _, err := runDryRunCommand(ctx, tmpDir, "git", "-C", tmpDir, "add", "main.go"); err != nil {
		return "", err
	}
	if _, err := runDryRunCommand(ctx, tmpDir, "git", "-C", tmpDir, "-c", "user.name=review-service", "-c", "user.email=review-service@example.invalid", "commit", "-q", "-m", "init"); err != nil {
		return "", err
	}

	output, err := runDryRunCommand(ctx, tmpDir, binary, "scan", "--preview", "--repo", tmpDir, "--path", "main.go", "--tools", toolsPath, "--audience", "agent")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// runDryRunCommand 阶段 A9 改为通过 ocr.Run 统一 exec 入口。
// 取代直接 exec.CommandContext 硬编码 + 自管 SIGKILL 三段式。
func runDryRunCommand(parent context.Context, dir, name string, args ...string) (string, error) {
	res, err := ocr.InternalRun(parent, ocr.CommandSpec{
		Binary:  name,
		Args:    args,
		Dir:     dir,
		Timeout: 15 * time.Second,
	})
	text := strings.TrimSpace(string(res.Stdout))
	if res.Stderr != nil {
		stderrText := strings.TrimSpace(string(res.Stderr))
		if stderrText != "" && text == "" {
			text = stderrText
		}
	}
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, fmt.Errorf("OCR dry-run 失败: %s", text)
	}
	return text, nil
}

func dbRulePackageToDTO(pkg *ocrasset.RulePackage) rulePackageDTO {
	var rules []map[string]string
	var languages, tags, includes, excludes, refs []string
	unmarshalJSON(pkg.Rules, &rules)
	unmarshalJSON(pkg.Languages, &languages)
	unmarshalJSON(pkg.Tags, &tags)
	unmarshalJSON(pkg.Includes, &includes)
	unmarshalJSON(pkg.Excludes, &excludes)
	unmarshalJSON(pkg.ProjectRefs, &refs)
	return rulePackageDTO{
		ID: pkg.ID, Name: pkg.Name, Description: pkg.Description, Languages: languages,
		Tags: tags, Includes: includes, Excludes: excludes, Rules: rules,
		Enabled: pkg.Enabled, ProjectRefs: refs, BuiltIn: pkg.BuiltIn, Scope: pkg.Scope,
	}
}

func dbToolPackageToDTO(pkg *ocrasset.ToolPackage) toolPackageDTO {
	var refs []string
	unmarshalJSON(pkg.ProjectRefs, &refs)
	return toolPackageDTO{
		ID: pkg.ID, Name: pkg.Name, Description: pkg.Description, ScanType: pkg.ScanType,
		Config: json.RawMessage(pkg.Config), Enabled: pkg.Enabled, ProjectRefs: refs, BuiltIn: pkg.BuiltIn,
	}
}

func ensureDefaultRulePackages(ctx context.Context) error {
	items, err := handlerInst.rulePackages.List(ctx)
	if err != nil || len(items) > 0 {
		return err
	}
	for _, pkg := range defaultRulePackages() {
		item, err := dtoToRulePackage(pkg)
		if err != nil {
			return err
		}
		item.BuiltIn = true
		if err := handlerInst.rulePackages.Save(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultToolPackages(ctx context.Context) error {
	items, err := handlerInst.toolPackages.List(ctx)
	if err != nil || len(items) > 0 {
		return err
	}
	for _, pkg := range defaultToolPackages() {
		item, err := dtoToToolPackage(pkg)
		if err != nil {
			return err
		}
		item.BuiltIn = true
		if err := handlerInst.toolPackages.Save(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func defaultRulePackages() []rulePackageDTO {
	return []rulePackageDTO{
		{Name: "Java 后端", Description: "Spring/Java 服务质量、安全和事务检查", Languages: []string{"java"}, Tags: []string{"backend", "java"}, Includes: []string{"**/*.java"}, Enabled: true, Rules: []map[string]string{{"path": "**/*.java", "rule": "重点检查鉴权、事务边界、空指针、并发安全、资源释放、SQL 注入、日志敏感信息和接口兼容性。"}}},
		{Name: "Vue 前端", Description: "Vue/TypeScript 交互质量检查", Languages: []string{"typescript", "vue"}, Tags: []string{"frontend", "vue"}, Includes: []string{"**/*.{vue,ts,tsx}"}, Enabled: true, Rules: []map[string]string{{"path": "**/*.{vue,ts,tsx}", "rule": "重点检查响应式状态、表单校验、接口错误处理、权限态、加载/空状态、可访问性和移动端布局。"}}},
		{Name: "SQL Mapper", Description: "SQL、MyBatis、Mapper 性能和安全检查", Languages: []string{"sql", "xml"}, Tags: []string{"database"}, Includes: []string{"**/*.{sql,xml}"}, Enabled: true, Rules: []map[string]string{{"path": "**/*.{sql,xml}", "rule": "重点检查索引缺失、全表扫描、N+1 查询、动态 SQL 注入、分页排序稳定性、事务一致性和软删除条件。"}}},
		{Name: "安全专项", Description: "跨语言安全风险检查", Tags: []string{"security"}, Includes: []string{"**/*"}, Enabled: true, Rules: []map[string]string{{"path": "**/*", "rule": "重点检查认证鉴权、越权访问、敏感信息泄露、输入校验、 SSRF、SQL/命令注入、路径穿越和不安全默认配置。"}}},
		{Name: "性能专项", Description: "热点路径性能和资源风险检查", Tags: []string{"performance"}, Includes: []string{"**/*"}, Enabled: true, Rules: []map[string]string{{"path": "**/*", "rule": "重点检查重复 IO、缓存失效、循环内远程调用、大对象分配、锁竞争、批处理缺失、超时/重试策略和资源泄露。"}}},
	}
}

func defaultToolPackages() []toolPackageDTO {
	return []toolPackageDTO{
		{Name: "标准代码洞察", Description: "保留 OCR 常用只读代码分析工具", ScanType: "all", Enabled: true, Config: json.RawMessage(`[{"name":"file_read","main_task":true,"plan_task":true,"definition":{"name":"file_read","description":"Read repository files","parameters":{"type":"object","properties":{}}}},{"name":"file_read_diff","main_task":true,"plan_task":true,"definition":{"name":"file_read_diff","description":"Read diff hunks","parameters":{"type":"object","properties":{}}}},{"name":"code_search","main_task":true,"plan_task":true,"definition":{"name":"code_search","description":"Search code by keyword","parameters":{"type":"object","properties":{}}}},{"name":"file_find","main_task":true,"plan_task":true,"definition":{"name":"file_find","description":"Find files by pattern","parameters":{"type":"object","properties":{}}}},{"name":"code_comment","main_task":true,"plan_task":false,"definition":{"name":"code_comment","description":"Create code review comments","parameters":{"type":"object","properties":{}}}}]`)},
		{Name: "只读扫描", Description: "用于全量体检，禁用写评论类工具", ScanType: "scan", Enabled: true, Config: json.RawMessage(`[{"name":"file_read","main_task":true,"plan_task":true,"definition":{"name":"file_read","description":"Read repository files","parameters":{"type":"object","properties":{}}}},{"name":"code_search","main_task":true,"plan_task":true,"definition":{"name":"code_search","description":"Search code by keyword","parameters":{"type":"object","properties":{}}}},{"name":"file_find","main_task":true,"plan_task":true,"definition":{"name":"file_find","description":"Find files by pattern","parameters":{"type":"object","properties":{}}}}]`)},
	}
}

func marshalJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	return json.RawMessage(mustMarshalJSON(v))
}

func unmarshalJSON(raw []byte, out any) {
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, out)
	}
}
