import { useEffect, useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import theme from './styles/theme';
import { useAuthStore } from './store/auth';
import { userAPI } from './api/user';
import ErrorBoundary from './components/ErrorBoundary';

import Login from './pages/auth/Login';
import Register from './pages/auth/Register';

import UserLayout from './layouts/UserLayout';
import Overview from './pages/user/Overview';
import ApiKeys from './pages/user/ApiKeys';
import UsageLogs from './pages/user/UsageLogs';
import TopUp from './pages/user/TopUp';
import Balance from './pages/user/Balance';
import Docs from './pages/user/Docs';
import UserSettings from './pages/user/Settings';

import AdminLayout from './layouts/AdminLayout';
import Dashboard from './pages/admin/Dashboard';
import Users from './pages/admin/Users';
import Channels from './pages/admin/Channels';
import Models from './pages/admin/Models';
import RedeemCodes from './pages/admin/RedeemCodes';
import Logs from './pages/admin/Logs';
import AdminSettings from './pages/admin/Settings';

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, setUser, user } = useAuthStore();
  const [loading, setLoading] = useState(!user && isAuthenticated);

  useEffect(() => {
    if (isAuthenticated && !user) {
      userAPI.getProfile()
        .then((res) => setUser(res.data.data ?? res.data))
        .catch(() => {/* interceptor handles 401 */})
        .finally(() => setLoading(false));
    }
  }, [isAuthenticated, user, setUser]);

  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (loading) return null;
  return <>{children}</>;
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const user = useAuthStore((s) => s.user);
  if (!user) return null;
  return user.role === 'admin' ? <>{children}</> : <Navigate to="/user/overview" replace />;
}

function App() {
  return (
    <ErrorBoundary>
    <ConfigProvider theme={theme}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />

          <Route
            path="/user/*"
            element={
              <PrivateRoute>
                <UserLayout />
              </PrivateRoute>
            }
          >
            <Route path="overview" element={<Overview />} />
            <Route path="api-keys" element={<ApiKeys />} />
            <Route path="logs" element={<UsageLogs />} />
            <Route path="topup" element={<TopUp />} />
            <Route path="balance" element={<Balance />} />
            <Route path="docs" element={<Docs />} />
            <Route path="settings" element={<UserSettings />} />
            <Route index element={<Navigate to="overview" replace />} />
          </Route>

          <Route
            path="/admin/*"
            element={
              <PrivateRoute>
                <AdminRoute>
                  <AdminLayout />
                </AdminRoute>
              </PrivateRoute>
            }
          >
            <Route path="dashboard" element={<Dashboard />} />
            <Route path="users" element={<Users />} />
            <Route path="channels" element={<Channels />} />
            <Route path="models" element={<Models />} />
            <Route path="redeem" element={<RedeemCodes />} />
            <Route path="logs" element={<Logs />} />
            <Route path="settings" element={<AdminSettings />} />
            <Route index element={<Navigate to="dashboard" replace />} />
          </Route>

          <Route path="*" element={<Navigate to="/login" replace />} />
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
    </ErrorBoundary>
  );
}

export default App;
