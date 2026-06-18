package session

import (
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/pkg/slicex"
)

func isValidPhaseTransition(from, to models.SessionPhase) bool {
	validTransitions := map[models.SessionPhase][]models.SessionPhase{
		models.SessionPhaseContext:       {models.SessionPhaseChangeContext},
		models.SessionPhaseChangeContext: {models.SessionPhaseAnalysis},
		models.SessionPhaseAnalysis:      {models.SessionPhaseReviews},
		models.SessionPhaseReviews:       {models.SessionPhaseAggregation},
		models.SessionPhaseAggregation:   {models.SessionPhaseDiscussion, models.SessionPhaseSynthesis},
		models.SessionPhaseDiscussion:    {models.SessionPhaseSynthesis},
		models.SessionPhaseSynthesis:     {models.SessionPhaseComplete},
		models.SessionPhaseComplete:      {},
	}
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	return slicex.Contains(allowed, to)
}
