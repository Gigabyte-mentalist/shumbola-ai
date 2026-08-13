import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { PendingApproval } from '../api/types';
import { AlertTriangleIcon, CheckIcon, XIcon } from './icons';

export function ApprovalsPanel({
  approvals,
  onDecide,
}: {
  approvals: PendingApproval[];
  onDecide: (approvalId: string, approved: boolean) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [pending, setPending] = useState<string | null>(null);

  if (approvals.length === 0) {
    return <p className="empty-state">{t('engagement.noApprovals')}</p>;
  }

  async function decide(id: string, approved: boolean) {
    setPending(id);
    try {
      await onDecide(id, approved);
    } finally {
      setPending(null);
    }
  }

  return (
    <ul className="approvals-panel">
      {approvals.map((a) => (
        <li key={a.id} className="approvals-panel__item">
          <span className="approvals-panel__icon">
            <AlertTriangleIcon width={18} height={18} />
          </span>
          <div className="approvals-panel__body">
            <p className="approvals-panel__desc">{a.description}</p>
            <div className="approvals-panel__actions">
              <button
                type="button"
                disabled={pending === a.id}
                onClick={() => decide(a.id, true)}
              >
                <CheckIcon width={14} height={14} />
                {t('engagement.approve')}
              </button>
              <button
                type="button"
                className="danger"
                disabled={pending === a.id}
                onClick={() => decide(a.id, false)}
              >
                <XIcon width={14} height={14} />
                {t('engagement.reject')}
              </button>
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}
