package httpapi

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"net/http"
	"strconv"
)

func (a *API) CheckTraceHandler(w http.ResponseWriter, r *http.Request) {
	q := application.TraceQuery{TargetID: r.URL.Query().Get("targetId"), RuleCode: r.URL.Query().Get("ruleCode"), Cursor: r.URL.Query().Get("cursor")}
	if raw := r.URL.Query().Get("baselineNo"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 0 {
			fail(w, domain.NewError(domain.CodeInvalid, "baselineNo", "基线号无效"))
			return
		}
		q.BaselineNo = &n
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil {
			fail(w, domain.NewError(domain.CodeInvalid, "limit", "limit 无效"))
			return
		}
		q.Limit = n
	}
	v, e := a.service.CheckTrace(r.Context(), r.PathValue("caseID"), q)
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}

func (a *API) TimelineHandler(w http.ResponseWriter, r *http.Request) {
	v, e := a.service.GetCase(r.Context(), r.PathValue("caseID"))
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, map[string]any{"caseId": v.Case.ID, "timeline": v.Events})
}
func (a *API) PermitHandler(w http.ResponseWriter, r *http.Request) {
	v, e := a.service.GetCase(r.Context(), r.PathValue("caseID"))
	if e != nil {
		fail(w, e)
		return
	}
	if v.Permit == nil {
		fail(w, domain.NewError(domain.CodeNotFound, "permit", "案件尚未签发许可"))
		return
	}
	respond(w, 200, v.Permit)
}
func (a *API) VerifyPermitHandler(w http.ResponseWriter, r *http.Request) {
	v, e := a.service.VerifyPermit(r.Context(), r.PathValue("caseID"))
	if e != nil {
		fail(w, e)
		return
	}
	respond(w, 200, v)
}
