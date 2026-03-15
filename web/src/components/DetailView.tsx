import { Diagnosis } from '../types/index';
import { format } from 'date-fns';

interface DetailViewProps {
  diagnosis: Diagnosis | null;
  timeline?: string[];
  firstSeen?: string;
  lastSeen?: string;
  occurrences?: number;
  durationSeconds?: number;
  onClose: () => void;
}

export function DetailView({ diagnosis, timeline, firstSeen, lastSeen, occurrences, durationSeconds, onClose }: DetailViewProps) {
  if (!diagnosis) return null;

  const evidence = diagnosis.evidence || [];
  const fixSuggestions = diagnosis.fixSuggestions || [];
  const quickCommands = diagnosis.quickCommands || [];
  const contextSignals = diagnosis.context || [];
  const replicasLine = contextSignals.find((c) => c.startsWith('Replicas: '));
  const suggestedPatch = fixSuggestions
    .map((f) => f.command?.trim() || '')
    .find((cmd) => cmd.includes(':') && !cmd.startsWith('kubectl ') && !cmd.startsWith('docker '));

  const impactSummary = (() => {
    const lines: string[] = [];
    lines.push('1 pod currently failing for this issue.');
    if (typeof occurrences === 'number' && occurrences > 1) {
      lines.push(`Observed ${occurrences} times.`);
    }
    if (typeof durationSeconds === 'number' && durationSeconds > 0) {
      lines.push(`Active for ${Math.max(1, Math.floor(durationSeconds / 60))} minutes.`);
    }
    if (replicasLine) {
      const m = replicasLine.match(/Replicas:\s*(\d+)\/(\d+)/);
      if (m) {
        const ready = Number(m[1]);
        const desired = Number(m[2]);
        if (!Number.isNaN(ready) && !Number.isNaN(desired) && desired > 0) {
          if (ready < desired) {
            lines.push(`Service impact: deployment degraded (${ready}/${desired} ready).`);
          } else {
            lines.push('Service impact: no replica degradation detected.');
          }
        }
      }
    }
    return lines;
  })();

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-3 md:p-5 z-50">
      <div className="bg-white rounded-2xl shadow-xl w-[96vw] md:w-[92vw] max-w-5xl max-h-[90vh] overflow-y-auto border border-gray-200">
        <div className="sticky top-0 bg-gradient-to-r from-kuberoot-700 to-kuberoot-500 text-white px-6 py-4 flex justify-between items-center">
          <h2 className="text-xl font-bold font-display">{diagnosis.podName}</h2>
          <button
            onClick={onClose}
            className="text-white hover:bg-white hover:bg-opacity-20 rounded-full p-2 transition"
          >
            X
          </button>
        </div>

        <div className="p-6 space-y-6">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1">
                Namespace
              </label>
              <p className="text-lg font-mono text-gray-900">{diagnosis.namespace}</p>
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1">
                Failure Type
              </label>
              <p>
                <span className={`px-3 py-1 rounded-full text-sm font-semibold border ${
                  diagnosis.confidence === 'high'
                    ? 'bg-red-100 text-red-800 border-red-300'
                    : diagnosis.confidence === 'medium'
                    ? 'bg-orange-100 text-orange-800 border-orange-300'
                    : 'bg-yellow-100 text-yellow-800 border-yellow-300'
                }`}>
                  {diagnosis.failureType}
                </span>
              </p>
              <p className="text-sm text-gray-500 mt-2">Category: {diagnosis.category || 'Runtime issue'}</p>
            </div>
          </div>

          {contextSignals.length > 0 && (
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Deployment Context
              </label>
              <div className="space-y-2">
                {contextSignals.map((item, idx) => (
                  <div key={idx} className="bg-indigo-50 border border-indigo-200 rounded p-3 text-sm text-indigo-900">
                    {item}
                  </div>
                ))}
              </div>
            </div>
          )}

          {timeline && timeline.length > 0 && (
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Timeline
              </label>
              <div className="space-y-2">
                {timeline.map((line, idx) => (
                  <div key={idx} className="bg-emerald-50 border border-emerald-200 rounded p-3 text-sm text-emerald-900">
                    {line}
                  </div>
                ))}
              </div>
              {(firstSeen || lastSeen) && (
                <p className="text-xs text-gray-500 mt-2">
                  {firstSeen ? `First seen: ${format(new Date(firstSeen), 'MMM d, HH:mm:ss')}` : ''}
                  {firstSeen && lastSeen ? ' | ' : ''}
                  {lastSeen ? `Last seen: ${format(new Date(lastSeen), 'MMM d, HH:mm:ss')}` : ''}
                </p>
              )}
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Impact
            </label>
            <div className="bg-rose-50 border border-rose-200 rounded-lg p-4 space-y-2">
              {impactSummary.map((line, idx) => (
                <p key={idx} className="text-sm text-rose-900">{line}</p>
              ))}
            </div>
          </div>

          {evidence.length > 0 && (
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Evidence
              </label>
              <div className="space-y-2">
                {evidence.map((item, idx) => (
                  <div key={idx} className="bg-gray-50 border border-gray-200 rounded p-3 text-sm text-gray-800 font-mono">
                    {item}
                  </div>
                ))}
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Root Cause
            </label>
            <div className="bg-amber-50 border border-amber-200 rounded-lg p-4">
              <p className="text-gray-800 leading-relaxed">{diagnosis.likelyCause}</p>
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Recommended Fix
            </label>
            <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 space-y-3">
              <p className="text-gray-800 leading-relaxed whitespace-pre-line">{diagnosis.suggestedFix}</p>
            </div>
          </div>

          {suggestedPatch && (
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Exact Fix (Patch Snippet)
              </label>
              <div className="bg-slate-950 text-slate-100 rounded-lg p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap border border-slate-700">
                {suggestedPatch}
              </div>
            </div>
          )}

          {fixSuggestions.length > 0 && (
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Exact Fix Commands
              </label>
              <div className="space-y-3">
                {fixSuggestions.map((fix, idx) => (
                  <div key={idx} className="border border-emerald-200 bg-emerald-50 rounded-xl p-4 space-y-3">
                    <div>
                      <h3 className="text-sm font-semibold text-emerald-950">{fix.title}</h3>
                      <p className="text-sm text-emerald-900 mt-1">{fix.explanation}</p>
                    </div>
                    {fix.command ? (
                      <div className="bg-slate-950 text-slate-100 rounded-lg p-3 text-sm font-mono overflow-x-auto whitespace-pre-wrap">
                        {fix.command}
                      </div>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Confidence
            </label>
            <div className="bg-white border border-gray-200 rounded-lg p-4 space-y-2">
              <p className="text-sm font-semibold text-gray-900">{diagnosis.confidence.toUpperCase()}</p>
              {diagnosis.confidenceNote && (
                <p className="text-sm text-gray-600">{diagnosis.confidenceNote}</p>
              )}
            </div>
          </div>

          {quickCommands.length > 0 && (
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
                Quick Commands
              </label>
              <div className="space-y-2">
                {quickCommands.map((cmd, idx) => (
                  <div key={idx} className="bg-slate-900 text-slate-100 rounded p-3 text-sm font-mono overflow-x-auto">
                    {cmd}
                  </div>
                ))}
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Events ({diagnosis.events.length})
            </label>
            <div className="space-y-2">
              {diagnosis.events.map((event, idx) => (
                <div key={idx} className="bg-gray-50 border border-gray-200 rounded p-3 text-sm text-gray-700 font-mono">
                  {event}
                </div>
              ))}
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1">
              Detected At
            </label>
            <p className="text-sm text-gray-600">
              {format(new Date(diagnosis.timestamp), 'EEEE, MMMM d, yyyy HH:mm:ss')}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
