import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { clearToken } from '../lib/api';
import { useCurrentUser } from '../hooks/useCurrentUser';

export default function Layout() {
  const navigate = useNavigate();
  const user     = useCurrentUser();
  const isAdmin  = user?.isAdmin ?? false;

  return (
    <div className="app">
      <aside className="sidebar">
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
        <Outlet />
      </main>
    </div>
  );
}
