package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"testing"
	"time"
)

func testService(t *testing.T) *Service {
	t.Helper()
	store, e := storage.Open(context.Background(), "file:"+t.Name()+"?mode=memory&cache=shared")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { store.Close() })
	return New(store)
}
func createForTest(t *testing.T, s *Service) (domain.CaseView, domain.Principal) {
	t.Helper()
	planner := domain.Principal{Name: "规划员甲", Role: domain.RolePlanner}
	now := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	in := CreateCaseInput{StationCode: "ST-001", Title: "测试案件", EffectiveFrom: now.Add(time.Hour), EffectiveUntil: now.Add(3 * time.Hour), Profile: ProfileInput{FrequencyHz: 100_000_000, BandwidthHz: 20_000, PowerWatts: 100, AntennaGainDB: 6, AzimuthDegrees: 90, SiteLatitude: 30, SiteLongitude: 120}}
	v, e := s.CreateCase(context.Background(), "create-1", planner, in)
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.CreateCase(context.Background(), "create-1", planner, in)
	if e != nil {
		t.Fatal(e)
	}
	if again.Case.ID != v.Case.ID || len(again.Events) != 1 {
		t.Fatal("idempotency replay changed result")
	}
	return v, planner
}
func TestPassingWorkflowIssuesImmutablePermit(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	v, planner := createForTest(t, s)
	target := TargetInput{ExpectedRevision: v.Case.Revision, Name: "航空保护台", ServiceClass: "safety", FrequencyLowHz: 200_000_000, FrequencyHighHz: 200_020_000, MinimumSeparationHz: 100_000, FieldStrengthLimitDBUVM: 40, RuleReference: "RULE-A"}
	v, e := s.AddTarget(ctx, v.Case.ID, "target-1", planner, target)
	if e != nil {
		t.Fatal(e)
	}
	v, e = s.Submit(ctx, v.Case.ID, "submit-1", planner, RevisionInput{ExpectedRevision: v.Case.Revision})
	if e != nil {
		t.Fatal(e)
	}
	if v.Case.Status != domain.StatusReviewed {
		t.Fatalf("status=%s", v.Case.Status)
	}
	v, e = s.Freeze(ctx, v.Case.ID, "freeze-1", planner, RevisionInput{ExpectedRevision: v.Case.Revision})
	if e != nil {
		t.Fatal(e)
	}
	leader := domain.Principal{Name: "负责人", Role: domain.RoleLeader}
	v, e = s.Decide(ctx, v.Case.ID, "approve-1", leader, DecisionInput{ExpectedRevision: v.Case.Revision, Decision: "approve"})
	if e != nil {
		t.Fatal(e)
	}
	if v.Permit == nil || v.Case.Status != domain.StatusApproved {
		t.Fatal("permit not issued")
	}
	verified, e := s.VerifyPermit(ctx, v.Case.ID)
	if e != nil || !verified.Valid {
		t.Fatalf("permit invalid: %+v %v", verified, e)
	}
	_, e = s.UpdateCase(ctx, v.Case.ID, "late-edit", planner, UpdateCaseInput{ExpectedRevision: v.Case.Revision})
	if domain.ErrorCodeOf(e) != domain.CodeState {
		t.Fatalf("approved case was editable: %v", e)
	}
}
func TestConflictRemediationRequiresIndependentReviewer(t *testing.T) {
	ctx := context.Background()
	s := testService(t)
	v, planner := createForTest(t, s)
	target := TargetInput{ExpectedRevision: v.Case.Revision, Name: "同频保护台", ServiceClass: "broadcast", FrequencyLowHz: 99_990_000, FrequencyHighHz: 100_010_000, MinimumSeparationHz: 50_000, FieldStrengthLimitDBUVM: 0, RuleReference: "RULE-B"}
	v, e := s.AddTarget(ctx, v.Case.ID, "target", planner, target)
	if e != nil {
		t.Fatal(e)
	}
	v, e = s.Submit(ctx, v.Case.ID, "submit", planner, RevisionInput{ExpectedRevision: v.Case.Revision})
	if e != nil {
		t.Fatal(e)
	}
	if v.Case.Status != domain.StatusRemediating || len(v.Conflicts) == 0 {
		t.Fatalf("expected conflicts: %+v", v)
	}
	reviewer := domain.Principal{Name: "复核员乙", Role: domain.RoleReviewer}
	conflictIDs := make([]string, len(v.Conflicts))
	for i := range v.Conflicts {
		conflictIDs[i] = v.Conflicts[i].ID
	}
	for i, id := range conflictIDs {
		adjusted := ProfileInput{FrequencyHz: 300_000_000 + int64(i)*10_000_000, BandwidthHz: 10_000, PowerWatts: 0.001, AntennaGainDB: 0, AzimuthDegrees: 180, SiteLatitude: 30, SiteLongitude: 120}
		v, e = s.SubmitResolution(ctx, v.Case.ID, id, "resolution-"+id, planner, ResolutionInput{ExpectedRevision: v.Case.Revision, ResolutionText: "调整至远离保护频段", EvidenceDigest: "sha256:evidence-" + id, AdjustedProfile: adjusted})
		if e != nil {
			t.Fatal(e)
		}
		_, e = s.ReviewResolution(ctx, v.Case.ID, id, "self-review-"+id, domain.Principal{Name: planner.Name, Role: domain.RoleReviewer}, ReviewInput{ExpectedRevision: v.Case.Revision, Decision: "accept"})
		if domain.ErrorCodeOf(e) != domain.CodeForbidden {
			t.Fatalf("self review accepted: %v", e)
		}
		v, e = s.ReviewResolution(ctx, v.Case.ID, id, "review-"+id, reviewer, ReviewInput{ExpectedRevision: v.Case.Revision, Decision: "accept", Comment: "证据充分"})
		if e != nil {
			t.Fatal(e)
		}
	}
	v, e = s.Freeze(ctx, v.Case.ID, "freeze", planner, RevisionInput{ExpectedRevision: v.Case.Revision})
	if e != nil {
		t.Fatal(e)
	}
	if v.Case.Status != domain.StatusFrozen {
		t.Fatalf("status=%s", v.Case.Status)
	}
}
func TestExpectedRevisionPreventsOverwrite(t *testing.T) {
	s := testService(t)
	v, planner := createForTest(t, s)
	_, e := s.AddTarget(context.Background(), v.Case.ID, "wrong-revision", planner, TargetInput{ExpectedRevision: v.Case.Revision + 1, Name: "x", FrequencyLowHz: 1, FrequencyHighHz: 2, RuleReference: "r"})
	if domain.ErrorCodeOf(e) != domain.CodeConflict {
		t.Fatalf("expected revision conflict, got %v", e)
	}
}
