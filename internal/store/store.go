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

type CurrentFailure struct {
	IssueKey        string             `json:"issueKey"`
	Diagnosis       analyzer.Diagnosis `json:"diagnosis"`
	FirstSeen       time.Time          `json:"firstSeen"`
	LastSeen        time.Time          `json:"lastSeen"`
	DurationSeconds int64              `json:"durationSeconds"`
	Occurrences     int                `json:"occurrences"`
	RestartDelta    int32              `json:"restartDelta"`
	RestartSpike    bool               `json:"restartSpike"`
	ImageChanged    bool               `json:"imageChanged"`
	PreviousImage   string             `json:"previousImage"`
	Timeline        []string           `json:"timeline"`
}

type DiagnosisStore interface {
	SaveDiagnoses(ctx context.Context, organizationID, clusterID string, diagnoses []analyzer.Diagnosis) error
	ListDiagnoses(ctx context.Context, organizationID, clusterID string, filter DiagnosisHistoryFilter) ([]analyzer.Diagnosis, error)
	ListCurrentFailures(ctx context.Context, organizationID, clusterID string, filter DiagnosisHistoryFilter) ([]CurrentFailure, error)
	FailureSeenRecently(ctx context.Context, organizationID, clusterID, namespace, podName, failureType string, window time.Duration) (bool, error)
	ValidateAPIKey(ctx context.Context, keyHash string) (string, error)
	CreateAPIKey(ctx context.Context, organizationID, name string) (string, error)
	RegisterCluster(ctx context.Context, organizationID, clusterID string) error
}
