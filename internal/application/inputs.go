package application

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"time"
)

type ProfileInput struct {
	FrequencyHz    int64   `json:"frequencyHz"`
	BandwidthHz    int64   `json:"bandwidthHz"`
	PowerWatts     float64 `json:"powerWatts"`
	AntennaGainDB  float64 `json:"antennaGainDb"`
	AzimuthDegrees float64 `json:"azimuthDegrees"`
	SiteLatitude   float64 `json:"siteLatitude"`
	SiteLongitude  float64 `json:"siteLongitude"`
}

func (i ProfileInput) entity(id, caseID string, no int) domain.EmissionProfile {
	return domain.EmissionProfile{ID: id, CaseID: caseID, BaselineNo: no, FrequencyHz: i.FrequencyHz, BandwidthHz: i.BandwidthHz, PowerWatts: i.PowerWatts, AntennaGainDB: i.AntennaGainDB, AzimuthDegrees: i.AzimuthDegrees, SiteLatitude: i.SiteLatitude, SiteLongitude: i.SiteLongitude}
}

type CreateCaseInput struct {
	StationCode    string       `json:"stationCode"`
	Title          string       `json:"title"`
	EffectiveFrom  time.Time    `json:"effectiveFrom"`
	EffectiveUntil time.Time    `json:"effectiveUntil"`
	Profile        ProfileInput `json:"profile"`
}
type UpdateCaseInput struct {
	ExpectedRevision int64        `json:"expectedRevision"`
	Title            string       `json:"title"`
	EffectiveFrom    time.Time    `json:"effectiveFrom"`
	EffectiveUntil   time.Time    `json:"effectiveUntil"`
	Profile          ProfileInput `json:"profile"`
}
type TargetInput struct {
	ExpectedRevision        int64   `json:"expectedRevision"`
	Name                    string  `json:"name"`
	ServiceClass            string  `json:"serviceClass"`
	FrequencyLowHz          int64   `json:"frequencyLowHz"`
	FrequencyHighHz         int64   `json:"frequencyHighHz"`
	MinimumSeparationHz     int64   `json:"minimumSeparationHz"`
	FieldStrengthLimitDBUVM float64 `json:"fieldStrengthLimitDbuvm"`
	RuleReference           string  `json:"ruleReference"`
}
type RevisionInput struct {
	ExpectedRevision int64  `json:"expectedRevision"`
	PreviewDigest    string `json:"previewDigest,omitempty"`
}
type ResolutionInput struct {
	ExpectedRevision int64        `json:"expectedRevision"`
	ResolutionText   string       `json:"resolutionText"`
	EvidenceDigest   string       `json:"evidenceDigest"`
	AdjustedProfile  ProfileInput `json:"adjustedProfile"`
}
type ReviewInput struct {
	ExpectedRevision int64  `json:"expectedRevision"`
	Decision         string `json:"decision"`
	Comment          string `json:"comment"`
}
type DecisionInput struct {
	ExpectedRevision int64  `json:"expectedRevision"`
	Decision         string `json:"decision"`
	Comment          string `json:"comment"`
}

type TargetMutationInput struct {
	ID                      string  `json:"id,omitempty"`
	Name                    string  `json:"name"`
	ServiceClass            string  `json:"serviceClass"`
	FrequencyLowHz          int64   `json:"frequencyLowHz"`
	FrequencyHighHz         int64   `json:"frequencyHighHz"`
	MinimumSeparationHz     int64   `json:"minimumSeparationHz"`
	FieldStrengthLimitDBUVM float64 `json:"fieldStrengthLimitDbuvm"`
	RuleReference           string  `json:"ruleReference"`
}
type TargetBatchInput struct {
	ExpectedRevision int64                 `json:"expectedRevision"`
	Creates          []TargetMutationInput `json:"creates,omitempty"`
	Updates          []TargetMutationInput `json:"updates,omitempty"`
	Deletes          []string              `json:"deletes,omitempty"`
}
type ConflictResolutionItem struct {
	ConflictID     string `json:"conflictId"`
	ResolutionText string `json:"resolutionText"`
	EvidenceDigest string `json:"evidenceDigest"`
}
type BatchResolutionInput struct {
	ExpectedRevision int64                    `json:"expectedRevision"`
	ConflictIDs      []string                 `json:"conflictIds"`
	AdjustedProfile  ProfileInput             `json:"adjustedProfile"`
	Resolutions      []ConflictResolutionItem `json:"resolutions"`
}
type ConflictReviewItem struct {
	ConflictID string `json:"conflictId"`
	Decision   string `json:"decision"`
	Comment    string `json:"comment"`
}
type BatchReviewInput struct {
	ExpectedRevision int64                `json:"expectedRevision"`
	Reviews          []ConflictReviewItem `json:"reviews"`
}
type BatchReviewResult struct {
	CaseID   string          `json:"caseId"`
	Revision int64           `json:"revision"`
	Accepted int             `json:"accepted"`
	Returned int             `json:"returned"`
	Pending  int             `json:"pending"`
	View     domain.CaseView `json:"view"`
}
