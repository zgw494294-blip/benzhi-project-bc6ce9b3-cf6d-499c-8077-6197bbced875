package permit_audit_cache_stale_test

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestPermitVerificationInvalidatesAuditCache(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "frequency-review.db")
	store, err := storage.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	service := application.New(store)
	planner := domain.Principal{Name: "规划员甲", Role: domain.RolePlanner}
	start := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	view, err := service.CreateCase(ctx, "cache-create", planner, application.CreateCaseInput{
		StationCode:    "CACHE-001",
		Title:          "许可缓存失效复现",
		EffectiveFrom:  start,
		EffectiveUntil: start.Add(time.Hour),
		Profile: application.ProfileInput{
			FrequencyHz: 100_000_000, BandwidthHz: 10_000, PowerWatts: 1,
			AntennaGainDB: 1, AzimuthDegrees: 90, SiteLatitude: 30, SiteLongitude: 120,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.AddTarget(ctx, view.Case.ID, "cache-target", planner, application.TargetInput{
		ExpectedRevision:        view.Case.Revision,
		Name:                    "远端保护对象",
		ServiceClass:            "safety",
		FrequencyLowHz:          200_000_000,
		FrequencyHighHz:         200_010_000,
		MinimumSeparationHz:     100_000,
		FieldStrengthLimitDBUVM: 40,
		RuleReference:           "CACHE-RULE",
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Submit(ctx, view.Case.ID, "cache-submit", planner, application.RevisionInput{ExpectedRevision: view.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Freeze(ctx, view.Case.ID, "cache-freeze", planner, application.RevisionInput{ExpectedRevision: view.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Decide(ctx, view.Case.ID, "cache-approve", domain.Principal{Name: "负责人甲", Role: domain.RoleLeader}, application.DecisionInput{
		ExpectedRevision: view.Case.Revision,
		Decision:         "approve",
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.VerifyPermit(ctx, view.Case.ID)
	if err != nil || !first.Valid {
		t.Fatalf("首次许可验证应有效: valid=%v err=%v", first.Valid, err)
	}

	tamperDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := tamperDB.ExecContext(ctx, `UPDATE audit_events SET actor=? WHERE case_id=? AND sequence=1`, "被改写的主体", view.Case.ID)
	if err != nil {
		_ = tamperDB.Close()
		t.Fatal(err)
	}
	if err = tamperDB.Close(); err != nil {
		t.Fatal(err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		t.Fatalf("审计事件改写失败: changed=%d err=%v", changed, err)
	}

	uncached, err := application.New(store).VerifyPermit(ctx, view.Case.ID)
	if err != nil || uncached.Valid {
		t.Fatalf("复现夹具未形成可检测的审计断链: valid=%v err=%v", uncached.Valid, err)
	}
	second, err := service.VerifyPermit(ctx, view.Case.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Valid {
		t.Fatal("持久化审计事件已改写，缓存命中的许可验证仍返回 valid=true")
	}
}
