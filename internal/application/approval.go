package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/audit"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"fmt"
)

func (s *Service) FreezeReadiness(ctx context.Context, id string) (domain.FreezeReadiness, error) {
	v, err := s.store.LoadCase(ctx, id)
	if err != nil {
		return domain.FreezeReadiness{}, err
	}
	return domain.FreezeDiagnosis(v), nil
}

func (s *Service) Freeze(ctx context.Context, id, key string, p domain.Principal, in RevisionInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RolePlanner); err != nil {
		return
	}
	op := "freeze:" + id
	digest := requestDigest(op, id, p, in)
	err = s.store.Write(ctx, func(tx *storage.Tx) error {
		if r, e := replayView(ctx, tx, key, op, id, p.Name, digest); e != nil {
			return e
		} else if r != nil {
			out = *r
			return nil
		}
		v, e := tx.LoadCase(ctx, id)
		if e != nil {
			return e
		}
		if v.Case.Revision != in.ExpectedRevision {
			return domain.NewError(domain.CodeConflict, "expectedRevision", "案件修订号不匹配")
		}
		readiness := domain.FreezeDiagnosis(v)
		if !readiness.Ready {
			return domain.NewDetailedError(domain.CodeState, "readiness", "案件尚未达到冻结条件", readiness)
		}
		occupied, e := tx.FindWindowOccupancies(ctx, v.Case.StationCode, id, v.Case.EffectiveFrom, v.Case.EffectiveUntil, 100)
		if e != nil {
			return e
		}
		if len(occupied) > 0 {
			return domain.NewDetailedError(domain.CodeConflict, "effectiveWindow", "同一台站的生效窗口已被占用", map[string]any{"occupancies": occupied})
		}
		profile, e := profileCurrent(v)
		if e != nil {
			return e
		}
		now := domain.NormalizeTime(s.now())
		if v.Case.Status == domain.StatusRemediating {
			if e = domain.Transition(&v.Case, domain.StatusReviewed); e != nil {
				return e
			}
		}
		if e = domain.Transition(&v.Case, domain.StatusFrozen); e != nil {
			return e
		}
		v.Case.UpdatedAt = now
		if e = tx.UpdateCase(ctx, v.Case, in.ExpectedRevision); e != nil {
			return e
		}
		if e = addEvent(ctx, tx, s, v, op, p, map[string]any{"baselineDigest": domain.ProfileDigest(profile), "fullChecks": len(v.Targets) * 2}); e != nil {
			return e
		}
		out, e = tx.LoadCase(ctx, id)
		if e != nil {
			return e
		}
		return saveReplay(ctx, tx, key, op, id, p.Name, digest, 200, out, now)
	})
	return
}
func (s *Service) Decide(ctx context.Context, id, key string, p domain.Principal, in DecisionInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RoleLeader); err != nil {
		return
	}
	if in.Decision != "approve" && in.Decision != "reject" {
		return out, domain.NewError(domain.CodeInvalid, "decision", "审批结论必须是 approve 或 reject")
	}
	op := "decision:" + id
	digest := requestDigest(op, id, p, in)
	err = s.store.Write(ctx, func(tx *storage.Tx) error {
		if r, e := replayView(ctx, tx, key, op, id, p.Name, digest); e != nil {
			return e
		} else if r != nil {
			out = *r
			return nil
		}
		v, e := tx.LoadCase(ctx, id)
		if e != nil {
			return e
		}
		if v.Case.Revision != in.ExpectedRevision {
			return domain.NewError(domain.CodeConflict, "expectedRevision", "案件修订号不匹配")
		}
		if v.Case.Status != domain.StatusFrozen {
			return domain.NewError(domain.CodeState, "status", "仅冻结案件可审批")
		}
		now := domain.NormalizeTime(s.now())
		old := v.Case.Revision
		if in.Decision == "reject" {
			if e = domain.Transition(&v.Case, domain.StatusRejected); e != nil {
				return e
			}
			v.Case.UpdatedAt = now
			if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
				return e
			}
			if e = addEvent(ctx, tx, s, v, op, p, map[string]string{"decision": "reject", "comment": in.Comment}); e != nil {
				return e
			}
		} else {
			profile, e := profileCurrent(v)
			if e != nil {
				return e
			}
			verification := audit.Verify(v.Events)
			if !verification.Valid {
				return fmt.Errorf("审计链验证失败: %s", verification.Reason)
			}
			serial, e := tx.NextPermitSerial(ctx, now.Year())
			if e != nil {
				return e
			}
			permit := domain.ActivationPermit{ID: s.newID(), CaseID: id, SerialNumber: serial, BaselineDigest: domain.ProfileDigest(profile), AuditDigest: verification.HeadDigest, ApprovedBy: p.Name, IssuedAt: now}
			permit.PermitDigest = domain.PermitDigest(permit)
			if e = domain.Transition(&v.Case, domain.StatusApproved); e != nil {
				return e
			}
			v.Case.UpdatedAt = now
			if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
				return e
			}
			if e = tx.AddPermit(ctx, permit); e != nil {
				return e
			}
			if e = addEvent(ctx, tx, s, v, op, p, audit.PermitPayload{SerialNumber: serial, BaselineDigest: permit.BaselineDigest, PermitDigest: permit.PermitDigest}); e != nil {
				return e
			}
		}
		out, e = tx.LoadCase(ctx, id)
		if e != nil {
			return e
		}
		return saveReplay(ctx, tx, key, op, id, p.Name, digest, 200, out, now)
	})
	return
}
