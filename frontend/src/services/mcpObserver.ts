// MCP Observer Service for read-only dashboard access
// Uses MCP SDK for tool calls and SSE for push notifications

import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { Transport } from '@modelcontextprotocol/sdk/shared/transport.js';
import type { ActiveSessionInfo, SessionDetail } from '../types/agentSession';
import type { BlastZoneSnapshot } from '../types/blastZone';
import type { TimelineEvent } from '../types/timeline';

// Custom HTTP transport for MCP client (browser-compatible)
class BrowserHTTPTransport implements Transport {
  private url: string;
  private oncloseCallback?: () => void;
  private onerrorCallback?: (error: Error) => void;

  constructor(url: string) {
    this.url = url;
  }

  async start(): Promise<void> {
    // No connection setup needed for HTTP
  }

  async close(): Promise<void> {
    if (this.oncloseCallback) {
      this.oncloseCallback();
    }
  }

  async send(message: any): Promise<void> {
    try {
      const response = await fetch(this.url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(message),
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      // For HTTP transport, we handle the response in the transport layer
      const result = await response.json();
      
      // Call the message handler if we received a response
      if (this.onmessage && result) {
        this.onmessage(result);
      }
    } catch (error) {
      if (this.onerrorCallback) {
        this.onerrorCallback(error as Error);
      }
      throw error;
    }
  }

  onclose?: () => void;
  onerror?: (error: Error) => void;
  onmessage?: (message: any) => void;
}

export class MCPObserver {
  private mcpClient: Client;
  private transport: BrowserHTTPTransport;
  private eventsURL: string;
  private eventSource: EventSource | null = null;
  private pollInterval: number = 10000; // 10 seconds (fallback only)
  private notificationHandlers: Map<string, ((data: any) => void)[]> = new Map();
  private isConnected: boolean = false;

  constructor(baseURL: string = '/mcp', eventsURL: string = '/events') {
    this.transport = new BrowserHTTPTransport(baseURL);
    this.mcpClient = new Client(
      {
        name: 'kkbase-dashboard',
        version: '1.0.0',
      },
      {
        capabilities: {},
      }
    );
    this.eventsURL = eventsURL;
  }

  // Initialize MCP client connection
  async connect(): Promise<void> {
    if (this.isConnected) {
      return;
    }

    try {
      await this.mcpClient.connect(this.transport);
      this.isConnected = true;
      console.log('MCP client connected');
    } catch (error) {
      console.error('Failed to connect MCP client:', error);
      throw error;
    }
  }

  // Disconnect MCP client
  async disconnect(): Promise<void> {
    if (this.isConnected) {
      await this.mcpClient.close();
      this.isConnected = false;
      console.log('MCP client disconnected');
    }
  }

  // Connect to SSE stream for push notifications
  connectSSE(): void {
    if (this.eventSource) {
      return; // Already connected
    }

    console.log('Connecting to SSE endpoint:', this.eventsURL);
    this.eventSource = new EventSource(this.eventsURL);

    this.eventSource.onopen = () => {
      console.log('SSE connection established');
    };

    this.eventSource.onerror = (error) => {
      console.error('SSE connection error:', error);
      // Will auto-reconnect
    };

    // Listen for specific notification events
    const notificationTypes = [
      'agent_session/created',
      'agent_session/query_executed',
      'agent_session/hypothesis_updated',
      'agent_session/finding_discovered',
      'agent_session/blast_zone_updated',
      'agent_session/investigation_spawned',
      'agent_session/completed',
    ];

    notificationTypes.forEach((eventType) => {
      this.eventSource!.addEventListener(eventType, (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data);
          console.log('Received SSE notification:', eventType, data);
          this.triggerHandlers(eventType, data);
        } catch (error) {
          console.error('Failed to parse SSE event:', error);
        }
      });
    });

    // Handle heartbeat
    this.eventSource.addEventListener('heartbeat', () => {
      // Keep-alive, no action needed
    });

