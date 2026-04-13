import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import theme from './styles/theme';
import { useAuthStore } from './store/auth';

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
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />;
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const user = useAuthStore((s) => s.user);
  return user?.role === 'admin' ? <>{children}</> : <Navigate to="/user/overview" replace />;
}

function App() {
  return (
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
            <Route path="keys" element={<ApiKeys />} />
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
  );
}

export default App;
