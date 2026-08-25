package httpapi

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"context"
	"net/http"
	"time"
)

type API struct {
	service *application.Service
	mux     *http.ServeMux
}

func New(service *application.Service) *API {
	a := &API{service: service, mux: http.NewServeMux()}
	a.routes()
	return a
}
func (a *API) Handler() http.Handler { return requestMiddleware(a.mux) }
func (a *API) routes() {
	a.mux.HandleFunc("GET /readyz", a.ReadyHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases", a.CreateCaseHandler)
	a.mux.HandleFunc("GET /api/v1/change-cases/{caseID}", a.GetCaseHandler)
	a.mux.HandleFunc("PUT /api/v1/change-cases/{caseID}", a.UpdateCaseHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/targets", a.AddTargetHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/targets/batch", a.BatchTargetsHandler)
	a.mux.HandleFunc("DELETE /api/v1/change-cases/{caseID}/targets/{targetID}", a.DeleteTargetHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/submit", a.SubmitHandler)
	a.mux.HandleFunc("GET /api/v1/change-cases/{caseID}/checks/preview", a.PreviewChecksHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/checks/preview", a.PreviewChecksHandler)
	a.mux.HandleFunc("GET /api/v1/change-cases/{caseID}/checks/trace", a.CheckTraceHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/conflicts/{conflictID}/resolution", a.SubmitResolutionHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/conflicts/{conflictID}/review", a.ReviewResolutionHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/conflicts/batch-resolution", a.BatchResolutionHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/conflicts/batch-review", a.BatchReviewHandler)
	a.mux.HandleFunc("GET /api/v1/change-cases/{caseID}/freeze/readiness", a.FreezeReadinessHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/freeze", a.FreezeHandler)
	a.mux.HandleFunc("POST /api/v1/change-cases/{caseID}/decision", a.DecisionHandler)
	a.mux.HandleFunc("GET /api/v1/change-cases/{caseID}/timeline", a.TimelineHandler)
	a.mux.HandleFunc("GET /api/v1/change-cases/{caseID}/permit", a.PermitHandler)
	a.mux.HandleFunc("GET /api/v1/change-cases/{caseID}/permit/verify", a.VerifyPermitHandler)
}
func requestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
