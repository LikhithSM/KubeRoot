package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"kuberoot/internal/analyzer"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	store := &PostgresStore{db: db}
	if err := store.ensureSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *PostgresStore) ensureSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS clusters (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL,
	first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE clusters ADD COLUMN IF NOT EXISTS first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE clusters ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE clusters ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE clusters ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE clusters ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE TABLE IF NOT EXISTS api_keys (
	id BIGSERIAL PRIMARY KEY,
	organization_id TEXT NOT NULL,
	key_hash TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	last_used_at TIMESTAMPTZ
);

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT 'legacy-key';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash
	ON api_keys(key_hash) WHERE active = true;

CREATE TABLE IF NOT EXISTS diagnoses (
	id BIGSERIAL PRIMARY KEY,
	organization_id TEXT NOT NULL,
	cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
	pod_name TEXT NOT NULL,
	namespace TEXT NOT NULL,
	container TEXT NOT NULL DEFAULT '',
	image TEXT NOT NULL DEFAULT '',
	restart_count INTEGER NOT NULL DEFAULT 0,
	failure_type TEXT NOT NULL,
	likely_cause TEXT NOT NULL,
	suggested_fix TEXT NOT NULL,
	confidence TEXT NOT NULL,
	confidence_note TEXT NOT NULL DEFAULT '',
	evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
	quick_commands JSONB NOT NULL DEFAULT '[]'::jsonb,
	diag_context JSONB NOT NULL DEFAULT '[]'::jsonb,
	events JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE diagnoses ADD COLUMN IF NOT EXISTS confidence_note TEXT NOT NULL DEFAULT '';
ALTER TABLE diagnoses ADD COLUMN IF NOT EXISTS evidence JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE diagnoses ADD COLUMN IF NOT EXISTS quick_commands JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE diagnoses ADD COLUMN IF NOT EXISTS diag_context JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE diagnoses ADD COLUMN IF NOT EXISTS container TEXT NOT NULL DEFAULT '';
ALTER TABLE diagnoses ADD COLUMN IF NOT EXISTS image TEXT NOT NULL DEFAULT '';
ALTER TABLE diagnoses ADD COLUMN IF NOT EXISTS restart_count INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_diagnoses_cluster_created_at
	ON diagnoses(cluster_id, created_at DESC);
`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

func (s *PostgresStore) ValidateAPIKey(ctx context.Context, keyHash string) (string, error) {
	var organizationID string
	err := s.db.QueryRowContext(
		ctx,
		`UPDATE api_keys
		 SET last_used_at = NOW()
		 WHERE key_hash = $1 AND active = true
		 RETURNING organization_id`,
		keyHash,
	).Scan(&organizationID)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("invalid or inactive API key")
	}
	if err != nil {
		return "", fmt.Errorf("validate API key: %w", err)
	}

	return organizationID, nil
}

func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func generateAPIKey() (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate random api key bytes: %w", err)
	}

	return "kr_live_" + hex.EncodeToString(randomBytes), nil
}

func (s *PostgresStore) CreateAPIKey(ctx context.Context, organizationID, name string) (string, error) {
	orgID := strings.TrimSpace(organizationID)
	if orgID == "" {
		return "", fmt.Errorf("organization_id is required")
	}

	keyName := strings.TrimSpace(name)
	if keyName == "" {
		keyName = fmt.Sprintf("generated-%d", time.Now().Unix())
	}

	for attempts := 0; attempts < 3; attempts++ {
		rawKey, err := generateAPIKey()
		if err != nil {
			return "", err
		}

		keyHash := hashAPIKey(rawKey)
		_, err = s.db.ExecContext(
			ctx,
			`INSERT INTO api_keys (organization_id, key_hash, name, active)
			 VALUES ($1, $2, $3, true)`,
			orgID,
			keyHash,
			keyName,
		)
		if err == nil {
			return rawKey, nil
		}

		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			continue
		}

		return "", fmt.Errorf("create api key: %w", err)
	}

	return "", fmt.Errorf("create api key: failed after retries")
}

func (s *PostgresStore) ListDiagnoses(ctx context.Context, organizationID, clusterID string, filter DiagnosisHistoryFilter) ([]analyzer.Diagnosis, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	var whereClauses []string
	args := make([]any, 0, 8)

	args = append(args, organizationID)
	whereClauses = append(whereClauses, fmt.Sprintf("organization_id = $%d", len(args)))

	args = append(args, clusterID)
	whereClauses = append(whereClauses, fmt.Sprintf("cluster_id = $%d", len(args)))

	if filter.FailureType != "" {
		args = append(args, filter.FailureType)
		whereClauses = append(whereClauses, fmt.Sprintf("failure_type = $%d", len(args)))
	}

	if filter.Namespace != "" {
		args = append(args, filter.Namespace)
		whereClauses = append(whereClauses, fmt.Sprintf("namespace = $%d", len(args)))
	}

	if filter.Since != nil {
		args = append(args, *filter.Since)
		whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", len(args)))
	}

	if filter.Until != nil {
		args = append(args, *filter.Until)
		whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", len(args)))
	}

	args = append(args, limit)
	limitArgPosition := len(args)

	query := fmt.Sprintf(`SELECT organization_id, cluster_id, pod_name, namespace, container, image, restart_count, failure_type,
	        likely_cause, suggested_fix, confidence, confidence_note, evidence, quick_commands, diag_context, events, created_at
	 FROM diagnoses
	 WHERE %s
	 ORDER BY created_at DESC
	 LIMIT $%d`, strings.Join(whereClauses, " AND "), limitArgPosition)

	rows, err := s.db.QueryContext(
		ctx,
		query,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("query diagnoses history: %w", err)
	}
	defer rows.Close()

	out := make([]analyzer.Diagnosis, 0, limit)
	for rows.Next() {
		var diagnosis analyzer.Diagnosis
		var evidenceJSON []byte
		var quickCommandsJSON []byte
		var contextJSON []byte
		var eventsJSON []byte

		if scanErr := rows.Scan(
			&diagnosis.OrganizationID,
			&diagnosis.ClusterID,
			&diagnosis.PodName,
			&diagnosis.Namespace,
			&diagnosis.Container,
			&diagnosis.Image,
			&diagnosis.RestartCount,
			&diagnosis.FailureType,
			&diagnosis.LikelyCause,
			&diagnosis.SuggestedFix,
			&diagnosis.Confidence,
			&diagnosis.ConfidenceNote,
			&evidenceJSON,
			&quickCommandsJSON,
			&contextJSON,
			&eventsJSON,
			&diagnosis.Timestamp,
		); scanErr != nil {
			return nil, fmt.Errorf("scan diagnosis history row: %w", scanErr)
		}

		if len(evidenceJSON) > 0 {
			if unmarshalErr := json.Unmarshal(evidenceJSON, &diagnosis.Evidence); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshal diagnosis evidence: %w", unmarshalErr)
			}
		}

		if len(quickCommandsJSON) > 0 {
			if unmarshalErr := json.Unmarshal(quickCommandsJSON, &diagnosis.QuickCommands); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshal diagnosis quick commands: %w", unmarshalErr)
			}
		}

		if len(contextJSON) > 0 {
			if unmarshalErr := json.Unmarshal(contextJSON, &diagnosis.Context); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshal diagnosis context: %w", unmarshalErr)
			}
		}

		if len(eventsJSON) > 0 {
			if unmarshalErr := json.Unmarshal(eventsJSON, &diagnosis.Events); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshal diagnosis events: %w", unmarshalErr)
			}
		}

		out = append(out, diagnosis)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate diagnosis history rows: %w", err)
	}

	return out, nil
}

func (s *PostgresStore) ListCurrentFailures(ctx context.Context, organizationID, clusterID string, filter DiagnosisHistoryFilter) ([]CurrentFailure, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	whereClauses := []string{"organization_id = $1", "cluster_id = $2"}
	args := []any{organizationID, clusterID}

	if filter.FailureType != "" {
		args = append(args, filter.FailureType)
		whereClauses = append(whereClauses, fmt.Sprintf("failure_type = $%d", len(args)))
	}

	if filter.Namespace != "" {
		args = append(args, filter.Namespace)
		whereClauses = append(whereClauses, fmt.Sprintf("namespace = $%d", len(args)))
	}

	if filter.Since != nil {
		args = append(args, *filter.Since)
		whereClauses = append(whereClauses, fmt.Sprintf("created_at >= $%d", len(args)))
	}

	if filter.Until != nil {
		args = append(args, *filter.Until)
		whereClauses = append(whereClauses, fmt.Sprintf("created_at <= $%d", len(args)))
	}

	args = append(args, limit)
	limitArgPosition := len(args)

	query := fmt.Sprintf(`WITH filtered AS (
		SELECT
			organization_id,
			cluster_id,
			pod_name,
			namespace,
			container,
			image,
			restart_count,
			failure_type,
			likely_cause,
			suggested_fix,
			confidence,
			confidence_note,
			evidence,
			quick_commands,
			diag_context,
			events,
			created_at,
			namespace || '/' || pod_name || '/' || failure_type AS issue_key
		FROM diagnoses
		WHERE %s
	), latest AS (
		SELECT DISTINCT ON (issue_key)
			issue_key,
			organization_id,
			cluster_id,
			pod_name,
			namespace,
			container,
			image,
			restart_count,
			failure_type,
			likely_cause,
			suggested_fix,
			confidence,
			confidence_note,
			evidence,
			quick_commands,
			diag_context,
			events,
			created_at
		FROM filtered
		ORDER BY issue_key, created_at DESC
	), agg AS (
		SELECT
			issue_key,
			MIN(created_at) AS first_seen,
			MAX(created_at) AS last_seen,
			COUNT(*) AS occurrences,
			MIN(restart_count) AS min_restart,
			MAX(restart_count) AS max_restart
		FROM filtered
		GROUP BY issue_key
	)
	SELECT
		latest.issue_key,
		latest.organization_id,
		latest.cluster_id,
		latest.pod_name,
		latest.namespace,
		latest.container,
		latest.image,
		latest.restart_count,
		latest.failure_type,
		latest.likely_cause,
		latest.suggested_fix,
		latest.confidence,
		latest.confidence_note,
		latest.evidence,
		latest.quick_commands,
		latest.diag_context,
		latest.events,
		agg.first_seen,
		agg.last_seen,
		agg.occurrences,
		agg.min_restart,
		agg.max_restart,
		(
			SELECT f2.image
			FROM filtered f2
			WHERE f2.issue_key = latest.issue_key
			  AND f2.image <> latest.image
			ORDER BY f2.created_at DESC
			LIMIT 1
		) AS previous_image
	FROM latest
	JOIN agg ON latest.issue_key = agg.issue_key
	ORDER BY agg.last_seen DESC
	LIMIT $%d`, strings.Join(whereClauses, " AND "), limitArgPosition)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query current failures: %w", err)
	}
	defer rows.Close()

	out := make([]CurrentFailure, 0, limit)
	now := time.Now().UTC()
	for rows.Next() {
		var failure CurrentFailure
		var evidenceJSON []byte
		var quickCommandsJSON []byte
		var contextJSON []byte
		var eventsJSON []byte
		var minRestart int32
		var maxRestart int32
		var previousImage sql.NullString

		if scanErr := rows.Scan(
			&failure.IssueKey,
			&failure.Diagnosis.OrganizationID,
			&failure.Diagnosis.ClusterID,
			&failure.Diagnosis.PodName,
			&failure.Diagnosis.Namespace,
			&failure.Diagnosis.Container,
			&failure.Diagnosis.Image,
			&failure.Diagnosis.RestartCount,
			&failure.Diagnosis.FailureType,
			&failure.Diagnosis.LikelyCause,
			&failure.Diagnosis.SuggestedFix,
			&failure.Diagnosis.Confidence,
			&failure.Diagnosis.ConfidenceNote,
			&evidenceJSON,
			&quickCommandsJSON,
			&contextJSON,
			&eventsJSON,
			&failure.FirstSeen,
			&failure.LastSeen,
			&failure.Occurrences,
			&minRestart,
			&maxRestart,
			&previousImage,
		); scanErr != nil {
			return nil, fmt.Errorf("scan current failure row: %w", scanErr)
		}

		if len(evidenceJSON) > 0 {
			if unmarshalErr := json.Unmarshal(evidenceJSON, &failure.Diagnosis.Evidence); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshal current failure evidence: %w", unmarshalErr)
			}
		}

		if len(quickCommandsJSON) > 0 {
			if unmarshalErr := json.Unmarshal(quickCommandsJSON, &failure.Diagnosis.QuickCommands); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshal current failure quick commands: %w", unmarshalErr)
			}
		}

		if len(contextJSON) > 0 {
			if unmarshalErr := json.Unmarshal(contextJSON, &failure.Diagnosis.Context); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshal current failure context: %w", unmarshalErr)
			}
		}

		if len(eventsJSON) > 0 {
			if unmarshalErr := json.Unmarshal(eventsJSON, &failure.Diagnosis.Events); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshal current failure events: %w", unmarshalErr)
			}
		}

		failure.Diagnosis.Timestamp = failure.LastSeen
		failure.DurationSeconds = int64(now.Sub(failure.FirstSeen).Seconds())
		if failure.DurationSeconds < 0 {
			failure.DurationSeconds = 0
		}
		failure.RestartDelta = maxRestart - minRestart
		failure.RestartSpike = failure.RestartDelta >= 10 && failure.DurationSeconds <= 15*60
		if previousImage.Valid && strings.TrimSpace(previousImage.String) != "" {
			failure.ImageChanged = true
			failure.PreviousImage = previousImage.String
		}
		failure.Timeline = buildFailureTimeline(failure)
		failure.Severity = computeCurrentFailureSeverity(failure)

		out = append(out, failure)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate current failure rows: %w", err)
	}

	return out, nil
}

func buildFailureTimeline(f CurrentFailure) []string {
	lines := []string{
		fmt.Sprintf("%s first seen", f.FirstSeen.UTC().Format("15:04")),
		fmt.Sprintf("%s latest observation", f.LastSeen.UTC().Format("15:04")),
	}

	if f.DurationSeconds <= 10*60 {
		lines = append(lines, "Recent rollout window detected")
	}
	if f.RestartSpike {
		lines = append(lines, fmt.Sprintf("Restart spike detected (%d increase)", f.RestartDelta))
	}
	if f.ImageChanged {
		lines = append(lines, fmt.Sprintf("Image changed: %s -> %s", f.PreviousImage, f.Diagnosis.Image))
	}

	return lines
}

// computeCurrentFailureSeverity escalates severity using aggregate signals
// (restartSpike, duration, imageChanged) on top of the base diagnosis severity.
func computeCurrentFailureSeverity(f CurrentFailure) string {
	base := computeDiagnosisSeverity(f.Diagnosis.Confidence, f.Diagnosis.FailureType, f.Diagnosis.RestartCount)
	score := 0
	switch base {
	case "critical":
		score = 4
	case "high":
		score = 3
	case "medium":
		score = 2
	default:
		score = 1
	}
	if f.RestartSpike {
		score += 2
	}
	if f.DurationSeconds > 30*60 { // > 30 min running
		score++
	}
	if f.ImageChanged {
		score++
	}
	switch {
	case score >= 6:
		return "critical"
	case score >= 4:
		return "high"
	case score >= 3:
		return "medium"
	default:
		return "low"
	}
}

// computeDiagnosisSeverity derives a base severity from confidence + failure type + restart count.
func computeDiagnosisSeverity(confidence, failureType string, restartCount int32) string {
	score := 0
	switch confidence {
	case "high":
		score += 3
	case "medium":
		score += 2
	default:
		score += 1
	}
	switch failureType {
	case "OOMKilled", "CrashLoopBackOff", "ConfigMapMissing", "SecretMissing":
		score++
	}
	if restartCount >= 10 {
		score += 2
	} else if restartCount >= 3 {
		score++
	}
	switch {
	case score >= 6:
		return "critical"
	case score >= 4:
		return "high"
	case score >= 3:
		return "medium"
	default:
		return "low"
	}
}

func (s *PostgresStore) SaveDiagnoses(ctx context.Context, organizationID, clusterID string, diagnoses []analyzer.Diagnosis) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Upsert cluster: set last_seen_at, maintain first_seen_at
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO clusters (id, organization_id, first_seen_at, last_seen_at, active)
		 VALUES ($1, $2, NOW(), NOW(), true)
		 ON CONFLICT (id)
		 DO UPDATE SET last_seen_at = NOW(), active = true`,
		clusterID,
		organizationID,
	); err != nil {
		return fmt.Errorf("upsert cluster: %w", err)
	}

	if len(diagnoses) == 0 {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit tx: %w", err)
		}
		return nil
	}

	stmt, err := tx.PrepareContext(
		ctx,
		`INSERT INTO diagnoses (
			organization_id, cluster_id, pod_name, namespace, failure_type,
			container, image, restart_count,
			likely_cause, suggested_fix, confidence, confidence_note,
			evidence, quick_commands, diag_context, events, created_at
		 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
	)
	if err != nil {
		return fmt.Errorf("prepare insert diagnosis: %w", err)
	}
	defer stmt.Close()

	for _, diagnosis := range diagnoses {
		evidenceJSON, marshalEvidenceErr := json.Marshal(diagnosis.Evidence)
		if marshalEvidenceErr != nil {
			return fmt.Errorf("marshal diagnosis evidence: %w", marshalEvidenceErr)
		}

		quickCommandsJSON, marshalQuickErr := json.Marshal(diagnosis.QuickCommands)
		if marshalQuickErr != nil {
			return fmt.Errorf("marshal diagnosis quick commands: %w", marshalQuickErr)
		}

		contextJSON, marshalContextErr := json.Marshal(diagnosis.Context)
		if marshalContextErr != nil {
			return fmt.Errorf("marshal diagnosis context: %w", marshalContextErr)
		}

		eventsJSON, marshalErr := json.Marshal(diagnosis.Events)
		if marshalErr != nil {
			return fmt.Errorf("marshal diagnosis events: %w", marshalErr)
		}

		if _, execErr := stmt.ExecContext(
			ctx,
			organizationID,
			clusterID,
			diagnosis.PodName,
			diagnosis.Namespace,
			diagnosis.FailureType,
			diagnosis.Container,
			diagnosis.Image,
			diagnosis.RestartCount,
			diagnosis.LikelyCause,
			diagnosis.SuggestedFix,
			diagnosis.Confidence,
			diagnosis.ConfidenceNote,
			evidenceJSON,
			quickCommandsJSON,
			contextJSON,
			eventsJSON,
			diagnosis.Timestamp,
		); execErr != nil {
			return fmt.Errorf("insert diagnosis: %w", execErr)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (s *PostgresStore) RegisterCluster(ctx context.Context, organizationID, clusterID string) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO clusters (id, organization_id, first_seen_at, last_seen_at, active)
		 VALUES ($1, $2, NOW(), NOW(), true)
		 ON CONFLICT (id)
		 DO UPDATE SET last_seen_at = NOW(), active = true`,
		clusterID,
		organizationID,
	)
	if err != nil {
		return fmt.Errorf("register cluster: %w", err)
	}
	return nil
}

func (s *PostgresStore) FailureSeenRecently(ctx context.Context, organizationID, clusterID, namespace, podName, failureType string, window time.Duration) (bool, error) {
	if window <= 0 {
		window = 10 * time.Minute
	}

	var exists bool
	err := s.db.QueryRowContext(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM diagnoses
			WHERE organization_id = $1
			  AND cluster_id = $2
			  AND namespace = $3
			  AND pod_name = $4
			  AND failure_type = $5
			  AND created_at >= NOW() - ($6 * INTERVAL '1 second')
		)`,
		organizationID,
		clusterID,
		namespace,
		podName,
		failureType,
		int64(window.Seconds()),
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failure seen recently query: %w", err)
	}

	return exists, nil
}
