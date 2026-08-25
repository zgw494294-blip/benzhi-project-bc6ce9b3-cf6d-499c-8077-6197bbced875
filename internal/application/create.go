package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/audit"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
)

func (s *Service) CreateCase(ctx context.Context, key string, p domain.Principal, in CreateCaseInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RolePlanner); err != nil {
		return
	}
	in.EffectiveFrom, in.EffectiveUntil, err = domain.NormalizeWindow(in.EffectiveFrom, in.EffectiveUntil)
	if err != nil {
		return
	}
	op := "create-case"
	digest := requestDigest(op, "", p, in)
	err = s.store.Write(ctx, func(tx *storage.Tx) error {
		if r, e := replayView(ctx, tx, key, op, "", p.Name, digest); e != nil {
			return e
		} else if r != nil {
			out = *r
			return nil
		}
		now := domain.NormalizeTime(s.now())
		id := s.newID()
		c := domain.FrequencyChangeCase{ID: id, StationCode: in.StationCode, Title: in.Title, Status: domain.StatusDraft, Revision: 1, EffectiveFrom: in.EffectiveFrom, EffectiveUntil: in.EffectiveUntil, CreatedAt: now, UpdatedAt: now}
		profile := in.Profile.entity(s.newID(), id, 0)
		if e := domain.ValidateCase(c); e != nil {
			return e
		}
		if e := domain.ValidateProfile(profile); e != nil {
			return e
		}
		occupied, e := tx.FindWindowOccupancies(ctx, c.StationCode, "", c.EffectiveFrom, c.EffectiveUntil, 100)
		if e != nil {
			return e
		}
		if len(occupied) > 0 {
			return domain.NewDetailedError(domain.CodeConflict, "effectiveWindow", "同一台站的生效窗口已被占用", map[string]any{"occupancies": occupied})
		}
		if e := tx.CreateCase(ctx, c, profile); e != nil {
			return e
		}
		event := audit.NewEvent(s.newID(), id, op, p.Name, 1, "", audit.CommandPayload{Operation: op, Revision: 1, ResultRef: id}, now)
		if e := tx.AddEvent(ctx, event); e != nil {
			return e
		}
		out = domain.CaseView{Case: c, Profiles: []domain.EmissionProfile{profile}, Events: []domain.AuditEvent{event}}
		return saveReplay(ctx, tx, key, op, id, p.Name, digest, 201, out, now)
	})
	return
}
func (s *Service) UpdateCase(ctx context.Context, id, key string, p domain.Principal, in UpdateCaseInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RolePlanner); err != nil {
		return
	}
	op := "update-case:" + id
	if !in.EffectiveFrom.IsZero() && !in.EffectiveUntil.IsZero() {
		in.EffectiveFrom = domain.NormalizeTime(in.EffectiveFrom)
		in.EffectiveUntil = domain.NormalizeTime(in.EffectiveUntil)
	}
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
		if !v.Case.Status.Mutable() {
			return domain.NewError(domain.CodeState, "status", "当前状态禁止编辑参数")
		}
		in.EffectiveFrom, in.EffectiveUntil, e = domain.NormalizeWindow(in.EffectiveFrom, in.EffectiveUntil)
		if e != nil {
			return e
		}
		prof, e := profileCurrent(v)
		if e != nil {
			return e
		}
		prof.FrequencyHz = in.Profile.FrequencyHz
		prof.BandwidthHz = in.Profile.BandwidthHz
		prof.PowerWatts = in.Profile.PowerWatts
		prof.AntennaGainDB = in.Profile.AntennaGainDB
		prof.AzimuthDegrees = in.Profile.AzimuthDegrees
		prof.SiteLatitude = in.Profile.SiteLatitude
		prof.SiteLongitude = in.Profile.SiteLongitude
		if e = domain.ValidateProfile(prof); e != nil {
			return e
		}
		v.Case.Title = in.Title
		v.Case.EffectiveFrom = in.EffectiveFrom
		v.Case.EffectiveUntil = in.EffectiveUntil
		if e = domain.ValidateCase(v.Case); e != nil {
			return e
		}
		occupied, e := tx.FindWindowOccupancies(ctx, v.Case.StationCode, id, v.Case.EffectiveFrom, v.Case.EffectiveUntil, 100)
		if e != nil {
			return e
		}
		if len(occupied) > 0 {
			return domain.NewDetailedError(domain.CodeConflict, "effectiveWindow", "同一台站的生效窗口已被占用", map[string]any{"occupancies": occupied})
		}
		old := v.Case.Revision
		v.Case.Revision++
		v.Case.UpdatedAt = domain.NormalizeTime(s.now())
		if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
			return e
		}
		if prof.SealedAt == nil {
			if e = tx.UpdateProfile(ctx, prof); e != nil {
				return e
			}
		} else {
			prof.ID = s.newID()
			prof.BaselineNo++
			prof.SealedAt = nil
			if e = tx.InsertProfile(ctx, prof); e != nil {
				return e
			}
			v.Profiles = append(v.Profiles, prof)
		}
		if e = addEvent(ctx, tx, s, v, op, p, audit.CommandPayload{Operation: op, Revision: v.Case.Revision, ResultRef: id}); e != nil {
			return e
		}
		out, e = tx.LoadCase(ctx, id)
		if e != nil {
			return e
		}
		return saveReplay(ctx, tx, key, op, id, p.Name, digest, 200, out, s.now())
	})
	return
}
