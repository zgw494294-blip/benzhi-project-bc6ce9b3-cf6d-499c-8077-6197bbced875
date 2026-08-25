package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/audit"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"context"
)

type PermitVerification struct {
	Valid             bool                     `json:"valid"`
	PermitDigestValid bool                     `json:"permitDigestValid"`
	AuditDigestValid  bool                     `json:"auditDigestValid"`
	Audit             audit.Verification       `json:"audit"`
	Permit            *domain.ActivationPermit `json:"permit,omitempty"`
}

func permitAuditCheckpointValid(verification audit.Verification, permit *domain.ActivationPermit) bool {
	if permit == nil || permit.AuditDigest == "" {
		return false
	}
	return verification.Valid && verification.EventCount > 0
}

func (s *Service) VerifyPermit(ctx context.Context, id string) (PermitVerification, error) {
	v, e := s.store.LoadCase(ctx, id)
	if e != nil {
		return PermitVerification{}, e
	}
	if v.Permit == nil {
		return PermitVerification{}, domain.NewError(domain.CodeNotFound, "permit", "案件尚未签发许可")
	}
	a := audit.Verify(v.Events)
	digestOK := domain.PermitDigest(*v.Permit) == v.Permit.PermitDigest
	auditDigestOK := permitAuditCheckpointValid(a, v.Permit)
	return PermitVerification{Valid: a.Valid && digestOK && auditDigestOK, PermitDigestValid: digestOK, AuditDigestValid: auditDigestOK, Audit: a, Permit: v.Permit}, nil
}
