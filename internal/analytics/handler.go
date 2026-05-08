package analytics

import (
	"net/http"
	"sort"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/store"
)

type Handler struct {
	deps runtime.Dependencies
}

func New(deps runtime.Dependencies) *Handler {
	return &Handler{deps: deps}
}

func (h *Handler) handleGlobalAnalytics(w http.ResponseWriter, r *http.Request) {
	total := 0
	passed := 0
	failurePoints := map[string]int{}
	store.Read(h.deps.Store, func() struct{} {
		for _, exam := range h.deps.Store.Exams {
			if exam.Status == domain.ExamFinalized || exam.Status == domain.ExamCompleted || exam.Status == domain.ExamStopped || exam.Status == domain.ExamReviewRequired {
				total++
				if exam.Score >= 70 {
					passed++
				}
				for _, violation := range exam.Violations {
					failurePoints[violation.Code]++
				}
			}
		}
		return struct{}{}
	})
	common := sortCounts(failurePoints)
	passRate := 0.0
	if total > 0 {
		passRate = float64(passed) / float64(total) * 100
	}
	httpx.Success(w, http.StatusOK, map[string]any{"total_exams": total, "passed": passed, "failed": total - passed, "pass_rate": passRate, "common_failure_points": common}, nil)
}

func (h *Handler) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	counts := map[string]int{}
	store.Read(h.deps.Store, func() struct{} {
		for _, exam := range h.deps.Store.Exams {
			for _, violation := range exam.Violations {
				key := violation.Track
				if key == "" {
					key = violation.Code
				}
				counts[key]++
			}
		}
		return struct{}{}
	})
	httpx.Success(w, http.StatusOK, map[string]any{"heatmap": sortCounts(counts)}, nil)
}

func sortCounts(values map[string]int) []map[string]any {
	type kv struct {
		Key   string
		Value int
	}
	items := make([]kv, 0, len(values))
	for key, value := range values {
		items = append(items, kv{Key: key, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Value == items[j].Value {
			return items[i].Key < items[j].Key
		}
		return items[i].Value > items[j].Value
	})
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"key": item.Key, "count": item.Value})
	}
	return result
}
