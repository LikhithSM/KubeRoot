import { Diagnosis } from '../types/index';

// Use direct backend endpoint (Railway) – proxy not needed because CORS and auth are handled server-side.
const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';
const API_KEY = import.meta.env.VITE_API_KEY || '';
const CLUSTER_ID = import.meta.env.VITE_CLUSTER_ID || 'demo-cluster';

export async function fetchDiagnoses(): Promise<Diagnosis[]> {
  // Queries history via the proxy (legacy cluster override applies)
  return fetchDiagnosesHistory({ cluster: CLUSTER_ID });
}

export async function fetchDiagnosesHistory(params?: {
  cluster?: string;
  limit?: number;
  failureType?: string;
  namespace?: string;
  since?: string;
  until?: string;
}): Promise<Diagnosis[]> {
  try {
    const queryParams = new URLSearchParams();
    if (params?.cluster) queryParams.append('cluster', params.cluster);
    if (params?.limit) queryParams.append('limit', String(params.limit));
    if (params?.failureType) queryParams.append('failureType', params.failureType);
    if (params?.namespace) queryParams.append('namespace', params.namespace);
    if (params?.since) queryParams.append('since', params.since);
    if (params?.until) queryParams.append('until', params.until);

    const query = queryParams.toString();
    // call backend directly (railway) using the `/diagnose/history` path
    const url = query
      ? `${API_BASE}/diagnose/history?${query}`
      : `${API_BASE}/diagnose/history`;

    const headers: HeadersInit = {
      'Content-Type': 'application/json',
    };
    if (API_KEY) {
      headers['X-API-Key'] = API_KEY;
    }
    
    const response = await fetch(url, { headers });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const data = await response.json();
    return data.items || [];
  } catch (error) {
    console.error('Failed to fetch history:', error);
    return [];
  }
}
