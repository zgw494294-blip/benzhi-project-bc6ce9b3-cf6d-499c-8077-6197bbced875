package rebasedauditcheckpoint_test

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/httpapi"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestPermitRejectsRebasedAuditHistory(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "checkpoint.db")
	dsn := "file:" + databasePath + "?_pragma=foreign_keys(1)"
	store, err := storage.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	service := application.New(store)
	view := issuePermit(t, ctx, service)
	originalCheckpoint := view.Permit.AuditDigest

	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })

	events := append([]domain.AuditEvent(nil), view.Events...)
	events[0].Actor = "被替换的规划员"
	previous := ""
	var decisionPrevious string
	for i := range events {
		events[i].PreviousDigest = previous
		events[i].Digest = domain.Digest(events[i].CaseID, events[i].Sequence, events[i].EventType, events[i].Actor, events[i].PayloadDigest, events[i].PreviousDigest, events[i].CreatedAt)
		if events[i].EventType == "decision:"+view.Case.ID {
			decisionPrevious = events[i].PreviousDigest
		}
		if _, err = raw.ExecContext(ctx, `UPDATE audit_events SET actor=?,previous_digest=?,digest=? WHERE id=?`, events[i].Actor, events[i].PreviousDigest, events[i].Digest, events[i].ID); err != nil {
			t.Fatal(err)
		}
		previous = events[i].Digest
	}
	if decisionPrevious == "" || decisionPrevious == originalCheckpoint {
		t.Fatal("测试未形成与许可检查点不同的有效重算链")
	}

	// 使用全新的 Service，确保结果与任何进程内许可验证缓存无关。
	freshService := application.New(store)
	api := httpapi.New(freshService)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/change-cases/"+view.Case.ID+"/permit/verify", nil)
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("验证接口状态码=%d body=%s", response.Code, response.Body.String())
	}
	var result application.PermitVerification
	if err = json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Audit.Valid || !result.PermitDigestValid {
		t.Fatalf("测试没有进入链内有效且许可摘要有效的目标路径: %+v", result)
	}
	if result.Valid || result.AuditDigestValid {
		t.Fatalf("重算后的审计链脱离许可检查点仍被接受: %+v", result)
	}
}

func issuePermit(t *testing.T, ctx context.Context, service *application.Service) domain.CaseView {
	t.Helper()
	planner := domain.Principal{Name: "规划员甲", Role: domain.RolePlanner}
	start := time.Date(2026, 8, 25, 2, 0, 0, 0, time.UTC)
	view, err := service.CreateCase(ctx, "checkpoint-create", planner, application.CreateCaseInput{
		StationCode: "ST-CHECKPOINT", Title: "审计检查点案件", EffectiveFrom: start, EffectiveUntil: start.Add(2 * time.Hour),
		Profile: application.ProfileInput{FrequencyHz: 100_000_000, BandwidthHz: 20_000, PowerWatts: 10, AntennaGainDB: 3, AzimuthDegrees: 90, SiteLatitude: 30, SiteLongitude: 120},
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.AddTarget(ctx, view.Case.ID, "checkpoint-target", planner, application.TargetInput{
		ExpectedRevision: view.Case.Revision, Name: "远端保护对象", ServiceClass: "safety", FrequencyLowHz: 200_000_000,
		FrequencyHighHz: 200_020_000, MinimumSeparationHz: 100_000, FieldStrengthLimitDBUVM: 40, RuleReference: "RULE-CHECKPOINT",
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Submit(ctx, view.Case.ID, "checkpoint-submit", planner, application.RevisionInput{ExpectedRevision: view.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Freeze(ctx, view.Case.ID, "checkpoint-freeze", planner, application.RevisionInput{ExpectedRevision: view.Case.Revision})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Decide(ctx, view.Case.ID, "checkpoint-approve", domain.Principal{Name: "负责人甲", Role: domain.RoleLeader}, application.DecisionInput{ExpectedRevision: view.Case.Revision, Decision: "approve"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Permit == nil {
		t.Fatal("许可未签发")
	}
	return view
}
