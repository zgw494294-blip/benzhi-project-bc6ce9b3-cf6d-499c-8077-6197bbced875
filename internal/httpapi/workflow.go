package httpapi

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"net/http"
)

func (a *API) SubmitHandler(w http.ResponseWriter, r *http.Request) {
	var in application.RevisionInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.Submit(r.Context(), r.PathValue("caseID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) PreviewChecksHandler(w http.ResponseWriter, r *http.Request) {
	v, e := a.service.PreviewChecks(r.Context(), r.PathValue("caseID"), principal(r))
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) SubmitResolutionHandler(w http.ResponseWriter, r *http.Request) {
	var in application.ResolutionInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.SubmitResolution(r.Context(), r.PathValue("caseID"), r.PathValue("conflictID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) ReviewResolutionHandler(w http.ResponseWriter, r *http.Request) {
	var in application.ReviewInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.ReviewResolution(r.Context(), r.PathValue("caseID"), r.PathValue("conflictID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) BatchResolutionHandler(w http.ResponseWriter, r *http.Request) {
	var in application.BatchResolutionInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.BatchResolve(r.Context(), r.PathValue("caseID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) BatchReviewHandler(w http.ResponseWriter, r *http.Request) {
	var in application.BatchReviewInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.BatchReview(r.Context(), r.PathValue("caseID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) FreezeHandler(w http.ResponseWriter, r *http.Request) {
	var in application.RevisionInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.Freeze(r.Context(), r.PathValue("caseID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) FreezeReadinessHandler(w http.ResponseWriter, r *http.Request) {
	v, e := a.service.FreezeReadiness(r.Context(), r.PathValue("caseID"))
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) DecisionHandler(w http.ResponseWriter, r *http.Request) {
	var in application.DecisionInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.Decide(r.Context(), r.PathValue("caseID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
