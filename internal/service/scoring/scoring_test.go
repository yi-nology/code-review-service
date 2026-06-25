package scoring

import (
	"math"
	"testing"
)

func TestComputePerfectScore(t *testing.T) {
	r := Compute(Counts{}, 0, 0, true)
	if r.Value != 100 {
		t.Fatalf("expected 100, got %d", r.Value)
	}
	if r.Grade != "优秀" {
		t.Fatalf("expected 优秀, got %s", r.Grade)
	}
	if r.Version != Version {
		t.Fatalf("expected %s, got %s", Version, r.Version)
	}
	if len(r.Deductions) != 0 {
		t.Fatalf("expected no deductions, got %d", len(r.Deductions))
	}
}

func TestComputeSeverityDeductions(t *testing.T) {
	r := Compute(Counts{Critical: 1, Major: 1, Minor: 1, Info: 1}, 100, 0, true)

	// changedLines=100 → volumeMultiplier=1.0, each category at base weight
	if r.Value != 100-(25+12+5+1) {
		t.Fatalf("expected %d, got %d", 100-(25+12+5+1), r.Value)
	}
	if len(r.Deductions) != 4 {
		t.Fatalf("expected 4 deductions, got %d", len(r.Deductions))
	}
}

func TestComputeVolumeMultiplierLargePR(t *testing.T) {
	// 1000 changed lines → multiplier clamped to 0.1→0.2 (min)
	// 1 critical: round(25*0.2)=5 → score=95
	r := Compute(Counts{Critical: 1}, 1000, 0, true)
	if r.Value != 95 {
		t.Fatalf("expected 95 (large PR discount), got %d", r.Value)
	}
	if r.ChangedLines != 1000 {
		t.Fatalf("expected changed_lines=1000, got %d", r.ChangedLines)
	}
}

func TestComputeVolumeMultiplierSmallPR(t *testing.T) {
	// 10 changed lines → multiplier clamped to 10→3.0 (max)
	// 1 critical: round(25*3.0)=75 → score=25
	r := Compute(Counts{Critical: 1}, 10, 0, true)
	if r.Value != 25 {
		t.Fatalf("expected 25 (small PR penalty), got %d", r.Value)
	}
}

func TestComputeClampsToZero(t *testing.T) {
	// 10 critical × 25 × multiplier(1.0) = 250 → clamped to 0
	r := Compute(Counts{Critical: 10}, 100, 0, true)
	if r.Value != 0 {
		t.Fatalf("expected 0, got %d", r.Value)
	}
	if r.Grade != "高风险" {
		t.Fatalf("expected 高风险, got %s", r.Grade)
	}
}

func TestComputeWarningDeduction(t *testing.T) {
	r := Compute(Counts{}, 100, 2, true)
	if r.Value != 94 {
		t.Fatalf("expected 94 (100-2*3), got %d", r.Value)
	}
	found := false
	for _, d := range r.Deductions {
		if d.Reason == "OCR 审查警告" && d.Count == 2 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected warning deduction with count=2")
	}
}

func TestComputeWarningCap(t *testing.T) {
	r := Compute(Counts{}, 100, 100, true)
	// 100*3=300 → clamped to 15
	if r.Value != 85 {
		t.Fatalf("expected 85 (100-15 capped), got %d", r.Value)
	}
}

func TestComputeIncompletePenalty(t *testing.T) {
	r := Compute(Counts{}, 100, 0, false)
	if r.Value != 80 {
		t.Fatalf("expected 80 (100-20), got %d", r.Value)
	}
}

func TestComputeLargeCleanPRBonus(t *testing.T) {
	// density = rawDeduction / (changedLines/100)
	// 1 critical, 300 lines: density = 25/(300/100) = 8.33 → no bonus (≥0.5)
	// multiplier=0.333, pts=round(25*0.333)=8 → value=92
	r := Compute(Counts{Critical: 1}, 300, 0, true)
	if r.Value != 92 {
		t.Fatalf("expected 92 (no bonus, density too high), got %d", r.Value)
	}

	// 1 info, 1000 lines: density = 1/(1000/100) = 0.1 → bonus (+5)
	// deduction: round(1 * 0.2) = 0 (multiplier clamped to 0.2)
	// value = 100 - 0 + 5 = 105 → clamped to 100
	r = Compute(Counts{Info: 1}, 1000, 0, true)
	if r.FindingsDensity >= 0.5 {
		t.Fatalf("expected density < 0.5, got %f", r.FindingsDensity)
	}
}

