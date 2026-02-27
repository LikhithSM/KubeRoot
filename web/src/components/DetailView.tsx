import { Diagnosis } from '../types/index';
import { format } from 'date-fns';

interface DetailViewProps {
  diagnosis: Diagnosis | null;
  onClose: () => void;
}

export function DetailView({ diagnosis, onClose }: DetailViewProps) {
  if (!diagnosis) return null;

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
                Status
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

          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Likely Cause
            </label>
            <p className="text-gray-700 leading-relaxed">{diagnosis.likelyCause}</p>
          </div>

          <div>
            <label className="block text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">
              Suggested Fix
            </label>
            <p className="text-gray-700 leading-relaxed bg-blue-50 border border-blue-200 rounded-lg p-4">
              {diagnosis.suggestedFix}
            </p>
          </div>

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
