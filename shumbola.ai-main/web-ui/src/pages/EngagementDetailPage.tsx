import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router-dom';
import {
  decideApproval,
  downloadReport,
  getEngagement,
  getTaskTree,
  listApprovals,
  startEngagement,
  stopEngagement,
} from '../api/client';
import type { ReportLang } from '../api/types';
import { useAuth } from '../auth/AuthContext';
import { ApprovalsPanel } from '../components/ApprovalsPanel';
import {
  ArrowLeftIcon,
  DownloadIcon,
  FileTextIcon,
  PlayIcon,
  StopIcon,
} from '../components/icons';
import { StatusBadge } from '../components/StatusBadge';
import { TaskTreeView } from '../components/TaskTreeView';
import { usePolling } from '../hooks/usePolling';

const REPORT_LANGS: ReportLang[] = ['uz-Latn', 'uz-Cyrl', 'ru'];

export function EngagementDetailPage() {
  const { t, i18n } = useTranslation();
  const { credentials } = useAuth();
  const creds = credentials!; // ProtectedRoute guarantees this
  const { id } = useParams<{ id: string }>();
  const engagementId = id!;

  const {
    data: engagement,
    error: engagementError,
    refetch: refetchEngagement,
  } = usePolling(
    () => getEngagement(creds, engagementId),
    4000,
    true,
    [engagementId],
  );
  const isRunning = engagement?.status === 'running';

  const { data: taskTree } = usePolling(
    () => getTaskTree(creds, engagementId),
    4000,
    isRunning || (engagement?.status !== undefined && engagement.status !== 'created'),
    [engagementId],
  );

  const { data: approvals, refetch: refetchApprovals } = usePolling(
    () => listApprovals(creds, engagementId),
    4000,
    isRunning,
    [engagementId],
  );

  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [reportLang, setReportLang] = useState<ReportLang>(
    (i18n.language as ReportLang) || 'uz-Latn',
  );
  const [downloading, setDownloading] = useState(false);

  async function handleStart() {
    setBusy(true);
    setActionError(null);
    try {
      await startEngagement(creds, engagementId);
      refetchEngagement();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleStop() {
    setBusy(true);
    setActionError(null);
    try {
      const { stopped } = await stopEngagement(creds, engagementId);
      if (!stopped) {
        setActionError(t('engagement.stopFailed'));
      }
      refetchEngagement();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function handleDecide(approvalId: string, approved: boolean) {
    await decideApproval(creds, engagementId, approvalId, approved);
    refetchApprovals();
  }

  async function handleDownloadReport() {
    setDownloading(true);
    setActionError(null);
    try {
      await downloadReport(creds, engagementId, reportLang);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err));
    } finally {
      setDownloading(false);
    }
  }

  if (engagementError) {
    return <p className="error">{engagementError}</p>;
  }
  if (!engagement) {
    return <p className="loading-state">{t('common.loading')}</p>;
  }

  return (
    <div className="engagement-detail-page">
      <Link to="/engagements" className="back-link">
        <ArrowLeftIcon width={14} height={14} />
        {t('engagement.back')}
      </Link>

      <div className="page-header">
        <h1>{engagement.id}</h1>
        <StatusBadge status={engagement.status} />
      </div>

      <div className="card">
        <div className="summary-grid">
          <div className="summary-field">
            <span className="summary-field__label">{t('engagement.scope')}</span>
            <span className="summary-field__value mono">
              {engagement.scope.targets.join(', ')}
            </span>
          </div>
          <div className="summary-field">
            <span className="summary-field__label">{t('engagement.createdAt')}</span>
            <span className="summary-field__value">
              {new Date(engagement.created_at).toLocaleString()}
            </span>
          </div>
          <div className="summary-field">
            <span className="summary-field__label">{t('engagement.createdBy')}</span>
            <span className="summary-field__value mono">{engagement.created_by}</span>
          </div>
          {engagement.status_note && (
            <div className="summary-field">
              <span className="summary-field__label">{t('engagement.statusNote')}</span>
              <span className="summary-field__value">{engagement.status_note}</span>
            </div>
          )}
        </div>

        {actionError && <p className="error">{actionError}</p>}

        <div className="form-actions">
          {engagement.status === 'created' && (
            <button type="button" disabled={busy} onClick={handleStart}>
              <PlayIcon width={14} height={14} />
              {t('engagement.start')}
            </button>
          )}
          {engagement.status === 'running' && (
            <button
              type="button"
              className="danger"
              disabled={busy}
              onClick={handleStop}
            >
              <StopIcon width={14} height={14} />
              {t('engagement.stop')}
            </button>
          )}
        </div>
      </div>

      {approvals && approvals.length > 0 && (
        <section className="card">
          <div className="card__header">
            <h2>{t('engagement.approvals')}</h2>
            <span className="card__count">{approvals.length}</span>
          </div>
          <ApprovalsPanel approvals={approvals} onDecide={handleDecide} />
        </section>
      )}

      <section className="card">
        <div className="card__header">
          <h2>{t('engagement.taskTree')}</h2>
          {taskTree && taskTree.length > 0 && (
            <span className="card__count">{taskTree.length}</span>
          )}
        </div>
        <TaskTreeView nodes={taskTree ?? []} />
      </section>

      <section className="card">
        <div className="card__header">
          <FileTextIcon width={16} height={16} />
          <h2>{t('engagement.report')}</h2>
        </div>
        <div className="report-controls">
          <label>
            {t('engagement.reportLang')}
            <select
              value={reportLang}
              onChange={(e) => setReportLang(e.target.value as ReportLang)}
            >
              {REPORT_LANGS.map((lang) => (
                <option key={lang} value={lang}>
                  {lang}
                </option>
              ))}
            </select>
          </label>
          <button type="button" disabled={downloading} onClick={handleDownloadReport}>
            <DownloadIcon width={14} height={14} />
            {t('engagement.downloadReport')}
          </button>
        </div>
      </section>
    </div>
  );
}