func TestComputeDefaultChangedLines(t *testing.T) {
	// changedLines=0 → treated as 100
	r1 := Compute(Counts{Major: 1}, 0, 0, true)
	r2 := Compute(Counts{Major: 1}, 100, 0, true)
	if r1.Value != r2.Value {
		t.Fatalf("expected same score for changedLines 0 and 100, got %d vs %d", r1.Value, r2.Value)
	}
}

func TestGrade(t *testing.T) {
	cases := []struct {
		score int
		grade string
	}{
		{100, "优秀"},
		{90, "优秀"},
		{89, "良好"},
		{80, "良好"},
		{79, "需改进"},
		{60, "需改进"},
		{59, "高风险"},
		{0, "高风险"},
	}
	for _, c := range cases {
		if g := Grade(c.score); g != c.grade {
			t.Errorf("Grade(%d) = %s, want %s", c.score, g, c.grade)
		}
	}
}

func TestIsOCRComplete(t *testing.T) {
	complete := []string{OCRStatusSuccess, OCRStatusSkipped, OCRStatusCompletedWithErrors}
	incomplete := []string{"", "timeout", "running", "unknown"}

	for _, s := range complete {
		if !IsOCRComplete(s) {
			t.Errorf("IsOCRComplete(%q) = false, want true", s)
		}
	}
	for _, s := range incomplete {
		if IsOCRComplete(s) {
			t.Errorf("IsOCRComplete(%q) = true, want false", s)
		}
	}
}

func TestCountDiffLines(t *testing.T) {
	cases := []struct {
		diff   string
		expect int
	}{
		{"", 0},
		{"+hello\n-world\n context", 2},
		{"+++ file\n--- file\n+real\n", 1},
		{"diff --git a b\n+added\n-removed\n", 2},
	}
	for _, c := range cases {
		if got := CountDiffLines(c.diff); got != c.expect {
			t.Errorf("CountDiffLines() = %d, want %d", got, c.expect)
		}
	}
}

func TestCountDiffFiles(t *testing.T) {
	cases := []struct {
		diff   string
		expect int
	}{
		{"", 0},
		{"diff --git a/foo b/foo\n+foo\n", 1},
		{"diff --git a/foo b/foo\ndiff --git a/bar b/bar\n", 2},
	}
	for _, c := range cases {
		if got := CountDiffFiles(c.diff); got != c.expect {
			t.Errorf("CountDiffFiles() = %d, want %d", got, c.expect)
		}
	}
}

func TestSeverityWeight(t *testing.T) {
	cases := map[string]int{
		"critical": 25,
		"major":    12,
		"minor":    5,
		"info":     1,
		"":         1,
	}
	for sev, want := range cases {
		if got := SeverityWeight(sev); got != want {
			t.Errorf("SeverityWeight(%q) = %d, want %d", sev, got, want)
		}
	}
}

// TestComputeDeductionsMatchScore 验证扣分明细与分数永远来自同一函数、不会矛盾。
// 不变量：value = clamp(100 - sum(deductions.points), 0, 100)。
// 这正是最初 bug 的根因——score 与 deductions 来自不同公式。现在统一后不可能再出现。
func TestComputeDeductionsMatchScore(t *testing.T) {
	cases := []struct {
		name     string
		counts   Counts
		lines    int
		warn     int
		complete bool
	}{
		{"mixed", Counts{Critical: 3, Major: 16, Minor: 5, Info: 2}, 100, 3, false},
		{"major only", Counts{Major: 16}, 100, 0, true},
		{"critical overflow", Counts{Critical: 10}, 100, 0, true},
		{"clean", Counts{}, 100, 0, true},
		{"large clean PR", Counts{Info: 1}, 1000, 0, true},
	}
	for _, c := range cases {
		r := Compute(c.counts, c.lines, c.warn, c.complete)
		sumDeduction := 0
		for _, d := range r.Deductions {
			sumDeduction += d.Points
		}
		unclamped := 100 - sumDeduction
		clamped := int(math.Max(0, math.Min(100, float64(unclamped))))
		if r.Value != clamped {
			t.Errorf("%s: value %d != clamp(100-sumDeductions %d) = %d",
				c.name, r.Value, sumDeduction, clamped)
		}
		if r.Value < 0 || r.Value > 100 {
			t.Errorf("%s: value %d out of range", c.name, r.Value)
		}
		if math.IsNaN(r.FindingsDensity) || math.IsInf(r.FindingsDensity, 0) {
			t.Errorf("%s: density %f not finite", c.name, r.FindingsDensity)
		}
	}
}
