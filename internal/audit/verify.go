package audit

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"fmt"
)

type Verification struct {
	Valid          bool   `json:"valid"`
	EventCount     int    `json:"eventCount"`
	HeadDigest     string `json:"headDigest"`
	BrokenSequence int64  `json:"brokenSequence,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

func Verify(events []domain.AuditEvent) Verification {
	v := Verification{Valid: true, EventCount: len(events)}
	previous := ""
	for i, e := range events {
		expectedSeq := int64(i + 1)
		if e.Sequence != expectedSeq || e.PreviousDigest != previous {
			v.Valid = false
			v.BrokenSequence = e.Sequence
			v.Reason = fmt.Sprintf("事件序号或前序摘要断裂，期望序号 %d", expectedSeq)
			return v
		}
		d := domain.Digest(e.CaseID, e.Sequence, e.EventType, e.Actor, e.PayloadDigest, e.PreviousDigest, e.CreatedAt)
		if d != e.Digest {
			v.Valid = false
			v.BrokenSequence = e.Sequence
			v.Reason = "事件内容摘要不一致"
			return v
		}
		previous = e.Digest
	}
	v.HeadDigest = previous
	return v
}
