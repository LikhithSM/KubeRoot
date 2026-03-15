import { useState, useEffect } from 'react';
import { CurrentFailure, Diagnosis } from '../types/index';
import { fetchCurrentFailures, fetchDiagnoses } from '../services/api';

export function useDiagnoses(interval: number = 5000) {
  const [diagnoses, setDiagnoses] = useState<Diagnosis[]>([]);
  const [currentFailures, setCurrentFailures] = useState<CurrentFailure[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasLoaded, setHasLoaded] = useState(false);

  useEffect(() => {
    const loadDiagnoses = async () => {
      try {
        if (!hasLoaded) {
          setLoading(true);
        }
        setError(null);
        const [historyData, currentData] = await Promise.all([
          fetchDiagnoses(),
          fetchCurrentFailures(),
        ]);
        setDiagnoses(historyData);
        setCurrentFailures(currentData);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Unknown error');
      } finally {
        setLoading(false);
        setHasLoaded(true);
      }
    };

    loadDiagnoses();
    const intervalId = setInterval(loadDiagnoses, interval);
    return () => clearInterval(intervalId);
  }, [interval, hasLoaded]);

  return { diagnoses, currentFailures, loading, error };
}
