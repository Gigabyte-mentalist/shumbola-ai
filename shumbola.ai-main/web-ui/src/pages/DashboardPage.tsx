import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router-dom';
import { createEngagement, listEngagements } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { PlusIcon } from '../components/icons';
import { StatusBadge } from '../components/StatusBadge';
import { usePolling } from '../hooks/usePolling';

export function DashboardPage() {
  const { t } = useTranslation();
  const { credentials } = useAuth();
  const navigate = useNavigate();
  const creds = credentials!; // ProtectedRoute guarantees this

  const { data: engagements, error } = usePolling(
    () => listEngagements(creds),
    5000,
    true,
    [creds.baseUrl, creds.apiKey],
  );

  const [showForm, setShowForm] = useState(false);
  const [id, setId] = useState('');
  const [targets, setTargets] = useState('');
  const [creating, setCreating] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setCreating(true);
    setFormError(null);
    try {
      const targetList = targets
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean);
      const eng = await createEngagement(creds, id.trim(), targetList);
      navigate(`/engagements/${encodeURIComponent(eng.id)}`);
    } catch (err) {
      setFormError(err instanceof Error ? err.message : String(err));
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="dashboard-page">
      <div className="page-header">
        <h1>{t('dashboard.title')}</h1>
        <button type="button" onClick={() => setShowForm((s) => !s)}>
          <PlusIcon width={15} height={15} />
          {t('dashboard.new')}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} className="card">
          <label>
            {t('dashboard.idLabel')}
            <input
              type="text"
              value={id}
              onChange={(e) => setId(e.target.value)}
              placeholder="eng-2026-08-11"
              required
            />
          </label>
          <label>
            {t('dashboard.targetsLabel')}
            <input
              type="text"
              value={targets}
              onChange={(e) => setTargets(e.target.value)}
              placeholder={t('dashboard.targetsPlaceholder')}
              required
            />
          </label>
          {formError && <p className="error">{formError}</p>}
          <div className="form-actions">
            <button type="submit" disabled={creating}>
              {t('dashboard.create')}
            </button>
            <button
              type="button"
              className="secondary"
              onClick={() => setShowForm(false)}
            >
              {t('common.cancel')}
            </button>
          </div>
        </form>
      )}

      {error && <p className="error">{error}</p>}

      {engagements && engagements.length === 0 && (
        <p className="empty-state">{t('dashboard.empty')}</p>
      )}

      {engagements && engagements.length > 0 && (
        <table className="data-table">
          <thead>
            <tr>
              <th>{t('dashboard.table.id')}</th>
              <th>{t('dashboard.table.status')}</th>
              <th>{t('dashboard.table.scope')}</th>
              <th>{t('dashboard.table.created')}</th>
            </tr>
          </thead>
          <tbody>
            {engagements.map((eng) => (
              <tr key={eng.id}>
                <td>
                  <Link to={`/engagements/${encodeURIComponent(eng.id)}`}>
                    {eng.id}
                  </Link>
                </td>
                <td>
                  <StatusBadge status={eng.status} />
                </td>
                <td className="scope-cell">{eng.scope.targets.join(', ')}</td>
                <td className="time-cell">
                  {new Date(eng.created_at).toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
