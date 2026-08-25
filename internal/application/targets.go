package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"fmt"
	"sort"
)

func targetEntity(caseID, id string, in TargetMutationInput) domain.ProtectionTarget {
	return domain.ProtectionTarget{ID: id, CaseID: caseID, Name: in.Name, ServiceClass: in.ServiceClass, FrequencyLowHz: in.FrequencyLowHz, FrequencyHighHz: in.FrequencyHighHz, MinimumSeparationHz: in.MinimumSeparationHz, FieldStrengthLimitDBUVM: in.FieldStrengthLimitDBUVM, RuleReference: in.RuleReference}
}

func validateTargetSet(targets []domain.ProtectionTarget) error {
	type rule struct {
		ref        string
		limit      float64
		separation int64
		id         string
	}
	seen := map[string]rule{}
	for _, t := range targets {
		if err := domain.ValidateTarget(t); err != nil {
			return err
		}
		key := fmt.Sprintf("%s\x00%d\x00%d", t.ServiceClass, t.FrequencyLowHz, t.FrequencyHighHz)
		if old, ok := seen[key]; ok && (old.ref != t.RuleReference || old.limit != t.FieldStrengthLimitDBUVM || old.separation != t.MinimumSeparationHz) {
			return domain.NewDetailedError(domain.CodeInvalid, "targets", "同一业务类别的重复频率范围规则不一致", map[string]any{"targetIds": []string{old.id, t.ID}})
		}
		seen[key] = rule{t.RuleReference, t.FieldStrengthLimitDBUVM, t.MinimumSeparationHz, t.ID}
	}
	return nil
}

