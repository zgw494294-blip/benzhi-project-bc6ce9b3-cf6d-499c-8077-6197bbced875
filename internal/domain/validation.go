package domain

import (
	"math"
	"strings"
	"time"
)

func ValidateCase(c FrequencyChangeCase) error {
	if strings.TrimSpace(c.StationCode) == "" {
		return NewError(CodeInvalid, "stationCode", "台站标识不能为空")
	}
	if strings.TrimSpace(c.Title) == "" {
		return NewError(CodeInvalid, "title", "标题不能为空")
	}
	if c.EffectiveFrom.IsZero() || !c.EffectiveUntil.After(c.EffectiveFrom) {
		return NewError(CodeInvalid, "effectiveUntil", "生效结束时间必须晚于开始时间")
	}
	if c.EffectiveUntil.Sub(c.EffectiveFrom) > 366*24*time.Hour {
		return NewError(CodeInvalid, "effectiveUntil", "生效窗口跨度不能超过 366 天")
	}
	return nil
}
func NormalizeWindow(from, until time.Time) (time.Time, time.Time, error) {
	from, until = NormalizeTime(from), NormalizeTime(until)
	c := FrequencyChangeCase{StationCode: "window", Title: "window", EffectiveFrom: from, EffectiveUntil: until}
	if err := ValidateCase(c); err != nil {
		return time.Time{}, time.Time{}, err
	}
	return from, until, nil
}

func WindowsOverlap(aFrom, aUntil, bFrom, bUntil time.Time) (time.Time, time.Time, bool) {
	from := aFrom
	if bFrom.After(from) {
		from = bFrom
	}
	until := aUntil
	if bUntil.Before(until) {
		until = bUntil
	}
	return from, until, until.After(from)
}
func ValidateProfile(p EmissionProfile) error {
	if p.FrequencyHz < 1000 || p.FrequencyHz > 300_000_000_000 {
		return NewError(CodeInvalid, "frequencyHz", "频率超出允许范围")
	}
	if p.BandwidthHz <= 0 || p.BandwidthHz >= p.FrequencyHz {
		return NewError(CodeInvalid, "bandwidthHz", "带宽必须为合理正数")
	}
	if p.PowerWatts <= 0 || p.PowerWatts > 10_000_000 || math.IsNaN(p.PowerWatts) {
		return NewError(CodeInvalid, "powerWatts", "发射功率无效")
	}
	if p.AntennaGainDB < -50 || p.AntennaGainDB > 100 {
		return NewError(CodeInvalid, "antennaGainDb", "天线增益无效")
	}
	if p.AzimuthDegrees < 0 || p.AzimuthDegrees >= 360 {
		return NewError(CodeInvalid, "azimuthDegrees", "方位角必须在 0 到 360 之间")
	}
	if p.SiteLatitude < -90 || p.SiteLatitude > 90 || p.SiteLongitude < -180 || p.SiteLongitude > 180 {
		return NewError(CodeInvalid, "site", "站址经纬度无效")
	}
	return nil
}
func ValidateTarget(t ProtectionTarget) error {
	if strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.ServiceClass) == "" || strings.TrimSpace(t.RuleReference) == "" {
		return NewError(CodeInvalid, "target", "保护对象名称与依据不能为空")
	}
	if t.FrequencyLowHz <= 0 || t.FrequencyHighHz < t.FrequencyLowHz || t.FrequencyHighHz > 300_000_000_000 {
		return NewError(CodeInvalid, "frequencyRange", "保护频率范围无效")
	}
	if t.MinimumSeparationHz < 0 || t.MinimumSeparationHz > 300_000_000_000 {
		return NewError(CodeInvalid, "minimumSeparationHz", "最小间隔不能为负数")
	}
	if math.IsNaN(t.FieldStrengthLimitDBUVM) || math.IsInf(t.FieldStrengthLimitDBUVM, 0) || t.FieldStrengthLimitDBUVM < -300 || t.FieldStrengthLimitDBUVM > 300 {
		return NewError(CodeInvalid, "fieldStrengthLimitDbuvm", "场强门限无效")
	}
	return nil
}
func ValidateResolution(text, evidence, actor string) error {
	if strings.TrimSpace(text) == "" || strings.TrimSpace(evidence) == "" || strings.TrimSpace(actor) == "" {
		return NewError(CodeInvalid, "resolution", "方案、证据摘要和提交人不能为空")
	}
	return nil
}
func NormalizeTime(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }
