package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

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
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_keys (
	id BIGSERIAL PRIMARY KEY,
	organization_id TEXT NOT NULL,
	key_hash TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash
	ON api_keys(key_hash) WHERE active = true;

CREATE TABLE IF NOT EXISTS diagnoses (
	id BIGSERIAL PRIMARY KEY,
	organization_id TEXT NOT NULL,
	cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
	pod_name TEXT NOT NULL,
	namespace TEXT NOT NULL,
	failure_type TEXT NOT NULL,
	likely_cause TEXT NOT NULL,
	suggested_fix TEXT NOT NULL,
	confidence TEXT NOT NULL,
	events JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

	query := fmt.Sprintf(`SELECT organization_id, cluster_id, pod_name, namespace, failure_type,
	        likely_cause, suggested_fix, confidence, events, created_at
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
		var eventsJSON []byte

		if scanErr := rows.Scan(
			&diagnosis.OrganizationID,
			&diagnosis.ClusterID,
			&diagnosis.PodName,
			&diagnosis.Namespace,
			&diagnosis.FailureType,
			&diagnosis.LikelyCause,
			&diagnosis.SuggestedFix,
			&diagnosis.Confidence,
			&eventsJSON,
			&diagnosis.Timestamp,
		); scanErr != nil {
			return nil, fmt.Errorf("scan diagnosis history row: %w", scanErr)
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

func (s *PostgresStore) SaveDiagnoses(ctx context.Context, organizationID, clusterID string, diagnoses []analyzer.Diagnosis) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO clusters (id, organization_id, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (id)
		 DO UPDATE SET organization_id = EXCLUDED.organization_id, updated_at = NOW()`,
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
			likely_cause, suggested_fix, confidence, events, created_at
		 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
	)
	if err != nil {
		return fmt.Errorf("prepare insert diagnosis: %w", err)
	}
	defer stmt.Close()

	for _, diagnosis := range diagnoses {
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
			diagnosis.LikelyCause,
			diagnosis.SuggestedFix,
			diagnosis.Confidence,
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
