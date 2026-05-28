import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { clearToken } from '../lib/api';

export default function Layout() {
  const navigate = useNavigate();
  return (
    <div className="app">
      <aside className="sidebar">
        <a href="http://localhost:3000" className="sidebar__brand" target="_blank" rel="noopener">
          <img src="/semrel.svg" alt="semrel" style={{ width:'1.5rem', height:'1.5rem', flexShrink:0 }} />
          <span>semrel Registry</span>
        </a>
        <nav className="sidebar__nav">
          <NavLink to="/" end className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
            Dashboard
          </NavLink>
          <NavLink to="/plugins" className={({ isActive }) => `sidebar__link${isActive ? ' active' : ''}`}>
            Plugins
          </NavLink>
          <a href="http://localhost:3000" className="sidebar__link" target="_blank" rel="noopener">
            Registry ↗
          </a>
          <a href="http://localhost:8080/api/v1/plugins" className="sidebar__link" target="_blank" rel="noopener">
            Raw API ↗
          </a>
        </nav>
        <div className="sidebar__footer">
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
