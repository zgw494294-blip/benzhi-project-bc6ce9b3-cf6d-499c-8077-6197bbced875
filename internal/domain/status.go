package domain

type CaseStatus string

const (
	StatusDraft       CaseStatus = "draft"
	StatusReviewing   CaseStatus = "reviewing"
	StatusRemediating CaseStatus = "remediating"
	StatusReviewed    CaseStatus = "reviewed"
	StatusFrozen      CaseStatus = "frozen"
	StatusApproved    CaseStatus = "approved"
	StatusRejected    CaseStatus = "rejected"
)

func (s CaseStatus) Mutable() bool { return s == StatusDraft }
func (s CaseStatus) CanTransition(next CaseStatus) bool {
	switch s {
	case StatusDraft:
		return next == StatusReviewing
	case StatusReviewing:
		return next == StatusReviewed || next == StatusRemediating
	case StatusRemediating:
		return next == StatusReviewed
	case StatusReviewed:
		return next == StatusFrozen || next == StatusRemediating
	case StatusFrozen:
		return next == StatusApproved || next == StatusRejected
	default:
		return false
	}
}
func Transition(c *FrequencyChangeCase, next CaseStatus) error {
	if !c.Status.CanTransition(next) {
		return NewError(CodeState, "status", "当前状态不允许该操作")
	}
	c.Status = next
	c.Revision++
	return nil
}
