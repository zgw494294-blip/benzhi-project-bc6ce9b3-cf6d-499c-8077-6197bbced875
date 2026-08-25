package audit

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"testing"
	"time"
)

func TestVerifyDetectsTampering(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := NewEvent("1", "case", "created", "p", 1, "", map[string]string{"v": "1"}, now)
	b := NewEvent("2", "case", "submitted", "p", 2, a.Digest, map[string]string{"v": "2"}, now.Add(time.Second))
	if v := Verify([]domain.AuditEvent{a, b}); !v.Valid {
		t.Fatalf("valid chain rejected: %+v", v)
	}
	b.Actor = "intruder"
	if v := Verify([]domain.AuditEvent{a, b}); v.Valid || v.BrokenSequence != 2 {
		t.Fatalf("tampering not detected: %+v", v)
	}
}
