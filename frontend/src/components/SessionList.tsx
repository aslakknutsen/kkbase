import type { ActiveSessionInfo } from '../types/agentSession';

interface SessionListProps {
  sessions: ActiveSessionInfo[];
  selected: string | null;
  onSelect: (sessionId: string) => void;
}

export function SessionList({ sessions, selected, onSelect }: SessionListProps) {
  if (sessions.length === 0) {
    return (
      <div className="text-center py-8">
        <p className="text-sm text-gray-500">No active sessions</p>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {sessions.map((session) => {
        const isCompleted = session.status === 'completed';
        const isSelected = selected === session.id;
        
        return (
          <button
            key={session.id}
            onClick={() => onSelect(session.id)}
            className={`w-full text-left p-4 rounded-lg border transition-all ${
              isSelected
                ? isCompleted
                  ? 'bg-green-50 border-green-500 shadow-sm'
                  : 'bg-blue-50 border-blue-500 shadow-sm'
                : isCompleted
                ? 'bg-green-50 border-green-200 hover:border-green-300 hover:shadow-sm'
                : 'bg-white border-gray-200 hover:border-gray-300 hover:shadow-sm'
            }`}
          >
            <div className="flex items-start justify-between mb-2">
              <h3 className="text-sm font-semibold text-gray-900 line-clamp-2 flex-1 pr-2">
                {session.initial_symptom}
              </h3>
              {isCompleted && (
                <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 whitespace-nowrap">
                  ✓ Complete
                </span>
              )}
            </div>
          
          <div className="flex items-center gap-3 text-xs text-gray-500 mb-2">
            <span className="inline-flex items-center">
              <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z"
                  clipRule="evenodd"
                />
              </svg>
              {new Date(session.created_at).toLocaleTimeString()}
            </span>
          </div>
          
          <div className="flex items-center gap-3 text-xs">
            <span className="inline-flex items-center px-2 py-1 rounded bg-purple-100 text-purple-800">
              {session.query_count} queries
            </span>
            <span className="inline-flex items-center px-2 py-1 rounded bg-orange-100 text-orange-800">
              {session.finding_count} findings
            </span>
            <span className="inline-flex items-center px-2 py-1 rounded bg-blue-100 text-blue-800">
              Stage {session.current_stage}
            </span>
          </div>
          </button>
        );
      })}
    </div>
  );
}

