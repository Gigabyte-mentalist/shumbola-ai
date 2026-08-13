import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { LogoIcon, LogoutIcon } from './icons';

const LANGUAGES = [
  { code: 'uz-Latn', label: "O'zbek (lotin)" },
  { code: 'uz-Cyrl', label: 'Ўзбек (кирилл)' },
  { code: 'ru', label: 'Русский' },
] as const;

export function Layout({ children }: { children: ReactNode }) {
  const { t, i18n } = useTranslation();
  const { credentials, logout } = useAuth();

  return (
    <div className="app-layout">
      <header className="app-header">
        <Link to="/engagements" className="app-header__brand">
          <LogoIcon width={20} height={20} />
          <span className="app-header__title">{t('app.title')}</span>
        </Link>
        <div className="app-header__controls">
          <select
            value={i18n.language}
            onChange={(e) => i18n.changeLanguage(e.target.value)}
            aria-label="Til / Язык"
          >
            {LANGUAGES.map((lang) => (
              <option key={lang.code} value={lang.code}>
                {lang.label}
              </option>
            ))}
          </select>
          {credentials && (
            <button type="button" onClick={logout} className="link-button">
              <LogoutIcon width={15} height={15} />
              {t('common.logout')}
            </button>
          )}
        </div>
      </header>
      <main className="app-main">{children}</main>
    </div>
  );
}
