package agent

import (
	"testing"

	types "github.com/yi-nology/code-review-service/internal/domain/finding"
)

// TestComputeRedundancyStatus 验证冗余状态标记（路径 A4）。
func TestComputeRedundancyStatus(t *testing.T) {
	tests := []struct {
		name          string
		group         []FindingWithAgent
		personaTotals map[string]int
		want          string
	}{
		{
			name: "两实例都发现→confirmed",
			group: []FindingWithAgent{
				{AgentName: "sec-1", Persona: "sec"},
				{AgentName: "sec-2", Persona: "sec"},
			},
			personaTotals: map[string]int{"sec": 2},
			want:          types.RedundancyConfirmed,
		},
		{
			name: "两实例仅一个发现→single",
			group: []FindingWithAgent{
				{AgentName: "sec-1", Persona: "sec"},
			},
			personaTotals: map[string]int{"sec": 2},
			want:          types.RedundancySingle,
		},
		{
			name: "单实例persona→空(未配置冗余)",
			group: []FindingWithAgent{
				{AgentName: "sec", Persona: "sec"},
			},
			personaTotals: map[string]int{"sec": 1},
			want:          "",
		},
		{
			name: "三实例全部发现→confirmed",
			group: []FindingWithAgent{
				{AgentName: "p-1", Persona: "p"},
				{AgentName: "p-2", Persona: "p"},
				{AgentName: "p-3", Persona: "p"},
			},
			personaTotals: map[string]int{"p": 3},
			want:          types.RedundancyConfirmed,
		},
		{
			name: "三实例仅两个发现→single(未全部命中)",
			group: []FindingWithAgent{
				{AgentName: "p-1", Persona: "p"},
				{AgentName: "p-2", Persona: "p"},
			},
			personaTotals: map[string]int{"p": 3},
			want:          types.RedundancySingle,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeRedundancyStatus(tc.group, tc.personaTotals)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAggregateDeduplicatesAndMarksRedundancy 端到端验证：聚合同一位置的两实例发现 → confirmed。
func TestAggregateDeduplicatesAndMarksRedundancy(t *testing.T) {
	agg := NewDefaultAggregator(DefaultAggregatorConfig())

	results := []AgentResult{
		{AgentName: "sec-1", Persona: "sec", Findings: []Finding{
			{ID: "a", Path: "x.go", Line: 5, EndLine: 5, Severity: "critical"},
		}},
		{AgentName: "sec-2", Persona: "sec", Findings: []Finding{
			{ID: "b", Path: "x.go", Line: 5, EndLine: 5, Severity: "critical"},
		}},
	}

	out := agg.Aggregate(results)
	if len(out.Findings) != 1 {
		t.Fatalf("expected 1 deduplicated finding, got %d", len(out.Findings))
	}
	if out.Findings[0].RedundancyStatus != types.RedundancyConfirmed {
		t.Fatalf("expected confirmed, got %q", out.Findings[0].RedundancyStatus)
	}
}

// TestMergeFindingsPreservesCodeFields 验证合并时 ExistingCode/SuggestionCode 不丢失。
func TestMergeFindingsPreservesCodeFields(t *testing.T) {
	agg := NewDefaultAggregator(DefaultAggregatorConfig())

	results := []AgentResult{
		{AgentName: "a-1", Persona: "p", Findings: []Finding{
			{ID: "x", Path: "main.go", Line: 10, EndLine: 10, Severity: "major", Body: "from agent 1"},
		}},
		{AgentName: "a-2", Persona: "p", Findings: []Finding{
			{
				ID:             "y",
				Path:           "main.go",
				Line:           10,
				EndLine:        10,
				Severity:       "major",
				Body:           "from agent 2",
				ExistingCode:   "var x = 1",
				SuggestionCode: "const x = 1",
			},
		}},
	}

	out := agg.Aggregate(results)
	if len(out.Findings) != 1 {
		t.Fatalf("expected 1 deduplicated finding, got %d", len(out.Findings))
	}
	f := out.Findings[0]
	if f.ExistingCode != "var x = 1" {
		t.Fatalf("expected ExistingCode %q, got %q", "var x = 1", f.ExistingCode)
	}
	if f.SuggestionCode != "const x = 1" {
		t.Fatalf("expected SuggestionCode %q, got %q", "const x = 1", f.SuggestionCode)
	}
}