    // Handle connection established
    this.eventSource.addEventListener('connected', (event: MessageEvent) => {
      const data = JSON.parse(event.data);
      console.log('SSE connected:', data.connection_id);
    });
  }

  // Disconnect from SSE
  disconnectSSE(): void {
    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
      console.log('SSE connection closed');
    }
  }

  // Subscribe to notifications
  onNotification(eventType: string, handler: (data: any) => void): void {
    if (!this.notificationHandlers.has(eventType)) {
      this.notificationHandlers.set(eventType, []);
    }
    this.notificationHandlers.get(eventType)!.push(handler);
  }

  // Trigger registered handlers for an event
  private triggerHandlers(eventType: string, data: any): void {
    const handlers = this.notificationHandlers.get(eventType);
    if (handlers) {
      handlers.forEach((handler) => handler(data));
    }
  }

  // Call MCP tool using the SDK
  private async callTool<T = any>(toolName: string, params: any = {}): Promise<T> {
    if (!this.isConnected) {
      await this.connect();
    }

    try {
      const result = await this.mcpClient.callTool({
        name: toolName,
        arguments: params,
      });

      // The MCP SDK returns the tool result in a structured format
      // Extract the actual data from the result
      if (result.content && Array.isArray(result.content) && result.content.length > 0) {
        // If the result contains content items, parse the first text item
        const textContent = result.content.find((item: any) => item.type === 'text');
        if (textContent && textContent.text) {
          try {
            // Try to parse as JSON if the tool returns JSON data
            return JSON.parse(textContent.text) as T;
          } catch {
            // If not JSON, return the text as-is
            return textContent.text as T;
          }
        }
      }

      // Fallback: return the whole result
      return result as T;
    } catch (error: any) {
      console.error(`MCP tool call failed (${toolName}):`, error);
      throw new Error(`MCP tool call failed: ${error.message || error}`);
    }
  }

  // Get list of active sessions
  async getActiveSessions(): Promise<ActiveSessionInfo[]> {
    try {
      const result = await this.callTool<ActiveSessionInfo[]>('get_active_sessions', {});
      return result || [];
    } catch (error) {
      console.error('Failed to get active sessions:', error);
      return [];
    }
  }

  // Get complete session details
  async getSessionDetails(sessionId: string): Promise<SessionDetail | null> {
    try {
      const result = await this.callTool<SessionDetail>('get_session_details', {
        session_id: sessionId,
      });
      return result;
    } catch (error) {
      console.error('Failed to get session details:', error);
      return null;
    }
  }

  // Get blast zone for a session
  async getBlastZone(sessionId: string): Promise<BlastZoneSnapshot | null> {
    try {
      const result = await this.callTool<BlastZoneSnapshot>('get_blast_zone', {
        session_id: sessionId,
      });
      return result;
    } catch (error) {
      console.error('Failed to get blast zone:', error);
      return null;
    }
  }

  // Get timeline for a session
  async getTimeline(sessionId: string): Promise<TimelineEvent[]> {
    try {
      const result = await this.callTool<TimelineEvent[]>('get_session_timeline', {
        session_id: sessionId,
      });
      return result || [];
    } catch (error) {
      console.error('Failed to get timeline:', error);
      return [];
    }
  }

  // Start watching for updates (SSE + fallback polling)
  startPolling(
    sessionId: string,
    callbacks: {
      onSessionUpdate?: (session: SessionDetail | null) => void;
      onBlastZoneUpdate?: (blastZone: BlastZoneSnapshot | null) => void;
      onTimelineUpdate?: (timeline: TimelineEvent[]) => void;
    }
  ): () => void {
    // Subscribe to SSE notifications for this session
    this.onNotification('agent_session/query_executed', async (data) => {
      if (data.session_id === sessionId) {
        if (callbacks.onSessionUpdate) {
          const session = await this.getSessionDetails(sessionId);
          callbacks.onSessionUpdate(session);
        }
        if (callbacks.onTimelineUpdate) {
          const timeline = await this.getTimeline(sessionId);
          callbacks.onTimelineUpdate(timeline);
        }
      }
    });

    this.onNotification('agent_session/hypothesis_updated', async (data) => {
      if (data.session_id === sessionId) {
        if (callbacks.onSessionUpdate) {
          const session = await this.getSessionDetails(sessionId);
          callbacks.onSessionUpdate(session);
        }
      }
    });

    this.onNotification('agent_session/blast_zone_updated', async (data) => {
      if (data.session_id === sessionId && callbacks.onBlastZoneUpdate) {
        const blastZone = await this.getBlastZone(sessionId);
        callbacks.onBlastZoneUpdate(blastZone);
      }
    });

    this.onNotification('agent_session/finding_discovered', async (data) => {
      if (data.session_id === sessionId) {
        if (callbacks.onSessionUpdate) {
          const session = await this.getSessionDetails(sessionId);
          callbacks.onSessionUpdate(session);
        }
      }
    });

    // Fallback polling (less frequent now that we have SSE)
    const interval = setInterval(async () => {
      try {
        const [session, blastZone, timeline] = await Promise.all([
          callbacks.onSessionUpdate ? this.getSessionDetails(sessionId) : Promise.resolve(null),
          callbacks.onBlastZoneUpdate ? this.getBlastZone(sessionId) : Promise.resolve(null),
          callbacks.onTimelineUpdate ? this.getTimeline(sessionId) : Promise.resolve([]),
        ]);

        if (callbacks.onSessionUpdate && session) {
          callbacks.onSessionUpdate(session);
        }
        if (callbacks.onBlastZoneUpdate && blastZone) {
          callbacks.onBlastZoneUpdate(blastZone);
        }
        if (callbacks.onTimelineUpdate) {
          callbacks.onTimelineUpdate(timeline);
        }
      } catch (error) {
        console.error('Polling error:', error);
      }
    }, this.pollInterval);

    // Return cleanup function
    return () => clearInterval(interval);
  }

  // Poll for new sessions (with SSE notifications)
  startSessionsPolling(callback: (sessions: ActiveSessionInfo[]) => void): () => void {
    // Subscribe to new session notifications
    this.onNotification('agent_session/created', async () => {
      const sessions = await this.getActiveSessions();
      callback(sessions);
    });

    this.onNotification('agent_session/completed', async () => {
      const sessions = await this.getActiveSessions();
      callback(sessions);
    });

    // Initial fetch
    this.getActiveSessions().then(callback);

    // Fallback polling (less frequent with SSE)
    const interval = setInterval(async () => {
      const sessions = await this.getActiveSessions();
      callback(sessions);
    }, this.pollInterval);

    return () => clearInterval(interval);
  }
}

