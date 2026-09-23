import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { onSessionExpired } from './lib/api';
import { useCurrentUser } from './hooks/useCurrentUser';
import LoginPage from './pages/LoginPage';
import RegistryPage from './pages/RegistryPage';
import PluginDetailPage from './pages/PluginDetailPage';
import NotFoundPage from './pages/NotFoundPage';
import Layout from './components/Layout';
import DashboardPage from './pages/DashboardPage';
import PluginsPage from './pages/PluginsPage';
import PluginEditPage from './pages/PluginEditPage';
import VersionsPage from './pages/VersionsPage';
import SubmitPage from './pages/SubmitPage';
import SubmissionsPage from './pages/SubmissionsPage';
import AccountPage from './pages/AccountPage';
import CookieConsent from './components/CookieConsent';
import RouteAnnouncer from './components/RouteAnnouncer';

/**
 * Sends the user to the login page when their session ends mid-visit,
 * remembering where they were so they land back there after signing in.
 */
function SessionWatcher() {
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => onSessionExpired(() => {
    const from = `${location.pathname}${location.search}`;
    navigate('/login', { replace: true, state: { expired: true, from } });
  }), [navigate, location.pathname, location.search]);

  return null;
}

function LoadingScreen({ label }: { readonly label: string }) {
  return (
    <div className="centered-status" role="status" aria-live="polite">
      <span className="spinner" aria-hidden="true" />
      <span>{label}</span>
    </div>
  );
}

function RequireAuth({ children }: Readonly<{ children: React.ReactNode }>) {
  const { user, loading } = useCurrentUser();
  const location = useLocation();

  if (loading) return <LoadingScreen label="Checking your session…" />;
  if (!user) {
    return <Navigate to="/login" replace state={{ from: `${location.pathname}${location.search}` }} />;
  }
  return <>{children}</>;
}

function AdminOnly({ children }: Readonly<{ children: React.ReactNode }>) {
  const { user, loading } = useCurrentUser();

  if (loading) return <LoadingScreen label="Checking your permissions…" />;
  if (!user?.isAdmin) return <Navigate to="/admin/plugins" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <CookieConsent />
      <RouteAnnouncer />
      <SessionWatcher />
      <Routes>
        {/* Public */}
        <Route path="/" element={<RegistryPage />} />
        <Route path="/plugins/:name" element={<PluginDetailPage />} />
        <Route path="/login" element={<LoginPage />} />

        {/* Protected admin area */}
        <Route path="/admin" element={<RequireAuth><Layout /></RequireAuth>}>
          <Route index element={<AdminOnly><DashboardPage /></AdminOnly>} />
          <Route path="plugins" element={<PluginsPage />} />
          <Route path="plugins/new" element={<PluginEditPage />} />
          <Route path="plugins/:id" element={<PluginEditPage />} />
          <Route path="plugins/:id/versions" element={<VersionsPage />} />
          <Route path="submit" element={<SubmitPage />} />
          <Route path="account" element={<AccountPage />} />
          <Route path="submissions" element={<AdminOnly><SubmissionsPage /></AdminOnly>} />
        </Route>

        {/* A mistyped plugin link used to redirect to the home page, which
            reads as "this plugin was deleted". Say what actually happened. */}
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </BrowserRouter>
  );
}
