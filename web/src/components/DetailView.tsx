import { Diagnosis } from '../types/index';
import { format } from 'date-fns';

interface DetailViewProps {
  diagnosis: Diagnosis | null;
  timeline?: string[];
  firstSeen?: string;
  lastSeen?: string;
  onClose: () => void;
}

export function DetailView({ diagnosis, timeline, firstSeen, lastSeen, onClose }: DetailViewProps) {
  if (!diagnosis) return null;

  const evidence = diagnosis.evidence || [];
  const quickCommands = diagnosis.quickCommands || [];
  const contextSignals = diagnosis.context || [];

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div className="bg-white rounded-2xl shadow-xl max-w-2xl w-full max-h-96 overflow-y-auto border border-gray-200">
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
                Failure
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
              Likely Cause
            </label>
            <p className="text-gray-700 leading-relaxed">{diagnosis.likelyCause}</p>
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Next Steps
            </label>
            <p className="text-gray-700 leading-relaxed bg-blue-50 border border-blue-200 rounded-lg p-4">
              {diagnosis.suggestedFix}
            </p>
          </div>

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
