// Timeline event types

export interface TimelineEvent {
  timestamp: string;
  type: 'hypothesis' | 'query' | 'finding' | 'investigation';
  data: Record<string, any>;
}

