package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/audit"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"context"
)

type PermitVerification struct {
	Valid             bool                     `json:"valid"`
	PermitDigestValid bool                     `json:"permitDigestValid"`
	Audit             audit.Verification       `json:"audit"`
	Permit            *domain.ActivationPermit `json:"permit,omitempty"`
}

type permitAuditCacheEntry struct {
	revision   int64
	eventCount int
	headDigest string
	result     audit.Verification
}

func (s *Service) VerifyPermit(ctx context.Context, id string) (PermitVerification, error) {
	v, e := s.store.LoadCase(ctx, id)
	if e != nil {
		return PermitVerification{}, e
	}
	if v.Permit == nil {
		return PermitVerification{}, domain.NewError(domain.CodeNotFound, "permit", "案件尚未签发许可")
	}
	headDigest := ""
	if len(v.Events) > 0 {
		headDigest = v.Events[len(v.Events)-1].Digest
	}
	s.permitMu.RLock()
	cached, ok := s.permitAudits[id]
	s.permitMu.RUnlock()
	var a audit.Verification
	if ok && cached.revision == v.Case.Revision && cached.eventCount == len(v.Events) && cached.headDigest == headDigest {
		a = cached.result
	} else {
		a = audit.Verify(v.Events)
		s.permitMu.Lock()
		s.permitAudits[id] = permitAuditCacheEntry{revision: v.Case.Revision, eventCount: len(v.Events), headDigest: headDigest, result: a}
		s.permitMu.Unlock()
	}
	digestOK := domain.PermitDigest(*v.Permit) == v.Permit.PermitDigest
	return PermitVerification{Valid: a.Valid && digestOK, PermitDigestValid: digestOK, Audit: a, Permit: v.Permit}, nil
}
