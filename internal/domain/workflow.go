package domain

import (
	"fmt"
	"sort"
	"time"
)

var requiredRuleCodes = []string{"field-strength", "frequency-separation"}

func PreviewFor(c FrequencyChangeCase, profile EmissionProfile, targets []ProtectionTarget) (CheckPreview, error) {
	if c.Status != StatusDraft {
		return CheckPreview{}, NewError(CodeState, "status", "仅草拟案件可预览送审核验")
	}
	if len(targets) == 0 {
		return CheckPreview{}, NewError(CodeInvalid, "targets", "至少需要一个保护对象")
	}
	if err := ValidateProfile(profile); err != nil {
		return CheckPreview{}, err
	}
	items := make([]CheckPreviewItem, 0, len(targets)*2)
	fails := 0
	for _, target := range targets {
		for _, check := range Evaluate(profile, target, profile.BaselineNo, "", time.Time{}) {
			if check.Result == CheckFail {
				fails++
			}
			items = append(items, CheckPreviewItem{
				TargetID: target.ID, RuleCode: check.RuleCode, RuleVersion: check.RuleVersion,
				InputDigest: check.InputDigest, InputSummary: map[string]any{
					"profileDigest": ProfileDigest(profile), "frequencyHz": profile.FrequencyHz,
					"bandwidthHz": profile.BandwidthHz, "powerWatts": profile.PowerWatts,
					"targetRangeHz":       []int64{target.FrequencyLowHz, target.FrequencyHighHz},
					"minimumSeparationHz": target.MinimumSeparationHz, "fieldStrengthLimitDbuvm": target.FieldStrengthLimitDBUVM,
				},
				MeasuredMargin: check.MeasuredMargin, Result: check.Result,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TargetID == items[j].TargetID {
			return items[i].RuleCode < items[j].RuleCode
		}
		return items[i].TargetID < items[j].TargetID
	})
	targetDigests := make([]string, 0, len(targets))
	for _, t := range targets {
		targetDigests = append(targetDigests, Digest(t.ID, t.Name, t.ServiceClass, t.FrequencyLowHz, t.FrequencyHighHz, t.MinimumSeparationHz, t.FieldStrengthLimitDBUVM, t.RuleReference))
	}
	sort.Strings(targetDigests)
	digest := Digest(c.ID, c.Revision, ProfileDigest(profile), targetDigests, items, RuleVersion)
	return CheckPreview{CaseID: c.ID, Revision: c.Revision, RuleVersion: RuleVersion, Results: items, ExpectedConflictCount: fails, PreviewDigest: digest}, nil
}

func FreezeDiagnosis(v CaseView) FreezeReadiness {
	r := FreezeReadiness{CaseID: v.Case.ID, Revision: v.Case.Revision, Counts: map[ResolutionStatus]int{
		ResolutionOpen: 0, ResolutionSubmitted: 0, ResolutionReturned: 0, ResolutionAccepted: 0,
	}}
	for _, c := range v.Conflicts {
		r.Counts[c.Status]++
		if c.Status != ResolutionAccepted {
			next := "等待独立复核"
			if c.Status == ResolutionOpen || c.Status == ResolutionReturned {
				next = "提交整改"
			}
			r.Blockers = append(r.Blockers, FreezeBlocker{Code: "conflict_not_accepted", ConflictID: c.ID, NextAction: next, Message: "冲突尚未接受"})
		}
	}
	if v.Case.Status != StatusRemediating && v.Case.Status != StatusReviewed {
		r.Blockers = append(r.Blockers, FreezeBlocker{Code: "case_status", Message: "案件状态不允许冻结"})
	}
	if len(v.Profiles) == 0 {
		r.Blockers = append(r.Blockers, FreezeBlocker{Code: "profile_missing", Message: "缺少当前参数基线"})
		r.Ready = false
		return r
	}
	profile := v.Profiles[len(v.Profiles)-1]
	if profile.SealedAt == nil {
		r.Blockers = append(r.Blockers, FreezeBlocker{Code: "profile_unsealed", Message: "当前参数基线尚未封存"})
	}
	latest := make(map[string]InterferenceCheck)
	for _, c := range v.Checks {
		if c.BaselineNo != profile.BaselineNo {
			continue
		}
		key := c.TargetID + "\x00" + c.RuleCode
		old, ok := latest[key]
		if !ok || c.CheckedAt.After(old.CheckedAt) || c.CheckedAt.Equal(old.CheckedAt) && c.ID > old.ID {
			latest[key] = c
		}
	}
	for _, target := range v.Targets {
		expected := Evaluate(profile, target, profile.BaselineNo, "", time.Time{})
		for _, rule := range requiredRuleCodes {
			key := target.ID + "\x00" + rule
			check, ok := latest[key]
			if !ok {
				r.Blockers = append(r.Blockers, FreezeBlocker{Code: "missing_rule", TargetID: target.ID, RuleCode: rule, Message: "当前基线缺少规则结果"})
				continue
			}
			if check.RuleVersion != RuleVersion {
				r.Blockers = append(r.Blockers, FreezeBlocker{Code: "stale_rule_version", TargetID: target.ID, RuleCode: rule, Message: "规则版本已过期"})
			}
			var wanted InterferenceCheck
			for _, x := range expected {
				if x.RuleCode == rule {
					wanted = x
				}
			}
			if check.InputDigest != wanted.InputDigest {
				r.Blockers = append(r.Blockers, FreezeBlocker{Code: "input_digest_mismatch", TargetID: target.ID, RuleCode: rule, Message: "检查输入摘要与当前参数不一致"})
			}
			if check.Result != CheckPass {
				r.Blockers = append(r.Blockers, FreezeBlocker{Code: "check_failed", TargetID: target.ID, RuleCode: rule, Message: "当前规则核验未通过"})
			}
		}
	}
	sort.Slice(r.Blockers, func(i, j int) bool {
		return fmt.Sprint(r.Blockers[i]) < fmt.Sprint(r.Blockers[j])
	})
	r.Ready = len(r.Blockers) == 0
	return r
}
