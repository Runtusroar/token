import { useEffect, useState } from 'react';
import { Table, Tag, Button, Input, Select } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { adminAPI } from '../../api/admin';

interface ModelOption {
  model_name: string;
  display_name: string;
}

interface Log {
  id: number;
  user_id: number;
  model: string;
  type: string;
  prompt_tokens: number;
  completion_tokens: number;
  cost: number;
  status: string;
  duration_ms: number;
  created_at: string;
}

const STATUS_COLORS: Record<string, string> = {
  success: 'success',
  error: 'error',
  active: 'success',
  disabled: 'default',
};

export default function Logs() {
  const [logs, setLogs] = useState<Log[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const pageSize = 20;
  const { t } = useTranslation();

  const [filterUserId, setFilterUserId] = useState('');
  const [filterModel, setFilterModel] = useState<string | undefined>(undefined);
  const [modelOptions, setModelOptions] = useState<{value: string, label: string}[]>([]);

  function fetchLogs(p = page, userId = filterUserId, model = filterModel) {
    setLoading(true);
    const uid = userId ? Number(userId) : undefined;
    adminAPI.listLogs(p, pageSize, uid, model)
      .then((res) => {
        const d = res.data.data;
        if (Array.isArray(d)) {
          setLogs(d);
          setTotal(d.length);
        } else {
          setLogs(d?.items ?? []);
          setTotal(d?.total ?? 0);
        }
      })
      .finally(() => setLoading(false));
  }

  useEffect(() => { fetchLogs(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    adminAPI.listModels().then((res) => {
      const d = res.data.data;
      const list: ModelOption[] = Array.isArray(d) ? d : (d?.items ?? []);
      setModelOptions(list.map((m) => ({
        value: m.model_name,
        label: m.display_name || m.model_name,
      })));
    });
  }, []);

  function handleSearch() {
    setPage(1);
    fetchLogs(1, filterUserId, filterModel);
  }

  const columns: ColumnsType<Log> = [
    { title: t('logs.userId'), dataIndex: 'user_id', key: 'user_id', width: 80 },
    { title: t('logs.model'), dataIndex: 'model', key: 'model' },
    { title: t('logs.type'), dataIndex: 'type', key: 'type', width: 80 },
    {
      title: t('logs.promptTokens'),
      dataIndex: 'prompt_tokens',
      key: 'prompt_tokens',
      width: 120,
      render: (v: number) => (v ?? 0).toLocaleString(),
    },
    {
      title: t('logs.completionTokens'),
      dataIndex: 'completion_tokens',
      key: 'completion_tokens',
      width: 150,
      render: (v: number) => (v ?? 0).toLocaleString(),
    },
    {
      title: t('logs.cost'),
      dataIndex: 'cost',
      key: 'cost',
      width: 90,
      render: (v: number) => `$${Number(v ?? 0).toFixed(6)}`,
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (s: string) => <Tag color={STATUS_COLORS[s] ?? 'default'}>{s}</Tag>,
    },
    {
      title: t('logs.duration'),
      dataIndex: 'duration_ms',
      key: 'duration_ms',
      width: 110,
      render: (v: number) => `${v ?? 0}ms`,
    },
    {
      title: t('logs.time'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (v: string) => new Date(v).toLocaleString(),
    },
  ];

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.logs')}</h2>

      <div style={{ display: 'flex', gap: 8, marginBottom: 12, alignItems: 'center' }}>
        <Input
          placeholder={t('logs.userId')}
          value={filterUserId}
          onChange={(e) => setFilterUserId(e.target.value)}
          style={{ width: 120 }}
          type="number"
        />
        <Select
          placeholder={t('logs.model')}
          allowClear
          value={filterModel}
          onChange={(v) => setFilterModel(v)}
          style={{ width: 240 }}
          options={modelOptions}
        />
        <Button onClick={handleSearch}>{t('common.search')}</Button>
      </div>

      <Table<Log>
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
          onChange: (p) => { setPage(p); fetchLogs(p); },
        }}
        scroll={{ x: 'max-content' }}
      />
    </div>
  );
}
