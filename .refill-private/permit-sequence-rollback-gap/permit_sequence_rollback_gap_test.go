package permit_sequence_rollback_gap_test

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestPermitSequenceRollsBackWithApproval(t *testing.T) {
	ctx := context.Background()
	dsn := "file:" + filepath.Join(t.TempDir(), "review.db")
	store, err := storage.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := application.New(store)
	planner := domain.Principal{Name: "规划员甲", Role: domain.RolePlanner}
	leader := domain.Principal{Name: "负责人甲", Role: domain.RoleLeader}
	start := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)

	view, err := service.CreateCase(ctx, "rollback-create", planner, application.CreateCaseInput{
		StationCode: "ST-ROLLBACK", Title: "许可序号回滚复现",
		EffectiveFrom: start, EffectiveUntil: start.Add(2 * time.Hour),
		Profile: application.ProfileInput{FrequencyHz: 100_000_000, BandwidthHz: 20_000, PowerWatts: 10, AntennaGainDB: 3, AzimuthDegrees: 90, SiteLatitude: 30, SiteLongitude: 120},
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.AddTarget(ctx, view.Case.ID, "rollback-target", planner, application.TargetInput{
		ExpectedRevision: view.Case.Revision, Name: "远端保护台", ServiceClass: "safety",
		FrequencyLowHz: 200_000_000, FrequencyHighHz: 200_020_000,
		MinimumSeparationHz: 100_000, FieldStrengthLimitDBUVM: 40, RuleReference: "RULE-ROLLBACK",
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Submit(ctx, view.Case.ID, "rollback-submit", planner, application.RevisionInput{ExpectedRevision: view.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Freeze(ctx, view.Case.ID, "rollback-freeze", planner, application.RevisionInput{ExpectedRevision: view.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}

	control, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = control.Close() })
	if _, err = control.ExecContext(ctx, `CREATE TRIGGER fail_first_permit BEFORE INSERT ON permits BEGIN SELECT RAISE(ABORT, 'forced permit failure'); END`); err != nil {
		t.Fatal(err)
	}
	_, err = service.Decide(ctx, view.Case.ID, "rollback-approve-failed", leader, application.DecisionInput{ExpectedRevision: view.Case.Revision, Decision: "approve"})
	if err == nil {
		t.Fatal("故障注入未使首次批准事务回滚")
	}
	if _, err = control.ExecContext(ctx, `DROP TRIGGER fail_first_permit`); err != nil {
		t.Fatal(err)
	}
	afterFailure, err := service.GetCase(ctx, view.Case.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterFailure.Case.Status != domain.StatusFrozen || afterFailure.Permit != nil {
		t.Fatalf("失败批准留下了持久化副作用: status=%s permit=%+v", afterFailure.Case.Status, afterFailure.Permit)
	}

	retried, err := service.Decide(ctx, view.Case.ID, "rollback-approve-retry", leader, application.DecisionInput{ExpectedRevision: view.Case.Revision, Decision: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	if retried.Permit == nil {
		t.Fatal("重试批准未签发许可")
	}
	if retried.Permit.SerialNumber != "FC-2026-000001" {
		t.Fatalf("回滚事务消耗了许可序号: got %s, want FC-2026-000001", retried.Permit.SerialNumber)
	}
}
