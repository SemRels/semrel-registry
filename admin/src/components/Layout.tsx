import { useEffect, useRef, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { signOut } from '../lib/api';
import { useCurrentUser } from '../hooks/useCurrentUser';
import { useFocusTrap } from '../hooks/useFocusTrap';
import LegalLinks from './LegalLinks';
import ThemeToggle from './ThemeToggle';

export default function Layout() {
  const navigate = useNavigate();
  const location = useLocation();
  const { user } = useCurrentUser();
  const isAdmin = user?.isAdmin ?? false;
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [isMobile, setIsMobile] = useState(() => globalThis.matchMedia?.('(max-width: 768px)').matches ?? false);
  const sidebarRef = useRef<HTMLElement>(null);

  // The sidebar is only a dialog on small screens, where it overlays the page.
  // On desktop it is permanent navigation and must not trap focus.
  useEffect(() => {
    const query = globalThis.matchMedia?.('(max-width: 768px)');
    if (!query) return;
    const onChange = (event: MediaQueryListEvent) => setIsMobile(event.matches);
    query.addEventListener('change', onChange);
    return () => query.removeEventListener('change', onChange);
  }, []);

  const drawerOpen = sidebarOpen && isMobile;
  useFocusTrap(sidebarRef, drawerOpen);

  useEffect(() => {
    setSidebarOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    if (!drawerOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setSidebarOpen(false);
    };
    globalThis.addEventListener('keydown', onKeyDown);
    return () => globalThis.removeEventListener('keydown', onKeyDown);
  }, [drawerOpen]);

  async function handleSignOut() {
    await signOut();
    navigate('/login');
  }

  const dialogProps = drawerOpen
    ? { role: 'dialog', 'aria-modal': true, 'aria-label': 'Navigation' }
    : {};

  return (
    <div className="app">
      <a className="skip-link" href="#main-content">Skip to main content</a>

      {/* Rendered only while open. A permanently present backdrop marked
          aria-hidden was still in the tab order — the combination assistive
          technology reports as an error. */}
      {drawerOpen && (
        <button
          type="button"
          className="sidebar-backdrop open"
          aria-label="Close navigation"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <aside
        id="admin-navigation"
        ref={sidebarRef}
        className={`sidebar${sidebarOpen ? ' open' : ''}`}
        {...dialogProps}
      >
        <NavLink to="/" className="sidebar__brand">
          <img src="/semrel.svg" alt="" aria-hidden="true" />
          <span>semrel Registry</span>
        </NavLink>
        <nav className="sidebar__nav" aria-label="Main">
          {isAdmin && (
            <NavLink to="/admin" end className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
              Dashboard
            </NavLink>
          )}
          {isAdmin && (
            <NavLink to="/admin/submissions" className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
              Submissions
            </NavLink>
          )}
          <NavLink to="/admin/plugins" className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
            {isAdmin ? 'Plugins' : 'My Plugins'}
          </NavLink>
          <NavLink to="/admin/account" className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
            Account
          </NavLink>
          {!isAdmin && (
            <NavLink to="/admin/submit" className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
              + Submit Plugin
            </NavLink>
          )}

          <div className="sidebar__section-label">Registry</div>
          <NavLink to="/" className="sidebar__link">
            Public Registry
          </NavLink>
          <a href="/api/v1/plugins" className="sidebar__link" target="_blank" rel="noopener noreferrer">
            Raw API <ExternalHint />
          </a>
          <a href="/schemas/core/v1.json" className="sidebar__link" target="_blank" rel="noopener noreferrer">
            Config Schema <ExternalHint />
          </a>

          <div className="sidebar__section-label">Resources</div>
          <a href="https://semrel.io/" className="sidebar__link" target="_blank" rel="noopener noreferrer">
            Docs <ExternalHint />
          </a>
          <a href="https://semrel.io/guide/configuration/" className="sidebar__link" target="_blank" rel="noopener noreferrer">
            Configuration <ExternalHint />
          </a>
          <a href="https://github.com/SemRels" className="sidebar__link" target="_blank" rel="noopener noreferrer">
            GitHub <ExternalHint />
          </a>
        </nav>
        <div className="sidebar__footer">
          {user && (
            <div className="sidebar__user">
              {user.avatarUrl && (
                <img src={user.avatarUrl} alt="" aria-hidden="true" className="sidebar__user-avatar" />
              )}
              <div className="sidebar__user-info">
                <div className="sidebar__user-login">
                  <span className="sr-only">Signed in as </span>{user.login}
                </div>
                <div className="sidebar__user-role">
                  <span className={isAdmin ? 'sidebar__user-role--admin' : ''}>
                    <span aria-hidden="true">{isAdmin ? '★ ' : ''}</span>{isAdmin ? 'admin' : 'community'}
                  </span>
                </div>
              </div>
            </div>
          )}
          <div className="sidebar__theme-row">
            <ThemeToggle />
          </div>
          <button className="sidebar__logout" type="button" onClick={() => { void handleSignOut(); }}>
            Sign out
          </button>

          {/* The terms notice used to be a full paragraph permanently taking up
              the bottom of the navigation. It is reference text, not something
              anyone reads twice, so it sits behind a disclosure — still one
              keystroke away, and still in the DOM for screen readers. */}
          <details className="sidebar__legal">
            <summary>Terms &amp; attribution</summary>
            <p className="legal-note">
              Changes are attributed to your signed-in account. By using this
              workspace you agree to the semrel terms for maintainers and
              contributors.
            </p>
            <LegalLinks inline className="legal-note__links" linkClassName="muted" />
          </details>
        </div>
      </aside>
      <main className="page" id="main-content" tabIndex={-1}>
        <div className="topbar">
          <button
            type="button"
            className="sidebar__toggle"
            onClick={() => setSidebarOpen((open) => !open)}
            aria-label="Toggle navigation"
            aria-controls="admin-navigation"
            aria-expanded={sidebarOpen}
          >
            <span aria-hidden="true">☰</span>
          </button>
          <NavLink to="/admin" className="topbar__brand">
            <img src="/semrel.svg" alt="" aria-hidden="true" />
            <span>semrel Registry</span>
          </NavLink>
        </div>
        <Outlet />
      </main>
    </div>
  );
}

/** Marks a link as leaving the app, for sighted and screen-reader users alike. */
function ExternalHint() {
  return (
    <>
      <span aria-hidden="true">↗</span>
      <span className="sr-only">(opens in a new tab)</span>
    </>
  );
}
