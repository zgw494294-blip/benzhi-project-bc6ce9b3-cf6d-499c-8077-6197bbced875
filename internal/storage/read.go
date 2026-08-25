package storage

import (
	"benzhi-project-bc6ce9b3-cf6d-499c-8077-6197bbced875/internal/domain"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (t *Tx) FindWindowOccupancies(ctx context.Context, stationCode, excludeID string, from, until time.Time, limit int) ([]domain.WindowOccupancy, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := t.tx.QueryContext(ctx, `SELECT id,status,effective_from,effective_until FROM cases WHERE station_code=? AND id<>? AND status IN (?,?) AND effective_from<? AND effective_until>? ORDER BY effective_from,id LIMIT ?`, stationCode, excludeID, domain.StatusFrozen, domain.StatusApproved, timeText(until), timeText(from), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.WindowOccupancy
	for rows.Next() {
		var id string
		var status domain.CaseStatus
		var a, b string
		if err = rows.Scan(&id, &status, &a, &b); err != nil {
			return nil, err
		}
		af, _ := parseTime(a)
		bu, _ := parseTime(b)
		of, ou, ok := domain.WindowsOverlap(from, until, af, bu)
		if ok {
			out = append(out, domain.WindowOccupancy{CaseID: id, Status: status, OverlapFrom: of, OverlapUntil: ou})
		}
	}
	return out, rows.Err()
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) LoadCase(ctx context.Context, id string) (domain.CaseView, error) {
	return loadView(ctx, s.db, id)
}
func (s *Store) CheckOwner(ctx context.Context, id string) (string, bool, error) {
	var caseID string
	err := s.db.QueryRowContext(ctx, `SELECT case_id FROM checks WHERE id=?`, id).Scan(&caseID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return caseID, err == nil, err
}
func (t *Tx) LoadCase(ctx context.Context, id string) (domain.CaseView, error) {
	return loadView(ctx, t.tx, id)
}
func readCaseHeader(ctx context.Context, q queryer, id string) (domain.CaseView, error) {
	var v domain.CaseView
	var ef, eu, ca, ua string
	e := q.QueryRowContext(ctx, `SELECT id,station_code,title,status,revision,effective_from,effective_until,created_at,updated_at FROM cases WHERE id=?`, id).Scan(&v.Case.ID, &v.Case.StationCode, &v.Case.Title, &v.Case.Status, &v.Case.Revision, &ef, &eu, &ca, &ua)
	if errors.Is(e, sql.ErrNoRows) {
		return v, domain.NewError(domain.CodeNotFound, "caseId", "案件不存在")
	}
	if e != nil {
		return v, e
	}
	v.Case.EffectiveFrom, _ = parseTime(ef)
	v.Case.EffectiveUntil, _ = parseTime(eu)
	v.Case.CreatedAt, _ = parseTime(ca)
	v.Case.UpdatedAt, _ = parseTime(ua)
	return v, nil
}
func loadView(ctx context.Context, q queryer, id string) (domain.CaseView, error) {
	v, e := readCaseHeader(ctx, q, id)
	if e != nil {
		return v, e
	}
	if v.Profiles, e = readProfiles(ctx, q, id); e != nil {
		return v, e
	}
	if v.Targets, e = readTargets(ctx, q, id); e != nil {
		return v, e
	}
	if v.Checks, e = readChecks(ctx, q, id); e != nil {
		return v, e
	}
	if v.Conflicts, e = readConflicts(ctx, q, id); e != nil {
		return v, e
	}
	if v.Events, e = readEvents(ctx, q, id); e != nil {
		return v, e
	}
	var p domain.ActivationPermit
	var issued string
	e = q.QueryRowContext(ctx, `SELECT id,case_id,serial_number,baseline_digest,audit_digest,approved_by,issued_at,permit_digest FROM permits WHERE case_id=?`, id).Scan(&p.ID, &p.CaseID, &p.SerialNumber, &p.BaselineDigest, &p.AuditDigest, &p.ApprovedBy, &issued, &p.PermitDigest)
	if e == nil {
		p.IssuedAt, _ = parseTime(issued)
		v.Permit = &p
	} else if !errors.Is(e, sql.ErrNoRows) {
		return v, e
	}
	return v, nil
}

func (s *Store) LoadTraceCase(ctx context.Context, id string) (domain.CaseView, error) {
	v, err := readCaseHeader(ctx, s.db, id)
	if err != nil {
		return v, err
	}
	if v.Profiles, err = readProfiles(ctx, s.db, id); err != nil {
		return v, err
	}
	if v.Targets, err = readTargets(ctx, s.db, id); err != nil {
		return v, err
	}
	return v, nil
}

func (s *Store) ReadChecksPage(ctx context.Context, caseID string, baseline *int, targetID, ruleCode string, limit, offset int) ([]domain.InterferenceCheck, int, bool, error) {
	var baselineValue any
	if baseline != nil {
		baselineValue = *baseline
	}
	args := []any{caseID, baselineValue, baselineValue, targetID, targetID, ruleCode, ruleCode}
	where := ` FROM checks WHERE case_id=? AND (? IS NULL OR baseline_no=?) AND (?='' OR target_id=?) AND (?='' OR rule_code=?)`
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, false, err
	}
	query := `SELECT id,case_id,baseline_no,target_id,rule_code,rule_version,input_digest,measured_margin,result,previous_check_id,checked_at` + where + ` ORDER BY baseline_no,checked_at,id LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, append(args, limit+1, offset)...)
	if err != nil {
		return nil, 0, false, err
	}
	out, err := scanChecks(rows)
	if err != nil {
		return nil, 0, false, err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, total, hasMore, nil
}

func (s *Store) ReadChecksByID(ctx context.Context, ids []string) (map[string]domain.InterferenceCheck, error) {
	out := map[string]domain.InterferenceCheck{}
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > 100 {
		return nil, domain.NewError(domain.CodeInvalid, "checks", "前序检查查询超过上限")
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i := range ids {
		args[i] = ids[i]
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT id,case_id,baseline_no,target_id,rule_code,rule_version,input_digest,measured_margin,result,previous_check_id,checked_at FROM checks WHERE id IN (%s)`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	checks, err := scanChecks(rows)
	if err != nil {
		return nil, err
	}
	for _, check := range checks {
		out[check.ID] = check
	}
	return out, nil
}

func (s *Store) TraceConflictStatuses(ctx context.Context, caseID string, ids []string) (map[string]domain.ResolutionStatus, error) {
	out := map[string]domain.ResolutionStatus{}
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > 100 {
		return nil, domain.NewError(domain.CodeInvalid, "checks", "冲突轨迹查询超过上限")
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, caseID)
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`WITH RECURSIVE chain(status,check_id,path) AS (
SELECT status,check_id,','||check_id||',' FROM conflicts WHERE case_id=?
UNION ALL
SELECT chain.status,checks.previous_check_id,chain.path||checks.previous_check_id||',' FROM chain JOIN checks ON checks.id=chain.check_id WHERE checks.previous_check_id<>'' AND instr(chain.path,','||checks.previous_check_id||',')=0
) SELECT check_id,status FROM chain WHERE check_id IN (%s)`, placeholders)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var status domain.ResolutionStatus
		if err = rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		if _, exists := out[id]; !exists {
			out[id] = status
		}
	}
	return out, rows.Err()
}

