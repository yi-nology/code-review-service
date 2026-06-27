package settings_service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type rulePackageDTO struct {
	ID          uint                   `json:"id,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Languages   []string               `json:"languages,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Includes    []string               `json:"includes,omitempty"`
	Excludes    []string               `json:"excludes,omitempty"`
	Rules       []map[string]string    `json:"rules"`
	Enabled     bool                   `json:"enabled"`
	ProjectRefs []string               `json:"project_refs,omitempty"`
	BuiltIn     bool                   `json:"built_in,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
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

func ListRulePackages(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, handlerInst.rulePackages, "规则包") {
		return
	}
	if err := ensureDefaultRulePackages(ctx); err != nil {
		common.Fail(c, err)
		return
	}
	items, err := handlerInst.rulePackages.List(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}
	resp := make([]rulePackageDTO, 0, len(items))
	for i := range items {
		resp = append(resp, dbRulePackageToDTO(&items[i]))
	}
	common.Success(c, map[string]any{"items": resp})
}

func UpsertRulePackage(ctx context.Context, c *app.RequestContext) {
	var req rulePackageDTO
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if req.ID == 0 && c.Param("id") != "" {
		id, err := parseUintID(c.Param("id"))
		if err != nil {
			common.Fail(c, common.BadRequest("规则包 ID 无效"))
			return
		}
		req.ID = id
	}
	pkg, err := dtoToRulePackage(req)
	if err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if err := handlerInst.rulePackages.Save(ctx, pkg); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"item": dbRulePackageToDTO(pkg)})
}

func DeleteRulePackage(ctx context.Context, c *app.RequestContext) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		common.Fail(c, common.BadRequest("规则包 ID 无效"))
		return
	}
	existing, err := handlerInst.rulePackages.GetByID(ctx, id)
	if err != nil {
		common.Fail(c, common.BadRequest("规则包不存在"))
		return
	}
	if existing.BuiltIn {
		common.Fail(c, common.BadRequest("内置规则包不能删除"))
		return
	}
	if err := handlerInst.rulePackages.Delete(ctx, id); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{})
}

func ListToolPackages(ctx context.Context, c *app.RequestContext) {
	if !common.FeatureGuard(c, handlerInst.toolPackages, "工具包") {
		return
	}
	if err := ensureDefaultToolPackages(ctx); err != nil {
		common.Fail(c, err)
		return
	}
	items, err := handlerInst.toolPackages.List(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}
	resp := make([]toolPackageDTO, 0, len(items))
	for i := range items {
		resp = append(resp, dbToolPackageToDTO(&items[i]))
	}
	common.Success(c, map[string]any{"items": resp})
}

func UpsertToolPackage(ctx context.Context, c *app.RequestContext) {
	var req toolPackageDTO
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if req.ID == 0 && c.Param("id") != "" {
		id, err := parseUintID(c.Param("id"))
		if err != nil {
			common.Fail(c, common.BadRequest("工具包 ID 无效"))
			return
		}
		req.ID = id
	}
	pkg, err := dtoToToolPackage(req)
	if err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if err := handlerInst.toolPackages.Save(ctx, pkg); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{"item": dbToolPackageToDTO(pkg)})
}

func DeleteToolPackage(ctx context.Context, c *app.RequestContext) {
	id, err := parseUintID(c.Param("id"))
	if err != nil {
		common.Fail(c, common.BadRequest("工具包 ID 无效"))
		return
	}
	existing, err := handlerInst.toolPackages.GetByID(ctx, id)
	if err != nil {
		common.Fail(c, common.BadRequest("工具包不存在"))
		return
	}
	if existing.BuiltIn {
		common.Fail(c, common.BadRequest("内置工具包不能删除"))
		return
	}
	if err := handlerInst.toolPackages.Delete(ctx, id); err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, map[string]any{})
}

func ValidateToolPackage(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Config json.RawMessage `json:"config"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	if err := validateToolConfig(req.Config); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
		return
	}
	common.Success(c, map[string]any{"valid": true, "message": "工具配置 JSON 可用"})
}

func dtoToRulePackage(req rulePackageDTO) (*models.OCRRulePackage, error) {
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
	return &models.OCRRulePackage{
		Model:       gorm.Model{ID: req.ID},
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
	}, nil
}

func dtoToToolPackage(req toolPackageDTO) (*models.OCRToolPackage, error) {
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
	return &models.OCRToolPackage{
		Model:       gorm.Model{ID: req.ID},
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		ScanType:    scanType,
		Config:      datatypes.JSON(req.Config),
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

func dbRulePackageToDTO(pkg *models.OCRRulePackage) rulePackageDTO {
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
		Enabled: pkg.Enabled, ProjectRefs: refs, BuiltIn: pkg.BuiltIn,
	}
}

func dbToolPackageToDTO(pkg *models.OCRToolPackage) toolPackageDTO {
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

func marshalJSON(v any) datatypes.JSON {
	if v == nil {
		return nil
	}
	return datatypes.JSON(mustMarshalJSON(v))
}

func unmarshalJSON(raw []byte, out any) {
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, out)
	}
}
