import { useEffect, useState } from 'react';
import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '../store/auth';
import { userAPI } from '../api/user';
import LanguageSwitch from '../components/LanguageSwitch';
import TimezoneLabel from '../components/TimezoneLabel';

const menuPaths = [
  { key: 'overview',   path: '/user/overview' },
  { key: 'apiKeys',    path: '/user/api-keys' },
  { key: 'usageLogs',  path: '/user/logs' },
  { key: 'topUp',      path: '/user/topup' },
  { key: 'balance',    path: '/user/balance' },
  { key: 'docs',       path: '/user/docs' },
  { key: 'settings',   path: '/user/settings' },
];

export default function UserLayout() {
  const { user, setUser, logout } = useAuthStore();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    userAPI.getProfile()
      .then((res) => setUser(res.data.data ?? res.data))
      .catch(() => {/* token expired → interceptor will redirect */});
  }, [setUser]);

  function handleLogout() {
    logout();
    navigate('/login');
  }

  return (
    <div style={{
      display: 'flex',
      height: '100vh',
      fontFamily: 'var(--font-mono)',
      backgroundColor: 'var(--bg-primary)',
      color: 'var(--text-primary)',
      overflow: 'hidden',
    }}>
      {/* Sidebar overlay (mobile) */}
      {sidebarOpen && <div className="sidebar-overlay" onClick={() => setSidebarOpen(false)} />}

      {/* Sidebar */}
      <aside className={sidebarOpen ? 'sidebar sidebar--open' : 'sidebar'} style={{
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
          <div style={{ color: 'var(--text-muted)', fontSize: 11, marginTop: 2 }}>user@panel</div>
        </div>

        {/* Menu */}
        <nav style={{ flex: 1, paddingTop: 8 }}>
          {menuPaths.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              onClick={() => setSidebarOpen(false)}
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

        {/* Sidebar footer (visible on mobile) */}
        <div className="sidebar-footer" style={{
          borderTop: '1px solid var(--border-color)',
          padding: '12px 16px',
          fontSize: 11,
          display: 'flex',
          flexDirection: 'column',
          gap: 8,
        }}>
          <span style={{ color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {user?.email ?? '...'}
          </span>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            <TimezoneLabel />
            <LanguageSwitch />
            {user?.role === 'admin' && (
              <NavLink
                to="/admin"
                onClick={() => setSidebarOpen(false)}
                style={{
                  background: 'none',
                  border: '1px solid var(--border-color)',
                  padding: '2px 10px',
                  fontFamily: 'var(--font-mono)',
                  fontSize: 11,
                  color: 'var(--accent-green)',
                  textDecoration: 'none',
                }}
              >
                admin
              </NavLink>
            )}
          </div>
        </div>
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
          <button
            className="mobile-menu-btn"
            onClick={() => setSidebarOpen(true)}
            style={{
              display: 'none',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'none',
              border: '1px solid var(--border-color)',
              cursor: 'pointer',
              fontFamily: 'var(--font-mono)',
              fontSize: 18,
              padding: '2px 8px',
              color: 'var(--text-primary)',
              marginRight: 'auto',
            }}
          >
            &#9776;
          </button>
          <span className="hide-mobile" style={{ color: 'var(--text-muted)' }}>
            {user?.email ?? '...'}
          </span>
          <span className="hide-mobile"><TimezoneLabel /></span>
          {user?.role === 'admin' && (
            <NavLink
              className="hide-mobile"
              to="/admin"
              style={{
                background: 'none',
                border: '1px solid var(--border-color)',
                padding: '2px 10px',
                fontFamily: 'var(--font-mono)',
                fontSize: 11,
                color: 'var(--accent-green)',
                textDecoration: 'none',
              }}
            >
              admin
            </NavLink>
          )}
          <span className="hide-mobile"><LanguageSwitch /></span>
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
        <main style={{ flex: 1, padding: 24, minWidth: 0, overflow: 'auto' }}>
          <Outlet />
        </main>
      </div>
    </div>
  );
}
