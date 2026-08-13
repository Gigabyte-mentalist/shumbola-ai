// Mirrors the Go structs in services/agent-core (engagement.Engagement,
// tasktree.Node, hitl.PendingApproval) — field names match the JSON tags
// exactly so no mapping layer is needed.

export type EngagementStatus =
  | 'created'
  | 'running'
  | 'done'
  | 'stopped'
  | 'error';

export interface Scope {
  targets: string[];
}

export interface Engagement {
  id: string;
  scope: Scope;
  status: EngagementStatus;
  status_note?: string;
  created_at: string;
  created_by: string;
}

export type NodeKind = 'target' | 'step' | 'finding' | 'error';

export interface TaskTreeNode {
  id: string;
  parent_id?: string;
  kind: NodeKind;
  summary: string;
  details?: Record<string, unknown>;
  created_at: string;
}

export interface PendingApproval {
  id: string;
  engagement_id: string;
  description: string;
  created_at: string;
}

export type ReportLang = 'uz-Latn' | 'uz-Cyrl' | 'ru';
