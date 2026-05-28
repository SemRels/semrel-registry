import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { hasToken } from './lib/api';
import LoginPage from './pages/LoginPage';
import Layout from './components/Layout';
import DashboardPage from './pages/DashboardPage';
import PluginsPage from './pages/PluginsPage';
import PluginEditPage from './pages/PluginEditPage';
import VersionsPage from './pages/VersionsPage';

function RequireAuth({ children }: { children: React.ReactNode }) {
  if (!hasToken()) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/"
          element={
            <RequireAuth>
              <Layout />
            </RequireAuth>
          }
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
