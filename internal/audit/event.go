package audit

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"time"
)

func NewEvent(id, caseID, kind, actor string, sequence int64, previous string, payload any, now time.Time) domain.AuditEvent {
	payloadDigest := domain.Digest(payload)
	e := domain.AuditEvent{ID: id, CaseID: caseID, Sequence: sequence, EventType: kind, Actor: actor, PayloadDigest: payloadDigest, PreviousDigest: previous, CreatedAt: domain.NormalizeTime(now)}
	e.Digest = domain.Digest(e.CaseID, e.Sequence, e.EventType, e.Actor, e.PayloadDigest, e.PreviousDigest, e.CreatedAt)
	return e
}
