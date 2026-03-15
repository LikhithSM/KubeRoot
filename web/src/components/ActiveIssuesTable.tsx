import { formatDistanceToNowStrict } from 'date-fns';
import { CurrentFailure } from '../types/index';

interface ActiveIssuesTableProps {
  items: CurrentFailure[];
  onSelectRow?: (issue: CurrentFailure) => void;
}

function severityBadgeClass(severity: string): string {
  switch (severity) {
    case 'critical':
      return 'bg-red-100 text-red-800 border-red-300';
    case 'high':
      return 'bg-orange-100 text-orange-800 border-orange-300';
    case 'medium':
      return 'bg-yellow-100 text-yellow-800 border-yellow-300';
    default:
      return 'bg-gray-100 text-gray-700 border-gray-300';
  }
}

function severityRowClass(severity: string): string {
  switch (severity) {
    case 'critical':
      return 'bg-red-50/40';
    case 'high':
      return 'bg-orange-50/30';
    default:
      return '';
  }
}

function severityDot(severity: string): string {
  switch (severity) {
    case 'critical': return '🔴';
    case 'high':     return '🟠';
    case 'medium':   return '🟡';
    default:         return '⚪';
  }
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  return `${Math.floor(seconds / 3600)}h`;
}

export function ActiveIssuesTable({ items, onSelectRow }: ActiveIssuesTableProps) {
  if (items.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-600 text-lg">No active failures</p>
        <p className="text-gray-500 text-sm mt-2">Your cluster is healthy right now.</p>
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead className="bg-white border-b border-gray-200">
          <tr>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Severity</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Workload</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Failure</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Duration</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Restarts</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Likely Cause</th>
            <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 uppercase tracking-wider">Updated</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200">
          {items.map((issue) => {
            const sev = issue.severity || issue.diagnosis.severity || 'low';
            return (
              <tr
                key={issue.issueKey}
                onClick={() => onSelectRow?.(issue)}
                className={`transition-colors ${severityRowClass(sev)} ${onSelectRow ? 'cursor-pointer hover:bg-kuberoot-50' : ''}`}
              >
                <td className="px-6 py-3 text-sm">
                  <span className={`inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-semibold border ${severityBadgeClass(sev)}`}>
                    {severityDot(sev)} {sev.toUpperCase()}
                  </span>
                </td>
                <td className="px-6 py-3 text-sm text-gray-900">
                  <div className="font-mono">{issue.diagnosis.podName}</div>
                  <div className="text-xs text-gray-500">{issue.diagnosis.namespace}</div>
                </td>
                <td className="px-6 py-3 text-sm text-gray-900">{issue.diagnosis.failureType}</td>
                <td className="px-6 py-3 text-sm text-gray-700">{formatDuration(issue.durationSeconds)}</td>
                <td className="px-6 py-3 text-sm text-gray-700">
                  {issue.diagnosis.restartCount ?? 0}
                  {issue.restartSpike ? (
                    <span className="ml-2 text-xs px-2 py-1 rounded border border-red-300 text-red-700 bg-red-50">Spike</span>
                  ) : null}
                  {issue.imageChanged ? (
                    <span className="ml-2 text-xs px-2 py-1 rounded border border-indigo-300 text-indigo-700 bg-indigo-50">Image changed</span>
                  ) : null}
                </td>
                <td className="px-6 py-3 text-sm text-gray-600 max-w-md truncate">{issue.diagnosis.likelyCause}</td>
                <td className="px-6 py-3 text-sm text-gray-500">
                  {formatDistanceToNowStrict(new Date(issue.lastSeen), { addSuffix: true })}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

