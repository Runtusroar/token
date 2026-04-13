import { useState } from 'react';
import { Button, Input, message } from 'antd';
import { Link, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { authAPI } from '../../api/auth';
import { useAuthStore } from '../../store/auth';
import LanguageSwitch from '../../components/LanguageSwitch';

export default function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const login = useAuthStore((s) => s.login);
  const { t } = useTranslation();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const response = await authAPI.login(email, password);
      const { access_token, refresh_token } = response.data.data;
      login(access_token, refresh_token);
      navigate('/user');
    } catch (err: unknown) {
      const error = err as { response?: { data?: { message?: string } } };
      message.error(error?.response?.data?.message ?? t('auth.loginFailed'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        background: 'var(--bg-primary)',
      }}
    >
      <div
        style={{
          border: '1px solid var(--border-color)',
          padding: 32,
          background: '#fff',
          maxWidth: 400,
          width: '100%',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
          <div style={{ fontSize: 18, fontWeight: 700, fontFamily: 'monospace' }}>
            {'// ' + t('auth.login')}
          </div>
          <LanguageSwitch />
        </div>

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: 16 }}>
            <div
              style={{
                fontSize: 11,
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                color: '#666',
                marginBottom: 6,
                fontFamily: 'monospace',
              }}
            >
              {t('auth.email')}
            </div>
            <Input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="user@example.com"
              required
              autoComplete="email"
            />
          </div>

          <div style={{ marginBottom: 24 }}>
            <div
              style={{
                fontSize: 11,
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                color: '#666',
                marginBottom: 6,
                fontFamily: 'monospace',
              }}
            >
              {t('auth.password')}
            </div>
            <Input.Password
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              required
              autoComplete="current-password"
            />
          </div>

          <Button
            type="primary"
            htmlType="submit"
            block
            loading={loading}
            style={{ marginBottom: 12 }}
          >
            {t('auth.login')}
          </Button>
        </form>

        <Button
          block
          href="/api/auth/google"
          style={{ marginBottom: 20 }}
        >
          {t('auth.loginWithGoogle')}
        </Button>

        <div style={{ textAlign: 'center', fontSize: 13, color: '#666' }}>
          {t('auth.noAccount')}{' '}
          <Link to="/register" style={{ fontFamily: 'monospace' }}>
            {t('auth.register')}
          </Link>
        </div>
      </div>
    </div>
  );
}
