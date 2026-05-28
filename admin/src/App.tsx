import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { hasToken, saveToken } from './lib/api';
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
      // Strip token from URL to avoid leaking it in browser history.
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
          <Route index element={<DashboardPage />} />
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
