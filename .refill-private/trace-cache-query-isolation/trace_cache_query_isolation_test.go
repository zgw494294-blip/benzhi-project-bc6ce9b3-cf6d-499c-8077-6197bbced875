package trace_cache_query_isolation_test

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/httpapi"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestTraceCacheSeparatesQueryDimensions(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "trace-cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	service := application.New(store)
	planner := domain.Principal{Name: "规划员甲", Role: domain.RolePlanner}
	start := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	view, err := service.CreateCase(ctx, "trace-create", planner, application.CreateCaseInput{
		StationCode:    "TRACE-001",
		Title:          "轨迹缓存隔离复现案件",
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
	view, err = service.AddTarget(ctx, view.Case.ID, "trace-target", planner, application.TargetInput{
		ExpectedRevision:        view.Case.Revision,
		Name:                    "同频保护台",
		ServiceClass:            "safety",
		FrequencyLowHz:          99_990_000,
		FrequencyHighHz:         100_010_000,
		MinimumSeparationHz:     50_000,
		FieldStrengthLimitDBUVM: 0,
		RuleReference:           "RULE-TRACE-A",
	})
	if err != nil {
		t.Fatal(err)
	}
	firstTargetID := view.Targets[0].ID
	view, err = service.AddTarget(ctx, view.Case.ID, "trace-target-2", planner, application.TargetInput{
		ExpectedRevision:        view.Case.Revision,
		Name:                    "异频保护台",
		ServiceClass:            "broadcast",
		FrequencyLowHz:          110_000_000,
		FrequencyHighHz:         110_020_000,
		MinimumSeparationHz:     50_000,
		FieldStrengthLimitDBUVM: 20,
		RuleReference:           "RULE-TRACE-B",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondTargetID := ""
	for _, target := range view.Targets {
		if target.ID != firstTargetID {
			secondTargetID = target.ID
		}
	}
	if secondTargetID == "" {
		t.Fatal("第二个保护对象未持久化")
	}
	view, err = service.Submit(ctx, view.Case.ID, "trace-submit", planner, application.RevisionInput{ExpectedRevision: view.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Conflicts) == 0 {
		t.Fatal("初始基线未产生预期冲突")
	}
	conflictIDs := make([]string, len(view.Conflicts))
	resolutions := make([]application.ConflictResolutionItem, len(view.Conflicts))
	for i, conflict := range view.Conflicts {
		conflictIDs[i] = conflict.ID
		resolutions[i] = application.ConflictResolutionItem{
			ConflictID: conflict.ID, ResolutionText: "统一迁频后全量复核", EvidenceDigest: "sha256:trace-" + conflict.ID,
		}
	}
	view, err = service.BatchResolve(ctx, view.Case.ID, "trace-resolve", planner, application.BatchResolutionInput{
		ExpectedRevision: view.Case.Revision,
		ConflictIDs:      conflictIDs,
		AdjustedProfile: application.ProfileInput{
			FrequencyHz: 300_000_000, BandwidthHz: 10_000, PowerWatts: 0.001,
			AntennaGainDB: 0, AzimuthDegrees: 180, SiteLatitude: 30, SiteLongitude: 120,
		},
		Resolutions: resolutions,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := httpapi.New(service).Handler()
	readTrace := func(query string) application.CheckTrace {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/change-cases/"+view.Case.ID+"/checks/trace?"+query, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Fatalf("trace query %s returned status %d: %s", query, response.Code, response.Body.String())
		}
		var trace application.CheckTrace
		if err := json.Unmarshal(response.Body.Bytes(), &trace); err != nil {
			t.Fatal(err)
		}
		return trace
	}

	first := readTrace("baselineNo=0&targetId=" + firstTargetID + "&ruleCode=frequency-separation")
	assertTraceSelection(t, first, 0, firstTargetID, "frequency-separation")
	second := readTrace("baselineNo=1&targetId=" + secondTargetID + "&ruleCode=field-strength")
	assertTraceSelection(t, second, 1, secondTargetID, "field-strength")
}

func assertTraceSelection(t *testing.T, trace application.CheckTrace, baselineNo int, targetID, ruleCode string) {
	t.Helper()
	count := 0
	for _, baseline := range trace.Baselines {
		for _, point := range baseline.Checks {
			count++
			if baseline.BaselineNo != baselineNo || point.Check.TargetID != targetID || point.Check.RuleCode != ruleCode {
				t.Fatalf("query for baseline=%d target=%s rule=%s reused cached selection baseline=%d target=%s rule=%s", baselineNo, targetID, ruleCode, baseline.BaselineNo, point.Check.TargetID, point.Check.RuleCode)
			}
		}
	}
	if count != 1 {
		t.Fatalf("selected trace returned %d checks, want 1", count)
	}
}
