import { useEffect, useState } from 'react';
import { Table, Tag, Button, Modal, InputNumber, DatePicker, message, Space } from 'antd';
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
  used_at: string | null;
  created_at: string;
  expires_at: string | null;
}

const STATUS_COLORS: Record<string, string> = {
  unused: 'default',
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

export default function RedeemCodes() {
  const [codes, setCodes] = useState<RedeemCode[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [genAmount, setGenAmount] = useState<number>(10);
  const [genCount, setGenCount] = useState<number>(1);
  const [genExpires, setGenExpires] = useState<Dayjs | null>(null);
  const [generating, setGenerating] = useState(false);
  const { t } = useTranslation();

  function fetchCodes() {
    setLoading(true);
    adminAPI.listRedeemCodes()
      .then((res) => {
        const d = res.data.data;
        setCodes(Array.isArray(d) ? d : (d?.list ?? []));
      })
      .finally(() => setLoading(false));
  }

  useEffect(() => { fetchCodes(); }, []);

  function handleGenerate() {
    const payload: Record<string, unknown> = {
      amount: genAmount,
      count: genCount,
    };
    if (genExpires) {
      payload.expires_at = genExpires.toISOString();
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
      width: 100,
      render: (v: number) => `$${v.toFixed(2)}`,
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (s: string) => <Tag color={STATUS_COLORS[s] ?? 'default'}>{s}</Tag>,
    },
    {
      title: 'used_by',
      dataIndex: 'used_by',
      key: 'used_by',
      width: 90,
      render: (v: number | null) => v ?? '-',
    },
    {
      title: 'used_at',
      dataIndex: 'used_at',
      key: 'used_at',
      width: 140,
      render: (v: string | null) => v ? new Date(v).toLocaleString() : '-',
    },
    {
      title: t('apiKeys.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 140,
      render: (v: string) => new Date(v).toLocaleDateString(),
    },
    {
      title: t('billing.expiresAt'),
      dataIndex: 'expires_at',
      key: 'expires_at',
      width: 140,
      render: (v: string | null) => v ? new Date(v).toLocaleDateString() : '-',
    },
    {
      title: t('common.actions'),
      key: 'actions',
      width: 100,
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
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ fontSize: 15, fontWeight: 700 }}>{'// ' + t('nav.redeemCodes')}</h2>
        <Button onClick={() => setModalOpen(true)}>{t('billing.generateCodes')}</Button>
      </div>

      <Table<RedeemCode>
        columns={columns}
        dataSource={codes}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        pagination={{ pageSize: 20 }}
      />

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
