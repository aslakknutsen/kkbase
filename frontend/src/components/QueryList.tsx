import type { QueryExecution } from '../types/agentSession';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';

interface QueryListProps {
  queries: QueryExecution[];
}

export function QueryList({ queries }: QueryListProps) {
  if (!queries || queries.length === 0) {
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
                    {(query.duration / 1_000_000)}ms
                    {query.findings && query.findings.length > 0 && (
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
                <h4 className="text-xs font-semibold text-gray-700 mb-2">Cypher Query</h4>
                <div className="rounded border border-gray-200 overflow-hidden">
                  <SyntaxHighlighter
                    language="cypher"
                    style={vscDarkPlus}
                    customStyle={{
                      margin: 0,
                      fontSize: '0.75rem',
                      lineHeight: '1.5',
                    }}
                    showLineNumbers={false}
                  >
                    {query.query}
                  </SyntaxHighlighter>
                </div>
              </div>
              
              {query.params && Object.keys(query.params).length > 0 && (
                <div>
                  <h4 className="text-xs font-semibold text-gray-700 mb-2">Parameters</h4>
                  <div className="rounded border border-gray-200 overflow-hidden">
                    <SyntaxHighlighter
                      language="json"
                      style={vscDarkPlus}
                      customStyle={{
                        margin: 0,
                        fontSize: '0.75rem',
                        lineHeight: '1.5',
                      }}
                      showLineNumbers={false}
                    >
                      {JSON.stringify(query.params, null, 2)}
                    </SyntaxHighlighter>
                  </div>
                </div>
              )}
              
              {query.results && query.results.length > 0 && (
                <div className="mt-3">
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="text-xs font-semibold text-gray-700">
                      Results ({query.results.length})
                    </h4>
                    {query.truncated && (
                      <span className="text-xs text-orange-600 font-medium">
                        Showing first 100 of {query.result_count} results
                      </span>
                    )}
                  </div>
                  <div className="rounded border border-gray-200 overflow-hidden max-h-96 overflow-y-auto">
                    <SyntaxHighlighter
                      language="json"
                      style={vscDarkPlus}
                      customStyle={{
                        margin: 0,
                        fontSize: '0.75rem',
                        lineHeight: '1.5',
                      }}
                      showLineNumbers={true}
                    >
                      {JSON.stringify(query.results, null, 2)}
                    </SyntaxHighlighter>
                  </div>
                </div>
              )}
            </div>
          </details>
        ))}
      </div>
    </div>
  );
}

