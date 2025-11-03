import { useEffect, useState } from 'react';
import { MCPObserver } from './services/mcpObserver';
import { SessionList } from './components/SessionList';
import { SessionView } from './components/SessionView';
import { EmptyState } from './components/EmptyState';
import type { ActiveSessionInfo } from './types/agentSession';

export function App() {
  const [observer] = useState(() => new MCPObserver('/mcp'));
  const [activeSessions, setActiveSessions] = useState<ActiveSessionInfo[]>([]);
  const [selectedSession, setSelectedSession] = useState<string | null>(null);
  const [isConnecting, setIsConnecting] = useState(true);

  useEffect(() => {
    // Connect to SSE for push notifications
    observer.connectSSE();
    setIsConnecting(false);

    // Start polling for active sessions (with SSE notifications)
    const cleanup = observer.startSessionsPolling((sessions) => {
      setActiveSessions(sessions);
      
      // Auto-select first session if none selected
      if (sessions.length > 0 && !selectedSession) {
        setSelectedSession(sessions[0].id);
      }
      
      // Clear selection if selected session is no longer active
      if (selectedSession && !sessions.find(s => s.id === selectedSession)) {
        setSelectedSession(sessions.length > 0 ? sessions[0].id : null);
      }
    });

    return () => {
      cleanup();
      observer.disconnectSSE();
    };
  }, [observer, selectedSession]);

  if (isConnecting) {
    return (
      <div className="flex items-center justify-center h-screen bg-gray-50">
        <div className="text-center">
          <svg
            className="animate-spin h-12 w-12 text-blue-500 mx-auto mb-4"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            ></circle>
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          <p className="text-sm text-gray-500">Connecting to KKBase...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside className="w-80 bg-white border-r border-gray-200 p-4 overflow-y-auto">
        <div className="mb-6">
          <h1 className="text-xl font-bold text-gray-900 mb-1">
            KKBase Dashboard
          </h1>
          <p className="text-xs text-gray-500">
            Agent Investigation Sessions
          </p>
        </div>
        
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-sm font-semibold text-gray-700">
            Active Sessions
          </h2>
          {activeSessions.length > 0 && (
            <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
              {activeSessions.length}
            </span>
          )}
        </div>
        
        <SessionList
          sessions={activeSessions}
          selected={selectedSession}
          onSelect={setSelectedSession}
        />
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-y-auto">
        {selectedSession ? (
          <SessionView sessionId={selectedSession} observer={observer} />
        ) : (
          <EmptyState />
        )}
      </main>
    </div>
  );
}

