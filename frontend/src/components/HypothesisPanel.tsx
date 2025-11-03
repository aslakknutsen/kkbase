import type { Hypothesis } from '../types/agentSession';

interface HypothesisPanelProps {
  hypothesis: Hypothesis | null | undefined;
  stage: number;
}

export function HypothesisPanel({ hypothesis, stage }: HypothesisPanelProps) {
  if (!hypothesis) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-3">Current Hypothesis</h2>
        <p className="text-sm text-gray-500">No hypothesis yet</p>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-start justify-between mb-3">
        <h2 className="text-lg font-semibold text-gray-900">Current Hypothesis</h2>
        <span className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
          Stage {stage}
        </span>
      </div>
      
      <div className="bg-blue-50 border-l-4 border-blue-400 p-4 rounded">
        <p className="text-sm text-gray-900">{hypothesis.text}</p>
        <div className="mt-2 text-xs text-gray-500">
          {new Date(hypothesis.created_at).toLocaleString()}
        </div>
      </div>
      
      <div className="mt-4">
        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
          hypothesis.status === 'active' ? 'status-active' :
          hypothesis.status === 'confirmed' ? 'bg-green-100 text-green-800' :
          'bg-gray-100 text-gray-800'
        }`}>
          {hypothesis.status}
        </span>
      </div>
    </div>
  );
}

