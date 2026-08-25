package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"context"
	"encoding/json"
	"strconv"
	"time"
)

type TraceQuery struct {
	BaselineNo *int
	TargetID   string
	RuleCode   string
	Cursor     string
	Limit      int
}
type CheckTracePoint struct {
	Check          domain.InterferenceCheck `json:"check"`
	MarginDelta    *float64                 `json:"marginDelta,omitempty"`
	ResultChanged  *bool                    `json:"resultChanged,omitempty"`
	ConflictStatus domain.ResolutionStatus  `json:"conflictStatus,omitempty"`
	Reproducible   bool                     `json:"reproducible"`
	Anomalies      []string                 `json:"anomalies,omitempty"`
}
type CheckTraceBaseline struct {
	BaselineNo int               `json:"baselineNo"`
	Checks     []CheckTracePoint `json:"checks"`
}
type CheckTrace struct {
	CaseID     string               `json:"caseId"`
	Baselines  []CheckTraceBaseline `json:"baselines"`
	NextCursor string               `json:"nextCursor,omitempty"`
}

type traceCacheKey struct {
	caseID   string
	revision int64
	cursor   string
	limit    int
}

func newTraceCacheKey(caseID string, revision int64, q TraceQuery) traceCacheKey {
	return traceCacheKey{caseID: caseID, revision: revision, cursor: q.Cursor, limit: q.Limit}
}

func (s *Service) loadTraceProjection(caseID string, revision int64, q TraceQuery) (CheckTrace, bool) {
	key := newTraceCacheKey(caseID, revision, q)
	s.traceMu.Lock()
	payload, ok := s.traceCache[key]
	s.traceMu.Unlock()
	if !ok {
		return CheckTrace{}, false
	}
	var trace CheckTrace
	if json.Unmarshal(payload, &trace) != nil {
		return CheckTrace{}, false
	}
	return trace, true
}

func (s *Service) saveTraceProjection(caseID string, revision int64, q TraceQuery, trace CheckTrace) {
	payload, err := json.Marshal(trace)
	if err != nil {
		return
	}
	key := newTraceCacheKey(caseID, revision, q)
	s.traceMu.Lock()
	s.traceCache[key] = payload
	s.traceMu.Unlock()
}

