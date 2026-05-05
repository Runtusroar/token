import { useEffect, useState } from 'react';
import {
  Table, Tag, Button, Modal, Input, AutoComplete, Select, message, Space,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { adminAPI } from '../../api/admin';

interface Channel {
  id: number;
  name: string;
  type: string;
  base_url: string;
  models: string[];
  status: string;
  priority: number;
  weight: number;
}

interface ModelOption {
  model_name: string;
  provider: string;
  display_name: string;
}

interface ChannelForm {
  name: string;
  type: string;
  api_key: string;
  base_url: string;
  models: string[];
  priority: number;
  weight: number;
}

const TYPE_COLORS: Record<string, string> = {
  claude: 'blue',
  openai: 'green',
  gemini: 'orange',
  azure: 'purple',
};

const STATUS_COLORS: Record<string, string> = {
  active: 'success',
  disabled: 'default',
  error: 'error',
};

const DEFAULT_PROVIDERS = ['claude', 'openai', 'gemini', 'azure', 'deepseek', 'mistral'];

const BASE_URLS: Record<string, string> = {
  claude: 'https://api.anthropic.com',
  openai: 'https://api.openai.com',
  gemini: 'https://generativelanguage.googleapis.com',
  deepseek: 'https://api.deepseek.com',
  mistral: 'https://api.mistral.ai',
  azure: 'https://<resource>.openai.azure.com/openai/deployments/<deployment>?api-version=2024-10-21',
};

const emptyForm: ChannelForm = {
  name: '',
  type: 'claude',
  api_key: '',
  base_url: BASE_URLS.claude,
  models: [],
  priority: 0,
  weight: 1,
};

const labelStyle: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--text-muted)',
  textTransform: 'uppercase',
  letterSpacing: 1,
  marginBottom: 4,
};

