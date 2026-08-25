package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/audit"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/storage"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

type Service struct {
	store      *storage.Store
	now        func() time.Time
	newID      func() string
	traceMu    sync.Mutex
	traceCache map[traceCacheKey][]byte
}

func New(store *storage.Store) *Service {
	return &Service{store: store, now: time.Now, newID: randomID, traceCache: make(map[traceCacheKey][]byte)}
}
func randomID() string                   { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Service) Store() *storage.Store { return s.store }
func (s *Service) GetCase(ctx context.Context, id string) (domain.CaseView, error) {
	return s.store.LoadCase(ctx, id)
}
func profileCurrent(v domain.CaseView) (domain.EmissionProfile, error) {
	if len(v.Profiles) == 0 {
		return domain.EmissionProfile{}, domain.NewError(domain.CodeState, "profile", "案件没有发射参数")
	}
	return v.Profiles[len(v.Profiles)-1], nil
}
func previousDigest(v domain.CaseView) (int64, string) {
	if len(v.Events) == 0 {
		return 1, ""
	}
	e := v.Events[len(v.Events)-1]
	return e.Sequence + 1, e.Digest
}
func addEvent(ctx context.Context, tx *storage.Tx, s *Service, v domain.CaseView, kind string, p domain.Principal, payload any) error {
	seq, prev := previousDigest(v)
	return tx.AddEvent(ctx, audit.NewEvent(s.newID(), v.Case.ID, kind, p.Name, seq, prev, payload, s.now()))
}
func requestDigest(op, caseID string, p domain.Principal, command any) string {
	return domain.Digest(op, caseID, p.Name, command)
}
func saveReplay(ctx context.Context, tx *storage.Tx, key, op, caseID, actor, digest string, status int, value any, now time.Time) error {
	b, e := json.Marshal(value)
	if e != nil {
		return e
	}
	return tx.SaveIdempotency(ctx, domain.IdempotencyRecord{Key: key, Operation: op, CaseID: caseID, Actor: actor, RequestDigest: digest, StatusCode: status, ResponseJSON: string(b), CreatedAt: now})
}
func replayView(ctx context.Context, tx *storage.Tx, key, op, caseID, actor, digest string) (*domain.CaseView, error) {
	var v domain.CaseView
	found, err := replayInto(ctx, tx, key, op, caseID, actor, digest, &v)
	if err != nil || !found {
		return nil, err
	}
	return &v, nil
}
func replayInto(ctx context.Context, tx *storage.Tx, key, op, caseID, actor, digest string, dst any) (bool, error) {
	if key == "" {
		return false, domain.NewError(domain.CodeInvalid, "Idempotency-Key", "幂等键不能为空")
	}
	r, e := tx.GetIdempotency(ctx, key)
	if e != nil || r == nil {
		return false, e
	}
	caseMatches := r.CaseID == caseID || op == "create-case" && caseID == ""
	legacy := r.RequestDigest == ""
	if r.Operation != op || !caseMatches || !legacy && (r.Actor != actor || r.RequestDigest != digest) {
		return false, domain.NewDetailedError(domain.CodeConflict, "Idempotency-Key", "幂等键已被其他请求使用", map[string]any{"firstOperation": r.Operation, "firstCaseId": r.CaseID, "createdAt": r.CreatedAt})
	}
	if e = json.Unmarshal([]byte(r.ResponseJSON), dst); e != nil {
		return false, e
	}
	return true, nil
}
