// Blast zone graph types

export interface BlastZoneNode {
  id: string;
  label: string;
  type: string;
  status: 'failed' | 'degraded' | 'healthy';
  properties?: Record<string, any>;
}

export interface BlastZoneEdge {
  source: string;
  target: string;
  type: string;
  status: 'failing' | 'ok';
  properties?: Record<string, any>;
}

export interface BlastZoneSnapshot {
  session_id: string;
  timestamp: string;
  trigger_event: string;
  nodes: BlastZoneNode[];
  edges: BlastZoneEdge[];
  impact_radius: number;
  affected_count: number;
}

