import { useEffect, useState } from 'react';
import { Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import StatCard from '../../components/StatCard';
import CodeBlock from '../../components/CodeBlock';
import { userAPI } from '../../api/user';

interface DashboardData {
  balance: string;
  today_requests: number;
  today_tokens: number;
  today_cost: string;
}

interface ApiKey {
  id: number;
  name: string;
  key: string;
  status: string;
  created_at: string;
  last_used_at: string | null;
}

interface DailyRow {
  date: string;
  requests: number;
  total_tokens: number;
  cost: string;
}

function maskKey(key: string): string {
  if (key.length <= 11) return key;
  return key.slice(0, 7) + '****' + key.slice(-4);
}

export default function Overview() {
  const [dashboard, setDashboard] = useState<DashboardData | null>(null);
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [daily, setDaily] = useState<DailyRow[]>([]);
  const [loadingDash, setLoadingDash] = useState(true);
  const [loadingKeys, setLoadingKeys] = useState(true);
  const [loadingDaily, setLoadingDaily] = useState(true);
  const { t } = useTranslation();

  const keyColumns: ColumnsType<ApiKey> = [
    { title: t('apiKeys.name'), dataIndex: 'name', key: 'name' },
    {
      title: t('apiKeys.key'),
      dataIndex: 'key',
      key: 'key',
      render: (k: string) => (
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{maskKey(k)}</span>
      ),
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      key: 'status',
      render: (s: string) => (
        <Tag color={s === 'active' ? 'success' : 'error'}>{s}</Tag>
      ),
    },
    {
      title: t('apiKeys.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      render: (v: string) => new Date(v).toLocaleDateString(),
    },
  ];

  useEffect(() => {
    userAPI.getDashboard()
      .then((res) => setDashboard(res.data.data))
      .finally(() => setLoadingDash(false));

    userAPI.listApiKeys()
      .then((res) => setKeys(res.data.data ?? []))
      .finally(() => setLoadingKeys(false));

    userAPI.getDailyStats(7)
      .then((res) => setDaily(res.data.data ?? []))
      .finally(() => setLoadingDaily(false));
  }, []);

  const claudeCodeSnippet = `# Claude Code setup
export ANTHROPIC_BASE_URL=https://juezhou.org
export ANTHROPIC_API_KEY=your-relay-api-key

# OpenAI SDK (Python)
import openai
client = openai.OpenAI(
    base_url="https://juezhou.org/v1",
    api_key="your-relay-api-key",
)`;

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.overview')}</h2>

      {/* Stat cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 24 }}>
        <StatCard
          label={t('dashboard.currentBalance')}
          value={loadingDash ? '...' : `$${Number(dashboard?.balance ?? 0).toFixed(4)}`}
          color="var(--accent-green)"
        />
        <StatCard
          label={t('overview.todayRequests')}
          value={loadingDash ? '...' : (dashboard?.today_requests ?? 0)}
          color="var(--text-primary)"
        />
        <StatCard
          label={t('overview.todayTokens')}
          value={loadingDash ? '...' : (dashboard?.today_tokens ?? 0).toLocaleString()}
          color="var(--text-primary)"
        />
        <StatCard
          label={t('dashboard.todayCost')}
          value={loadingDash ? '...' : `$${Number(dashboard?.today_cost ?? 0).toFixed(4)}`}
          color="var(--accent-red, #e53e3e)"
        />
      </div>

      {/* Daily consumption */}
      <div style={{ marginBottom: 24 }}>
        <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 8, textTransform: 'uppercase', letterSpacing: 1 }}>
          {t('overview.dailyStats')}
        </div>
        <Table<DailyRow>
          columns={[
            { title: t('overview.date'), dataIndex: 'date', key: 'date', width: 110 },
            {
              title: t('overview.requests'), dataIndex: 'requests', key: 'requests',
              width: 90, align: 'right',
              render: (v: number) => (v ?? 0).toLocaleString(),
            },
            {
              title: t('overview.tokens'), dataIndex: 'total_tokens', key: 'total_tokens',
              width: 110, align: 'right',
              render: (v: number) => (v ?? 0).toLocaleString(),
            },
            {
              title: t('overview.cost'), dataIndex: 'cost', key: 'cost',
              width: 100, align: 'right',
              render: (v: string) => <span style={{ color: '#e53e3e' }}>${Number(v ?? 0).toFixed(4)}</span>,
            },
          ]}
          dataSource={daily}
          rowKey="date"
          loading={loadingDaily}
          bordered
          size="small"
          pagination={false}
        />
      </div>

      {/* Quick start */}
      <div style={{ marginBottom: 24 }}>
        <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 8, textTransform: 'uppercase', letterSpacing: 1 }}>
          {t('overview.quickStart')}
        </div>
        <CodeBlock title="config">{claudeCodeSnippet}</CodeBlock>
      </div>

      {/* API Keys table */}
      <div>
        <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 8, textTransform: 'uppercase', letterSpacing: 1 }}>
          {t('overview.apiKeys')}
        </div>
        <Table<ApiKey>
          columns={keyColumns}
          dataSource={keys}
          rowKey="id"
          loading={loadingKeys}
          bordered
          size="small"
          pagination={false}
        />
      </div>
    </div>
  );
}
