import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { clearToken } from '../lib/api';

export default function Layout() {
  const navigate = useNavigate();

  function handleLogout() {
    clearToken();
    navigate('/login');
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a href="/" className="sidebar__brand">
          <span className="brand-mark">SR</span>
          <span>
            <strong>registry.semrel.io</strong>
            <small>Admin Panel</small>
          </span>
        </a>

        <span className="sidebar__section">Registry</span>
        <NavLink to="/" end className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
          📊 Dashboard
        </NavLink>
        <NavLink to="/plugins" className={({ isActive }) => `nav-item ${isActive ? 'active' : ''}`}>
          🔌 Plugins
        </NavLink>

        <span className="sidebar__section" style={{ marginTop: 'auto' }}>Account</span>
        <a
          href="http://localhost:3000"
          target="_blank"
          rel="noopener"
          className="nav-item"
        >
          ↗ Open registry
        </a>
        <button type="button" className="nav-item" onClick={handleLogout}>
          🔓 Logout
        </button>
      </aside>

      <main className="main-content">
        <Outlet />
      </main>
    </div>
  );
}
