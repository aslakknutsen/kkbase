// TypeScript types matching backend Go types

export interface AgentSession {
  id: string;
  initial_symptom: string;
  initial_resource?: string;
  status: 'active' | 'completed' | 'abandoned';
  created_at: string;
  completed_at?: string;
  current_stage: number;
  query_count: number;
  finding_count: number;
  summary?: string;
}

export interface Hypothesis {
  id: string;
  stage: number;
  text: string;
  status: 'active' | 'superseded' | 'confirmed';
  created_at: string;
}

export interface QueryExecution {
  id: string;
  query: string;
  reasoning: string;
  params?: Record<string, any>;
  result_count: number;
  duration: number; // milliseconds
  executed_at: string;
  findings: string[]; // Finding IDs
}

export interface Finding {
  id: string;
  type: string; // 'failed_dependency', 'unhealthy_pod', 'error_spike', 'deployment_change'
  severity: 'critical' | 'warning' | 'info';
  resource_id: string;
  resource_type?: string;
  description: string;
  evidence?: Record<string, any>;
  detection_method: 'automatic' | 'agent_recorded';
  discovered_at: string;
}

export interface ActiveSessionInfo {
  id: string;
  initial_symptom: string;
  created_at: string;
  query_count: number;
  finding_count: number;
  current_stage: number;
}

export interface SessionDetail {
  session: AgentSession;
  hypotheses: Hypothesis[];
  queries: QueryExecution[];
  findings: Finding[];
  investigations: string[]; // Investigation IDs
  current_hypothesis?: Hypothesis;
}

