package trace_output_revalidation

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

func TestTraceRejectsForgedRuleOutput(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "trace.db")
	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := application.New(store)
	planner := domain.Principal{Name: "规划员", Role: domain.RolePlanner}
	base := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	view, err := service.CreateCase(ctx, "create", planner, application.CreateCaseInput{StationCode: "ST-TRACE", Title: "轨迹输出复算", EffectiveFrom: base, EffectiveUntil: base.Add(time.Hour), Profile: application.ProfileInput{FrequencyHz: 100_000_000, BandwidthHz: 10_000, PowerWatts: 1, AntennaGainDB: 0, AzimuthDegrees: 0, SiteLatitude: 30, SiteLongitude: 120}})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.AddTarget(ctx, view.Case.ID, "target", planner, application.TargetInput{ExpectedRevision: view.Case.Revision, Name: "保护对象", ServiceClass: "safety", FrequencyLowHz: 200_000_000, FrequencyHighHz: 200_010_000, MinimumSeparationHz: 100_000, FieldStrengthLimitDBUVM: 40, RuleReference: "RULE-PASS"})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Submit(ctx, view.Case.ID, "submit", planner, application.RevisionInput{ExpectedRevision: view.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Checks) == 0 {
		t.Fatal("送审后没有规则检查")
	}
	forgedID := view.Checks[0].ID

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := db.ExecContext(ctx, "UPDATE checks SET measured_margin=measured_margin+12345, result='fail' WHERE id=?", forgedID)
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		t.Fatalf("改写检查输出失败：changed=%d err=%v", changed, err)
	}

	trace, err := service.CheckTrace(ctx, view.Case.ID, application.TraceQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, baseline := range trace.Baselines {
		for _, point := range baseline.Checks {
			if point.Check.ID == forgedID {
				if point.Reproducible {
					t.Fatalf("规则输出已被改写但轨迹仍标记为可复现：result=%s margin=%v", point.Check.Result, point.Check.MeasuredMargin)
				}
				return
			}
		}
	}
	t.Fatal("轨迹中缺少被改写的检查")
}
