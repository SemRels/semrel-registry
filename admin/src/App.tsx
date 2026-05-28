import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { hasToken, saveToken } from './lib/api';
import { useCurrentUser } from './hooks/useCurrentUser';
import LoginPage from './pages/LoginPage';
import Layout from './components/Layout';
import DashboardPage from './pages/DashboardPage';
import PluginsPage from './pages/PluginsPage';
import PluginEditPage from './pages/PluginEditPage';
import VersionsPage from './pages/VersionsPage';

/** Handles ?token= injected by the GitHub OAuth callback redirect. */
function OAuthCallback() {
  const navigate = useNavigate();
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const token = params.get('token');
    if (token) {
      saveToken(token);
      window.history.replaceState({}, '', window.location.pathname);
    }
    navigate('/', { replace: true });
  }, [navigate]);
  return null;
}

function RequireAuth({ children }: { children: React.ReactNode }) {
  if (!hasToken()) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

/** Redirects non-admin users away from admin-only routes. */
function AdminOnly({ children }: { children: React.ReactNode }) {
  const user = useCurrentUser();
  if (user === null) return null; // still loading
  if (!user.isAdmin) return <Navigate to="/plugins" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/oauth/callback" element={<OAuthCallback />} />
        <Route
          path="/"
          element={<RequireAuth><Layout /></RequireAuth>}
        >
          {/* Dashboard: admin-only, redirect community users to /plugins */}
          <Route index element={<AdminOnly><DashboardPage /></AdminOnly>} />
          <Route path="plugins" element={<PluginsPage />} />
          <Route path="plugins/new" element={<PluginEditPage />} />
          <Route path="plugins/:id" element={<PluginEditPage />} />
          <Route path="plugins/:id/versions" element={<VersionsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
