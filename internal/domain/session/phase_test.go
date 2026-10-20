package session

import "testing"

// TestCanTransitionPhase 阶段顺序表的行为约定。
//
// 顺序表是全仓唯一一份（service/session 与 service/progress 都引用它），
// 改错一处会让阶段推进在运行期被拒或放行。
func TestCanTransitionPhase(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"正常推进", PhaseContext, PhaseChangeContext, true},
		{"跳过中间阶段", PhaseContext, PhaseAnalysis, false},
		{"同阶段自迁", PhaseContext, PhaseContext, false},
		{"讨论开启时的分支", PhaseAggregation, PhaseDiscussion, true},
		{"讨论关闭时的分支", PhaseAggregation, PhaseSynthesis, true},
		{"讨论之后只能进综合", PhaseDiscussion, PhaseSynthesis, true},
		{"终态无后继", PhaseComplete, PhaseSynthesis, false},
		{"未知源阶段", "unknown", PhaseAnalysis, false},
		{"未知目标阶段", PhaseContext, "unknown", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanTransitionPhase(tc.from, tc.to); got != tc.want {
				t.Fatalf("CanTransitionPhase(%q, %q) = %v, 期望 %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

// TestIsKnownPhase 未知阶段必须被识别出来。
func TestIsKnownPhase(t *testing.T) {
	if !IsKnownPhase(PhaseSynthesis) {
		t.Fatal("synthesis 应是已知阶段")
	}
	if IsKnownPhase("not_a_phase") {
		t.Fatal("not_a_phase 不应是已知阶段")
	}
}

// TestPhaseOrder 阶段顺序表覆盖全部阶段且终态在末位。
func TestPhaseOrder(t *testing.T) {
	order := PhaseOrder()
	if len(order) != 8 {
		t.Fatalf("阶段数应为 8，实际 %d", len(order))
	}
	if order[0] != PhaseContext {
		t.Fatalf("首个阶段应为 context，实际 %q", order[0])
	}
	if order[len(order)-1] != PhaseComplete {
		t.Fatalf("末个阶段应为 complete，实际 %q", order[len(order)-1])
	}
	// 返回的是拷贝：调用方改它不得影响本包状态。
	order[0] = "mutated"
	if PhaseOrder()[0] != PhaseContext {
		t.Fatal("PhaseOrder 应返回拷贝，调用方改动不得污染包内状态")
	}
}

// TestPhaseInFlightProgress 阶段进行中的 progress 值必须稳定。
//
// 这些值对外可见（会话 API 返回 session.progress），随手改动等于
// 无声地改了对端看到的行为。
func TestPhaseInFlightProgress(t *testing.T) {
	cases := map[string]float64{
		PhaseContext:       0,
		PhaseChangeContext: 50,
		PhaseAnalysis:      50,
		PhaseReviews:       0,
		PhaseAggregation:   50,
		PhaseDiscussion:    0,
		PhaseSynthesis:     0,
		PhaseComplete:      0,
	}
	for phase, want := range cases {
		if got := PhaseInFlightProgress(phase); got != want {
			t.Fatalf("阶段 %q 的 progress 应为 %v，实际 %v", phase, want, got)
		}
	}
	if got := PhaseInFlightProgress("not_a_phase"); got != 0 {
		t.Fatalf("未知阶段应返回 0，实际 %v", got)
	}
}
