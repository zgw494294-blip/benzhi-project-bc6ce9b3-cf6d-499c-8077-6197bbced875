package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"fmt"
	"sort"
	"strings"
)

func findConflict(v domain.CaseView, id string) (domain.ConflictResolution, error) {
	for _, x := range v.Conflicts {
		if x.ID == id {
			return x, nil
		}
	}
	return domain.ConflictResolution{}, domain.NewError(domain.CodeNotFound, "conflictId", "冲突不存在")
}
func targetForCheck(v domain.CaseView, checkID string) (domain.ProtectionTarget, error) {
	targetID := ""
	for _, x := range v.Checks {
		if x.ID == checkID {
			targetID = x.TargetID
			break
		}
	}
	for _, x := range v.Targets {
		if x.ID == targetID {
			return x, nil
		}
	}
	return domain.ProtectionTarget{}, domain.NewError(domain.CodeNotFound, "targetId", "冲突关联的保护对象不存在")
}
func (s *Service) SubmitResolution(ctx context.Context, id, conflictID, key string, p domain.Principal, in ResolutionInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RolePlanner); err != nil {
		return
	}
	if err = domain.ValidateResolution(in.ResolutionText, in.EvidenceDigest, p.Name); err != nil {
		return
	}
	op := "resolve:" + id + ":" + conflictID
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
		if v.Case.Status != domain.StatusRemediating {
			return domain.NewError(domain.CodeState, "status", "案件不在整改阶段")
		}
		conflict, e := findConflict(v, conflictID)
		if e != nil {
			return e
		}
		if conflict.Status != domain.ResolutionOpen && conflict.Status != domain.ResolutionReturned {
			return domain.NewError(domain.CodeState, "conflict", "仅 open 或 returned 冲突可整改")
		}
		if _, e = targetForCheck(v, conflict.CheckID); e != nil {
			return e
		}
		original, ok := checkByID(v, conflict.CheckID)
		if !ok {
			return domain.NewError(domain.CodeState, "checkId", "冲突关联检查不存在")
		}
		current, e := profileCurrent(v)
		if e != nil {
			return e
		}
		profile := in.AdjustedProfile.entity(s.newID(), id, current.BaselineNo+1)
		now := domain.NormalizeTime(s.now())
		profile.SealedAt = &now
		if e = domain.ValidateProfile(profile); e != nil {
			return e
		}
		latest := map[string]domain.InterferenceCheck{}
		for _, oldCheck := range v.Checks {
			k := oldCheck.TargetID + "\x00" + oldCheck.RuleCode
			old, exists := latest[k]
			if !exists || oldCheck.BaselineNo > old.BaselineNo || oldCheck.BaselineNo == old.BaselineNo && oldCheck.CheckedAt.After(old.CheckedAt) {
				latest[k] = oldCheck
			}
		}
		checks := make([]domain.InterferenceCheck, 0, len(v.Targets)*2)
		allPass := true
		conflictCheckID := ""
		for _, checkTarget := range v.Targets {
			evaluated := domain.Evaluate(profile, checkTarget, profile.BaselineNo, "", now)
			for i := range evaluated {
				evaluated[i].ID = s.newID()
				if previous, exists := latest[evaluated[i].TargetID+"\x00"+evaluated[i].RuleCode]; exists {
					evaluated[i].PreviousCheckID = previous.ID
				}
				if evaluated[i].TargetID == original.TargetID && evaluated[i].RuleCode == original.RuleCode {
					conflictCheckID = evaluated[i].ID
				}
				if evaluated[i].Result == domain.CheckFail {
					allPass = false
				}
			}
			checks = append(checks, evaluated...)
		}
		if !allPass {
			return domain.NewError(domain.CodeInvalid, "adjustedProfile", "调整参数仍未通过全量规则")
		}
		if e = tx.InsertProfile(ctx, profile); e != nil {
			return e
		}
		for _, x := range checks {
			if e = tx.AddCheck(ctx, x); e != nil {
				return e
			}
		}
		conflict.CheckID = conflictCheckID
		conflict.Status = domain.ResolutionSubmitted
		conflict.ResolutionText = in.ResolutionText
		conflict.EvidenceDigest = in.EvidenceDigest
		conflict.SubmittedBy = p.Name
		conflict.ReviewedBy = ""
		conflict.ReviewComment = ""
		conflict.ReviewedAt = nil
		conflict.UpdatedAt = now
		if e = tx.UpdateConflict(ctx, conflict); e != nil {
			return e
		}
		old := v.Case.Revision
		v.Case.Revision++
		v.Case.UpdatedAt = now
		if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
			return e
		}
		if e = addEvent(ctx, tx, s, v, op, p, map[string]any{"conflictId": conflictID, "baselineNo": profile.BaselineNo, "evidenceDigest": in.EvidenceDigest}); e != nil {
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
func (s *Service) ReviewResolution(ctx context.Context, id, conflictID, key string, p domain.Principal, in ReviewInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RoleReviewer); err != nil {
		return
	}
	if in.Decision != "accept" && in.Decision != "return" {
		return out, domain.NewError(domain.CodeInvalid, "decision", "复核结论必须是 accept 或 return")
	}
	op := "review:" + id + ":" + conflictID
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
		conflict, e := findConflict(v, conflictID)
		if e != nil {
			return e
		}
		if conflict.Status != domain.ResolutionSubmitted {
			return domain.NewError(domain.CodeState, "conflict", "冲突尚未提交整改")
		}
		if conflict.SubmittedBy == p.Name {
			return domain.NewError(domain.CodeForbidden, "reviewer", "不能复核自己提交的整改")
		}
		now := domain.NormalizeTime(s.now())
		if in.Decision == "accept" {
			conflict.Status = domain.ResolutionAccepted
		} else {
			conflict.Status = domain.ResolutionReturned
		}
		conflict.ReviewedBy = p.Name
		conflict.ReviewComment = in.Comment
		conflict.ReviewedAt = &now
		conflict.UpdatedAt = now
		if e = tx.UpdateConflict(ctx, conflict); e != nil {
			return e
		}
		old := v.Case.Revision
		v.Case.Revision++
		v.Case.UpdatedAt = now
		if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
			return e
		}
		if e = addEvent(ctx, tx, s, v, op, p, map[string]string{"conflictId": conflictID, "decision": in.Decision, "comment": in.Comment}); e != nil {
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

func checkByID(v domain.CaseView, id string) (domain.InterferenceCheck, bool) {
	for _, x := range v.Checks {
		if x.ID == id {
			return x, true
		}
	}
	return domain.InterferenceCheck{}, false
}

func (s *Service) BatchResolve(ctx context.Context, id, key string, p domain.Principal, in BatchResolutionInput) (out domain.CaseView, err error) {
	if err = p.Validate(domain.RolePlanner); err != nil {
		return
	}
	if len(in.ConflictIDs) == 0 {
		return out, domain.NewError(domain.CodeInvalid, "conflictIds", "冲突列表不能为空")
	}
	op := "batch-resolve:" + id
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
		if v.Case.Status != domain.StatusRemediating {
			return domain.NewError(domain.CodeState, "status", "案件不在整改阶段")
		}
		resolutions := map[string]ConflictResolutionItem{}
		for i, item := range in.Resolutions {
			if _, ok := resolutions[item.ConflictID]; ok {
				return domain.NewError(domain.CodeInvalid, fmt.Sprintf("resolutions[%d].conflictId", i), "冲突标识重复")
			}
			if e = domain.ValidateResolution(item.ResolutionText, item.EvidenceDigest, p.Name); e != nil {
				return domain.NewError(domain.CodeInvalid, fmt.Sprintf("resolutions[%d]", i), e.Error())
			}
			resolutions[item.ConflictID] = item
		}
		byConflict := map[string]domain.ConflictResolution{}
		for _, c := range v.Conflicts {
			byConflict[c.ID] = c
		}
		selected := make([]domain.ConflictResolution, 0, len(in.ConflictIDs))
		seen := map[string]bool{}
		targets := map[string]domain.ProtectionTarget{}
		for i, conflictID := range in.ConflictIDs {
			path := fmt.Sprintf("conflictIds[%d]", i)
			if seen[conflictID] {
				return domain.NewError(domain.CodeInvalid, path, "冲突标识重复")
			}
			seen[conflictID] = true
			c, ok := byConflict[conflictID]
			if !ok {
				return domain.NewError(domain.CodeInvalid, path, "冲突不存在或不属于当前案件")
			}
			if c.Status != domain.ResolutionOpen && c.Status != domain.ResolutionReturned {
				return domain.NewError(domain.CodeState, path, "仅 open 或 returned 冲突可纳入批次")
			}
			if _, ok = resolutions[conflictID]; !ok {
				return domain.NewError(domain.CodeInvalid, path, "缺少独立处置方案和证据摘要")
			}
			check, ok := checkByID(v, c.CheckID)
			if !ok {
				return domain.NewError(domain.CodeInvalid, path, "冲突关联检查不存在")
			}
			for _, t := range v.Targets {
				if t.ID == check.TargetID {
					targets[t.ID] = t
				}
			}
			if _, ok = targets[check.TargetID]; !ok {
				return domain.NewError(domain.CodeInvalid, path, "冲突关联保护对象不存在")
			}
			selected = append(selected, c)
		}
		if len(resolutions) != len(selected) {
			return domain.NewError(domain.CodeInvalid, "resolutions", "处置项必须与 conflictIds 一一对应")
		}
		current, e := profileCurrent(v)
		if e != nil {
			return e
		}
		profile := in.AdjustedProfile.entity(s.newID(), id, current.BaselineNo+1)
		now := domain.NormalizeTime(s.now())
		profile.SealedAt = &now
		if e = domain.ValidateProfile(profile); e != nil {
			return e
		}
		latest := map[string]domain.InterferenceCheck{}
		for _, c := range v.Checks {
			k := c.TargetID + "\x00" + c.RuleCode
			old, ok := latest[k]
			if !ok || c.BaselineNo > old.BaselineNo || c.BaselineNo == old.BaselineNo && c.CheckedAt.After(old.CheckedAt) {
				latest[k] = c
			}
		}
		newChecks := map[string]domain.InterferenceCheck{}
		failures := []map[string]any{}
		for _, target := range v.Targets {
			checks := domain.Evaluate(profile, target, profile.BaselineNo, "", now)
			for i := range checks {
				checks[i].ID = s.newID()
				if prev, ok := latest[target.ID+"\x00"+checks[i].RuleCode]; ok {
					checks[i].PreviousCheckID = prev.ID
				}
				newChecks[target.ID+"\x00"+checks[i].RuleCode] = checks[i]
				if checks[i].Result == domain.CheckFail {
					failures = append(failures, map[string]any{"targetId": target.ID, "ruleCode": checks[i].RuleCode, "measuredMargin": checks[i].MeasuredMargin})
				}
			}
		}
		if len(failures) > 0 {
			return domain.NewDetailedError(domain.CodeInvalid, "adjustedProfile", "统一调整参数仍有规则未通过", map[string]any{"failures": failures})
		}
		if e = tx.InsertProfile(ctx, profile); e != nil {
			return e
		}
		for _, c := range newChecks {
			if e = tx.AddCheck(ctx, c); e != nil {
				return e
			}
		}
		for _, c := range selected {
			oldCheck, _ := checkByID(v, c.CheckID)
			newCheck := newChecks[oldCheck.TargetID+"\x00"+oldCheck.RuleCode]
			if newCheck.ID == "" {
				return domain.NewError(domain.CodeState, "checks", "新基线缺少冲突对应规则结果")
			}
			item := resolutions[c.ID]
			c.CheckID = newCheck.ID
			c.Status = domain.ResolutionSubmitted
			c.ResolutionText = item.ResolutionText
			c.EvidenceDigest = item.EvidenceDigest
			c.SubmittedBy = p.Name
			c.ReviewedBy = ""
			c.ReviewComment = ""
			c.ReviewedAt = nil
			c.UpdatedAt = now
			if e = tx.UpdateConflict(ctx, c); e != nil {
				return e
			}
		}
		old := v.Case.Revision
		v.Case.Revision++
		v.Case.UpdatedAt = now
		if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
			return e
		}
		conflictSet := append([]string(nil), in.ConflictIDs...)
		sort.Strings(conflictSet)
		if e = addEvent(ctx, tx, s, v, op, p, map[string]any{"baselineNo": profile.BaselineNo, "baselineDigest": domain.ProfileDigest(profile), "conflictDigest": domain.Digest(conflictSet), "count": len(selected)}); e != nil {
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

func (s *Service) BatchReview(ctx context.Context, id, key string, p domain.Principal, in BatchReviewInput) (out BatchReviewResult, err error) {
	if err = p.Validate(domain.RoleReviewer); err != nil {
		return
	}
	if len(in.Reviews) == 0 {
		return out, domain.NewError(domain.CodeInvalid, "reviews", "复核批次不能为空")
	}
	op := "batch-review:" + id
	digest := requestDigest(op, id, p, in)
	err = s.store.Write(ctx, func(tx *storage.Tx) error {
		if ok, e := replayInto(ctx, tx, key, op, id, p.Name, digest, &out); e != nil {
			return e
		} else if ok {
			return nil
		}
		v, e := tx.LoadCase(ctx, id)
		if e != nil {
			return e
		}
		if v.Case.Revision != in.ExpectedRevision {
			return domain.NewError(domain.CodeConflict, "expectedRevision", "案件修订号不匹配")
		}
		byID := map[string]domain.ConflictResolution{}
		for _, c := range v.Conflicts {
			byID[c.ID] = c
		}
		updates := make([]domain.ConflictResolution, 0, len(in.Reviews))
		seen := map[string]bool{}
		blockers := make([]map[string]any, 0)
		now := domain.NormalizeTime(s.now())
		for i, item := range in.Reviews {
			path := fmt.Sprintf("reviews[%d]", i)
			if seen[item.ConflictID] {
				blockers = append(blockers, map[string]any{"path": path + ".conflictId", "code": "duplicate", "message": "冲突标识重复"})
				continue
			}
			seen[item.ConflictID] = true
			if item.Decision != "accept" && item.Decision != "return" {
				blockers = append(blockers, map[string]any{"path": path + ".decision", "code": "invalid_decision", "message": "结论必须是 accept 或 return"})
				continue
			}
			if strings.TrimSpace(item.Comment) == "" {
				blockers = append(blockers, map[string]any{"path": path + ".comment", "code": "required", "message": "复核意见不能为空"})
				continue
			}
			c, ok := byID[item.ConflictID]
			if !ok {
				blockers = append(blockers, map[string]any{"path": path + ".conflictId", "code": "not_in_case", "message": "冲突不存在或不属于当前案件"})
				continue
			}
			if c.Status != domain.ResolutionSubmitted {
				blockers = append(blockers, map[string]any{"path": path + ".conflictId", "code": "invalid_status", "message": "冲突不是 submitted 状态"})
				continue
			}
			if c.SubmittedBy == p.Name {
				blockers = append(blockers, map[string]any{"path": path + ".conflictId", "code": "self_review", "message": "不能复核自己提交的整改"})
				continue
			}
			if item.Decision == "accept" {
				c.Status = domain.ResolutionAccepted
			} else {
				c.Status = domain.ResolutionReturned
			}
			c.ReviewedBy = p.Name
			c.ReviewComment = item.Comment
			c.ReviewedAt = &now
			c.UpdatedAt = now
			updates = append(updates, c)
		}
		if len(blockers) > 0 {
			return domain.NewDetailedError(domain.CodeConflict, "reviews", "复核批次存在阻断项", map[string]any{"items": blockers})
		}
		for _, c := range updates {
			if e = tx.UpdateConflict(ctx, c); e != nil {
				return e
			}
			byID[c.ID] = c
		}
		old := v.Case.Revision
		v.Case.Revision++
		v.Case.UpdatedAt = now
		if e = tx.UpdateCase(ctx, v.Case, old); e != nil {
			return e
		}
		if e = addEvent(ctx, tx, s, v, op, p, map[string]any{"count": len(updates), "reviewsDigest": domain.Digest(in.Reviews)}); e != nil {
			return e
		}
		view, e := tx.LoadCase(ctx, id)
		if e != nil {
			return e
		}
		out = BatchReviewResult{CaseID: id, Revision: view.Case.Revision, View: view}
		for _, c := range view.Conflicts {
			if c.Status == domain.ResolutionAccepted {
				out.Accepted++
			} else if c.Status == domain.ResolutionReturned {
				out.Returned++
			} else if c.Status == domain.ResolutionSubmitted {
				out.Pending++
			}
		}
		return saveReplay(ctx, tx, key, op, id, p.Name, digest, 200, out, now)
	})
	return
}
