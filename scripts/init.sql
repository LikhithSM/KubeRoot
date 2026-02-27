-- Initialize Kuberoot schema
CREATE TABLE IF NOT EXISTS diagnoses (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id VARCHAR(256) NOT NULL,
  cluster_id VARCHAR(256) NOT NULL,
  pod_name VARCHAR(256) NOT NULL,
  namespace VARCHAR(256) NOT NULL,
  failure_type VARCHAR(128) NOT NULL,
  likely_cause TEXT,
  suggested_fix TEXT,
  confidence VARCHAR(32),
  events TEXT[],
  timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  indexed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_diagnoses_org_cluster ON diagnoses(organization_id, cluster_id);
CREATE INDEX IF NOT EXISTS idx_diagnoses_timestamp ON diagnoses(timestamp DESC);
