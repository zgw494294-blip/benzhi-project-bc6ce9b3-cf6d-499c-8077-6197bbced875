package freeze_readiness_cache_race_test

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/application"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func approveCase(t *testing.T, service *application.Service, suffix string) string {
	t.Helper()
	ctx := context.Background()
	planner := domain.Principal{Name: "规划员甲", Role: domain.RolePlanner}
	leader := domain.Principal{Name: "负责人甲", Role: domain.RoleLeader}
	windowStart := time.Date(2027, 1, 2, 3, 0, 0, 0, time.UTC)
	created, err := service.CreateCase(ctx, "create-"+suffix, planner, application.CreateCaseInput{
		StationCode:    "ST-" + suffix,
		Title:          "并发缓存复现案件 " + suffix,
		EffectiveFrom:  windowStart,
		EffectiveUntil: windowStart.Add(2 * time.Hour),
		Profile: application.ProfileInput{
			FrequencyHz: 100_000_000, BandwidthHz: 20_000, PowerWatts: 100,
			AntennaGainDB: 6, AzimuthDegrees: 90, SiteLatitude: 30, SiteLongitude: 120,
		},
	})
	if err != nil {
		t.Fatalf("创建案件 %s: %v", suffix, err)
	}
	withTarget, err := service.AddTarget(ctx, created.Case.ID, "target-"+suffix, planner, application.TargetInput{
		ExpectedRevision: created.Case.Revision,
		Name:             "远端保护对象", ServiceClass: "safety",
		FrequencyLowHz: 200_000_000, FrequencyHighHz: 200_020_000,
		MinimumSeparationHz: 100_000, FieldStrengthLimitDBUVM: 40, RuleReference: "RULE-A",
	})
	if err != nil {
		t.Fatalf("增加保护对象 %s: %v", suffix, err)
	}
	reviewed, err := service.Submit(ctx, created.Case.ID, "submit-"+suffix, planner, application.RevisionInput{ExpectedRevision: withTarget.Case.Revision})
	if err != nil {
		t.Fatalf("送审 %s: %v", suffix, err)
	}
	frozen, err := service.Freeze(ctx, created.Case.ID, "freeze-"+suffix, planner, application.RevisionInput{ExpectedRevision: reviewed.Case.Revision})
	if err != nil {
		t.Fatalf("冻结 %s: %v", suffix, err)
	}
	approved, err := service.Decide(ctx, created.Case.ID, "approve-"+suffix, leader, application.DecisionInput{
		ExpectedRevision: frozen.Case.Revision, Decision: "approve", Comment: "同意启用",
	})
	if err != nil || approved.Case.Status != domain.StatusApproved {
		t.Fatalf("批准 %s: status=%s err=%v", suffix, approved.Case.Status, err)
	}
	return approved.Case.ID
}

func TestFreezeReadinessCacheIsSynchronized(t *testing.T) {
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "review.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := application.New(store)
	cachedCaseID := approveCase(t, service, "cached")
	uncachedCaseID := approveCase(t, service, "uncached")
	if _, err = service.FreezeReadiness(context.Background(), cachedCaseID); err != nil {
		t.Fatalf("预热缓存: %v", err)
	}

	const readers = 8
	start := make(chan struct{})
	errorsFound := make(chan error, readers+1)
	var group sync.WaitGroup
	for i := 0; i < readers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			for n := 0; n < 20_000; n++ {
				if _, readErr := service.FreezeReadiness(context.Background(), cachedCaseID); readErr != nil {
					errorsFound <- readErr
					return
				}
			}
		}()
	}
	group.Add(1)
	go func() {
		defer group.Done()
		<-start
		_, writeErr := service.FreezeReadiness(context.Background(), uncachedCaseID)
		if writeErr != nil {
			errorsFound <- writeErr
		}
	}()
	close(start)
	group.Wait()
	close(errorsFound)
	for callErr := range errorsFound {
		t.Error(fmt.Errorf("并发查询失败: %w", callErr))
	}
}
