// MCP Observer Service for read-only dashboard access
// Uses MCP SDK for tool calls and SSE for push notifications

import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StreamableHTTPClientTransport } from '@modelcontextprotocol/sdk/client/streamableHttp.js';
import type { ActiveSessionInfo, SessionDetail } from '../types/agentSession';
import type { BlastZoneSnapshot } from '../types/blastZone';
import type { TimelineEvent } from '../types/timeline';

export class MCPObserver {
  private mcpClient: Client;
  private transport: StreamableHTTPClientTransport;
  private eventsURL: string;
  private eventSource: EventSource | null = null;
  private pollInterval: number = 10000; // 10 seconds (fallback only)
  private notificationHandlers: Map<string, ((data: any) => void)[]> = new Map();
  private isConnected: boolean = false;
  private isConnecting: boolean = false;
  private connectPromise: Promise<void> | null = null;

  constructor(baseURL: string = '/mcp', eventsURL: string = '/events') {
    // Create absolute URL from relative path
    const mcpUrl = new URL(baseURL, window.location.origin);
    
    this.transport = new StreamableHTTPClientTransport(mcpUrl);
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
    // If already connected, return immediately
    if (this.isConnected) {
      return;
    }

    // If already connecting, return the existing promise to prevent double connection
    if (this.isConnecting && this.connectPromise) {
      return this.connectPromise;
    }

    // Set connecting flag and create promise
    this.isConnecting = true;
    this.connectPromise = (async () => {
      try {
        await this.mcpClient.connect(this.transport);
        this.isConnected = true;
        console.log('MCP client connected');
      } catch (error) {
        console.error('Failed to connect MCP client:', error);
        throw error;
      } finally {
        this.isConnecting = false;
      }
    })();

    return this.connectPromise;
  }

  // Disconnect MCP client
  async disconnect(): Promise<void> {
    if (this.isConnected) {
      await this.mcpClient.close();
      this.isConnected = false;
      this.isConnecting = false;
      this.connectPromise = null;
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

      console.log(`Tool ${toolName} raw result:`, result);

      // Priority 1: Check for structuredContent (direct JSON object from Go handler's second return value)
      // Note: SDK expects structuredContent to be an object, not an array
      if (result.structuredContent && typeof result.structuredContent === 'object') {
        console.log(`Tool ${toolName} returned structuredContent:`, result.structuredContent);
        return result.structuredContent as T;
      }

      // Priority 2: Parse content array for text content
      if (result.content && Array.isArray(result.content) && result.content.length > 0) {
        const textContent = result.content.find((item: any) => item.type === 'text');
        if (textContent && textContent.text) {
          console.log(`Tool ${toolName} text content:`, textContent.text.substring(0, 200));
          try {
            // The backend may include descriptive text before JSON
            // Try to extract JSON array or object from the text
            const text = textContent.text;
            
            // Look for JSON array or object in the text
            const jsonMatch = text.match(/(\[[\s\S]*\]|\{[\s\S]*\})/);
            if (jsonMatch) {
              const parsed = JSON.parse(jsonMatch[1]);
              console.log(`Tool ${toolName} parsed from text:`, parsed);
              return parsed as T;
            }
            
            // Try to parse the whole text as JSON
            const parsed = JSON.parse(text);
            console.log(`Tool ${toolName} parsed whole text:`, parsed);
            return parsed as T;
          } catch (parseError) {
            // If not JSON, return the text as-is
            console.warn(`Tool ${toolName} returned non-JSON text content:`, parseError);
            return textContent.text as T;
          }
        }
      }

      // Fallback: return the whole result
      console.warn(`Tool ${toolName} returned unexpected format, using raw result:`, result);
      return result as T;
    } catch (error: any) {
      console.error(`MCP tool call failed (${toolName}):`, error);
      throw new Error(`MCP tool call failed: ${error.message || error}`);
    }
  }

  // Get list of active sessions
  async getActiveSessions(): Promise<ActiveSessionInfo[]> {
    try {
      const result = await this.callTool<any>('get_active_sessions', {});
      // Backend wraps array in object: { sessions: [...] }
      if (result && typeof result === 'object' && 'sessions' in result) {
        return result.sessions || [];
      }
      // Fallback for direct array (backwards compatibility)
      return Array.isArray(result) ? result : [];
    } catch (error) {
      console.error('Failed to get active sessions:', error);
      return [];
    }
  }

  // Get complete session details
  async getSessionDetails(sessionId: string): Promise<SessionDetail | null> {
    try {
      console.log('Fetching session details for:', sessionId);
      const result = await this.callTool<SessionDetail>('get_session_details', {
        session_id: sessionId,
      });
      console.log('Session details received:', result);
      return result;
    } catch (error) {
      console.error('Failed to get session details:', error);
      return null;
    }
  }

  // Get blast zone for a session
  async getBlastZone(sessionId: string): Promise<BlastZoneSnapshot | null> {
    try {
      console.log('Fetching blast zone for:', sessionId);
      const result = await this.callTool<BlastZoneSnapshot>('get_blast_zone', {
        session_id: sessionId,
      });
      console.log('Blast zone received:', result);
      return result;
    } catch (error) {
      console.error('Failed to get blast zone:', error);
      return null;
    }
  }

  // Get timeline for a session
  async getTimeline(sessionId: string): Promise<TimelineEvent[]> {
    try {
      console.log('Fetching timeline for:', sessionId);
      const result = await this.callTool<any>('get_session_timeline', {
        session_id: sessionId,
      });
      console.log('Timeline result:', result);
      
      // Backend wraps array in object: { events: [...] }
      if (result && typeof result === 'object' && 'events' in result) {
        const events = result.events || [];
        console.log('Timeline events extracted:', events);
        return Array.isArray(events) ? events : [];
      }
      // Fallback for direct array (backwards compatibility)
      const events = Array.isArray(result) ? result : [];
      console.log('Timeline fallback:', events);
      return events;
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