export default function Channels() {
  const [channels, setChannels] = useState<Channel[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<ChannelForm>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [allModels, setAllModels] = useState<ModelOption[]>([]);
  const { t } = useTranslation();

  function fetchChannels() {
    setLoading(true);
    adminAPI.listChannels()
      .then((res) => {
        const d = res.data.data;
        setChannels(Array.isArray(d) ? d : (d?.items ?? []));
      })
      .finally(() => setLoading(false));
  }

  function fetchModels() {
    adminAPI.listModels()
      .then((res) => {
        const d = res.data.data;
        const list: ModelOption[] = Array.isArray(d) ? d : (d?.items ?? []);
        setAllModels(list);
      });
  }

  useEffect(() => { fetchChannels(); fetchModels(); }, []);

  // Build provider options: defaults + any custom ones from existing models.
  const providerOptions = (() => {
    const set = new Set(DEFAULT_PROVIDERS);
    allModels.forEach((m) => { if (m.provider) set.add(m.provider); });
    return [...set].map((p) => ({ value: p, label: p }));
  })();

  function modelsForType(type: string) {
    return allModels.filter((m) => m.provider === type).map((m) => m.model_name);
  }

  function openCreate() {
    const type = emptyForm.type;
    setEditId(null);
    setForm({
      ...emptyForm,
      models: modelsForType(type),
    });
    setModalOpen(true);
  }

  function openEdit(ch: Channel) {
    setEditId(ch.id);
    setForm({
      name: ch.name,
      type: ch.type,
      api_key: '',
      base_url: ch.base_url,
      models: Array.isArray(ch.models) ? ch.models : [],
      priority: ch.priority,
      weight: ch.weight,
    });
    setModalOpen(true);
  }

  function handleSave() {
    const payload: Record<string, unknown> = {
      name: form.name,
      type: form.type,
      base_url: form.base_url,
      models: form.models,
      priority: form.priority,
      weight: form.weight,
    };
    if (form.api_key) payload.api_key = form.api_key;

    setSaving(true);
    const req = editId
      ? adminAPI.updateChannel(editId, payload)
      : adminAPI.createChannel(payload);

    req
      .then(() => {
        message.success(editId ? t('channels.updateSuccess') : t('channels.createSuccess'));
        setModalOpen(false);
        fetchChannels();
      })
      .catch((err: { response?: { data?: { error?: string } } }) => {
        message.error(err?.response?.data?.error ?? t('channels.saveFailed'));
      })
      .finally(() => setSaving(false));
  }

  function handleDelete(id: number) {
    Modal.confirm({
      title: t('channels.deleteConfirm'),
      onOk: () =>
        adminAPI.deleteChannel(id)
          .then(() => { message.success('Deleted'); fetchChannels(); })
          .catch(() => message.error(t('channels.deleteFailed'))),
    });
  }

  function handleTest(id: number) {
    message.loading({ content: t('channels.testing'), key: 'test' });
    adminAPI.testChannel(id)
      .then((res) => {
        const result = res.data?.data;
        if (result?.success) {
          message.success({ content: t('channels.testSuccess'), key: 'test' });
        } else {
          message.error({ content: result?.message ?? t('channels.testFailed'), key: 'test' });
        }
      })
      .catch(() => message.error({ content: t('channels.testFailed'), key: 'test' }));
  }

  const columns: ColumnsType<Channel> = [
    { title: t('channels.name'), dataIndex: 'name', key: 'name' },
    {
      title: t('channels.type'),
      dataIndex: 'type',
      key: 'type',
      width: 90,
      render: (tp: string) => <Tag color={TYPE_COLORS[tp] ?? 'default'}>{tp}</Tag>,
    },
    { title: t('channels.baseUrl'), dataIndex: 'base_url', key: 'base_url' },
    {
      title: t('channels.models'),
      dataIndex: 'models',
      key: 'models',
      render: (ms: string[]) => (
        <span style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
          {(Array.isArray(ms) ? ms : []).map((m) => (
            <Tag key={m} style={{ margin: 0 }}>{m}</Tag>
          ))}
        </span>
      ),
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (s: string) => <Tag color={STATUS_COLORS[s] ?? 'default'}>{s}</Tag>,
    },
    { title: t('channels.priority'), dataIndex: 'priority', key: 'priority', width: 80 },
    { title: t('channels.weight'), dataIndex: 'weight', key: 'weight', width: 80 },
    {
      title: t('common.actions'),
      key: 'actions',
      width: 180,
      render: (_, ch) => (
        <Space size={6}>
          <Button size="small" onClick={() => openEdit(ch)}>{t('common.edit')}</Button>
          <Button size="small" onClick={() => handleTest(ch.id)}>{t('channels.test')}</Button>
          <Button size="small" danger onClick={() => handleDelete(ch.id)}>{t('common.delete')}</Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ fontSize: 15, fontWeight: 700 }}>{'// ' + t('nav.channels')}</h2>
        <Button onClick={openCreate}>{t('channels.newChannel')}</Button>
      </div>

      <Table<Channel>
        columns={columns}
        dataSource={channels}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        pagination={false}
      />

      <Modal
        title={editId ? t('channels.editChannel') : t('channels.newChannelTitle')}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        okText={t('common.save')}
        width={520}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 12 }}>
          <div>
            <div style={labelStyle}>{t('channels.name')}</div>
            <Input
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
            />
          </div>
          <div>
            <div style={labelStyle}>{t('channels.type')}</div>
            <AutoComplete
              value={form.type}
              onChange={(v) => setForm({
                ...form,
                type: v,
                base_url: BASE_URLS[v] ?? form.base_url,
                models: modelsForType(v),
              })}
              style={{ width: '100%' }}
              options={providerOptions}
              placeholder="claude / openai / deepseek ..."
              filterOption={(input, option) => {
                if (providerOptions.some((o) => o.value === input)) return true;
                return (option?.value as string)?.toLowerCase().includes(input.toLowerCase()) ?? false;
              }}
            />
          </div>
          <div>
            <div style={labelStyle}>
              {t('channels.apiKey')} {editId ? t('channels.apiKeyHint') : ''}
            </div>
            <Input.Password
              value={form.api_key}
              onChange={(e) => setForm({ ...form, api_key: e.target.value })}
              placeholder={editId ? '••••••••' : 'sk-...'}
            />
          </div>
          <div>
            <div style={labelStyle}>{t('channels.baseUrl')}</div>
            {form.type === 'azure' && (
              <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 4 }}>
                Azure: include the deployment path and ?api-version=… in Base URL
              </div>
            )}
            <Input
              value={form.base_url}
              onChange={(e) => setForm({ ...form, base_url: e.target.value })}
            />
          </div>
          <div>
            <div style={labelStyle}>{t('channels.modelsHint')}</div>
            <Select
              mode="multiple"
              value={form.models}
              onChange={(v) => setForm({ ...form, models: v })}
              style={{ width: '100%' }}
              placeholder={t('channels.selectModels')}
              options={allModels
                .filter((m) => m.provider === form.type)
                .map((m) => ({
                  value: m.model_name,
                  label: m.display_name || m.model_name,
                }))}
            />
          </div>
          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <div style={labelStyle}>{t('channels.priority')}</div>
              <Input
                type="number"
                value={form.priority}
                onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })}
              />
            </div>
            <div style={{ flex: 1 }}>
              <div style={labelStyle}>{t('channels.weight')}</div>
              <Input
                type="number"
                value={form.weight}
                onChange={(e) => setForm({ ...form, weight: Number(e.target.value) })}
              />
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
}
