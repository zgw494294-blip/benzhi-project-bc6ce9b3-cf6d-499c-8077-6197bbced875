package preview_cache_alias_test

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"testing"
	"time"
)

func TestPreviewCacheDoesNotShareMutableState(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, "file:preview-cache-alias?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	service := application.New(store)
	planner := domain.Principal{Name: "规划员甲", Role: domain.RolePlanner}
	start := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	created, err := service.CreateCase(ctx, "preview-alias-create", planner, application.CreateCaseInput{
		StationCode:    "ST-PREVIEW-ALIAS",
		Title:          "预览缓存所有权测试",
		EffectiveFrom:  start.Add(time.Hour),
		EffectiveUntil: start.Add(3 * time.Hour),
		Profile: application.ProfileInput{
			FrequencyHz: 100_000_000, BandwidthHz: 20_000, PowerWatts: 100,
			AntennaGainDB: 6, AzimuthDegrees: 90, SiteLatitude: 30, SiteLongitude: 120,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.AddTarget(ctx, created.Case.ID, "preview-alias-target", planner, application.TargetInput{
		ExpectedRevision: created.Case.Revision,
		Name:             "航空保护台", ServiceClass: "safety",
		FrequencyLowHz: 200_000_000, FrequencyHighHz: 200_020_000,
		MinimumSeparationHz: 100_000, FieldStrengthLimitDBUVM: 40, RuleReference: "RULE-PREVIEW",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.PreviewChecks(ctx, view.Case.ID, planner)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Results) == 0 {
		t.Fatal("预览没有规则结果")
	}
	originalFrequency := first.Results[0].InputSummary["frequencyHz"]
	originalRange := append([]int64(nil), first.Results[0].InputSummary["targetRangeHz"].([]int64)...)
	first.Results[0].InputSummary["frequencyHz"] = int64(1)
	first.Results[0].InputSummary["targetRangeHz"].([]int64)[0] = 1

	second, err := service.PreviewChecks(ctx, view.Case.ID, planner)
	if err != nil {
		t.Fatal(err)
	}
	gotFrequency := second.Results[0].InputSummary["frequencyHz"]
	gotRange := second.Results[0].InputSummary["targetRangeHz"].([]int64)
	if gotFrequency != originalFrequency || gotRange[0] != originalRange[0] {
		t.Fatalf("缓存返回了被前一调用方污染的预览: frequencyHz=%v targetRangeHz=%v previewDigest=%s", gotFrequency, gotRange, second.PreviewDigest)
	}
}
