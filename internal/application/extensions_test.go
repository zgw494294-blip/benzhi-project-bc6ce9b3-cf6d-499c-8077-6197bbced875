package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"context"
	"testing"
	"time"
)

func passingTarget(revision int64, name string, low int64) TargetInput {
	return TargetInput{ExpectedRevision: revision, Name: name, ServiceClass: "safety", FrequencyLowHz: low, FrequencyHighHz: low + 20_000, MinimumSeparationHz: 100_000, FieldStrengthLimitDBUVM: 40, RuleReference: "RULE-PASS"}
}

func TestPreviewIsReadOnlyAndDigestGuardsSubmit(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	v, planner := createForTest(t, s)
	v, err := s.AddTarget(ctx, v.Case.ID, "preview-target", planner, passingTarget(v.Case.Revision, "保护台", 200_000_000))
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewChecks(ctx, v.Case.ID, planner)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Results) != 2 || preview.PreviewDigest == "" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	before := v.Case.Revision
	again, err := s.PreviewChecks(ctx, v.Case.ID, planner)
	if err != nil || again.PreviewDigest != preview.PreviewDigest {
		t.Fatalf("preview not stable: %v", err)
	}
	current := v.Profiles[len(v.Profiles)-1]
	v, err = s.UpdateCase(ctx, v.Case.ID, "preview-edit", planner, UpdateCaseInput{ExpectedRevision: v.Case.Revision, Title: v.Case.Title, EffectiveFrom: v.Case.EffectiveFrom, EffectiveUntil: v.Case.EffectiveUntil, Profile: ProfileInput{FrequencyHz: current.FrequencyHz, BandwidthHz: current.BandwidthHz, PowerWatts: current.PowerWatts / 2, AntennaGainDB: current.AntennaGainDB, AzimuthDegrees: current.AzimuthDegrees, SiteLatitude: current.SiteLatitude, SiteLongitude: current.SiteLongitude}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Submit(ctx, v.Case.ID, "stale-preview", planner, RevisionInput{ExpectedRevision: v.Case.Revision, PreviewDigest: preview.PreviewDigest})
	if domain.ErrorCodeOf(err) != domain.CodeConflict {
		t.Fatalf("stale digest accepted: %v", err)
	}
	unchanged, _ := s.GetCase(ctx, v.Case.ID)
	if unchanged.Case.Revision != before+1 || len(unchanged.Checks) != 0 {
		t.Fatal("stale preview submit changed case")
	}
}

func TestTargetBatchAtomicAndIdempotencyPayloadBound(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	v, planner := createForTest(t, s)
	in := TargetBatchInput{ExpectedRevision: v.Case.Revision, Creates: []TargetMutationInput{{Name: "目标一", ServiceClass: "broadcast", FrequencyLowHz: 200_000_000, FrequencyHighHz: 200_010_000, MinimumSeparationHz: 1_000, FieldStrengthLimitDBUVM: 30, RuleReference: "R1"}, {Name: "目标二", ServiceClass: "safety", FrequencyLowHz: 300_000_000, FrequencyHighHz: 300_010_000, MinimumSeparationHz: 1_000, FieldStrengthLimitDBUVM: 30, RuleReference: "R2"}}}
	v, err := s.BatchTargets(ctx, v.Case.ID, "target-batch", planner, in)
	if err != nil {
		t.Fatal(err)
	}
	if v.Case.Revision != 2 || len(v.Targets) != 2 || len(v.Events) != 2 {
		t.Fatalf("batch not atomic: %+v", v)
	}
	replay, err := s.BatchTargets(ctx, v.Case.ID, "target-batch", planner, in)
	if err != nil || replay.Case.Revision != v.Case.Revision {
		t.Fatalf("replay failed: %v", err)
	}
	in.Creates[0].FieldStrengthLimitDBUVM = 31
	_, err = s.BatchTargets(ctx, v.Case.ID, "target-batch", planner, in)
	if domain.ErrorCodeOf(err) != domain.CodeConflict {
		t.Fatalf("idempotency key misuse not rejected: %v", err)
	}
	after, _ := s.GetCase(ctx, v.Case.ID)
	if after.Case.Revision != 2 || len(after.Targets) != 2 {
		t.Fatal("misuse changed case")
	}
}

func TestBatchResolutionAndReviewUseSingleRevision(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	v, planner := createForTest(t, s)
	target := TargetInput{ExpectedRevision: v.Case.Revision, Name: "同频目标", ServiceClass: "broadcast", FrequencyLowHz: 99_990_000, FrequencyHighHz: 100_010_000, MinimumSeparationHz: 50_000, FieldStrengthLimitDBUVM: 0, RuleReference: "R"}
	v, err := s.AddTarget(ctx, v.Case.ID, "batch-conflict-target", planner, target)
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Submit(ctx, v.Case.ID, "batch-conflict-submit", planner, RevisionInput{ExpectedRevision: v.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(v.Conflicts))
	items := make([]ConflictResolutionItem, len(v.Conflicts))
	for i, c := range v.Conflicts {
		ids[i] = c.ID
		items[i] = ConflictResolutionItem{ConflictID: c.ID, ResolutionText: "统一迁频", EvidenceDigest: "sha256:evidence-" + c.ID}
	}
	profiles := len(v.Profiles)
	revision := v.Case.Revision
	v, err = s.BatchResolve(ctx, v.Case.ID, "batch-resolve", planner, BatchResolutionInput{ExpectedRevision: revision, ConflictIDs: ids, AdjustedProfile: ProfileInput{FrequencyHz: 300_000_000, BandwidthHz: 10_000, PowerWatts: 0.001, AntennaGainDB: 0, AzimuthDegrees: 180, SiteLatitude: 30, SiteLongitude: 120}, Resolutions: items})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Profiles) != profiles+1 || v.Case.Revision != revision+1 {
		t.Fatalf("batch resolution made multiple baselines/revisions: %+v", v)
	}
	reviews := make([]ConflictReviewItem, len(ids))
	for i, id := range ids {
		decision := "accept"
		if i == len(ids)-1 {
			decision = "return"
		}
		reviews[i] = ConflictReviewItem{ConflictID: id, Decision: decision, Comment: "批量复核"}
	}
	result, err := s.BatchReview(ctx, v.Case.ID, "batch-review", domain.Principal{Name: "复核员", Role: domain.RoleReviewer}, BatchReviewInput{ExpectedRevision: v.Case.Revision, Reviews: reviews})
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision != v.Case.Revision+1 || result.Accepted != len(ids)-1 || result.Returned != 1 {
		t.Fatalf("unexpected batch review: %+v", result)
	}
}

func TestTraceAndReadinessAreReadOnly(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	v, planner := createForTest(t, s)
	v, err := s.AddTarget(ctx, v.Case.ID, "trace-target", planner, passingTarget(v.Case.Revision, "轨迹目标", 200_000_000))
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Submit(ctx, v.Case.ID, "trace-submit", planner, RevisionInput{ExpectedRevision: v.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	revision, events := v.Case.Revision, len(v.Events)
	trace, err := s.CheckTrace(ctx, v.Case.ID, TraceQuery{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(trace.Baselines) != 1 || len(trace.Baselines[0].Checks) != 1 || trace.NextCursor == "" {
		t.Fatalf("unexpected trace: %+v", trace)
	}
	secondPage, err := s.CheckTrace(ctx, v.Case.ID, TraceQuery{Limit: 1, Cursor: trace.NextCursor})
	if err != nil || len(secondPage.Baselines) != 1 || len(secondPage.Baselines[0].Checks) != 1 || secondPage.NextCursor != "" {
		t.Fatalf("unexpected trace page 2: %+v %v", secondPage, err)
	}
	ready, err := s.FreezeReadiness(ctx, v.Case.ID)
	if err != nil || !ready.Ready {
		t.Fatalf("not ready: %+v %v", ready, err)
	}
	after, _ := s.GetCase(ctx, v.Case.ID)
	if after.Case.Revision != revision || len(after.Events) != events {
		t.Fatal("read query changed aggregate")
	}
}

func TestSubmitRechecksStationWindowWithoutChangingDraft(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	first, planner := createForTest(t, s)
	baseProfile := first.Profiles[0]
	second, err := s.CreateCase(ctx, "window-second", planner, CreateCaseInput{StationCode: first.Case.StationCode, Title: "并发窗口草稿", EffectiveFrom: first.Case.EffectiveFrom.Add(time.Hour), EffectiveUntil: first.Case.EffectiveUntil.Add(time.Hour), Profile: ProfileInput{FrequencyHz: baseProfile.FrequencyHz, BandwidthHz: baseProfile.BandwidthHz, PowerWatts: baseProfile.PowerWatts, AntennaGainDB: baseProfile.AntennaGainDB, AzimuthDegrees: baseProfile.AzimuthDegrees, SiteLatitude: baseProfile.SiteLatitude, SiteLongitude: baseProfile.SiteLongitude}})
	if err != nil {
		t.Fatal(err)
	}
	first, err = s.AddTarget(ctx, first.Case.ID, "window-first-target", planner, passingTarget(first.Case.Revision, "首案目标", 200_000_000))
	if err != nil {
		t.Fatal(err)
	}
	first, err = s.Submit(ctx, first.Case.ID, "window-first-submit", planner, RevisionInput{ExpectedRevision: first.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	first, err = s.Freeze(ctx, first.Case.ID, "window-first-freeze", planner, RevisionInput{ExpectedRevision: first.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	second, err = s.AddTarget(ctx, second.Case.ID, "window-second-target", planner, passingTarget(second.Case.Revision, "次案目标", 220_000_000))
	if err != nil {
		t.Fatal(err)
	}
	revision, events := second.Case.Revision, len(second.Events)
	_, err = s.Submit(ctx, second.Case.ID, "window-second-submit", planner, RevisionInput{ExpectedRevision: second.Case.Revision})
	if domain.ErrorCodeOf(err) != domain.CodeConflict {
		t.Fatalf("overlapping submit accepted: %v", err)
	}
	after, _ := s.GetCase(ctx, second.Case.ID)
	if after.Case.Revision != revision || len(after.Events) != events || len(after.Checks) != 0 || after.Profiles[0].SealedAt != nil {
		t.Fatal("window conflict changed draft")
	}
	_, err = s.CreateCase(ctx, "window-adjacent", planner, CreateCaseInput{StationCode: first.Case.StationCode, Title: "边界相接案件", EffectiveFrom: first.Case.EffectiveUntil, EffectiveUntil: first.Case.EffectiveUntil.Add(time.Hour), Profile: ProfileInput{FrequencyHz: baseProfile.FrequencyHz, BandwidthHz: baseProfile.BandwidthHz, PowerWatts: baseProfile.PowerWatts, AntennaGainDB: baseProfile.AntennaGainDB, AzimuthDegrees: baseProfile.AzimuthDegrees, SiteLatitude: baseProfile.SiteLatitude, SiteLongitude: baseProfile.SiteLongitude}})
	if err != nil {
		t.Fatalf("adjacent window rejected: %v", err)
	}
}
