import { Diagnosis } from '../types/index';
import { format } from 'date-fns';

const confidenceColors = {
  low: 'bg-yellow-100 text-yellow-800 border-yellow-300',
  medium: 'bg-orange-100 text-orange-800 border-orange-300',
  high: 'bg-red-100 text-red-800 border-red-300',
};

const failureTypeColors: Record<string, string> = {
  'CrashLoopBackOff': 'bg-red-50 border-red-200 text-red-700',
  'ImagePullBackOff': 'bg-amber-50 border-amber-200 text-amber-700',
  'Pending': 'bg-teal-50 border-teal-200 text-teal-700',
  'Failed': 'bg-red-50 border-red-200 text-red-700',
  'Unknown': 'bg-slate-50 border-slate-200 text-slate-700',
};

interface FailureTableProps {
  diagnoses: Diagnosis[];
  onSelectRow?: (diagnosis: Diagnosis) => void;
  filters?: {
    namespace?: string;
    type?: string;
    confidence?: string;
  };
}

export function FailureTable({ diagnoses, onSelectRow, filters }: FailureTableProps) {
  const filtered = diagnoses.filter((d) => {
    if (filters?.namespace && d.namespace !== filters.namespace) return false;
    if (filters?.type && d.failureType !== filters.type) return false;
    if (filters?.confidence && d.confidence !== filters.confidence) return false;
    return true;
  });

  if (filtered.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-600 text-lg">No failures found</p>
        <p className="text-gray-500 text-sm mt-2">Your cluster looks healthy.</p>
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead className="bg-white border-b border-gray-200">
          <tr>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Namespace</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Pod</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Type</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Likely Cause</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Confidence</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Time</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200">
          {filtered.map((diagnosis, idx) => {
            const typeColor = failureTypeColors[diagnosis.failureType] || failureTypeColors['Unknown'];
            return (
              <tr
                key={idx}
                onClick={() => onSelectRow?.(diagnosis)}
                className={`transition-colors ${onSelectRow ? 'cursor-pointer hover:bg-kuberoot-50' : ''}`}
              >
                <td className="px-6 py-3 text-sm text-gray-900">{diagnosis.namespace}</td>
                <td className="px-6 py-3 text-sm font-mono text-gray-700">{diagnosis.podName}</td>
                <td className="px-6 py-3 text-sm">
                  <span className={`px-3 py-1 rounded-full text-xs font-semibold border ${typeColor}`}>
                    {diagnosis.failureType}
                  </span>
                </td>
                <td className="px-6 py-3 text-sm text-gray-600 max-w-md truncate">{diagnosis.likelyCause}</td>
                <td className="px-6 py-3 text-sm">
                  <span className={`px-3 py-1 rounded-full text-xs font-semibold border ${confidenceColors[diagnosis.confidence as keyof typeof confidenceColors]}`}>
                    {diagnosis.confidence.toUpperCase()}
                  </span>
                </td>
                <td className="px-6 py-3 text-sm text-gray-500">
                  {format(new Date(diagnosis.timestamp), 'MMM d, HH:mm')}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
