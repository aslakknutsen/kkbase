import type { Recommendation } from '../types/agentSession';

interface RecommendationsListProps {
  recommendations: Recommendation[];
}

const priorityConfig = {
  critical: { bg: 'bg-red-50', border: 'border-red-300', text: 'text-red-800', badge: 'bg-red-100 text-red-800', icon: '🔴' },
  high: { bg: 'bg-orange-50', border: 'border-orange-300', text: 'text-orange-800', badge: 'bg-orange-100 text-orange-800', icon: '🟠' },
  medium: { bg: 'bg-yellow-50', border: 'border-yellow-300', text: 'text-yellow-800', badge: 'bg-yellow-100 text-yellow-800', icon: '🟡' },
  low: { bg: 'bg-green-50', border: 'border-green-300', text: 'text-green-800', badge: 'bg-green-100 text-green-800', icon: '🟢' },
};

const typeLabels = {
  root_cause_fix: 'Root Cause Fix',
  preventive_action: 'Preventive Action',
  optimization: 'Optimization',
  monitoring_improvement: 'Monitoring Improvement',
  cleanup: 'Cleanup',
};

export function RecommendationsList({ recommendations }: RecommendationsListProps) {
  if (!recommendations || recommendations.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Recommendations</h2>
        <p className="text-sm text-gray-500">No recommendations yet</p>
      </div>
    );
  }

  // Sort by priority
  const sorted = [...recommendations].sort((a, b) => {
    const priorityOrder = { critical: 0, high: 1, medium: 2, low: 3 };
    return priorityOrder[a.priority] - priorityOrder[b.priority];
  });

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4">
        Recommendations ({recommendations.length})
      </h2>
      <div className="space-y-4">
        {sorted.map((rec) => {
          const config = priorityConfig[rec.priority];
          return (
            <div
              key={rec.id}
              className={`p-4 rounded-lg border ${config.border} ${config.bg}`}
            >
              <div className="flex items-start justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span className="text-xl">{config.icon}</span>
                  <h3 className={`font-semibold ${config.text}`}>{rec.title}</h3>
                </div>
                <div className="flex gap-2">
                  <span className={`px-2 py-1 rounded text-xs font-medium ${config.badge}`}>
                    {rec.priority.toUpperCase()}
                  </span>
                  <span className="px-2 py-1 rounded text-xs font-medium bg-gray-100 text-gray-700">
                    {typeLabels[rec.type]}
                  </span>
                </div>
              </div>

              <p className="text-sm text-gray-700 mb-3">{rec.description}</p>

              {rec.rationale && (
                <div className="mb-3">
                  <p className="text-xs font-semibold text-gray-600 mb-1">Rationale:</p>
                  <p className="text-xs text-gray-600 italic">{rec.rationale}</p>
                </div>
              )}

              {rec.action_items && rec.action_items.length > 0 && (
                <div className="mb-3">
                  <p className="text-xs font-semibold text-gray-700 mb-1">Action Items:</p>
                  <ol className="list-decimal list-inside space-y-1">
                    {rec.action_items.map((item, idx) => (
                      <li key={idx} className="text-xs text-gray-600">{item}</li>
                    ))}
                  </ol>
                </div>
              )}

              {rec.automation_hint && (
                <div className="mb-3 p-2 bg-gray-900 rounded">
                  <p className="text-xs font-semibold text-gray-300 mb-1">Automation:</p>
                  <code className="text-xs text-green-400 font-mono">{rec.automation_hint}</code>
                </div>
              )}

              <div className="flex items-center gap-4 text-xs text-gray-500">
                {rec.estimated_effort && (
                  <span>⏱️ Est. {rec.estimated_effort}</span>
                )}
                {rec.related_findings && rec.related_findings.length > 0 && (
                  <span>🔗 Based on {rec.related_findings.length} finding(s)</span>
                )}
                {rec.tags && rec.tags.length > 0 && (
                  <div className="flex gap-1">
                    {rec.tags.map((tag) => (
                      <span key={tag} className="px-1.5 py-0.5 bg-gray-200 rounded text-xs">
                        {tag}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

