package domain

import (
	"math"
	"time"
)

const RuleVersion = "emc-2026.1"

func Evaluate(profile EmissionProfile, target ProtectionTarget, baseline int, previous string, now time.Time) []InterferenceCheck {
	low := float64(profile.FrequencyHz) - float64(profile.BandwidthHz)/2
	high := float64(profile.FrequencyHz) + float64(profile.BandwidthHz)/2
	distance := 0.0
	if high < float64(target.FrequencyLowHz) {
		distance = float64(target.FrequencyLowHz) - high
	} else if low > float64(target.FrequencyHighHz) {
		distance = low - float64(target.FrequencyHighHz)
	}
	sepMargin := distance - float64(target.MinimumSeparationHz)
	sepResult := CheckPass
	if sepMargin < 0 {
		sepResult = CheckFail
	}
	input := Digest(ProfileDigest(profile), target.ID, target.FrequencyLowHz, target.FrequencyHighHz, target.MinimumSeparationHz)
	sep := InterferenceCheck{CaseID: profile.CaseID, BaselineNo: baseline, TargetID: target.ID, RuleCode: "frequency-separation", RuleVersion: RuleVersion, InputDigest: input, MeasuredMargin: sepMargin, Result: sepResult, PreviousCheckID: previous, CheckedAt: now}
	// Conservative deterministic free-space proxy: ERP converted to dB and attenuated by frequency separation.
	erpDB := 10*math.Log10(profile.PowerWatts) + profile.AntennaGainDB
	attenuation := 20 * math.Log10(1+distance/1000)
	predicted := erpDB - attenuation
	fieldMargin := target.FieldStrengthLimitDBUVM - predicted
	fieldResult := CheckPass
	if fieldMargin < 0 {
		fieldResult = CheckFail
	}
	field := InterferenceCheck{CaseID: profile.CaseID, BaselineNo: baseline, TargetID: target.ID, RuleCode: "field-strength", RuleVersion: RuleVersion, InputDigest: Digest(input, profile.PowerWatts, profile.AntennaGainDB, target.FieldStrengthLimitDBUVM), MeasuredMargin: fieldMargin, Result: fieldResult, PreviousCheckID: previous, CheckedAt: now}
	return []InterferenceCheck{sep, field}
}
