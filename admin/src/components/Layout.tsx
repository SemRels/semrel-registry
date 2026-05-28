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
        <a href="http://localhost:3000" className="sidebar__brand" target="_blank" rel="noopener">
          <img src="/semrel.svg" alt="semrel" style={{ width:'1.5rem', height:'1.5rem', flexShrink:0 }} />
          <span>semrel Registry</span>
        </a>
        <nav className="sidebar__nav">
          {isAdmin && (
            <NavLink to="/" end className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
              Dashboard
            </NavLink>
          )}
          <NavLink to="/plugins" className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
            {isAdmin ? 'Plugins' : 'My Plugins'}
          </NavLink>
          <a href="http://localhost:3000" className="sidebar__link" target="_blank" rel="noopener">
            Registry ↗
          </a>
          <a href="http://localhost:8080/api/v1/plugins" className="sidebar__link" target="_blank" rel="noopener">
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
