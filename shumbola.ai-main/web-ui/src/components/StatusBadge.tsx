import { useTranslation } from 'react-i18next';
import type { EngagementStatus } from '../api/types';

export function StatusBadge({ status }: { status: EngagementStatus }) {
  const { t } = useTranslation();
  return (
    <span className={`status-badge status-badge--${status}`}>
      <span className="status-badge__dot" />
      {t(`status.${status}`)}
    </span>
  );
}
