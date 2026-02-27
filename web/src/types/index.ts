export interface PodFailure {
  namespace: string;
  name: string;
  container: string;
  types: string[];
  message: string;
  events: string[];
}

export interface Diagnosis {
  organizationId: string;
  clusterId: string;
  podName: string;
  namespace: string;
  failureType: string;
  likelyCause: string;
  suggestedFix: string;
  confidence: "low" | "medium" | "high";
  events: string[];
  timestamp: string;
}

export interface DiagnoseResponse {
  cluster: string;
  failures: Diagnosis[];
}
