import React from 'react';

interface FilterPanelProps {
  namespaces: string[];
  types: string[];
  onFilter: (filters: { namespace?: string; type?: string; confidence?: string }) => void;
}

export function FilterPanel({ namespaces, types, onFilter }: FilterPanelProps) {
  const [selectedNamespace, setSelectedNamespace] = React.useState<string>('');
  const [selectedType, setSelectedType] = React.useState<string>('');
  const [selectedConfidence, setSelectedConfidence] = React.useState<string>('');

  const updateFilters = (next: { namespace?: string; type?: string; confidence?: string }) => {
    const namespace = next.namespace ?? selectedNamespace;
    const type = next.type ?? selectedType;
    const confidence = next.confidence ?? selectedConfidence;

    onFilter({
      namespace: namespace || undefined,
      type: type || undefined,
      confidence: confidence || undefined,
    });
  };

  return (
    <div className="surface-card rounded-2xl p-5 mb-6">
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div>
          <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider mb-2">Namespace</label>
          <select
            value={selectedNamespace}
            onChange={(e) => {
              setSelectedNamespace(e.target.value);
              updateFilters({ namespace: e.target.value });
            }}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-kuberoot-500 focus:border-transparent bg-white"
          >
            <option value="">All Namespaces</option>
            {namespaces.map((ns) => (
              <option key={ns} value={ns}>
                {ns}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider mb-2">Failure Type</label>
          <select
            value={selectedType}
            onChange={(e) => {
              setSelectedType(e.target.value);
              updateFilters({ type: e.target.value });
            }}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-kuberoot-500 focus:border-transparent bg-white"
          >
            <option value="">All Types</option>
            {types.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-xs font-semibold text-gray-600 uppercase tracking-wider mb-2">Confidence</label>
          <select
            value={selectedConfidence}
            onChange={(e) => {
              setSelectedConfidence(e.target.value);
              updateFilters({ confidence: e.target.value });
            }}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-kuberoot-500 focus:border-transparent bg-white"
          >
            <option value="">All Levels</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
        </div>

        <div>
          <button
            onClick={() => {
              setSelectedNamespace('');
              setSelectedType('');
              setSelectedConfidence('');
              onFilter({});
            }}
            className="mt-6 w-full px-4 py-2 bg-kuberoot-50 text-kuberoot-700 rounded-lg hover:bg-kuberoot-100 transition font-semibold"
          >
            Clear Filters
          </button>
        </div>
      </div>
    </div>
  );
}
