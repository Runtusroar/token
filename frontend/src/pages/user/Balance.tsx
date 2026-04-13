import { useEffect, useState } from 'react';
import { Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { userAPI } from '../../api/user';

interface BalanceLog {
  id: number;
  type: string;
  amount: number;
  balance_after: number;
  description: string;
  created_at: string;
}

function typeColor(type: string): string {
  switch (type) {
    case 'recharge':
    case 'redeem':
    case 'admin_add':
      return 'success';
    case 'consume':
    case 'deduct':
      return 'error';
    default:
      return 'default';
  }
}

export default function Balance() {
  const [logs, setLogs] = useState<BalanceLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const { t } = useTranslation();

  const columns: ColumnsType<BalanceLog> = [
    {
      title: t('logs.type'),
      dataIndex: 'type',
      key: 'type',
      render: (tp: string) => (
        <Tag color={typeColor(tp)}>{tp}</Tag>
      ),
    },
    {
      title: t('billing.amount'),
      dataIndex: 'amount',
      key: 'amount',
      align: 'right',
      render: (v: number) => (
        <span style={{ color: v >= 0 ? 'var(--accent-green)' : 'var(--accent-red)', fontWeight: 600 }}>
          {v >= 0 ? '+' : ''}{v.toFixed(6)}
        </span>
      ),
    },
    {
      title: t('logs.balanceAfter'),
      dataIndex: 'balance_after',
      key: 'balance_after',
      align: 'right',
      render: (v: number) => `$${v.toFixed(4)}`,
    },
    {
      title: t('logs.description'),
      dataIndex: 'description',
      key: 'description',
      render: (v: string) => v || '-',
    },
    {
      title: t('logs.time'),
      dataIndex: 'created_at',
      key: 'created_at',
      render: (v: string) => new Date(v).toLocaleString(),
    },
  ];

  function load(p: number, ps: number) {
    setLoading(true);
    userAPI.listBalanceLogs(p, ps)
      .then((res) => {
        const d = res.data.data;
        setLogs(d.data ?? []);
        setTotal(d.total ?? 0);
      })
      .finally(() => setLoading(false));
  }

  useEffect(() => { load(page, pageSize); }, [page, pageSize]);

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.balance')}</h2>

      <Table<BalanceLog>
        columns={columns}
        dataSource={logs}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          pageSizeOptions: ['10', '20', '50'],
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
          },
        }}
      />
    </div>
  );
}
