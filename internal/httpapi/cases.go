package httpapi

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"net/http"
)

func (a *API) ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if e := a.service.Store().Ping(r.Context()); e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]string{"status": "ready"})
}
func (a *API) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var in application.CreateCaseInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.CreateCase(r.Context(), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 201, v)
}
func (a *API) GetCaseHandler(w http.ResponseWriter, r *http.Request) {
	v, e := a.service.GetCase(r.Context(), r.PathValue("caseID"))
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) UpdateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var in application.UpdateCaseInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.UpdateCase(r.Context(), r.PathValue("caseID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) AddTargetHandler(w http.ResponseWriter, r *http.Request) {
	var in application.TargetInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.AddTarget(r.Context(), r.PathValue("caseID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 201, v)
}
func (a *API) BatchTargetsHandler(w http.ResponseWriter, r *http.Request) {
	var in application.TargetBatchInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.BatchTargets(r.Context(), r.PathValue("caseID"), idem(r), principal(r), in)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
func (a *API) DeleteTargetHandler(w http.ResponseWriter, r *http.Request) {
	var in application.RevisionInput
	if e := decode(w, r, &in); e != nil {
		fail(w, e)
		return
	}
	v, e := a.service.DeleteTarget(r.Context(), r.PathValue("caseID"), r.PathValue("targetID"), idem(r), principal(r), in.ExpectedRevision)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
