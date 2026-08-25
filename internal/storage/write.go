package storage

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"context"
	"database/sql"
	"encoding/json"
)

func (t *Tx) CreateCase(ctx context.Context, c domain.FrequencyChangeCase, p domain.EmissionProfile) error {
	_, err := t.tx.ExecContext(ctx, `INSERT INTO cases VALUES(?,?,?,?,?,?,?,?,?)`, c.ID, c.StationCode, c.Title, c.Status, c.Revision, timeText(c.EffectiveFrom), timeText(c.EffectiveUntil), timeText(c.CreatedAt), timeText(c.UpdatedAt))
	if err != nil {
		return err
	}
	return t.InsertProfile(ctx, p)
}
func (t *Tx) InsertProfile(ctx context.Context, p domain.EmissionProfile) error {
	_, e := t.tx.ExecContext(ctx, `INSERT INTO profiles VALUES(?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.CaseID, p.BaselineNo, p.FrequencyHz, p.BandwidthHz, p.PowerWatts, p.AntennaGainDB, p.AzimuthDegrees, p.SiteLatitude, p.SiteLongitude, nullableTime(p.SealedAt))
	return e
}
func (t *Tx) UpdateProfile(ctx context.Context, p domain.EmissionProfile) error {
	_, e := t.tx.ExecContext(ctx, `UPDATE profiles SET frequency_hz=?,bandwidth_hz=?,power_watts=?,antenna_gain_db=?,azimuth_degrees=?,site_latitude=?,site_longitude=? WHERE id=? AND case_id=? AND sealed_at IS NULL`, p.FrequencyHz, p.BandwidthHz, p.PowerWatts, p.AntennaGainDB, p.AzimuthDegrees, p.SiteLatitude, p.SiteLongitude, p.ID, p.CaseID)
	return e
}
func (t *Tx) SealProfile(ctx context.Context, id string, at domainTime) error {
	_, e := t.tx.ExecContext(ctx, `UPDATE profiles SET sealed_at=? WHERE id=? AND sealed_at IS NULL`, timeText(at.Time), id)
	return e
}
func (t *Tx) UpdateCase(ctx context.Context, c domain.FrequencyChangeCase, expected int64) error {
	r, e := t.tx.ExecContext(ctx, `UPDATE cases SET station_code=?,title=?,status=?,revision=?,effective_from=?,effective_until=?,updated_at=? WHERE id=? AND revision=?`, c.StationCode, c.Title, c.Status, c.Revision, timeText(c.EffectiveFrom), timeText(c.EffectiveUntil), timeText(c.UpdatedAt), c.ID, expected)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return domain.NewError(domain.CodeConflict, "expectedRevision", "案件已被其他请求修改")
	}
	return nil
}
func (t *Tx) AddTarget(ctx context.Context, x domain.ProtectionTarget) error {
	_, e := t.tx.ExecContext(ctx, `INSERT INTO targets VALUES(?,?,?,?,?,?,?,?,?)`, x.ID, x.CaseID, x.Name, x.ServiceClass, x.FrequencyLowHz, x.FrequencyHighHz, x.MinimumSeparationHz, x.FieldStrengthLimitDBUVM, x.RuleReference)
	return e
}
func (t *Tx) UpdateTarget(ctx context.Context, x domain.ProtectionTarget) error {
	r, e := t.tx.ExecContext(ctx, `UPDATE targets SET name=?,service_class=?,frequency_low_hz=?,frequency_high_hz=?,minimum_separation_hz=?,field_strength_limit=?,rule_reference=? WHERE id=? AND case_id=?`, x.Name, x.ServiceClass, x.FrequencyLowHz, x.FrequencyHighHz, x.MinimumSeparationHz, x.FieldStrengthLimitDBUVM, x.RuleReference, x.ID, x.CaseID)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return domain.NewError(domain.CodeNotFound, "targetId", "保护对象不存在或不属于当前案件")
	}
	return nil
}
func (t *Tx) DeleteTarget(ctx context.Context, caseID, id string) error {
	_, e := t.tx.ExecContext(ctx, `DELETE FROM targets WHERE id=? AND case_id=?`, id, caseID)
	return e
}
func (t *Tx) AddCheck(ctx context.Context, x domain.InterferenceCheck) error {
	_, e := t.tx.ExecContext(ctx, `INSERT INTO checks VALUES(?,?,?,?,?,?,?,?,?,?,?)`, x.ID, x.CaseID, x.BaselineNo, x.TargetID, x.RuleCode, x.RuleVersion, x.InputDigest, x.MeasuredMargin, x.Result, x.PreviousCheckID, timeText(x.CheckedAt))
	return e
}
func (t *Tx) AddConflict(ctx context.Context, x domain.ConflictResolution) error {
	_, e := t.tx.ExecContext(ctx, `INSERT INTO conflicts VALUES(?,?,?,?,?,?,?,?,?,?,?)`, x.ID, x.CaseID, x.CheckID, x.Status, x.ResolutionText, x.EvidenceDigest, x.SubmittedBy, x.ReviewedBy, x.ReviewComment, nullableTime(x.ReviewedAt), timeText(x.UpdatedAt))
	return e
}
func (t *Tx) UpdateConflict(ctx context.Context, x domain.ConflictResolution) error {
	_, e := t.tx.ExecContext(ctx, `UPDATE conflicts SET check_id=?,status=?,resolution_text=?,evidence_digest=?,submitted_by=?,reviewed_by=?,review_comment=?,reviewed_at=?,updated_at=? WHERE id=? AND case_id=?`, x.CheckID, x.Status, x.ResolutionText, x.EvidenceDigest, x.SubmittedBy, x.ReviewedBy, x.ReviewComment, nullableTime(x.ReviewedAt), timeText(x.UpdatedAt), x.ID, x.CaseID)
	return e
}
func (t *Tx) AddPermit(ctx context.Context, x domain.ActivationPermit) error {
	_, e := t.tx.ExecContext(ctx, `INSERT INTO permits VALUES(?,?,?,?,?,?,?,?)`, x.ID, x.CaseID, x.SerialNumber, x.BaselineDigest, x.AuditDigest, x.ApprovedBy, timeText(x.IssuedAt), x.PermitDigest)
	return e
}
func (t *Tx) AddEvent(ctx context.Context, x domain.AuditEvent) error {
	_, e := t.tx.ExecContext(ctx, `INSERT INTO audit_events VALUES(?,?,?,?,?,?,?,?,?)`, x.ID, x.CaseID, x.Sequence, x.EventType, x.Actor, x.PayloadDigest, x.PreviousDigest, x.Digest, timeText(x.CreatedAt))
	return e
}
func (t *Tx) GetIdempotency(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	var x domain.IdempotencyRecord
	var created string
	e := t.tx.QueryRowContext(ctx, `SELECT key,operation,case_id,actor,request_digest,status_code,response_json,created_at FROM idempotency WHERE key=? ORDER BY created_at LIMIT 1`, key).Scan(&x.Key, &x.Operation, &x.CaseID, &x.Actor, &x.RequestDigest, &x.StatusCode, &x.ResponseJSON, &created)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	x.CreatedAt, _ = parseTime(created)
	return &x, nil
}
func (t *Tx) SaveIdempotency(ctx context.Context, x domain.IdempotencyRecord) error {
	if !json.Valid([]byte(x.ResponseJSON)) {
		return domain.NewError(domain.CodeInvalid, "response", "幂等响应不是合法 JSON")
	}
	_, e := t.tx.ExecContext(ctx, `INSERT INTO idempotency(key,operation,case_id,actor,request_digest,status_code,response_json,created_at) VALUES(?,?,?,?,?,?,?,?)`, x.Key, x.Operation, x.CaseID, x.Actor, x.RequestDigest, x.StatusCode, x.ResponseJSON, timeText(x.CreatedAt))
	return e
}
