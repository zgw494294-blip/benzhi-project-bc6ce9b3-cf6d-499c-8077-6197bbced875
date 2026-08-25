package domain

import (
	"testing"
	"time"
)

func TestEvaluateDeterministic(t *testing.T) {
	p := EmissionProfile{CaseID: "c", FrequencyHz: 100_000_000, BandwidthHz: 10_000, PowerWatts: 10, AntennaGainDB: 3}
	target := ProtectionTarget{ID: "t", FrequencyLowHz: 99_990_000, FrequencyHighHz: 100_010_000, MinimumSeparationHz: 20_000, FieldStrengthLimitDBUVM: 10}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := Evaluate(p, target, 1, "", now)
	b := Evaluate(p, target, 1, "", now)
	if len(a) != 2 || len(b) != 2 {
		t.Fatalf("unexpected checks")
	}
	for i := range a {
		if a[i].InputDigest != b[i].InputDigest || a[i].Result != b[i].Result {
			t.Fatalf("rule output is not deterministic")
		}
	}
	if a[0].Result != CheckFail {
		t.Fatalf("overlapping frequency should fail separation")
	}
}
func TestValidationRejectsInvalidValues(t *testing.T) {
	if ValidateProfile(EmissionProfile{FrequencyHz: 100, BandwidthHz: 0}) == nil {
		t.Fatal("invalid profile accepted")
	}
	if ValidateTarget(ProtectionTarget{FrequencyLowHz: 2, FrequencyHighHz: 1}) == nil {
		t.Fatal("invalid target accepted")
	}
}

func TestEffectiveWindowsUseHalfOpenIntervals(t *testing.T) {
	base := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	if _, _, overlap := WindowsOverlap(base, base.Add(2*time.Hour), base.Add(2*time.Hour), base.Add(3*time.Hour)); overlap {
		t.Fatal("边界相接的半开窗口不应重叠")
	}
	from, until, overlap := WindowsOverlap(base, base.Add(2*time.Hour), base.Add(time.Hour), base.Add(3*time.Hour))
	if !overlap || !from.Equal(base.Add(time.Hour)) || !until.Equal(base.Add(2*time.Hour)) {
		t.Fatalf("重叠区间错误: %s %s", from, until)
	}
	if _, _, err := NormalizeWindow(base, base.Add(367*24*time.Hour)); err == nil {
		t.Fatal("超长生效窗口未被拒绝")
	}
}
