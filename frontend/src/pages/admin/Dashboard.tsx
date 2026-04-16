import { useEffect, useState } from 'react';
import { Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import StatCard from '../../components/StatCard';
import { adminAPI } from '../../api/admin';

interface DashboardData {
  total_users: number;
  today_requests: number;
  today_tokens: number;
  redeem_income: string;
  topup_income: string;
  total_income: string;
  today_consumption: string;
  total_consumption: string;
  today_upstream: string;
  total_upstream: string;
  total_profit: string;
  total_balance: string;
}

interface DailyRow {
  date: string;
  redeem_income: string;
  topup_income: string;
  consumption: string;
  upstream_cost: string;
  profit: string;
  requests: number;
}

const sectionTitle: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--text-muted)',
  textTransform: 'uppercase',
  letterSpacing: 1,
  marginBottom: 8,
};

export default function Dashboard() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [daily, setDaily] = useState<DailyRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [dailyLoading, setDailyLoading] = useState(true);
  const { t } = useTranslation();

  useEffect(() => {
    adminAPI.getDashboard()
      .then((res) => setData(res.data.data))
      .finally(() => setLoading(false));

    adminAPI.getDailyStats(30)
      .then((res) => setDaily(res.data.data ?? []))
      .finally(() => setDailyLoading(false));
  }, []);

  const $ = (val: string | undefined) => `$${Number(val ?? 0).toFixed(2)}`;
  const L = (s: string) => loading ? '...' : s;

  const dailyColumns: ColumnsType<DailyRow> = [
    {
      title: t('dashboard.date'),
      dataIndex: 'date',
      key: 'date',
      width: 110,
      render: (v: string) => v,
    },
    {
      title: t('dashboard.requests'),
      dataIndex: 'requests',
      key: 'requests',
      width: 80,
      align: 'right',
      render: (v: number) => v.toLocaleString(),
    },
    {
      title: t('dashboard.redeemIncome'),
      dataIndex: 'redeem_income',
      key: 'redeem_income',
      width: 110,
      align: 'right',
      render: (v: string) => {
        const n = Number(v);
        return n > 0 ? <span style={{ color: 'var(--accent-green)' }}>+${n.toFixed(2)}</span> : '-';
      },
    },
    {
      title: t('dashboard.topupIncome'),
      dataIndex: 'topup_income',
      key: 'topup_income',
      width: 110,
      align: 'right',
      render: (v: string) => {
        const n = Number(v);
        return n > 0 ? <span style={{ color: 'var(--accent-green)' }}>+${n.toFixed(2)}</span> : '-';
      },
    },
    {
      title: t('dashboard.dailyConsumption'),
      dataIndex: 'consumption',
      key: 'consumption',
      width: 120,
      align: 'right',
      render: (v: string) => `$${Number(v).toFixed(4)}`,
    },
    {
      title: t('dashboard.dailyUpstream'),
      dataIndex: 'upstream_cost',
      key: 'upstream_cost',
      width: 120,
      align: 'right',
      render: (v: string) => <span style={{ color: '#e53e3e' }}>${Number(v).toFixed(4)}</span>,
    },
    {
      title: t('dashboard.dailyProfit'),
      dataIndex: 'profit',
      key: 'profit',
      width: 110,
      align: 'right',
      render: (v: string) => {
        const n = Number(v);
        return <Tag color={n > 0 ? 'success' : n < 0 ? 'error' : 'default'}>${n.toFixed(4)}</Tag>;
      },
    },
  ];

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.dashboard')}</h2>

      {/* Row 1: Overview */}
      <div style={sectionTitle}>{t('dashboard.todayStats')}</div>
      <div className="responsive-grid-4" style={{ marginBottom: 24 }}>
        <StatCard label={t('dashboard.totalUsers')} value={L(String(data?.total_users ?? 0))} color="var(--text-primary)" />
        <StatCard label={t('dashboard.todayRequests')} value={L(String(data?.today_requests ?? 0))} />
        <StatCard label={t('dashboard.todayTokens')} value={L((data?.today_tokens ?? 0).toLocaleString())} />
        <StatCard label={t('dashboard.todayConsumption')} value={L($(data?.today_consumption))} />
      </div>

      {/* Row 2: Income / Expense / Profit */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 16, marginBottom: 24 }}>
        {/* Income */}
        <div style={{ border: '1px solid var(--border-color)', padding: 16, backgroundColor: 'var(--bg-card)' }}>
          <div style={{ ...sectionTitle, color: 'var(--accent-green)', marginBottom: 12 }}>{t('dashboard.income')}</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <DashRow label={t('dashboard.redeemIncome')} value={L($(data?.redeem_income))} color="var(--accent-green)" />
            <DashRow label={t('dashboard.topupIncome')} value={L($(data?.topup_income))} color="var(--accent-green)" />
            <div style={{ borderTop: '1px solid var(--border-color)', paddingTop: 8 }}>
              <DashRow label={t('dashboard.totalIncome')} value={L($(data?.total_income))} color="var(--accent-green)" bold />
            </div>
          </div>
        </div>

        {/* Expense */}
        <div style={{ border: '1px solid var(--border-color)', padding: 16, backgroundColor: 'var(--bg-card)' }}>
          <div style={{ ...sectionTitle, color: '#e53e3e', marginBottom: 12 }}>{t('dashboard.expense')}</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <DashRow label={t('dashboard.todayUpstream')} value={L($(data?.today_upstream))} color="#e53e3e" />
            <DashRow label={t('dashboard.totalUpstream')} value={L($(data?.total_upstream))} color="#e53e3e" />
            <div style={{ borderTop: '1px solid var(--border-color)', paddingTop: 8 }}>
              <DashRow label={t('dashboard.totalConsumption')} value={L($(data?.total_consumption))} color="var(--text-muted)" />
            </div>
          </div>
        </div>

        {/* Profit */}
        <div style={{ border: '1px solid var(--border-color)', padding: 16, backgroundColor: 'var(--bg-card)' }}>
          <div style={{ ...sectionTitle, marginBottom: 12 }}>{t('dashboard.profitSummary')}</div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <DashRow label={t('dashboard.totalProfit')} value={L($(data?.total_profit))} color="var(--accent-green)" bold />
            <DashRow label={t('dashboard.totalBalance')} value={L($(data?.total_balance))} color="var(--text-muted)" />
          </div>
          <div style={{ marginTop: 12, fontSize: 10, color: 'var(--text-muted)', lineHeight: 1.6, whiteSpace: 'pre-line' }}>
            {t('dashboard.profitFormula')}
          </div>
        </div>
      </div>

      {/* Row 3: Daily breakdown table */}
      <div style={sectionTitle}>{t('dashboard.dailyBreakdown')}</div>
      <Table<DailyRow>
        columns={dailyColumns}
        dataSource={daily}
        rowKey="date"
        loading={dailyLoading}
        bordered
        size="small"
        scroll={{ y: 'calc(100vh - 520px)' }}
        pagination={false}
        summary={() => {
          if (!daily.length) return null;
          const sum = (key: keyof DailyRow) => daily.reduce((a, r) => a + Number(r[key] ?? 0), 0);
          return (
            <Table.Summary fixed>
              <Table.Summary.Row>
                <Table.Summary.Cell index={0}><strong>{t('dashboard.total')}</strong></Table.Summary.Cell>
                <Table.Summary.Cell index={1} align="right"><strong>{sum('requests').toLocaleString()}</strong></Table.Summary.Cell>
                <Table.Summary.Cell index={2} align="right"><strong style={{ color: 'var(--accent-green)' }}>${sum('redeem_income').toFixed(2)}</strong></Table.Summary.Cell>
                <Table.Summary.Cell index={3} align="right"><strong style={{ color: 'var(--accent-green)' }}>${sum('topup_income').toFixed(2)}</strong></Table.Summary.Cell>
                <Table.Summary.Cell index={4} align="right"><strong>${sum('consumption').toFixed(4)}</strong></Table.Summary.Cell>
                <Table.Summary.Cell index={5} align="right"><strong style={{ color: '#e53e3e' }}>${sum('upstream_cost').toFixed(4)}</strong></Table.Summary.Cell>
                <Table.Summary.Cell index={6} align="right"><strong style={{ color: 'var(--accent-green)' }}>${sum('profit').toFixed(4)}</strong></Table.Summary.Cell>
              </Table.Summary.Row>
            </Table.Summary>
          );
        }}
      />
    </div>
  );
}

function DashRow({ label, value, color, bold }: { label: string; value: string; color: string; bold?: boolean }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
      <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>{label}</span>
      <span style={{ fontSize: bold ? 18 : 14, fontWeight: bold ? 700 : 500, color, fontFamily: 'var(--font-mono)' }}>
        {value}
      </span>
    </div>
  );
}
