import { Diagnosis, DiagnoseResponse } from '../types/index';

const API_BASE = import.meta.env.VITE_API_BASE || '';

export async function fetchDiagnoses(): Promise<Diagnosis[]> {
  try {
    const response = await fetch(`${API_BASE}/diagnose`);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const data: DiagnoseResponse = await response.json();
    return data.failures || [];
  } catch (error) {
    console.error('Failed to fetch diagnoses:', error);
    return [];
  }
}

export async function fetchDiagnosesHistory(params?: {
  limit?: number;
  failureType?: string;
  namespace?: string;
  since?: string;
  until?: string;
}): Promise<Diagnosis[]> {
  try {
    const queryParams = new URLSearchParams();
    if (params?.limit) queryParams.append('limit', String(params.limit));
    if (params?.failureType) queryParams.append('failureType', params.failureType);
    if (params?.namespace) queryParams.append('namespace', params.namespace);
    if (params?.since) queryParams.append('since', params.since);
    if (params?.until) queryParams.append('until', params.until);

    const query = queryParams.toString();
    const url = query ? `${API_BASE}/diagnose/history?${query}` : `${API_BASE}/diagnose/history`;
    
    const response = await fetch(url);
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const data = await response.json();
    return data.items || [];
  } catch (error) {
    console.error('Failed to fetch history:', error);
    return [];
  }
}
