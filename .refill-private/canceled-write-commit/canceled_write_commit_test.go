package canceledwritecommit_test

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"errors"
	"testing"
	"time"
)

func TestCanceledWriteDoesNotCommit(t *testing.T) {
	store, err := storage.Open(context.Background(), "file:canceled-write-commit?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	changeCase := domain.FrequencyChangeCase{
		ID:             "case-canceled-write",
		StationCode:    "ST-CANCEL",
		Title:          "取消边界事务",
		Status:         domain.StatusDraft,
		Revision:       1,
		EffectiveFrom:  now.Add(time.Hour),
		EffectiveUntil: now.Add(2 * time.Hour),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	profile := domain.EmissionProfile{
		ID:            "profile-canceled-write",
		CaseID:        changeCase.ID,
		BaselineNo:    0,
		FrequencyHz:   100_000_000,
		BandwidthHz:   25_000,
		PowerWatts:    10,
		SiteLatitude:  30,
		SiteLongitude: 120,
	}

	ctx, cancel := context.WithCancel(context.Background())
	err = store.Write(ctx, func(tx *storage.Tx) error {
		if writeErr := tx.CreateCase(ctx, changeCase, profile); writeErr != nil {
			return writeErr
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("写调用应返回 context.Canceled，实际为 %v", err)
	}

	_, err = store.LoadCase(context.Background(), changeCase.ID)
	if err == nil {
		t.Fatalf("已取消的写事务仍被提交，案件 %s 可被读取", changeCase.ID)
	}
	if domain.ErrorCodeOf(err) != domain.CodeNotFound {
		t.Fatalf("读取已取消事务应返回 not_found，实际为 %v", err)
	}
}
