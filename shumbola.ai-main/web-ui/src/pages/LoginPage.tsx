import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { listEngagements } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { LogoIcon } from '../components/icons';

export function LoginPage() {
  const { t } = useTranslation();
  const { login, credentials } = useAuth();
  const navigate = useNavigate();

  const [baseUrl, setBaseUrl] = useState(
    credentials?.baseUrl ?? 'http://localhost:8080',
  );
  const [apiKey, setApiKey] = useState(credentials?.apiKey ?? '');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const creds = { baseUrl: baseUrl.trim(), apiKey: apiKey.trim() };
      await listEngagements(creds); // validates the key/URL before saving
      login(creds);
      navigate('/engagements');
    } catch {
      setError(t('login.error'));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-shell">
      <div className="login-page">
        <div className="login-page__logo">
          <LogoIcon width={26} height={26} />
        </div>
        <h1>{t('app.title')}</h1>
        <p className="tagline">{t('app.tagline')}</p>
        <form onSubmit={handleSubmit} className="login-form">
          <h2>{t('login.title')}</h2>
          <label>
            {t('login.serverUrl')}
            <input
              type="text"
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder="http://localhost:8080"
              required
            />
          </label>
          <label>
            {t('login.apiKey')}
            <input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              required
            />
          </label>
          {error && <p className="error">{error}</p>}
          <button type="submit" disabled={submitting}>
            {submitting ? t('login.submitting') : t('login.submit')}
          </button>
        </form>
      </div>
    </div>
  );
}
