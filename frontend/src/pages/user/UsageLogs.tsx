import { useEffect, useState } from 'react';
import { Table } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { userAPI } from '../../api/user';

interface LogEntry {
  id: number;
  model: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost: number;
  duration_ms: number;
  created_at: string;
}

export default function UsageLogs() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const { t } = useTranslation();

  const columns: ColumnsType<LogEntry> = [
    { title: t('logs.model'), dataIndex: 'model', key: 'model' },
    {
      title: t('logs.promptTokens'),
      dataIndex: 'prompt_tokens',
      key: 'prompt_tokens',
      align: 'right',
      render: (v: number) => v.toLocaleString(),
    },
    {
      title: t('logs.completionTokens'),
      dataIndex: 'completion_tokens',
      key: 'completion_tokens',
      align: 'right',
      render: (v: number) => v.toLocaleString(),
    },
    {
      title: t('logs.totalTokens'),
      dataIndex: 'total_tokens',
      key: 'total_tokens',
      align: 'right',
      render: (v: number) => v.toLocaleString(),
    },
    {
      title: t('logs.cost'),
      dataIndex: 'cost',
      key: 'cost',
      align: 'right',
      render: (v: number) => (
        <span style={{ color: 'var(--accent-red)' }}>${Number(v).toFixed(6)}</span>
      ),
    },
    {
      title: t('logs.duration'),
      dataIndex: 'duration_ms',
      key: 'duration_ms',
      align: 'right',
      render: (v: number) => `${v}ms`,
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
    userAPI.listLogs(p, ps)
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
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.usageLogs')}</h2>

      <Table<LogEntry>
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
