package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"time"
)

type previewCacheKey struct {
	caseID   string
	revision int64
}

func (s *Service) loadPreview(key previewCacheKey) (domain.CheckPreview, bool) {
	s.previewMu.RLock()
	defer s.previewMu.RUnlock()
	if !s.previewSet || s.previewKey != key {
		return domain.CheckPreview{}, false
	}
	return clonePreview(s.preview), true
}

func (s *Service) savePreview(key previewCacheKey, preview domain.CheckPreview) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.previewKey = key
	s.preview = clonePreview(preview)
	s.previewSet = true
}

// clonePreview returns a deep copy of preview so cached entries never share
// mutable state with the values returned to callers. Results, each item's
// InputSummary map and the nested targetRangeHz slice are all duplicated;
// scalar fields are immutable and copied by value. This keeps successive
// PreviewChecks calls returning a preview generated from case data that stays
// consistent with PreviewDigest regardless of how a previous caller mutates
// its returned value.
func clonePreview(p domain.CheckPreview) domain.CheckPreview {
	out := p
	if p.Results == nil {
		return out
	}
	results := make([]domain.CheckPreviewItem, len(p.Results))
	for i := range p.Results {
		item := p.Results[i]
		item.InputSummary = cloneInputSummary(p.Results[i].InputSummary)
		results[i] = item
	}
	out.Results = results
	return out
}

// cloneInputSummary copies the summary map and its nested targetRangeHz slice
// so callers cannot mutate cached data through aliased references.
func cloneInputSummary(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case []int64:
			cp := make([]int64, len(val))
			copy(cp, val)
			dst[k] = cp
		default:
			dst[k] = v
		}
	}
	return dst
}

func evaluateAll(s *Service, profile domain.EmissionProfile, targets []domain.ProtectionTarget, now time.Time) []domain.InterferenceCheck {
	var out []domain.InterferenceCheck
	for _, t := range targets {
		xs := domain.Evaluate(profile, t, profile.BaselineNo, "", now)
		for i := range xs {
			xs[i].ID = s.newID()
		}
		out = append(out, xs...)
	}
	return out
}

func (s *Service) PreviewChecks(ctx context.Context, id string, p domain.Principal) (domain.CheckPreview, error) {
	if err := p.Validate(domain.RolePlanner); err != nil {
		return domain.CheckPreview{}, err
	}
	v, err := s.store.LoadCase(ctx, id)
	if err != nil {
		return domain.CheckPreview{}, err
	}
	key := previewCacheKey{caseID: id, revision: v.Case.Revision}
	if preview, ok := s.loadPreview(key); ok {
		return preview, nil
	}
	profile, err := profileCurrent(v)
	if err != nil {
		return domain.CheckPreview{}, err
	}
	preview, err := domain.PreviewFor(v.Case, profile, v.Targets)
	if err != nil {
		return domain.CheckPreview{}, err
	}
	s.savePreview(key, preview)
	return preview, nil
}

func (s *Service) Submit(ctx context.Context, id, key string, p domain.Principal, in RevisionInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RolePlanner); err != nil {
		return
	}
	op := "submit:" + id
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
		if v.Case.Status != domain.StatusDraft {
			return domain.NewError(domain.CodeState, "status", "仅草拟案件可送审")
		}
		if len(v.Targets) == 0 {
			return domain.NewError(domain.CodeInvalid, "targets", "至少需要一个保护对象")
		}
		profile, e := profileCurrent(v)
		if e != nil {
			return e
		}
		if e = domain.ValidateProfile(profile); e != nil {
			return e
		}
		occupied, e := tx.FindWindowOccupancies(ctx, v.Case.StationCode, id, v.Case.EffectiveFrom, v.Case.EffectiveUntil, 100)
		if e != nil {
			return e
		}
		if len(occupied) > 0 {
			return domain.NewDetailedError(domain.CodeConflict, "effectiveWindow", "同一台站的生效窗口已被占用", map[string]any{"occupancies": occupied})
		}
		if in.PreviewDigest != "" {
			preview, pe := domain.PreviewFor(v.Case, profile, v.Targets)
			if pe != nil {
				return pe
			}
			if preview.PreviewDigest != in.PreviewDigest {
				return domain.NewDetailedError(domain.CodeConflict, "previewDigest", "预览摘要已过期，请重新预览", map[string]any{"currentRevision": v.Case.Revision})
			}
		}
		now := domain.NormalizeTime(s.now())
		if e = tx.SealProfile(ctx, profile.ID, storage.TimeValue(now)); e != nil {
			return e
		}
		profile.SealedAt = &now
		if e = domain.Transition(&v.Case, domain.StatusReviewing); e != nil {
			return e
		}
		checks := evaluateAll(s, profile, v.Targets, now)
		failed := 0
		for _, x := range checks {
			if e = tx.AddCheck(ctx, x); e != nil {
				return e
			}
			if x.Result == domain.CheckFail {
				failed++
				c := domain.ConflictResolution{ID: s.newID(), CaseID: id, CheckID: x.ID, Status: domain.ResolutionOpen, UpdatedAt: now}
				if e = tx.AddConflict(ctx, c); e != nil {
					return e
				}
			}
		}
		if failed > 0 {
			if e = domain.Transition(&v.Case, domain.StatusRemediating); e != nil {
				return e
			}
		} else {
			if e = domain.Transition(&v.Case, domain.StatusReviewed); e != nil {
				return e
			}
		}
		v.Case.UpdatedAt = now
		if e = tx.UpdateCase(ctx, v.Case, in.ExpectedRevision); e != nil {
			return e
		}
		if e = addEvent(ctx, tx, s, v, op, p, map[string]any{"baselineNo": profile.BaselineNo, "checks": len(checks), "failed": failed}); e != nil {
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
