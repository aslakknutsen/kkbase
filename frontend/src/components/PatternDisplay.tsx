import type { Pattern } from '../types/agentSession';

interface PatternDisplayProps {
  patterns: Pattern[];
}

export function PatternDisplay({ patterns }: PatternDisplayProps) {
  if (!patterns || patterns.length === 0) {
    return null;
  }

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4 flex items-center gap-2">
        <svg className="w-5 h-5 text-purple-500" fill="currentColor" viewBox="0 0 20 20">
          <path d="M9 4.804A7.968 7.968 0 005.5 4c-1.255 0-2.443.29-3.5.804v10A7.969 7.969 0 015.5 14c1.669 0 3.218.51 4.5 1.385A7.962 7.962 0 0114.5 14c1.255 0 2.443.29 3.5.804v-10A7.968 7.968 0 0014.5 4c-1.255 0-2.443.29-3.5.804V12a1 1 0 11-2 0V4.804z" />
        </svg>
        Discovered Pattern{patterns.length > 1 ? 's' : ''}
      </h2>
      
      <div className="space-y-4">
        {patterns.map((pattern) => (
          <div key={pattern.id} className="border border-purple-200 rounded-lg p-4 bg-purple-50">
            <div className="flex items-start justify-between mb-3">
              <div>
                <h3 className="text-base font-semibold text-gray-900">{pattern.name}</h3>
                <div className="flex gap-2 mt-1">
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800">
                    {pattern.root_cause_resource_type}
                  </span>
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">
                    {pattern.root_cause_issue_type}
                  </span>
                </div>
              </div>
              <span className="text-xs text-gray-500">
                {new Date(pattern.created_at).toLocaleString()}
              </span>
            </div>

            <div className="space-y-3">
              {/* Investigation Steps */}
              <div>
                <h4 className="text-sm font-medium text-gray-700 mb-1">Investigation Steps:</h4>
                <ol className="list-decimal list-inside text-sm text-gray-600 space-y-1">
                  {pattern.investigation_steps.map((step, idx) => (
                    <li key={idx}>{step}</li>
                  ))}
                </ol>
              </div>

              {/* Diagnosis Guidance */}
              <div>
                <h4 className="text-sm font-medium text-gray-700 mb-1">Diagnosis:</h4>
                <p className="text-sm text-gray-600">{pattern.diagnosis_guidance}</p>
              </div>

              {/* Recommendations */}
              {pattern.recommendations.length > 0 && (
                <div>
                  <h4 className="text-sm font-medium text-gray-700 mb-1">Recommendations:</h4>
                  <ul className="list-disc list-inside text-sm text-gray-600 space-y-1">
                    {pattern.recommendations.map((rec, idx) => (
                      <li key={idx}>{rec}</li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

