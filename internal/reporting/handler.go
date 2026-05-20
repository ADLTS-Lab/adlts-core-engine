package reporting

import (
	"net/http"
	"os"

	"adlts/internal/platform/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)


type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Mount(r chi.Router) {
	r.Route("/reports", func(r chi.Router) {
		r.Post("/{testID}/generate", h.generateReport)
		r.Get("/{testID}/pdf", h.getReportPDF)
	})
}

func (h *Handler) generateReport(w http.ResponseWriter, r *http.Request) {
	testID := chi.URLParam(r, "testID")
	if _, err := uuid.Parse(testID); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test_id must be a valid UUID", nil)
		return
	}
	if _, err := h.svc.GenerateReport(r.Context(), testID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "REPORT_FAILED", err.Error(), nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{
		"test_id":    testID,
		"report_url": "/api/v1/reports/" + testID + "/pdf",
	}, nil)
}

func (h *Handler) getReportPDF(w http.ResponseWriter, r *http.Request) {
	testID := chi.URLParam(r, "testID")
	if _, err := uuid.Parse(testID); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test_id must be a valid UUID", nil)
		return
	}
	path := h.svc.CachedPDFPath(testID)
	if _, err := os.Stat(path); err != nil {
		if _, genErr := h.svc.GenerateReport(r.Context(), testID); genErr != nil {
			httpx.Failure(w, http.StatusInternalServerError, "REPORT_FAILED", genErr.Error(), nil)
			return
		}
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
	http.ServeFile(w, r, path)
}
