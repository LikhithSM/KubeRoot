import { useState, useMemo } from 'react';
import { useDiagnoses } from '../hooks/useDiagnoses';
import { FailureTable } from './FailureTable';
import { DetailView } from './DetailView';
import { FilterPanel } from './FilterPanel';
import { Diagnosis } from '../types/index';
import { format } from 'date-fns';

export function Dashboard() {
  const { diagnoses, loading, error } = useDiagnoses(5000);
  const [selectedDiagnosis, setSelectedDiagnosis] = useState<Diagnosis | null>(null);
  const [filters, setFilters] = useState<{ namespace?: string; type?: string; confidence?: string }>({});

  const uniqueNamespaces = useMemo(() => {
    const ns = new Set(diagnoses.map((d) => d.namespace));
    return Array.from(ns).sort();
  }, [diagnoses]);

  const uniqueTypes = useMemo(() => {
    const types = new Set(diagnoses.map((d) => d.failureType));
    return Array.from(types).sort();
  }, [diagnoses]);

  const stats = useMemo(() => {
    return {
      total: diagnoses.length,
      high: diagnoses.filter((d) => d.confidence === 'high').length,
      medium: diagnoses.filter((d) => d.confidence === 'medium').length,
      low: diagnoses.filter((d) => d.confidence === 'low').length,
    };
  }, [diagnoses]);

  const lastUpdated = useMemo(() => format(new Date(), 'HH:mm:ss'), [diagnoses]);
  const activeCluster = diagnoses[0]?.clusterId || 'unassigned';

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
                    <h2 className="text-lg font-semibold text-gray-900">Pod Failures</h2>
                    <p className="text-sm text-gray-500">Live diagnoses from agent reports</p>
                  </div>
                  <span className="text-xs text-gray-500 uppercase tracking-wider">
                    {diagnoses.length} failures - Updated {lastUpdated}
                  </span>
                </div>
                <FailureTable
                  diagnoses={diagnoses}
                  filters={filters}
                  onSelectRow={setSelectedDiagnosis}
                />
              </div>
            </>
          ) : null}
        </main>

        {/* Detail Modal */}
        <DetailView
          diagnosis={selectedDiagnosis}
          onClose={() => setSelectedDiagnosis(null)}
        />

        {/* Footer */}
        <footer className="text-gray-500 text-center py-8 mt-12 border-t border-white/60">
          <p className="text-sm">Kuberoot - Built for Kubernetes operators</p>
        </footer>
      </div>
    </div>
  );
}
