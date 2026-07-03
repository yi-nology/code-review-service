package session

import (
	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
	"github.com/oekit/pkg/slicex"
)

func isValidPhaseTransition(from, to string) bool {
	validTransitions := map[string][]string{
		dsession.PhaseContext:       {dsession.PhaseChangeContext},
		dsession.PhaseChangeContext: {dsession.PhaseAnalysis},
		dsession.PhaseAnalysis:      {dsession.PhaseReviews},
		dsession.PhaseReviews:       {dsession.PhaseAggregation},
		dsession.PhaseAggregation:   {dsession.PhaseDiscussion, dsession.PhaseSynthesis},
		dsession.PhaseDiscussion:    {dsession.PhaseSynthesis},
		dsession.PhaseSynthesis:     {dsession.PhaseComplete},
		dsession.PhaseComplete:      {},
	}
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	return slicex.Contains(allowed, to)
}
