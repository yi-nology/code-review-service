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
