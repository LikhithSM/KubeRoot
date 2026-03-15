import { useState, useMemo } from 'react';
import { useDiagnoses } from '../hooks/useDiagnoses';
import { FailureTable } from './FailureTable';
import { ActiveIssuesTable } from './ActiveIssuesTable';
import { DetailView } from './DetailView';
import { FilterPanel } from './FilterPanel';
import { CurrentFailure, Diagnosis } from '../types/index';
import { format } from 'date-fns';

export function Dashboard() {
  const { diagnoses, currentFailures, loading, error } = useDiagnoses(5000);
  const [selectedDiagnosis, setSelectedDiagnosis] = useState<Diagnosis | null>(null);
  const [selectedCurrentIssue, setSelectedCurrentIssue] = useState<CurrentFailure | null>(null);
  const [activeTab, setActiveTab] = useState<'active' | 'history'>('active');
  const [filters, setFilters] = useState<{ namespace?: string; type?: string; confidence?: string }>({});

  // Deduplicate: keep only the latest entry per (namespace, podName, failureType).
  // The API returns records newest-first, so the first occurrence of each key is the most recent.
  const activeDiagnoses = useMemo(() => {
    const seen = new Set<string>();
    return diagnoses.filter((d) => {
      const key = `${d.namespace}/${d.podName}/${d.failureType}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }, [diagnoses]);

  const currentDiagnoses = useMemo(() => currentFailures.map((i) => i.diagnosis), [currentFailures]);

  const filteredCurrentFailures = useMemo(() => {
    return currentFailures.filter((issue) => {
      const d = issue.diagnosis;
      if (filters.namespace && d.namespace !== filters.namespace) return false;
      if (filters.type && d.failureType !== filters.type) return false;
      if (filters.confidence && d.confidence !== filters.confidence) return false;
      return true;
    });
  }, [currentFailures, filters]);

  const uniqueNamespaces = useMemo(() => {
    const source = activeTab === 'active' ? currentDiagnoses : activeDiagnoses;
    const ns = new Set(source.map((d) => d.namespace));
    return Array.from(ns).sort();
  }, [activeDiagnoses, currentDiagnoses, activeTab]);

  const uniqueTypes = useMemo(() => {
    const source = activeTab === 'active' ? currentDiagnoses : activeDiagnoses;
    const types = new Set(source.map((d) => d.failureType));
    return Array.from(types).sort();
  }, [activeDiagnoses, currentDiagnoses, activeTab]);

  const stats = useMemo(() => {
    const source = activeTab === 'active' ? currentDiagnoses : activeDiagnoses;
    return {
      total: source.length,
      high: source.filter((d) => d.confidence === 'high').length,
      medium: source.filter((d) => d.confidence === 'medium').length,
      low: source.filter((d) => d.confidence === 'low').length,
    };
  }, [activeDiagnoses, currentDiagnoses, activeTab]);

  const lastUpdated = useMemo(() => format(new Date(), 'HH:mm:ss'), [diagnoses]);
  const activeCluster = currentFailures[0]?.diagnosis.clusterId || diagnoses[0]?.clusterId || 'unassigned';

  return (
    <div className="shell">
      <div className="content-layer">
        {/* Header */}
        <header className="border-b border-white/70 bg-white/70 backdrop-blur">
          <div className="max-w-7xl mx-auto px-4 py-10">
            <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-6">
              <div className="space-y-3">
                <span className="pill-live">Live</span>
                <h1 className="text-4xl md:text-5xl font-display text-gray-900">Kuberoot</h1>
                <p className="text-gray-600 max-w-xl">
                  Kubernetes incident intelligence that turns pod failures into clear, actionable fixes.
                </p>
              </div>
              <div className="surface-card rounded-2xl px-6 py-4 min-w-[240px]">
                <p className="text-xs uppercase tracking-wider text-gray-500">Active Cluster</p>
                <p className="text-lg font-semibold text-gray-900 mt-1">{activeCluster}</p>
                <p className="text-sm text-gray-500 mt-2">Auto-refresh every 5s</p>
              </div>
            </div>
          </div>
        </header>

        {/* Main Content */}
        <main className="max-w-7xl mx-auto px-4 py-10">
          {/* Stats Cards */}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
            <div className="surface-card rounded-2xl p-6 stat-accent animate-rise" style={{ animationDelay: '0ms' }}>
              <p className="text-gray-600 text-xs font-semibold uppercase tracking-wider">Total Failures</p>
              <p className="text-3xl font-bold text-gray-900 mt-2">{stats.total}</p>
              <p className="text-sm text-gray-500 mt-1">Across all namespaces</p>
            </div>

            <div className="surface-card rounded-2xl p-6 animate-rise" style={{ animationDelay: '60ms' }}>
              <p className="text-gray-600 text-xs font-semibold uppercase tracking-wider">Critical</p>
              <p className="text-3xl font-bold text-red-600 mt-2">{stats.high}</p>
              <p className="text-sm text-gray-500 mt-1">High confidence fixes</p>
            </div>

            <div className="surface-card rounded-2xl p-6 animate-rise" style={{ animationDelay: '120ms' }}>
              <p className="text-gray-600 text-xs font-semibold uppercase tracking-wider">Watchlist</p>
              <p className="text-3xl font-bold text-orange-600 mt-2">{stats.medium}</p>
              <p className="text-sm text-gray-500 mt-1">Needs review</p>
            </div>

            <div className="surface-card rounded-2xl p-6 animate-rise" style={{ animationDelay: '180ms' }}>
              <p className="text-gray-600 text-xs font-semibold uppercase tracking-wider">Low Signal</p>
              <p className="text-3xl font-bold text-yellow-700 mt-2">{stats.low}</p>
              <p className="text-sm text-gray-500 mt-1">Monitor only</p>
            </div>
          </div>

          {/* Error Alert */}
          {error && (
            <div className="surface-muted rounded-2xl p-4 mb-6 border border-red-200 bg-red-50">
              <p className="text-red-800 font-semibold">Error loading data</p>
              <p className="text-red-700 text-sm">{error}</p>
            </div>
          )}

          {/* Loading State */}
          {loading && diagnoses.length === 0 && (
            <div className="text-center py-12">
              <div className="spinner" />
              <p className="text-gray-600 mt-4">Loading cluster data...</p>
            </div>
          )}

          {/* Main Content */}
          {!loading || diagnoses.length > 0 ? (
            <>
              {/* Filters */}
              <FilterPanel
                namespaces={uniqueNamespaces}
                types={uniqueTypes}
                onFilter={setFilters}
              />

              {/* Data Table */}
              <div className="surface-card rounded-2xl overflow-hidden">
                <div className="px-6 py-4 bg-white border-b border-gray-200 flex flex-col md:flex-row md:items-center md:justify-between gap-2">
                  <div>
                    <h2 className="text-lg font-semibold text-gray-900">Incident Intelligence</h2>
                    <p className="text-sm text-gray-500">
                      {activeTab === 'active' ? 'Current failing pods with duration and restart patterns' : 'Historical diagnosis events'}
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="inline-flex bg-gray-100 rounded-lg p-1">
                      <button
                        className={`px-3 py-1.5 text-xs font-semibold rounded ${activeTab === 'active' ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-600'}`}
                        onClick={() => setActiveTab('active')}
                      >
                        Active Issues
                      </button>
                      <button
                        className={`px-3 py-1.5 text-xs font-semibold rounded ${activeTab === 'history' ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-600'}`}
                        onClick={() => setActiveTab('history')}
                      >
                        History
                      </button>
                    </div>
                    <span className="text-xs text-gray-500 uppercase tracking-wider">
                      {stats.total} items - Updated {lastUpdated}
                    </span>
                  </div>
                </div>
                {activeTab === 'active' ? (
                  <ActiveIssuesTable
                    items={filteredCurrentFailures}
                    onSelectRow={(issue) => {
                      setSelectedCurrentIssue(issue);
                      setSelectedDiagnosis(issue.diagnosis);
                    }}
                  />
                ) : (
                  <FailureTable
                    diagnoses={activeDiagnoses}
                    filters={filters}
                    onSelectRow={(d) => {
                      setSelectedCurrentIssue(null);
                      setSelectedDiagnosis(d);
                    }}
                  />
                )}
              </div>
            </>
          ) : null}
        </main>

        {/* Detail Modal */}
        <DetailView
          diagnosis={selectedDiagnosis}
          timeline={selectedCurrentIssue?.timeline}
          firstSeen={selectedCurrentIssue?.firstSeen}
          lastSeen={selectedCurrentIssue?.lastSeen}
          occurrences={selectedCurrentIssue?.occurrences}
          durationSeconds={selectedCurrentIssue?.durationSeconds}
          onClose={() => {
            setSelectedDiagnosis(null);
            setSelectedCurrentIssue(null);
          }}
        />

        {/* Footer */}
        <footer className="text-gray-500 text-center py-8 mt-12 border-t border-white/60">
          <p className="text-sm">Kuberoot - Built for Kubernetes operators</p>
        </footer>
      </div>
    </div>
  );
}
