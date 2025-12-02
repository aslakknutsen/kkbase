import { useState } from 'react';
import type { Pattern } from '../types/agentSession';

interface PatternDisplayProps {
  patterns: Pattern[];
}

export function PatternDisplay({ patterns }: PatternDisplayProps) {
  const [expandedPatterns, setExpandedPatterns] = useState<Set<string>>(new Set());

  if (!patterns || patterns.length === 0) {
    return null;
  }

  const togglePattern = (patternId: string) => {
    setExpandedPatterns((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(patternId)) {
        newSet.delete(patternId);
      } else {
        newSet.add(patternId);
      }
      return newSet;
    });
  };

  const getRelationshipBadge = (relationshipType?: string) => {
    switch (relationshipType) {
      case 'presented':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">
            📋 Suggested
          </span>
        );
      case 'used':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">
            ✓ Applied
          </span>
        );
      case 'discovered':
        return (
          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800">
            ⭐ Discovered
          </span>
        );
      default:
        return null;
    }
  };

  const getBorderColor = (relationshipType?: string) => {
    switch (relationshipType) {
      case 'presented':
        return 'border-blue-200 bg-blue-50';
      case 'used':
        return 'border-green-200 bg-green-50';
      case 'discovered':
        return 'border-purple-200 bg-purple-50';
      default:
        return 'border-gray-200 bg-gray-50';
    }
  };

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4 flex items-center gap-2">
        <svg className="w-5 h-5 text-purple-500" fill="currentColor" viewBox="0 0 20 20">
          <path d="M9 4.804A7.968 7.968 0 005.5 4c-1.255 0-2.443.29-3.5.804v10A7.969 7.969 0 015.5 14c1.669 0 3.218.51 4.5 1.385A7.962 7.962 0 0114.5 14c1.255 0 2.443.29 3.5.804v-10A7.968 7.968 0 0014.5 4c-1.255 0-2.443.29-3.5.804V12a1 1 0 11-2 0V4.804z" />
        </svg>
        Pattern{patterns.length > 1 ? 's' : ''} ({patterns.length})
      </h2>
      
      <div className="space-y-3">
        {patterns.map((pattern) => {
          const isExpanded = expandedPatterns.has(pattern.id);
          return (
            <div key={pattern.id} className={`border rounded-lg ${getBorderColor(pattern.relationship_type)}`}>
              {/* Collapsed view */}
              <div
                className="p-4 cursor-pointer hover:opacity-75 transition-opacity"
                onClick={() => togglePattern(pattern.id)}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-2">
                      <h3 className="text-base font-semibold text-gray-900">{pattern.name}</h3>
                      {getRelationshipBadge(pattern.relationship_type)}
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600">
                        Used {pattern.usage_count} time{pattern.usage_count !== 1 ? 's' : ''}
                      </span>
                    </div>
                    <div className="flex gap-2">
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-200 text-gray-700">
                        {pattern.root_cause_resource_type}
                      </span>
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-200 text-gray-700">
                        {pattern.root_cause_issue_type}
                      </span>
                    </div>
                  </div>
                  <button className="text-gray-400 hover:text-gray-600 ml-4">
                    <svg
                      className={`w-5 h-5 transition-transform ${isExpanded ? 'rotate-180' : ''}`}
                      fill="currentColor"
                      viewBox="0 0 20 20"
                    >
                      <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                    </svg>
                  </button>
                </div>
              </div>

              {/* Expanded view */}
              {isExpanded && (
                <div className="px-4 pb-4 space-y-3 border-t">
                  {/* Investigation Steps */}
                  {pattern.investigation_steps.length > 0 && (
                    <div className="pt-3">
                      <h4 className="text-sm font-medium text-gray-700 mb-2">Investigation Steps:</h4>
                      <ol className="list-decimal list-inside text-sm text-gray-600 space-y-1">
                        {pattern.investigation_steps.map((step, idx) => (
                          <li key={idx}>{step}</li>
                        ))}
                      </ol>
                    </div>
                  )}

                  {/* Diagnosis Guidance */}
                  {pattern.diagnosis_guidance && (
                    <div>
                      <h4 className="text-sm font-medium text-gray-700 mb-1">Diagnosis Guidance:</h4>
                      <p className="text-sm text-gray-600">{pattern.diagnosis_guidance}</p>
                    </div>
                  )}

                  {/* Symptom Keywords */}
                  {pattern.symptom_keywords && pattern.symptom_keywords.length > 0 && (
                    <div>
                      <h4 className="text-sm font-medium text-gray-700 mb-1">Symptom Keywords:</h4>
                      <div className="flex flex-wrap gap-1">
                        {pattern.symptom_keywords.map((keyword, idx) => (
                          <span key={idx} className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-700">
                            {keyword}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}

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

                  {/* Metadata */}
                  <div className="text-xs text-gray-500 pt-2 border-t">
                    Created: {new Date(pattern.created_at).toLocaleString()}
                    {pattern.updated_at && ` | Updated: ${new Date(pattern.updated_at).toLocaleString()}`}
                    {' | Source: '}{pattern.source}
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

