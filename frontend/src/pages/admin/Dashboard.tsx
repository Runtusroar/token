import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import StatCard from '../../components/StatCard';
import { adminAPI } from '../../api/admin';

interface DashboardData {
  total_users: number;
  today_requests: number;
  today_tokens: number;
  today_revenue: number;
}

export default function Dashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const { t } = useTranslation();

  useEffect(() => {
    adminAPI.getDashboard()
      .then((res) => setData(res.data.data))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.dashboard')}</h2>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12 }}>
        <StatCard
          label={t('dashboard.totalUsers')}
          value={loading ? '...' : (data?.total_users ?? 0)}
          color="var(--text-primary)"
        />
        <StatCard
          label={t('dashboard.todayRequests')}
          value={loading ? '...' : (data?.today_requests ?? 0)}
          color="var(--accent-green)"
        />
        <StatCard
          label={t('dashboard.todayTokens')}
          value={loading ? '...' : (data?.today_tokens ?? 0).toLocaleString()}
          color="var(--accent-green)"
        />
        <StatCard
          label={t('dashboard.todayRevenue')}
          value={loading ? '...' : `$${Number(data?.today_revenue ?? 0).toFixed(4)}`}
          color="var(--accent-green)"
        />
      </div>
    </div>
  );
}
