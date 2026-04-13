import { useEffect } from 'react';
import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '../store/auth';
import { userAPI } from '../api/user';
import LanguageSwitch from '../components/LanguageSwitch';

const menuPaths = [
  { key: 'dashboard',    path: '/admin/dashboard' },
  { key: 'users',        path: '/admin/users' },
  { key: 'channels',     path: '/admin/channels' },
  { key: 'models',       path: '/admin/models' },
  { key: 'redeemCodes',  path: '/admin/redeem' },
  { key: 'logs',         path: '/admin/logs' },
  { key: 'settings',     path: '/admin/settings' },
];

export default function AdminLayout() {
  const { user, setUser, logout } = useAuthStore();
  const navigate = useNavigate();
  const { t } = useTranslation();

  useEffect(() => {
    userAPI.getProfile()
      .then((res) => setUser(res.data))
      .catch(() => {/* token expired → interceptor will redirect */});
  }, [setUser]);

  function handleLogout() {
    logout();
    navigate('/login');
  }

  return (
    <div style={{
      display: 'flex',
      minHeight: '100vh',
      fontFamily: 'var(--font-mono)',
      backgroundColor: 'var(--bg-primary)',
      color: 'var(--text-primary)',
    }}>
      {/* Sidebar */}
      <aside style={{
        width: 200,
        minWidth: 200,
        backgroundColor: 'var(--bg-sidebar)',
        borderRight: '1px solid var(--border-color)',
        display: 'flex',
        flexDirection: 'column',
      }}>
        {/* Brand */}
        <div style={{
          padding: '20px 16px 16px',
          borderBottom: '1px solid var(--border-color)',
        }}>
          <div style={{ fontWeight: 700, fontSize: 14, letterSpacing: 1 }}>AI_RELAY</div>
          <div style={{ color: 'var(--text-muted)', fontSize: 11, marginTop: 2 }}>admin@root</div>
        </div>

        {/* Menu */}
        <nav style={{ flex: 1, paddingTop: 8 }}>
          {menuPaths.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              style={({ isActive }) => ({
                display: 'block',
                padding: '7px 16px',
                textDecoration: 'none',
                fontSize: 12,
                color: isActive ? 'var(--text-primary)' : 'var(--text-secondary)',
                fontWeight: isActive ? 700 : 400,
                backgroundColor: isActive ? '#f0f0ea' : 'transparent',
                borderLeft: isActive
                  ? '3px solid var(--text-primary)'
                  : '3px solid transparent',
              })}
            >
              {t(`nav.${item.key}`)}
            </NavLink>
          ))}
        </nav>
      </aside>

      {/* Main content */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        {/* Top bar */}
        <header style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'flex-end',
          gap: 12,
          padding: '8px 24px',
          borderBottom: '1px solid var(--border-color)',
          backgroundColor: 'var(--bg-sidebar)',
          fontSize: 12,
        }}>
          <span style={{ color: 'var(--text-muted)' }}>
            {user?.email ?? '...'}
          </span>
          <LanguageSwitch />
          <button
            onClick={handleLogout}
            style={{
              background: 'none',
              border: '1px solid var(--border-color)',
              cursor: 'pointer',
              fontFamily: 'var(--font-mono)',
              fontSize: 11,
              padding: '2px 10px',
              color: 'var(--text-primary)',
            }}
          >
            {t('auth.logout')}
          </button>
        </header>

        {/* Page content */}
        <main style={{ flex: 1, padding: 24, minWidth: 0 }}>
          <Outlet />
        </main>
      </div>
    </div>
  );
}
