import { useEffect, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { clearToken } from '../lib/api';
import { useCurrentUser } from '../hooks/useCurrentUser';

export default function Layout() {
  const navigate = useNavigate();
  const location = useLocation();
  const user     = useCurrentUser();
  const isAdmin  = user?.isAdmin ?? false;
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    setSidebarOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    if (!sidebarOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setSidebarOpen(false);
    };
    globalThis.addEventListener('keydown', onKeyDown);
    return () => globalThis.removeEventListener('keydown', onKeyDown);
  }, [sidebarOpen]);

  return (
    <div className="app">
      <button
        type="button"
        className={`sidebar-backdrop${sidebarOpen ? ' open' : ''}`}
        aria-label="Close navigation"
        aria-hidden={!sidebarOpen}
        onClick={() => setSidebarOpen(false)}
      />

      <aside id="admin-navigation" className={`sidebar${sidebarOpen ? ' open' : ''}`}>
        <NavLink to="/" className="sidebar__brand">
          <img src="/semrel.svg" alt="semrel" style={{ width:'1.5rem', height:'1.5rem', flexShrink:0 }} />
          <span>semrel Registry</span>
        </NavLink>
        <nav className="sidebar__nav">
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
          {!isAdmin && (
            <NavLink to="/admin/submit" className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
              + Submit Plugin
            </NavLink>
          )}
          <NavLink to="/" className="sidebar__link">
            Public Registry ↗
          </NavLink>
          <a href="/api/v1/plugins" className="sidebar__link" target="_blank" rel="noopener">
            Raw API ↗
          </a>
        </nav>
        <div className="sidebar__footer">
          {user && (
            <div style={{ display:'flex', alignItems:'center', gap:'.5rem', marginBottom:'.5rem', fontSize:'var(--fs-sm)', color:'var(--muted)', overflow:'hidden' }}>
              {user.avatarUrl && (
                <img src={user.avatarUrl} alt={user.login} style={{ width:'1.5rem', height:'1.5rem', borderRadius:'50%', flexShrink:0 }} />
              )}
              <div style={{ overflow:'hidden' }}>
                <div style={{ color:'var(--fg)', overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap' }}>{user.login}</div>
                <div style={{ fontSize:'var(--fs-xs)' }}>
                  <span style={{ color: isAdmin ? 'var(--accent)' : 'var(--muted)' }}>
                    {isAdmin ? '★ admin' : 'community'}
                  </span>
                </div>
              </div>
            </div>
          )}
          <button className="sidebar__logout" type="button" onClick={() => { clearToken(); navigate('/login'); }}>
            Sign out
          </button>
        </div>
      </aside>
      <main className="page">
        <div className="topbar">
          <button
            type="button"
            className="sidebar__toggle"
            onClick={() => setSidebarOpen((open) => !open)}
            aria-label="Toggle navigation"
            aria-controls="admin-navigation"
            aria-expanded={sidebarOpen}
          >
            ☰
          </button>
          <NavLink to="/admin" className="topbar__brand">
            <img src="/semrel.svg" alt="semrel" style={{ width:'1.25rem', height:'1.25rem', flexShrink:0 }} />
            <span>semrel Registry</span>
          </NavLink>
        </div>
        <Outlet />
      </main>
    </div>
  );
}
