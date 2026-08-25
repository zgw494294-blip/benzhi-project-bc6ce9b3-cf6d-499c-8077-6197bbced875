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
	return PermitVerification{Valid: a.Valid && digestOK, PermitDigestValid: digestOK, Audit: a, Permit: v.Permit}, nil
}