func (s *Service) CheckTrace(ctx context.Context, id string, q TraceQuery) (CheckTrace, error) {
	v, err := s.store.LoadTraceCase(ctx, id)
	if err != nil {
		return CheckTrace{}, err
	}
	if q.Limit == 0 {
		q.Limit = 50
	}
	if q.Limit < 1 || q.Limit > 100 {
		return CheckTrace{}, domain.NewError(domain.CodeInvalid, "limit", "limit 必须在 1 到 100 之间")
	}
	if q.BaselineNo != nil {
		found := false
		for _, p := range v.Profiles {
			if p.BaselineNo == *q.BaselineNo {
				found = true
			}
		}
		if !found {
			return CheckTrace{}, domain.NewError(domain.CodeNotFound, "baselineNo", "基线不存在")
		}
	}
	if q.TargetID != "" {
		found := false
		for _, t := range v.Targets {
			if t.ID == q.TargetID {
				found = true
			}
		}
		if !found {
			return CheckTrace{}, domain.NewError(domain.CodeNotFound, "targetId", "保护对象不存在")
		}
	}
	if q.RuleCode != "" && q.RuleCode != "frequency-separation" && q.RuleCode != "field-strength" {
		return CheckTrace{}, domain.NewError(domain.CodeNotFound, "ruleCode", "规则不存在")
	}
	offset := 0
	if q.Cursor != "" {
		offset, err = strconv.Atoi(q.Cursor)
		if err != nil || offset < 0 {
			return CheckTrace{}, domain.NewError(domain.CodeInvalid, "cursor", "分页游标无效")
		}
	}
	if cached, ok := s.loadTraceProjection(id, v.Case.Revision, q); ok {
		return cached, nil
	}
	checks, total, hasMore, err := s.store.ReadChecksPage(ctx, id, q.BaselineNo, q.TargetID, q.RuleCode, q.Limit, offset)
	if err != nil {
		return CheckTrace{}, err
	}
	if offset > total {
		return CheckTrace{}, domain.NewError(domain.CodeInvalid, "cursor", "分页游标超出范围")
	}
	index := map[string]domain.InterferenceCheck{}
	previousIDs := make([]string, 0, len(checks))
	previousSeen := map[string]bool{}
	checkIDs := make([]string, 0, len(checks))
	for _, c := range checks {
		index[c.ID] = c
		checkIDs = append(checkIDs, c.ID)
		if c.PreviousCheckID != "" && !previousSeen[c.PreviousCheckID] {
			previousSeen[c.PreviousCheckID] = true
			previousIDs = append(previousIDs, c.PreviousCheckID)
		}
	}
	previous, err := s.store.ReadChecksByID(ctx, previousIDs)
	if err != nil {
		return CheckTrace{}, err
	}
	for checkID, check := range previous {
		index[checkID] = check
	}
	profiles := map[int]domain.EmissionProfile{}
	for _, p := range v.Profiles {
		profiles[p.BaselineNo] = p
	}
	targets := map[string]domain.ProtectionTarget{}
	for _, t := range v.Targets {
		targets[t.ID] = t
	}
	statuses, err := s.store.TraceConflictStatuses(ctx, id, checkIDs)
	if err != nil {
		return CheckTrace{}, err
	}
	out := CheckTrace{CaseID: id}
	groups := map[int]int{}
	for _, c := range checks {
		point := CheckTracePoint{Check: c, ConflictStatus: statuses[c.ID], Reproducible: true}
		if c.PreviousCheckID != "" {
			prev, ok := index[c.PreviousCheckID]
			if !ok {
				owner, exists, lookupErr := s.store.CheckOwner(ctx, c.PreviousCheckID)
				if lookupErr != nil {
					return CheckTrace{}, lookupErr
				}
				if exists && owner != id {
					point.Anomalies = append(point.Anomalies, "cross_case_previous")
				} else {
					point.Anomalies = append(point.Anomalies, "missing_previous")
				}
				point.Reproducible = false
			} else if prev.CaseID != id {
				point.Anomalies = append(point.Anomalies, "cross_case_previous")
				point.Reproducible = false
			} else {
				d := c.MeasuredMargin - prev.MeasuredMargin
				changed := c.Result != prev.Result
				point.MarginDelta = &d
				point.ResultChanged = &changed
			}
		}
		profile, pok := profiles[c.BaselineNo]
		target, tok := targets[c.TargetID]
		if !pok {
			point.Anomalies = append(point.Anomalies, "missing_profile")
			point.Reproducible = false
		}
		if !tok {
			point.Anomalies = append(point.Anomalies, "missing_target")
			point.Reproducible = false
		}
		if c.RuleVersion != domain.RuleVersion {
			point.Anomalies = append(point.Anomalies, "unsupported_rule_version")
			point.Reproducible = false
		}
		if pok && tok && c.RuleVersion == domain.RuleVersion {
			var expected string
			for _, x := range domain.Evaluate(profile, target, c.BaselineNo, "", time.Time{}) {
				if x.RuleCode == c.RuleCode {
					expected = x.InputDigest
				}
			}
			if expected == "" || expected != c.InputDigest {
				point.Anomalies = append(point.Anomalies, "input_digest_mismatch")
				point.Reproducible = false
			}
		}
		idx, ok := groups[c.BaselineNo]
		if !ok {
			idx = len(out.Baselines)
			groups[c.BaselineNo] = idx
			out.Baselines = append(out.Baselines, CheckTraceBaseline{BaselineNo: c.BaselineNo})
		}
		out.Baselines[idx].Checks = append(out.Baselines[idx].Checks, point)
	}
	if hasMore {
		out.NextCursor = strconv.Itoa(offset + len(checks))
	}
	s.saveTraceProjection(id, v.Case.Revision, q, out)
	return out, nil
}