func scanChecks(rows *sql.Rows) ([]domain.InterferenceCheck, error) {
	defer rows.Close()
	var out []domain.InterferenceCheck
	for rows.Next() {
		var x domain.InterferenceCheck
		var checked string
		if err := rows.Scan(&x.ID, &x.CaseID, &x.BaselineNo, &x.TargetID, &x.RuleCode, &x.RuleVersion, &x.InputDigest, &x.MeasuredMargin, &x.Result, &x.PreviousCheckID, &checked); err != nil {
			return nil, err
		}
		x.CheckedAt, _ = parseTime(checked)
		out = append(out, x)
	}
	return out, rows.Err()
}
func readProfiles(ctx context.Context, q queryer, id string) (out []domain.EmissionProfile, err error) {
	rows, e := q.QueryContext(ctx, `SELECT id,case_id,baseline_no,frequency_hz,bandwidth_hz,power_watts,antenna_gain_db,azimuth_degrees,site_latitude,site_longitude,sealed_at FROM profiles WHERE case_id=? ORDER BY baseline_no`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var x domain.EmissionProfile
		var st sql.NullString
		if e = rows.Scan(&x.ID, &x.CaseID, &x.BaselineNo, &x.FrequencyHz, &x.BandwidthHz, &x.PowerWatts, &x.AntennaGainDB, &x.AzimuthDegrees, &x.SiteLatitude, &x.SiteLongitude, &st); e != nil {
			return nil, e
		}
		x.SealedAt, _ = scanNullTime(st)
		out = append(out, x)
	}
	return out, rows.Err()
}
func readTargets(ctx context.Context, q queryer, id string) (out []domain.ProtectionTarget, err error) {
	rows, e := q.QueryContext(ctx, `SELECT id,case_id,name,service_class,frequency_low_hz,frequency_high_hz,minimum_separation_hz,field_strength_limit,rule_reference FROM targets WHERE case_id=? ORDER BY id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var x domain.ProtectionTarget
		if e = rows.Scan(&x.ID, &x.CaseID, &x.Name, &x.ServiceClass, &x.FrequencyLowHz, &x.FrequencyHighHz, &x.MinimumSeparationHz, &x.FieldStrengthLimitDBUVM, &x.RuleReference); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func readChecks(ctx context.Context, q queryer, id string) (out []domain.InterferenceCheck, err error) {
	rows, e := q.QueryContext(ctx, `SELECT id,case_id,baseline_no,target_id,rule_code,rule_version,input_digest,measured_margin,result,previous_check_id,checked_at FROM checks WHERE case_id=? ORDER BY checked_at,id`, id)
	if e != nil {
		return nil, e
	}
	return scanChecks(rows)
}
func readConflicts(ctx context.Context, q queryer, id string) (out []domain.ConflictResolution, err error) {
	rows, e := q.QueryContext(ctx, `SELECT id,case_id,check_id,status,resolution_text,evidence_digest,submitted_by,reviewed_by,review_comment,reviewed_at,updated_at FROM conflicts WHERE case_id=? ORDER BY updated_at,id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var x domain.ConflictResolution
		var reviewed sql.NullString
		var updated string
		if e = rows.Scan(&x.ID, &x.CaseID, &x.CheckID, &x.Status, &x.ResolutionText, &x.EvidenceDigest, &x.SubmittedBy, &x.ReviewedBy, &x.ReviewComment, &reviewed, &updated); e != nil {
			return nil, e
		}
		x.ReviewedAt, _ = scanNullTime(reviewed)
		x.UpdatedAt, _ = parseTime(updated)
		out = append(out, x)
	}
	return out, rows.Err()
}
func readEvents(ctx context.Context, q queryer, id string) (out []domain.AuditEvent, err error) {
	rows, e := q.QueryContext(ctx, `SELECT id,case_id,sequence,event_type,actor,payload_digest,previous_digest,digest,created_at FROM audit_events WHERE case_id=? ORDER BY sequence`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var x domain.AuditEvent
		var created string
		if e = rows.Scan(&x.ID, &x.CaseID, &x.Sequence, &x.EventType, &x.Actor, &x.PayloadDigest, &x.PreviousDigest, &x.Digest, &created); e != nil {
			return nil, e
		}
		x.CreatedAt, _ = parseTime(created)
		out = append(out, x)
	}
	return out, rows.Err()
}
