import type { Finding } from '../types/agentSession';

interface FindingsListProps {
  findings: Finding[];
}

export function FindingsList({ findings }: FindingsListProps) {
  if (findings.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Findings</h2>
        <p className="text-sm text-gray-500">No findings discovered yet</p>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4">
        Findings ({findings.length})
      </h2>
      
      <div className="space-y-3">
        {findings.map((finding) => (
          <div
            key={finding.id}
            className="border border-gray-200 rounded-lg p-4 hover:shadow-sm transition-shadow"
          >
            <div className="flex items-start justify-between mb-2">
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-1">
                  <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium severity-${finding.severity}`}>
                    {finding.severity}
                  </span>
                  <span className="text-xs text-gray-500">{finding.type}</span>
                </div>
                <p className="text-sm text-gray-900 font-medium">{finding.description}</p>
              </div>
            </div>
            
            <div className="text-xs text-gray-500 space-y-1">
              <div>Resource: <code className="bg-gray-100 px-1 py-0.5 rounded">{finding.resource_id}</code></div>
              <div>Detected: {finding.detection_method} • {new Date(finding.discovered_at).toLocaleString()}</div>
            </div>
            
            {finding.evidence && Object.keys(finding.evidence).length > 0 && (
              <details className="mt-2">
                <summary className="text-xs text-blue-600 cursor-pointer hover:text-blue-800">
                  Show evidence
                </summary>
                <pre className="mt-2 text-xs bg-gray-50 p-2 rounded overflow-x-auto">
                  {JSON.stringify(finding.evidence, null, 2)}
                </pre>
              </details>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

