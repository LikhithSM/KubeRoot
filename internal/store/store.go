package store

import (
	"context"
	"time"

	"kuberoot/internal/analyzer"
)

type DiagnosisHistoryFilter struct {
	Limit       int
	FailureType string
	Namespace   string
	Since       *time.Time
	Until       *time.Time
}

type DiagnosisStore interface {
	SaveDiagnoses(ctx context.Context, organizationID, clusterID string, diagnoses []analyzer.Diagnosis) error
	ListDiagnoses(ctx context.Context, organizationID, clusterID string, filter DiagnosisHistoryFilter) ([]analyzer.Diagnosis, error)
	ValidateAPIKey(ctx context.Context, keyHash string) (string, error)
}

type NoopStore struct{}

func NewNoopStore() DiagnosisStore {
	return NoopStore{}
}

func (NoopStore) SaveDiagnoses(_ context.Context, _ string, _ string, _ []analyzer.Diagnosis) error {
	return nil
}

func (NoopStore) ListDiagnoses(_ context.Context, _ string, _ string, _ DiagnosisHistoryFilter) ([]analyzer.Diagnosis, error) {
	return []analyzer.Diagnosis{}, nil
}

func (NoopStore) ValidateAPIKey(_ context.Context, _ string) (string, error) {
	return "local-org", nil
}
