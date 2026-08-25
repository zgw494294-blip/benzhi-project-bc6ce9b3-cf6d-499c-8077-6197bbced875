package storage

const schema = `
PRAGMA foreign_keys=ON;
CREATE TABLE IF NOT EXISTS schema_version(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
INSERT OR IGNORE INTO schema_version(version,applied_at) VALUES(1,CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS cases(id TEXT PRIMARY KEY, station_code TEXT NOT NULL, title TEXT NOT NULL, status TEXT NOT NULL, revision INTEGER NOT NULL, effective_from TEXT NOT NULL, effective_until TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS profiles(id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id), baseline_no INTEGER NOT NULL, frequency_hz INTEGER NOT NULL, bandwidth_hz INTEGER NOT NULL, power_watts REAL NOT NULL, antenna_gain_db REAL NOT NULL, azimuth_degrees REAL NOT NULL, site_latitude REAL NOT NULL, site_longitude REAL NOT NULL, sealed_at TEXT, UNIQUE(case_id,baseline_no));
CREATE TABLE IF NOT EXISTS targets(id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id), name TEXT NOT NULL, service_class TEXT NOT NULL, frequency_low_hz INTEGER NOT NULL, frequency_high_hz INTEGER NOT NULL, minimum_separation_hz INTEGER NOT NULL, field_strength_limit REAL NOT NULL, rule_reference TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS checks(id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id), baseline_no INTEGER NOT NULL, target_id TEXT NOT NULL REFERENCES targets(id), rule_code TEXT NOT NULL, rule_version TEXT NOT NULL, input_digest TEXT NOT NULL, measured_margin REAL NOT NULL, result TEXT NOT NULL, previous_check_id TEXT, checked_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS conflicts(id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id), check_id TEXT NOT NULL REFERENCES checks(id), status TEXT NOT NULL, resolution_text TEXT NOT NULL DEFAULT '', evidence_digest TEXT NOT NULL DEFAULT '', submitted_by TEXT NOT NULL DEFAULT '', reviewed_by TEXT NOT NULL DEFAULT '', review_comment TEXT NOT NULL DEFAULT '', reviewed_at TEXT, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS permits(id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES cases(id), serial_number TEXT NOT NULL UNIQUE, baseline_digest TEXT NOT NULL, audit_digest TEXT NOT NULL, approved_by TEXT NOT NULL, issued_at TEXT NOT NULL, permit_digest TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS audit_events(id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id), sequence INTEGER NOT NULL, event_type TEXT NOT NULL, actor TEXT NOT NULL, payload_digest TEXT NOT NULL, previous_digest TEXT NOT NULL, digest TEXT NOT NULL, created_at TEXT NOT NULL, UNIQUE(case_id,sequence), UNIQUE(case_id,digest));
CREATE TABLE IF NOT EXISTS idempotency(key TEXT NOT NULL PRIMARY KEY, operation TEXT NOT NULL, case_id TEXT NOT NULL, actor TEXT NOT NULL DEFAULT '', request_digest TEXT NOT NULL DEFAULT '', status_code INTEGER NOT NULL, response_json TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_profiles_case ON profiles(case_id,baseline_no);
CREATE INDEX IF NOT EXISTS idx_targets_case ON targets(case_id);
CREATE INDEX IF NOT EXISTS idx_checks_case ON checks(case_id,baseline_no);
CREATE INDEX IF NOT EXISTS idx_conflicts_case ON conflicts(case_id);
CREATE INDEX IF NOT EXISTS idx_events_case ON audit_events(case_id,sequence);
CREATE INDEX IF NOT EXISTS idx_cases_station_window ON cases(station_code,status,effective_from,effective_until);
CREATE TRIGGER IF NOT EXISTS permits_no_update BEFORE UPDATE ON permits BEGIN SELECT RAISE(ABORT,'activation permits are immutable'); END;
CREATE TRIGGER IF NOT EXISTS permits_no_delete BEFORE DELETE ON permits BEGIN SELECT RAISE(ABORT,'activation permits are immutable'); END;
CREATE TRIGGER IF NOT EXISTS sealed_profiles_no_update BEFORE UPDATE ON profiles WHEN OLD.sealed_at IS NOT NULL BEGIN SELECT RAISE(ABORT,'sealed profiles are immutable'); END;
CREATE TRIGGER IF NOT EXISTS sealed_profiles_no_delete BEFORE DELETE ON profiles WHEN OLD.sealed_at IS NOT NULL BEGIN SELECT RAISE(ABORT,'sealed profiles are immutable'); END;
`
