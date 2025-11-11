import { useEffect, useState } from 'react';
import type { MCPObserver } from '../services/mcpObserver';
import type { SessionDetail } from '../types/agentSession';
import type { BlastZoneSnapshot } from '../types/blastZone';
import type { TimelineEvent } from '../types/timeline';
import { BlastZoneGraph } from './BlastZoneGraph';
import { HypothesisPanel } from './HypothesisPanel';
import { FindingsList } from './FindingsList';
import { QueryList } from './QueryList';
import { Timeline } from './Timeline';
import { RecommendationsList } from './RecommendationsList';

interface SessionViewProps {
  sessionId: string;
  observer: MCPObserver;
}

export function SessionView({ sessionId, observer }: SessionViewProps) {
  const [sessionDetail, setSessionDetail] = useState<SessionDetail | null>(null);
  const [blastZone, setBlastZone] = useState<BlastZoneSnapshot | null>(null);
  const [timeline, setTimeline] = useState<TimelineEvent[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cleanup: (() => void) | undefined;

    // Reset states for new session
    setSessionDetail(null);
    setBlastZone(null);
    setTimeline([]);
    setIsLoading(true);

    // Load session details first (critical data, usually fast)
    observer.getSessionDetails(sessionId)
      .then((detail) => {
        if (detail) {
          detail.hypotheses = detail.hypotheses || [];
          detail.queries = detail.queries || [];
          detail.findings = detail.findings || [];
          detail.investigations = detail.investigations || [];
        }
        setSessionDetail(detail);
        setIsLoading(false); // UI renders here
      })
      .catch((error) => {
        console.error('Failed to load session:', error);
        setIsLoading(false);
      });

    // Load timeline independently (fast, non-blocking)
    observer.getTimeline(sessionId)
      .then((events) => {
        setTimeline(events || []);
      })
      .catch((error) => {
        console.error('Failed to load timeline:', error);
      });

    // Load blast zone independently (slow, non-blocking)
    observer.getBlastZone(sessionId)
      .then((zone) => {
        setBlastZone(zone);
      })
      .catch((error) => {
        console.error('Failed to load blast zone:', error);
      });

    // Start polling for updates
    cleanup = observer.startPolling(sessionId, {
      onSessionUpdate: (detail) => {
        // Ensure arrays are never null
        if (detail) {
          detail.hypotheses = detail.hypotheses || [];
          detail.queries = detail.queries || [];
          detail.findings = detail.findings || [];
          detail.investigations = detail.investigations || [];
        }
        setSessionDetail(detail);
      },
      onBlastZoneUpdate: setBlastZone,
      onTimelineUpdate: (events) => setTimeline(events || []),
    });

    return () => {
      if (cleanup) cleanup();
    };
  }, [sessionId, observer]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
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
          <p className="text-sm text-gray-500">Loading session...</p>
        </div>
      </div>
    );
  }

  if (!sessionDetail) {
    return (
      <div className="flex items-center justify-center h-full">
        <p className="text-sm text-gray-500">Session not found</p>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900 mb-2">
              {sessionDetail.session.initial_symptom}
            </h1>
            <p className="text-sm text-gray-500">
              Started {new Date(sessionDetail.session.created_at).toLocaleString()}
            </p>
            {sessionDetail.session.event_id && (
              <p className="text-xs text-gray-500 mt-1 flex items-center gap-2 flex-wrap">
                <span>Triggered by:</span>
                {sessionDetail.session.event_source && (
                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-700">
                    {sessionDetail.session.event_source}
                  </span>
                )}
                <span>Event ID: <code className="bg-gray-100 px-1 py-0.5 rounded">{sessionDetail.session.event_id.slice(0, 16)}...</code></span>
                {sessionDetail.session.event_timestamp && (
                  <span>at {new Date(sessionDetail.session.event_timestamp).toLocaleTimeString()}</span>
                )}
                {sessionDetail.session.processing_delay && (
                  <span className="text-gray-600">
                    (Delay: {(sessionDetail.session.processing_delay / 1e9).toFixed(1)}s)
                  </span>
                )}
              </p>
            )}
            {sessionDetail.session.initial_resource && (
              <p className="text-xs text-gray-500 mt-1">
                Initial resource: <code className="bg-gray-100 px-1 py-0.5 rounded">{sessionDetail.session.initial_resource}</code>
              </p>
            )}
          </div>
          <span
            className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium ${
              sessionDetail.session.status === 'active'
                ? 'status-active'
                : sessionDetail.session.status === 'timeout'
                ? 'status-timeout'
                : sessionDetail.session.status === 'incomplete'
                ? 'status-incomplete'
                : 'status-completed'
            }`}
          >
            {sessionDetail.session.status === 'timeout' && '⏱️ '}
            {sessionDetail.session.status === 'incomplete' && '⚠️ '}
            {sessionDetail.session.status === 'completed' && '✓ '}
            {sessionDetail.session.status.charAt(0).toUpperCase() + sessionDetail.session.status.slice(1)}
          </span>
        </div>
      </div>

      {/* Current Hypothesis */}
      <HypothesisPanel
        hypothesis={sessionDetail.current_hypothesis}
        stage={sessionDetail.session.current_stage}
      />

      {/* Recommendations */}
      <RecommendationsList recommendations={sessionDetail.recommendations} />

      {/* Blast Zone */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-900">Blast Zone</h2>
          {blastZone && (
            <div className="flex gap-4 text-sm text-gray-600">
              <span>
                <strong>{blastZone.affected_count}</strong> affected
              </span>
              <span>
                <strong>{blastZone.impact_radius}</strong> hop radius
              </span>
            </div>
          )}
        </div>
        <BlastZoneGraph data={blastZone} />
      </div>

      {/* Findings */}
      <FindingsList findings={sessionDetail.findings} />

      {/* Query History */}
      <QueryList queries={sessionDetail.queries} />

      {/* Timeline */}
      <Timeline events={timeline} />

      {/* Linked Investigations */}
      {sessionDetail.investigations && sessionDetail.investigations.length > 0 && (
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">
            Linked Investigations ({sessionDetail.investigations.length})
          </h2>
          <div className="space-y-2">
            {sessionDetail.investigations.map((invId) => (
              <div
                key={invId}
                className="flex items-center gap-2 text-sm p-3 bg-green-50 rounded-lg border border-green-200"
              >
                <svg
                  className="w-4 h-4 text-green-600"
                  fill="currentColor"
                  viewBox="0 0 20 20"
                >
                  <path d="M9 2a1 1 0 000 2h2a1 1 0 100-2H9z" />
                  <path
                    fillRule="evenodd"
                    d="M4 5a2 2 0 012-2 3 3 0 003 3h2a3 3 0 003-3 2 2 0 012 2v11a2 2 0 01-2 2H6a2 2 0 01-2-2V5zm3 4a1 1 0 000 2h.01a1 1 0 100-2H7zm3 0a1 1 0 000 2h3a1 1 0 100-2h-3zm-3 4a1 1 0 100 2h.01a1 1 0 100-2H7zm3 0a1 1 0 100 2h3a1 1 0 100-2h-3z"
                    clipRule="evenodd"
                  />
                </svg>
                <code className="text-green-800 font-mono">{invId}</code>
                <span className="text-gray-500">(Metrics investigation)</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

