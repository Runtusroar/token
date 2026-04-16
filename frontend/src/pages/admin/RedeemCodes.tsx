import { useEffect, useMemo, useState } from 'react';
import { Table, Tag, Button, Modal, InputNumber, DatePicker, Input, message, Space } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { Dayjs } from 'dayjs';
import { useTranslation } from 'react-i18next';
import { adminAPI } from '../../api/admin';

interface RedeemCode {
  id: number;
  code: string;
  amount: number;
  status: string;
  used_by: number | null;
  used_by_email: string;
  used_at: string | null;
  created_at: string;
  expires_at: string | null;
}

const STATUS_COLORS: Record<string, string> = {
  unused: 'processing',
  used: 'success',
  disabled: 'default',
};

const labelStyle: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--text-muted)',
  textTransform: 'uppercase',
  letterSpacing: 1,
  marginBottom: 4,
};

type StatusFilter = 'all' | 'unused' | 'used' | 'disabled';

export default function RedeemCodes() {
  const [codes, setCodes] = useState<RedeemCode[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [genAmount, setGenAmount] = useState<number>(10);
  const [genCount, setGenCount] = useState<number>(1);
  const [genExpires, setGenExpires] = useState<Dayjs | null>(null);
  const [generating, setGenerating] = useState(false);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [search, setSearch] = useState('');
  const { t } = useTranslation();

  function fetchCodes() {
    setLoading(true);
    const pageSize = 100;
    const loadPage = (p: number, acc: RedeemCode[]): Promise<RedeemCode[]> =>
      adminAPI.listRedeemCodes(p, pageSize).then((res) => {
        const d = res.data.data;
        const items: RedeemCode[] = Array.isArray(d) ? d : (d?.items ?? []);
        const all = [...acc, ...items];
        const total = d?.total ?? items.length;
        if (all.length >= total || items.length < pageSize) return all;
        return loadPage(p + 1, all);
      });

    loadPage(1, [])
      .then(setCodes)
      .finally(() => setLoading(false));
  }

  useEffect(() => { fetchCodes(); }, []);

  // Client-side filter by status + search keyword.
  const filtered = useMemo(() => {
    let result = codes;
    if (statusFilter !== 'all') {
      result = result.filter((c) => c.status === statusFilter);
    }
    if (search.trim()) {
      const q = search.trim().toLowerCase();
      result = result.filter((c) =>
        c.code.toLowerCase().includes(q) ||
        (c.used_by_email && c.used_by_email.toLowerCase().includes(q))
      );
    }
    return result;
  }, [codes, statusFilter, search]);

  // Counts per status for the filter tabs.
  const counts = useMemo(() => {
    const m: Record<string, number> = { all: codes.length, unused: 0, used: 0, disabled: 0 };
    codes.forEach((c) => { m[c.status] = (m[c.status] ?? 0) + 1; });
    return m;
  }, [codes]);

  function handleGenerate() {
    const payload: Record<string, unknown> = {
      amount: genAmount,
      count: genCount,
    };
    if (genExpires) {
      payload.expires_at = genExpires.format('YYYY-MM-DD');
    }
    setGenerating(true);
    adminAPI.createRedeemCodes(payload)
      .then(() => {
        message.success(t('billing.generatedSuccess', { count: genCount }));
        setModalOpen(false);
        fetchCodes();
      })
      .catch(() => message.error(t('billing.generationFailed')))
      .finally(() => setGenerating(false));
  }

  function handleDisable(id: number) {
    adminAPI.updateRedeemCode(id, { status: 'disabled' })
      .then(() => { message.success(t('common.disabled')); fetchCodes(); })
      .catch(() => message.error('Failed'));
  }

  const tabStyle = (active: boolean): React.CSSProperties => ({
    padding: '4px 12px',
    fontSize: 12,
    fontFamily: 'var(--font-mono)',
    border: '1px solid var(--border-color)',
    background: active ? 'var(--text-primary)' : 'transparent',
    color: active ? '#fff' : 'var(--text-secondary)',
    cursor: 'pointer',
  });

  const columns: ColumnsType<RedeemCode> = [
    {
      title: t('billing.redeemCode'),
      dataIndex: 'code',
      key: 'code',
      render: (c: string) => (
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 12 }}>{c}</span>
      ),
    },
    {
      title: t('billing.amount'),
      dataIndex: 'amount',
      key: 'amount',
      width: 90,
      render: (v: number) => `$${Number(v).toFixed(2)}`,
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (s: string) => <Tag color={STATUS_COLORS[s] ?? 'default'}>{s}</Tag>,
    },
    {
      title: t('billing.usedBy'),
      dataIndex: 'used_by_email',
      key: 'used_by_email',
      width: 180,
      render: (v: string) => v || '-',
    },
    {
      title: t('billing.usedAt'),
      dataIndex: 'used_at',
      key: 'used_at',
      width: 140,
      render: (v: string | null) => v ? new Date(v).toLocaleString() : '-',
    },
    {
      title: t('apiKeys.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 130,
      render: (v: string) => new Date(v).toLocaleDateString(),
    },
    {
      title: t('billing.expiresAt'),
      dataIndex: 'expires_at',
      key: 'expires_at',
      width: 130,
      render: (v: string | null) => v ? new Date(v).toLocaleDateString() : '-',
    },
    {
      title: t('common.actions'),
      key: 'actions',
      width: 80,
      render: (_, code) => (
        <Space size={6}>
          {code.status === 'unused' && (
            <Button size="small" danger onClick={() => handleDisable(code.id)}>
              {t('common.disable')}
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
        <h2 style={{ fontSize: 15, fontWeight: 700, margin: 0 }}>{'// ' + t('nav.redeemCodes')}</h2>
        <Button onClick={() => setModalOpen(true)}>{t('billing.generateCodes')}</Button>
      </div>

      {/* Filter bar */}
      <div style={{ display: 'flex', gap: 8, marginBottom: 12, alignItems: 'center', flexWrap: 'wrap' }}>
        {/* Status tabs */}
        <div style={{ display: 'flex', gap: 0 }}>
          {(['all', 'unused', 'used', 'disabled'] as StatusFilter[]).map((s) => (
            <button
              key={s}
              style={tabStyle(statusFilter === s)}
              onClick={() => setStatusFilter(s)}
            >
              {s === 'all' ? t('common.all') : s} ({counts[s] ?? 0})
            </button>
          ))}
        </div>

        {/* Search */}
        <Input
          placeholder={t('billing.searchCode')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          allowClear
          style={{ width: 240 }}
        />
      </div>

      {/* Table with scroll */}
      <div style={{ flex: 1, minHeight: 0 }}>
        <Table<RedeemCode>
          columns={columns}
          dataSource={filtered}
          rowKey="id"
          loading={loading}
          bordered
          size="small"
          scroll={{ y: 'calc(100vh - 300px)' }}
          pagination={{ pageSize: 20, showTotal: (total) => `${total} ${t('common.items')}` }}
        />
      </div>

      <Modal
        title={t('billing.generateTitle')}
        open={modalOpen}
        onOk={handleGenerate}
        onCancel={() => setModalOpen(false)}
        confirmLoading={generating}
        okText={t('billing.generate')}
        width={400}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 12 }}>
          <div>
            <div style={labelStyle}>{t('billing.amountPerCode')}</div>
            <InputNumber
              min={0.01}
              step={1}
              value={genAmount}
              onChange={(v) => setGenAmount(v ?? 10)}
              style={{ width: '100%' }}
            />
          </div>
          <div>
            <div style={labelStyle}>{t('billing.count')}</div>
            <InputNumber
              min={1}
              max={100}
              value={genCount}
              onChange={(v) => setGenCount(v ?? 1)}
              style={{ width: '100%' }}
            />
          </div>
          <div>
            <div style={labelStyle}>{t('billing.expiresAt')}</div>
            <DatePicker
              value={genExpires}
              onChange={(d) => setGenExpires(d)}
              style={{ width: '100%' }}
              placeholder={t('billing.noExpiry')}
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
