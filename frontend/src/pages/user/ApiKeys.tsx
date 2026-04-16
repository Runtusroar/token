import { useEffect, useState } from 'react';
import { Table, Button, Tag, Modal, Input, message, Popconfirm, Space } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { userAPI } from '../../api/user';

interface ApiKey {
  id: number;
  name: string;
  key: string;
  status: string;
  created_at: string;
  last_used_at: string | null;
  request_count: number;
  total_tokens: number;
  total_cost: number;
}

function maskKey(key: string): string {
  if (key.length <= 11) return key;
  return key.slice(0, 7) + '****' + key.slice(-4);
}

export default function ApiKeys() {
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [creating, setCreating] = useState(false);
  const [revealOpen, setRevealOpen] = useState(false);
  const [revealedKey, setRevealedKey] = useState('');
  const [togglingId, setTogglingId] = useState<number | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const { t } = useTranslation();

  function load() {
    setLoading(true);
    userAPI.listApiKeys()
      .then((res) => setKeys(res.data.data ?? []))
      .finally(() => setLoading(false));
  }

  useEffect(() => { load(); }, []);

  async function handleCreate() {
    const name = newKeyName.trim();
    if (!name) {
      message.error(t('apiKeys.nameRequired'));
      return;
    }
    setCreating(true);
    try {
      const res = await userAPI.createApiKey(name);
      const created = res.data.data as ApiKey;
      setCreateOpen(false);
      setNewKeyName('');
      setRevealedKey(created.key);
      setRevealOpen(true);
      load();
    } catch {
      message.error(t('apiKeys.createFailed'));
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(id: number) {
    setDeletingId(id);
    try {
      await userAPI.deleteApiKey(id);
      message.success(t('common.delete'));
      load();
    } catch {
      message.error(t('apiKeys.deleteFailed'));
    } finally {
      setDeletingId(null);
    }
  }

  async function handleToggle(record: ApiKey) {
    setTogglingId(record.id);
    const newStatus = record.status === 'active' ? 'disabled' : 'active';
    try {
      await userAPI.updateApiKey(record.id, { status: newStatus });
      message.success(newStatus === 'active' ? t('common.active') : t('common.disabled'));
      load();
    } catch {
      message.error(t('apiKeys.updateFailed'));
    } finally {
      setTogglingId(null);
    }
  }

  const columns: ColumnsType<ApiKey> = [
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
      render: (v: string) => new Date(v).toLocaleString(),
    },
    {
      title: t('apiKeys.requestCount'),
      dataIndex: 'request_count',
      key: 'request_count',
      width: 80,
      align: 'right',
      render: (v: number) => (v ?? 0).toLocaleString(),
    },
    {
      title: t('apiKeys.totalTokens'),
      dataIndex: 'total_tokens',
      key: 'total_tokens',
      width: 100,
      align: 'right',
      render: (v: number) => (v ?? 0).toLocaleString(),
    },
    {
      title: t('apiKeys.totalCost'),
      dataIndex: 'total_cost',
      key: 'total_cost',
      width: 90,
      align: 'right',
      render: (v: number) => `$${Number(v ?? 0).toFixed(4)}`,
    },
    {
      title: t('apiKeys.lastUsed'),
      dataIndex: 'last_used_at',
      key: 'last_used_at',
      render: (v: string | null) => v ? new Date(v).toLocaleString() : '-',
    },
    {
      title: t('common.actions'),
      key: 'actions',
      render: (_: unknown, record: ApiKey) => (
        <Space size={4}>
          <Button
            size="small"
            loading={togglingId === record.id}
            onClick={() => handleToggle(record)}
          >
            {record.status === 'active' ? t('common.disable') : t('common.enable')}
          </Button>
          <Popconfirm
            title={t('apiKeys.deleteConfirm')}
            onConfirm={() => handleDelete(record.id)}
            okText={t('common.delete')}
            cancelText={t('common.cancel')}
          >
            <Button
              size="small"
              danger
              loading={deletingId === record.id}
            >
              {t('common.delete')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ fontSize: 15, fontWeight: 700, margin: 0 }}>{'// ' + t('nav.apiKeys')}</h2>
        <Button type="primary" size="small" onClick={() => setCreateOpen(true)}>
          {t('apiKeys.newKey')}
        </Button>
      </div>

      <Table<ApiKey>
        columns={columns}
        dataSource={keys}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        pagination={false}
      />

      {/* Create modal */}
      <Modal
        title={t('apiKeys.createTitle')}
        open={createOpen}
        onOk={handleCreate}
        onCancel={() => { setCreateOpen(false); setNewKeyName(''); }}
        okText={t('common.create')}
        confirmLoading={creating}
      >
        <Input
          placeholder={t('apiKeys.keyName')}
          value={newKeyName}
          onChange={(e) => setNewKeyName(e.target.value)}
          onPressEnter={handleCreate}
          style={{ marginTop: 8 }}
        />
      </Modal>

      {/* Reveal modal — show once */}
      <Modal
        title={t('apiKeys.createdSuccess')}
        open={revealOpen}
        onOk={() => setRevealOpen(false)}
        onCancel={() => setRevealOpen(false)}
        cancelButtonProps={{ style: { display: 'none' } }}
        okText={t('common.close')}
      >
        <p style={{ color: 'var(--text-muted)', fontSize: 12, marginBottom: 12 }}>
          {t('apiKeys.saveWarning')}
        </p>
        <div style={{
          backgroundColor: 'var(--bg-code)',
          color: '#e0e0e0',
          fontFamily: 'var(--font-mono)',
          fontSize: 13,
          padding: '12px 16px',
          wordBreak: 'break-all',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 12,
        }}>
          <span id="revealed-key" style={{ userSelect: 'all' }}>{revealedKey}</span>
          <Button
            size="small"
            onClick={() => {
              const el = document.getElementById('revealed-key');
              if (el) {
                const range = document.createRange();
                range.selectNodeContents(el);
                const sel = window.getSelection();
                sel?.removeAllRanges();
                sel?.addRange(range);
                document.execCommand('copy');
                sel?.removeAllRanges();
              }
              message.success(t('common.copied'));
            }}
          >
            {t('common.copy')}
          </Button>
        </div>
      </Modal>
    </div>
  );
}
