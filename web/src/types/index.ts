export interface PodFailure {
  namespace: string;
  name: string;
  container: string;
  types: string[];
  message: string;
  events: string[];
}

export interface FixSuggestion {
  title: string;
  explanation: string;
  command: string;
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
  category?: string;
  severity?: "critical" | "high" | "medium" | "low";
  likelyCause: string;
  suggestedFix: string;
  confidence: "low" | "medium" | "high";
  confidenceNote?: string;
  evidence?: string[];
  fixSuggestions?: FixSuggestion[];
  quickCommands?: string[];
  context?: string[];
  events: string[];
  timestamp: string;
}

export interface CurrentFailure {
  issueKey: string;
  diagnosis: Diagnosis;
  severity: "critical" | "high" | "medium" | "low";
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
