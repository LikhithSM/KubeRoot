import { CurrentFailure, Diagnosis } from '../types/index';

// Use the same-origin proxy in production so the browser does not need the API key.
const USE_PROXY = import.meta.env.PROD && import.meta.env.VITE_USE_PROXY !== 'false';
const API_BASE = USE_PROXY ? '' : import.meta.env.VITE_API_URL || 'http://localhost:8080';
const API_KEY = import.meta.env.VITE_API_KEY || '';
const CLUSTER_ID = import.meta.env.VITE_CLUSTER_ID || 'demo-cluster';

async function readErrorMessage(response: Response): Promise<string> {
  const contentType = response.headers.get('content-type') || '';

  try {
    if (contentType.includes('application/json')) {
      const body = await response.json();
      if (typeof body?.message === 'string' && body.message) {
        return body.message;
      }
      if (typeof body?.error === 'string' && body.error) {
        return body.error;
      }
    }

    const text = await response.text();
    if (text) {
      return text;
    }
  } catch {
    // Fall back to HTTP status below.
  }

  return `HTTP ${response.status}`;
}

export async function fetchDiagnoses(): Promise<Diagnosis[]> {
  // Queries history via the proxy (legacy cluster override applies)
  return fetchDiagnosesHistory({ cluster: CLUSTER_ID });
}

export async function fetchCurrentFailures(params?: {
  cluster?: string;
  limit?: number;
  failureType?: string;
  namespace?: string;
  since?: string;
  until?: string;
}): Promise<CurrentFailure[]> {
  const queryParams = new URLSearchParams();
  if (params?.cluster) queryParams.append('cluster', params.cluster);
  if (params?.limit) queryParams.append('limit', String(params.limit));
  if (params?.failureType) queryParams.append('failureType', params.failureType);
  if (params?.namespace) queryParams.append('namespace', params.namespace);
  if (params?.since) queryParams.append('since', params.since);
  if (params?.until) queryParams.append('until', params.until);

  const query = queryParams.toString();
  const path = USE_PROXY ? '/api/current-failures' : '/diagnose/current';
  const url = query ? `${API_BASE}${path}?${query}` : `${API_BASE}${path}`;

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
  };

  if (!USE_PROXY && API_KEY) {
    headers['X-API-Key'] = API_KEY;
  }

  const response = await fetch(url, { headers });
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  const data = await response.json();
  return data.items || [];
}

export async function fetchDiagnosesHistory(params?: {
  cluster?: string;
  limit?: number;
  failureType?: string;
  namespace?: string;
  since?: string;
  until?: string;
}): Promise<Diagnosis[]> {
  const queryParams = new URLSearchParams();
  if (params?.cluster) queryParams.append('cluster', params.cluster);
  if (params?.limit) queryParams.append('limit', String(params.limit));
  if (params?.failureType) queryParams.append('failureType', params.failureType);
  if (params?.namespace) queryParams.append('namespace', params.namespace);
  if (params?.since) queryParams.append('since', params.since);
  if (params?.until) queryParams.append('until', params.until);

  const query = queryParams.toString();
  const path = USE_PROXY ? '/api/history' : '/diagnose/history';
  const url = query ? `${API_BASE}${path}?${query}` : `${API_BASE}${path}`;

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
  };

  if (!USE_PROXY && API_KEY) {
    headers['X-API-Key'] = API_KEY;
  }

  const response = await fetch(url, { headers });
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  const data = await response.json();
  return data.items || [];
}
