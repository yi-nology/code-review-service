package policy_test

import (
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/policy"
)

func TestEvaluateNoFindingsPasses(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "critical"})
	decision := engine.Evaluate(nil)

	if !decision.Passed {
		t.Fatalf("expected pass with no findings, got failed: %s", decision.Reason)
	}
}

func TestEvaluateEmptyFindingsPasses(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "critical"})
	decision := engine.Evaluate([]policy.Finding{})

	if !decision.Passed {
		t.Fatalf("expected pass with empty findings, got failed: %s", decision.Reason)
	}
}

func TestEvaluateCriticalFindingFailsOnCritical(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "critical"})
	findings := []policy.Finding{
		{Severity: "critical", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if decision.Passed {
		t.Fatal("expected fail with critical finding on critical threshold")
	}
	if decision.Reason == "" {
		t.Fatal("expected a failure reason")
	}
}

func TestEvaluateMajorFindingPassesOnCritical(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "critical"})
	findings := []policy.Finding{
		{Severity: "major", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if !decision.Passed {
		t.Fatalf("expected pass with major finding on critical threshold, got failed: %s", decision.Reason)
	}
}

func TestEvaluateMinorFindingPassesOnCritical(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "critical"})
	findings := []policy.Finding{
		{Severity: "minor", Path: "util.go"},
	}

	decision := engine.Evaluate(findings)
	if !decision.Passed {
		t.Fatalf("expected pass with minor finding on critical threshold, got failed: %s", decision.Reason)
	}
}

func TestEvaluateInfoFindingPassesOnCritical(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "critical"})
	findings := []policy.Finding{
		{Severity: "info", Path: "util.go"},
	}

	decision := engine.Evaluate(findings)
	if !decision.Passed {
		t.Fatalf("expected pass with info finding on critical threshold, got failed: %s", decision.Reason)
	}
}

func TestEvaluateMajorFindingFailsOnMajor(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "major"})
	findings := []policy.Finding{
		{Severity: "major", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if decision.Passed {
		t.Fatal("expected fail with major finding on major threshold")
	}
}

func TestEvaluateCriticalFindingFailsOnMajor(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "major"})
	findings := []policy.Finding{
		{Severity: "critical", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if decision.Passed {
		t.Fatal("expected fail with critical finding on major threshold")
	}
}

func TestEvaluateMinorFindingPassesOnMajor(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "major"})
	findings := []policy.Finding{
		{Severity: "minor", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if !decision.Passed {
		t.Fatalf("expected pass with minor finding on major threshold, got failed: %s", decision.Reason)
	}
}

func TestEvaluateMinorFindingFailsOnMinor(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "minor"})
	findings := []policy.Finding{
		{Severity: "minor", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if decision.Passed {
		t.Fatal("expected fail with minor finding on minor threshold")
	}
}

func TestEvaluateInfoFindingFailsOnInfo(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "info"})
	findings := []policy.Finding{
		{Severity: "info", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if decision.Passed {
		t.Fatal("expected fail with info finding on info threshold")
	}
}

func TestEvaluateIgnorePaths(t *testing.T) {
	engine := policy.NewEngine(policy.Config{
		FailOn:      "critical",
		IgnorePaths: []string{"vendor/**", "*.generated.go"},
	})
	findings := []policy.Finding{
		{Severity: "critical", Path: "vendor/lib/utils.go"},
		{Severity: "critical", Path: "proto.generated.go"},
		{Severity: "major", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if !decision.Passed {
		t.Fatalf("expected pass when critical findings are in ignored paths, got failed: %s", decision.Reason)
	}
}

func TestEvaluateIgnorePathsDoesNotIgnoreNonMatching(t *testing.T) {
	engine := policy.NewEngine(policy.Config{
		FailOn:      "critical",
		IgnorePaths: []string{"vendor/**"},
	})
	findings := []policy.Finding{
		{Severity: "critical", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if decision.Passed {
		t.Fatal("expected fail when critical finding is not in ignored paths")
	}
}

func TestEvaluateDefaultThresholdIsCritical(t *testing.T) {
	engine := policy.NewEngine(policy.Config{})
	findings := []policy.Finding{
		{Severity: "major", Path: "main.go"},
	}

	decision := engine.Evaluate(findings)
	if !decision.Passed {
		t.Fatalf("expected pass with major finding on default (critical) threshold, got failed: %s", decision.Reason)
	}
}

func TestEvaluateMultipleFindingsStopsOnFirst(t *testing.T) {
	engine := policy.NewEngine(policy.Config{FailOn: "critical"})
	findings := []policy.Finding{
		{Severity: "minor", Path: "a.go"},
		{Severity: "critical", Path: "b.go"},
		{Severity: "critical", Path: "c.go"},
	}

	decision := engine.Evaluate(findings)
	if decision.Passed {
		t.Fatal("expected fail with critical finding in list")
	}
}

func TestEngineDynamicSource(t *testing.T) {
	cur := "major"
	e := policy.NewEngine(policy.Config{FailOn: "major"}).WithSource(func() policy.Config {
		return policy.Config{FailOn: cur}
	})

	findings := []policy.Finding{{Severity: "major", Path: "a.go"}}
	if d := e.Evaluate(findings); d.Passed {
		t.Fatal("major>=major 应不通过")
	}

	// 配置热更新：调宽阈值后同一批 findings 通过，无需重建引擎。
	cur = "critical"
	if d := e.Evaluate(findings); !d.Passed {
		t.Fatalf("热更新后应通过: %+v", d)
	}

	// source 返回零值（读取失败）回落启动配置。
	e2 := policy.NewEngine(policy.Config{FailOn: "critical"}).WithSource(func() policy.Config { return policy.Config{} })
	if d := e2.Evaluate(findings); !d.Passed {
		t.Fatalf("零值 source 应回落启动配置: %+v", d)
	}
}

func TestEngineWithFailOnOverrideKeepsDefaults(t *testing.T) {
	// 回归锚点：项目 fail_on 覆盖曾用裸 Config 新建引擎，把全局 IgnorePaths 丢掉。
	e := policy.NewEngine(policy.Config{FailOn: "minor", IgnorePaths: []string{"vendor/**"}}).
		WithFailOnOverride("critical")

	d := e.Evaluate([]policy.Finding{{Severity: "major", Path: "a.go"}})
	if !d.Passed {
		t.Fatalf("major < critical 应通过: %+v", d)
	}
	d = e.Evaluate([]policy.Finding{{Severity: "major", Path: "vendor/x.go"}})
	if !d.Passed {
		t.Fatalf("覆盖后仍应保留 IgnorePaths: %+v", d)
	}
	d = e.Evaluate([]policy.Finding{{Severity: "critical", Path: "a.go"}})
	if d.Passed {
		t.Fatal("critical >= critical 应不通过")
	}
}
