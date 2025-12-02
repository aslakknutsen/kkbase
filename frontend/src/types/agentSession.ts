// TypeScript types matching backend Go types

export interface AgentSession {
  id: string;
  initial_symptom: string;
  initial_resource?: string;
  event_id?: string;
  event_source?: string;
  event_timestamp?: string;
  processing_delay?: number; // nanoseconds from Go
  status: 'active' | 'completed' | 'abandoned' | 'timeout' | 'incomplete';
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
  results?: Record<string, any>[]; // Stored query results (if enabled)
  truncated?: boolean;              // True if results were truncated
  duration: number; // nanoseconds (from Go time.Duration JSON marshaling)
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

export interface Recommendation {
  id: string;
  type: 'root_cause_fix' | 'preventive_action' | 'optimization' | 'monitoring_improvement' | 'cleanup';
  priority: 'critical' | 'high' | 'medium' | 'low';
  title: string;
  description: string;
  rationale: string;
  related_findings: string[]; // Finding IDs
  action_items: string[];
  estimated_effort?: string;
  automation_hint?: string;
  tags?: string[];
  metadata?: Record<string, any>;
  created_at: string;
}

export interface Pattern {
  id: string;
  name: string;
  root_cause_resource_type: string;
  root_cause_issue_type: string;
  investigation_steps: string[];
  diagnosis_guidance: string;
  recommendations: string[];
  bundle_id?: string;
  source: 'discovered' | 'bundled';
  usage_count: number;
  created_at: string;
  metadata?: Record<string, any>;
}

export interface ActiveSessionInfo {
  id: string;
  initial_symptom: string;
  event_id?: string;
  event_source?: string;
  event_timestamp?: string;
  processing_delay?: number; // nanoseconds from Go
  status: string;
  created_at: string;
  completed_at?: string;
  query_count: number;
  finding_count: number;
  current_stage: number;
}

export interface SessionDetail {
  session: AgentSession;
  hypotheses: Hypothesis[];
  queries: QueryExecution[];
  findings: Finding[];
  recommendations: Recommendation[];
  patterns: Pattern[];
  investigations: string[]; // Investigation IDs
  current_hypothesis?: Hypothesis;
}

