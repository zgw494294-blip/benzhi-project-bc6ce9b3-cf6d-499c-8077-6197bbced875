package freeze_window_recheck

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"testing"
	"time"
)

func TestFreezeRechecksWindowOccupancy(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, "file:freeze-window-recheck?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := application.New(store)
	planner := domain.Principal{Name: "规划员", Role: domain.RolePlanner}
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	profile := application.ProfileInput{FrequencyHz: 100_000_000, BandwidthHz: 10_000, PowerWatts: 1, AntennaGainDB: 0, AzimuthDegrees: 0, SiteLatitude: 30, SiteLongitude: 120}

	first, err := service.CreateCase(ctx, "create-first", planner, application.CreateCaseInput{StationCode: "ST-WINDOW", Title: "首案", EffectiveFrom: base, EffectiveUntil: base.Add(3 * time.Hour), Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateCase(ctx, "create-second", planner, application.CreateCaseInput{StationCode: "ST-WINDOW", Title: "次案", EffectiveFrom: base.Add(time.Hour), EffectiveUntil: base.Add(4 * time.Hour), Profile: profile})
	if err != nil {
		t.Fatal(err)
	}

	addPassingTarget := func(view domain.CaseView, key, name string) domain.CaseView {
		t.Helper()
		out, addErr := service.AddTarget(ctx, view.Case.ID, key, planner, application.TargetInput{ExpectedRevision: view.Case.Revision, Name: name, ServiceClass: "safety", FrequencyLowHz: 200_000_000, FrequencyHighHz: 200_010_000, MinimumSeparationHz: 100_000, FieldStrengthLimitDBUVM: 40, RuleReference: "RULE-PASS"})
		if addErr != nil {
			t.Fatal(addErr)
		}
		return out
	}
	first = addPassingTarget(first, "target-first", "首案目标")
	second = addPassingTarget(second, "target-second", "次案目标")
	first, err = service.Submit(ctx, first.Case.ID, "submit-first", planner, application.RevisionInput{ExpectedRevision: first.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	second, err = service.Submit(ctx, second.Case.ID, "submit-second", planner, application.RevisionInput{ExpectedRevision: second.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	first, err = service.Freeze(ctx, first.Case.ID, "freeze-first", planner, application.RevisionInput{ExpectedRevision: first.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	second, err = service.Freeze(ctx, second.Case.ID, "freeze-second", planner, application.RevisionInput{ExpectedRevision: second.Case.Revision})
	if err == nil {
		t.Fatalf("重叠窗口在首案已冻结后仍被冻结：first=%s second=%s", first.Case.Status, second.Case.Status)
	}
	if domain.ErrorCodeOf(err) != domain.CodeConflict {
		t.Fatalf("重叠冻结应返回 conflict，实际为 %v", err)
	}
}
