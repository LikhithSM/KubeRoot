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
  container?: string;
  image?: string;
  restartCount?: number;
  failureType: string;
  likelyCause: string;
  suggestedFix: string;
  confidence: "low" | "medium" | "high";
  confidenceNote?: string;
  evidence?: string[];
  quickCommands?: string[];
  context?: string[];
  events: string[];
  timestamp: string;
}

export interface CurrentFailure {
  issueKey: string;
  diagnosis: Diagnosis;
  firstSeen: string;
  lastSeen: string;
  durationSeconds: number;
  occurrences: number;
  restartDelta: number;
  restartSpike: boolean;
  imageChanged: boolean;
  previousImage?: string;
  timeline: string[];
}

export interface DiagnoseResponse {
  cluster: string;
  failures: Diagnosis[];
}
