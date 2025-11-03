import type { QueryExecution } from '../types/agentSession';

interface QueryListProps {
  queries: QueryExecution[];
}

export function QueryList({ queries }: QueryListProps) {
  if (queries.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Query History</h2>
        <p className="text-sm text-gray-500">No queries executed yet</p>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4">
        Query History ({queries.length})
      </h2>
      
      <div className="space-y-4">
        {queries.map((query, index) => (
          <details
            key={query.id}
            className="border border-gray-200 rounded-lg overflow-hidden"
          >
            <summary className="p-4 cursor-pointer hover:bg-gray-50 transition-colors">
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs font-mono text-gray-500">#{index + 1}</span>
                    <span className="text-sm font-medium text-gray-900">
                      {query.reasoning}
                    </span>
                  </div>
                  <div className="text-xs text-gray-500">
                    {new Date(query.executed_at).toLocaleString()} • 
                    {query.result_count} results • 
                    {query.duration}ms
                    {query.findings.length > 0 && (
                      <span className="ml-2 text-orange-600 font-medium">
                        → {query.findings.length} findings
                      </span>
                    )}
                  </div>
                </div>
              </div>
            </summary>
            
            <div className="p-4 bg-gray-50 border-t border-gray-200">
              <div className="mb-3">
                <h4 className="text-xs font-semibold text-gray-700 mb-1">Cypher Query</h4>
                <pre className="text-xs bg-white p-3 rounded border border-gray-200 overflow-x-auto">
                  {query.query}
                </pre>
              </div>
              
              {query.params && Object.keys(query.params).length > 0 && (
                <div>
                  <h4 className="text-xs font-semibold text-gray-700 mb-1">Parameters</h4>
                  <pre className="text-xs bg-white p-3 rounded border border-gray-200 overflow-x-auto">
                    {JSON.stringify(query.params, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          </details>
        ))}
      </div>
    </div>
  );
}

