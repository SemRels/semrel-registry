import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import { hasToken, saveToken } from './lib/api';
import { useCurrentUser } from './hooks/useCurrentUser';
import LoginPage from './pages/LoginPage';
import RegistryPage from './pages/RegistryPage';
import PluginDetailPage from './pages/PluginDetailPage';
import Layout from './components/Layout';
import DashboardPage from './pages/DashboardPage';
import PluginsPage from './pages/PluginsPage';
import PluginEditPage from './pages/PluginEditPage';
import VersionsPage from './pages/VersionsPage';
import SubmitPage from './pages/SubmitPage';
import SubmissionsPage from './pages/SubmissionsPage';

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
    navigate('/admin', { replace: true });
  }, [navigate]);
  return null;
}

function RequireAuth({ children }: { children: React.ReactNode }) {
  const params = new URLSearchParams(window.location.search);
  const urlToken = params.get('token');
  if (urlToken) {
    saveToken(urlToken);
    window.history.replaceState({}, '', window.location.pathname);
  }
  if (!hasToken()) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function AdminOnly({ children }: { children: React.ReactNode }) {
  const user = useCurrentUser();
  if (user === null) return null;
  if (!user.isAdmin) return <Navigate to="/admin/plugins" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* Public */}
        <Route path="/" element={<RegistryPage />} />
        <Route path="/plugins/:name" element={<PluginDetailPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/oauth/callback" element={<OAuthCallback />} />

        {/* Protected admin area */}
        <Route path="/admin" element={<RequireAuth><Layout /></RequireAuth>}>
          <Route index element={<AdminOnly><DashboardPage /></AdminOnly>} />
          <Route path="plugins" element={<PluginsPage />} />
          <Route path="plugins/new" element={<PluginEditPage />} />
          <Route path="plugins/:id" element={<PluginEditPage />} />
          <Route path="plugins/:id/versions" element={<VersionsPage />} />
          <Route path="submit" element={<SubmitPage />} />
          <Route path="submissions" element={<AdminOnly><SubmissionsPage /></AdminOnly>} />
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
