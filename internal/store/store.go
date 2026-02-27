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
	RegisterCluster(ctx context.Context, organizationID, clusterID string) error
}
