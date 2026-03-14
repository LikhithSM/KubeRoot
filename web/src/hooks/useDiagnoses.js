import { useState, useEffect } from 'react';
import { fetchDiagnoses } from '../services/api';
export function useDiagnoses(interval = 5000) {
    const [diagnoses, setDiagnoses] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);
    const [hasLoaded, setHasLoaded] = useState(false);
    useEffect(() => {
        const loadDiagnoses = async () => {
            try {
                if (!hasLoaded) {
                    setLoading(true);
                }
                setError(null);
                const data = await fetchDiagnoses();
                setDiagnoses(data);
            }
            catch (err) {
                setError(err instanceof Error ? err.message : 'Unknown error');
            }
            finally {
                setLoading(false);
                setHasLoaded(true);
            }
        };
        loadDiagnoses();
        const intervalId = setInterval(loadDiagnoses, interval);
        return () => clearInterval(intervalId);
    }, [interval, hasLoaded]);
    return { diagnoses, loading, error };
}