func (s *Service) BatchTargets(ctx context.Context, id, key string, p domain.Principal, in TargetBatchInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RolePlanner); err != nil {
		return
	}
	if len(in.Creates)+len(in.Updates)+len(in.Deletes) == 0 {
		return out, domain.NewError(domain.CodeInvalid, "batch", "批次不能为空")
	}
	op := "batch-targets:" + id
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
			return domain.NewError(domain.CodeState, "status", "仅草拟案件可维护保护对象")
		}
		byID := make(map[string]domain.ProtectionTarget, len(v.Targets))
		for _, t := range v.Targets {
			byID[t.ID] = t
		}
		used := map[string]string{}
		updates := make([]domain.ProtectionTarget, 0, len(in.Updates))
		for i, item := range in.Updates {
			path := fmt.Sprintf("updates[%d].id", i)
			if item.ID == "" {
				return domain.NewError(domain.CodeInvalid, path, "更新目标标识不能为空")
			}
			if prev := used[item.ID]; prev != "" {
				return domain.NewError(domain.CodeInvalid, path, "批次内目标标识重复，首次位于 "+prev)
			}
			used[item.ID] = path
			if _, ok := byID[item.ID]; !ok {
				return domain.NewError(domain.CodeInvalid, path, "目标不存在或属于其他案件")
			}
			t := targetEntity(id, item.ID, item)
			if e = domain.ValidateTarget(t); e != nil {
				return domain.NewError(domain.CodeInvalid, fmt.Sprintf("updates[%d]", i), e.Error())
			}
			byID[t.ID] = t
			updates = append(updates, t)
		}
		for i, targetID := range in.Deletes {
			path := fmt.Sprintf("deletes[%d]", i)
			if prev := used[targetID]; prev != "" {
				return domain.NewError(domain.CodeInvalid, path, "批次内目标标识重复，首次位于 "+prev)
			}
			used[targetID] = path
			if _, ok := byID[targetID]; !ok {
				return domain.NewError(domain.CodeInvalid, path, "目标不存在或属于其他案件")
			}
			delete(byID, targetID)
		}
		creates := make([]domain.ProtectionTarget, 0, len(in.Creates))
		for i, item := range in.Creates {
			if item.ID != "" {
				return domain.NewError(domain.CodeInvalid, fmt.Sprintf("creates[%d].id", i), "新增项不能指定标识")
			}
			t := targetEntity(id, s.newID(), item)
			if e = domain.ValidateTarget(t); e != nil {
				return domain.NewError(domain.CodeInvalid, fmt.Sprintf("creates[%d]", i), e.Error())
			}
			byID[t.ID] = t
			creates = append(creates, t)
		}
		final := make([]domain.ProtectionTarget, 0, len(byID))
		for _, t := range byID {
			final = append(final, t)
		}
		sort.Slice(final, func(i, j int) bool { return final[i].ID < final[j].ID })
		if e = validateTargetSet(final); e != nil {
			return e
		}
		for _, t := range updates {
			if e = tx.UpdateTarget(ctx, t); e != nil {
				return e
			}
		}
		for _, targetID := range in.Deletes {
			if e = tx.DeleteTarget(ctx, id, targetID); e != nil {
				return e
			}
		}
		for _, t := range creates {
			if e = tx.AddTarget(ctx, t); e != nil {
				return e
			}
		}
		old := v.Case.Revision
		v.Case.Revision++
		v.Case.UpdatedAt = domain.NormalizeTime(s.now())
		if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
			return e
		}
		summary := map[string]any{"created": len(creates), "updated": len(updates), "deleted": len(in.Deletes), "targetDigest": domain.Digest(final)}
		if e = addEvent(ctx, tx, s, v, op, p, summary); e != nil {
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

func (s *Service) AddTarget(ctx context.Context, id, key string, p domain.Principal, in TargetInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RolePlanner); err != nil {
		return
	}
	op := "add-target:" + id
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
			return domain.NewError(domain.CodeState, "status", "仅草拟案件可维护保护对象")
		}
		x := domain.ProtectionTarget{ID: s.newID(), CaseID: id, Name: in.Name, ServiceClass: in.ServiceClass, FrequencyLowHz: in.FrequencyLowHz, FrequencyHighHz: in.FrequencyHighHz, MinimumSeparationHz: in.MinimumSeparationHz, FieldStrengthLimitDBUVM: in.FieldStrengthLimitDBUVM, RuleReference: in.RuleReference}
		if e = domain.ValidateTarget(x); e != nil {
			return e
		}
		if e = tx.AddTarget(ctx, x); e != nil {
			return e
		}
		old := v.Case.Revision
		v.Case.Revision++
		v.Case.UpdatedAt = domain.NormalizeTime(s.now())
		if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
			return e
		}
		if e = addEvent(ctx, tx, s, v, op, p, x); e != nil {
			return e
		}
		out, e = tx.LoadCase(ctx, id)
		if e != nil {
			return e
		}
		return saveReplay(ctx, tx, key, op, id, p.Name, digest, 201, out, s.now())
	})
	return
}
func (s *Service) DeleteTarget(ctx context.Context, id, targetID, key string, p domain.Principal, expected int64) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RolePlanner); err != nil {
		return
	}
	op := "delete-target:" + id + ":" + targetID
	digest := requestDigest(op, id, p, map[string]any{"expectedRevision": expected, "targetId": targetID})
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
		if v.Case.Revision != expected {
			return domain.NewError(domain.CodeConflict, "expectedRevision", "案件修订号不匹配")
		}
		if v.Case.Status != domain.StatusDraft {
			return domain.NewError(domain.CodeState, "status", "仅草拟案件可维护保护对象")
		}
		found := false
		for _, x := range v.Targets {
			if x.ID == targetID {
				found = true
			}
		}
		if !found {
			return domain.NewError(domain.CodeNotFound, "targetId", "保护对象不存在")
		}
		if e = tx.DeleteTarget(ctx, id, targetID); e != nil {
			return e
		}
		old := v.Case.Revision
		v.Case.Revision++
		v.Case.UpdatedAt = domain.NormalizeTime(s.now())
		if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
			return e
		}
		if e = addEvent(ctx, tx, s, v, op, p, map[string]string{"targetId": targetID}); e != nil {
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
