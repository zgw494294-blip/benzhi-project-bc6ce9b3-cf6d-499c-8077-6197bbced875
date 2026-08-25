package domain

import "time"

type FrequencyChangeCase struct {
	ID             string     `json:"id"`
	StationCode    string     `json:"stationCode"`
	Title          string     `json:"title"`
	Status         CaseStatus `json:"status"`
	Revision       int64      `json:"revision"`
	EffectiveFrom  time.Time  `json:"effectiveFrom"`
	EffectiveUntil time.Time  `json:"effectiveUntil"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}
type EmissionProfile struct {
	ID             string     `json:"id"`
	CaseID         string     `json:"caseId"`
	BaselineNo     int        `json:"baselineNo"`
	FrequencyHz    int64      `json:"frequencyHz"`
	BandwidthHz    int64      `json:"bandwidthHz"`
	PowerWatts     float64    `json:"powerWatts"`
	AntennaGainDB  float64    `json:"antennaGainDb"`
	AzimuthDegrees float64    `json:"azimuthDegrees"`
	SiteLatitude   float64    `json:"siteLatitude"`
	SiteLongitude  float64    `json:"siteLongitude"`
	SealedAt       *time.Time `json:"sealedAt,omitempty"`
}
type ProtectionTarget struct {
	ID                      string  `json:"id"`
	CaseID                  string  `json:"caseId"`
	Name                    string  `json:"name"`
	ServiceClass            string  `json:"serviceClass"`
	FrequencyLowHz          int64   `json:"frequencyLowHz"`
	FrequencyHighHz         int64   `json:"frequencyHighHz"`
	MinimumSeparationHz     int64   `json:"minimumSeparationHz"`
	FieldStrengthLimitDBUVM float64 `json:"fieldStrengthLimitDbuvm"`
	RuleReference           string  `json:"ruleReference"`
}
type CheckResult string

const (
	CheckPass CheckResult = "pass"
	CheckFail CheckResult = "fail"
)

type InterferenceCheck struct {
	ID              string      `json:"id"`
	CaseID          string      `json:"caseId"`
	BaselineNo      int         `json:"baselineNo"`
	TargetID        string      `json:"targetId"`
	RuleCode        string      `json:"ruleCode"`
	RuleVersion     string      `json:"ruleVersion"`
	InputDigest     string      `json:"inputDigest"`
	MeasuredMargin  float64     `json:"measuredMargin"`
	Result          CheckResult `json:"result"`
	PreviousCheckID string      `json:"previousCheckId,omitempty"`
	CheckedAt       time.Time   `json:"checkedAt"`
}
type ResolutionStatus string

const (
	ResolutionOpen      ResolutionStatus = "open"
	ResolutionSubmitted ResolutionStatus = "submitted"
	ResolutionAccepted  ResolutionStatus = "accepted"
	ResolutionReturned  ResolutionStatus = "returned"
)

type ConflictResolution struct {
	ID             string           `json:"id"`
	CaseID         string           `json:"caseId"`
	CheckID        string           `json:"checkId"`
	Status         ResolutionStatus `json:"status"`
	ResolutionText string           `json:"resolutionText,omitempty"`
	EvidenceDigest string           `json:"evidenceDigest,omitempty"`
	SubmittedBy    string           `json:"submittedBy,omitempty"`
	ReviewedBy     string           `json:"reviewedBy,omitempty"`
	ReviewComment  string           `json:"reviewComment,omitempty"`
	ReviewedAt     *time.Time       `json:"reviewedAt,omitempty"`
	UpdatedAt      time.Time        `json:"updatedAt"`
}
type ActivationPermit struct {
	ID             string    `json:"id"`
	CaseID         string    `json:"caseId"`
	SerialNumber   string    `json:"serialNumber"`
	BaselineDigest string    `json:"baselineDigest"`
	AuditDigest    string    `json:"auditDigest"`
	ApprovedBy     string    `json:"approvedBy"`
	IssuedAt       time.Time `json:"issuedAt"`
	PermitDigest   string    `json:"permitDigest"`
}
type AuditEvent struct {
	ID             string    `json:"id"`
	CaseID         string    `json:"caseId"`
	Sequence       int64     `json:"sequence"`
	EventType      string    `json:"eventType"`
	Actor          string    `json:"actor"`
	PayloadDigest  string    `json:"payloadDigest"`
	PreviousDigest string    `json:"previousDigest"`
	Digest         string    `json:"digest"`
	CreatedAt      time.Time `json:"createdAt"`
}
type IdempotencyRecord struct {
	Key, Operation, CaseID string
	Actor                  string
	RequestDigest          string
	StatusCode             int
	ResponseJSON           string
	CreatedAt              time.Time
}

type WindowOccupancy struct {
	CaseID       string     `json:"caseId"`
	Status       CaseStatus `json:"status"`
	OverlapFrom  time.Time  `json:"overlapFrom"`
	OverlapUntil time.Time  `json:"overlapUntil"`
}

type CheckPreviewItem struct {
	TargetID       string         `json:"targetId"`
	RuleCode       string         `json:"ruleCode"`
	RuleVersion    string         `json:"ruleVersion"`
	InputDigest    string         `json:"inputDigest"`
	InputSummary   map[string]any `json:"inputSummary"`
	MeasuredMargin float64        `json:"measuredMargin"`
	Result         CheckResult    `json:"result"`
}

type CheckPreview struct {
	CaseID                string             `json:"caseId"`
	Revision              int64              `json:"revision"`
	RuleVersion           string             `json:"ruleVersion"`
	Results               []CheckPreviewItem `json:"results"`
	ExpectedConflictCount int                `json:"expectedConflictCount"`
	PreviewDigest         string             `json:"previewDigest"`
}

type FreezeBlocker struct {
	Code       string `json:"code"`
	ConflictID string `json:"conflictId,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	RuleCode   string `json:"ruleCode,omitempty"`
	NextAction string `json:"nextAction,omitempty"`
	Message    string `json:"message"`
}

type FreezeReadiness struct {
	CaseID   string                   `json:"caseId"`
	Revision int64                    `json:"revision"`
	Ready    bool                     `json:"ready"`
	Counts   map[ResolutionStatus]int `json:"conflictCounts"`
	Blockers []FreezeBlocker          `json:"blockers"`
}
type CaseView struct {
	Case      FrequencyChangeCase  `json:"case"`
	Profiles  []EmissionProfile    `json:"profiles"`
	Targets   []ProtectionTarget   `json:"targets"`
	Checks    []InterferenceCheck  `json:"checks"`
	Conflicts []ConflictResolution `json:"conflicts"`
	Permit    *ActivationPermit    `json:"permit,omitempty"`
	Events    []AuditEvent         `json:"timeline"`
}
