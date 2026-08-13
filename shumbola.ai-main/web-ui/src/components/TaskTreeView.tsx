import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import type { NodeKind, TaskTreeNode } from '../api/types';
import { AlertTriangleIcon, CrosshairIcon, TerminalIcon, XCircleIcon } from './icons';

function detailString(node: TaskTreeNode, key: string): string {
  const v = node.details?.[key];
  return typeof v === 'string' ? v : '';
}

function chipClass(value: string): string {
  const v = value.toLowerCase();
  if (v === 'critical' || v === 'high') return 'chip chip--critical';
  if (v === 'medium') return 'chip chip--medium';
  if (v === 'low') return 'chip chip--low';
  if (v === 'true') return 'chip chip--true';
  if (v === 'false') return 'chip chip--false';
  return 'chip';
}

const NODE_ICON: Record<NodeKind, typeof CrosshairIcon> = {
  target: CrosshairIcon,
  step: TerminalIcon,
  finding: AlertTriangleIcon,
  error: XCircleIcon,
};

interface TreeEntry {
  node: TaskTreeNode;
  depth: number;
}

function flattenTree(nodes: TaskTreeNode[]): TreeEntry[] {
  const byParent = new Map<string, TaskTreeNode[]>();
  const known = new Set(nodes.map((n) => n.id));
  const roots: TaskTreeNode[] = [];

  for (const node of nodes) {
    if (node.parent_id && known.has(node.parent_id)) {
      const siblings = byParent.get(node.parent_id) ?? [];
      siblings.push(node);
      byParent.set(node.parent_id, siblings);
    } else {
      roots.push(node);
    }
  }

  const out: TreeEntry[] = [];
  function visit(list: TaskTreeNode[], depth: number) {
    for (const node of list) {
      out.push({ node, depth });
      visit(byParent.get(node.id) ?? [], depth + 1);
    }
  }
  visit(roots, 0);
  return out;
}

export function TaskTreeView({ nodes }: { nodes: TaskTreeNode[] }) {
  const { t } = useTranslation();
  const entries = useMemo(() => flattenTree(nodes), [nodes]);

  if (nodes.length === 0) {
    return <p className="empty-state">{t('engagement.noTaskTree')}</p>;
  }

  return (
    <ul className="task-tree">
      {entries.map(({ node, depth }) => {
        const Icon = NODE_ICON[node.kind];
        return (
          <li
            key={node.id}
            className={`task-tree__node task-tree__node--${node.kind}`}
            style={{ marginLeft: depth * 20 }}
          >
            <span className="task-tree__icon">
              <Icon width={13} height={13} />
            </span>
            <div className="task-tree__body">
              <div className="task-tree__summary">{node.summary}</div>
              {node.kind === 'finding' && (
                <div className="task-tree__details">
                  {detailString(node, 'target') && (
                    <span className="chip">{detailString(node, 'target')}</span>
                  )}
                  {detailString(node, 'tool') && (
                    <span className="chip">{detailString(node, 'tool')}</span>
                  )}
                  {detailString(node, 'severity') && (
                    <span className={chipClass(detailString(node, 'severity'))}>
                      {t('engagement.severity')}: {detailString(node, 'severity')}
                    </span>
                  )}
                  {detailString(node, 'confidence') && (
                    <span className="chip">
                      {t('engagement.confidence')}: {detailString(node, 'confidence')}
                    </span>
                  )}
                  {node.details?.proven !== undefined && (
                    <span className={chipClass(String(node.details.proven))}>
                      {t('engagement.proven')}: {String(node.details.proven)}
                    </span>
                  )}
                </div>
              )}
            </div>
            <div className="task-tree__time">
              {new Date(node.created_at).toLocaleTimeString()}
            </div>
          </li>
        );
      })}
    </ul>
  );
}
