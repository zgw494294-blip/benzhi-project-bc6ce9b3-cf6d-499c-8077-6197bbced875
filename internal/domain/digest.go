package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func Digest(parts ...any) string {
	b, _ := json.Marshal(parts)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func ProfileDigest(p EmissionProfile) string {
	return Digest(p.CaseID, p.BaselineNo, p.FrequencyHz, p.BandwidthHz, fmt.Sprintf("%.6f", p.PowerWatts), fmt.Sprintf("%.6f", p.AntennaGainDB), fmt.Sprintf("%.6f", p.AzimuthDegrees), fmt.Sprintf("%.7f", p.SiteLatitude), fmt.Sprintf("%.7f", p.SiteLongitude))
}
func PermitDigest(p ActivationPermit) string {
	return Digest(p.CaseID, p.SerialNumber, p.BaselineDigest, p.AuditDigest, p.ApprovedBy, p.IssuedAt.UTC().Format("2006-01-02T15:04:05.000000Z"))
}
