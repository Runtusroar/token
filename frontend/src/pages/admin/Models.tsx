import { useEffect, useState } from 'react';
import { Table, Button, Modal, Input, Switch, Tag, message, Space } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { adminAPI } from '../../api/admin';

interface Model {
  id: number;
  model_name: string;
  provider: string;
  display_name: string;
  rate: number;
  input_price: number;
  output_price: number;
  enabled: boolean;
}

interface ModelForm {
  model_name: string;
  provider: string;
  display_name: string;
  rate: number;
  input_price: number;
  output_price: number;
}

const emptyForm: ModelForm = {
  model_name: '',
  provider: 'claude',
  display_name: '',
  rate: 1,
  input_price: 0,
  output_price: 0,
};

const labelStyle: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--text-muted)',
  textTransform: 'uppercase',
  letterSpacing: 1,
  marginBottom: 4,
};

export default function Models() {
  const [models, setModels] = useState<Model[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editId, setEditId] = useState<number | null>(null);
  const [form, setForm] = useState<ModelForm>(emptyForm);
  const [saving, setSaving] = useState(false);
  const { t } = useTranslation();

  function fetchModels() {
    setLoading(true);
    adminAPI.listModels()
      .then((res) => {
        const d = res.data.data;
        setModels(Array.isArray(d) ? d : (d?.items ?? []));
      })
      .finally(() => setLoading(false));
  }

  useEffect(() => { fetchModels(); }, []);


  function openCreate() {
    setEditId(null);
    setForm(emptyForm);
    setModalOpen(true);
  }

  function openEdit(m: Model) {
    setEditId(m.id);
    setForm({
      model_name: m.model_name,
      provider: m.provider || 'claude',
      display_name: m.display_name,
      rate: Number(m.rate),
      input_price: Number(m.input_price),
      output_price: Number(m.output_price),
    });
    setModalOpen(true);
  }

  function handleSave() {
    const payload: Record<string, unknown> = {
      ...form,
      rate: Number(form.rate),
      input_price: Number(form.input_price),
      output_price: Number(form.output_price),
    };
    setSaving(true);
    const req = editId
      ? adminAPI.updateModel(editId, payload)
      : adminAPI.createModel(payload);

    req
      .then(() => {
        message.success(editId ? t('models.updateSuccess') : t('models.createSuccess'));
        setModalOpen(false);
        fetchModels();
      })
      .catch(() => message.error(t('models.saveFailed')))
      .finally(() => setSaving(false));
  }

  function handleToggle(m: Model) {
    adminAPI.updateModel(m.id, { enabled: !m.enabled })
      .then(() => fetchModels())
      .catch(() => message.error(t('models.toggleFailed')));
  }

  const columns: ColumnsType<Model> = [
    { title: t('models.modelName'), dataIndex: 'model_name', key: 'model_name' },
    {
      title: t('models.provider'),
      dataIndex: 'provider',
      key: 'provider',
      width: 90,
      render: (v: string) => <Tag color={v === 'claude' ? 'blue' : v === 'openai' ? 'green' : 'orange'}>{v || 'claude'}</Tag>,
    },
    { title: t('models.displayName'), dataIndex: 'display_name', key: 'display_name' },
    {
      title: t('models.rate'),
      dataIndex: 'rate',
      key: 'rate',
      width: 80,
      render: (v: number) => Number(v).toFixed(2),
    },
    {
      title: t('models.inputPrice'),
      dataIndex: 'input_price',
      key: 'input_price',
      width: 110,
      render: (v: number) => `$${Number(v).toFixed(2)}`,
    },
    {
      title: t('models.outputPrice'),
      dataIndex: 'output_price',
      key: 'output_price',
      width: 110,
      render: (v: number) => `$${Number(v).toFixed(2)}`,
    },
    {
      title: t('models.enabled'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 80,
      render: (enabled: boolean, m) => (
        <Switch
          size="small"
          checked={enabled}
          onChange={() => handleToggle(m)}
        />
      ),
    },
    {
      title: t('common.actions'),
      key: 'actions',
      width: 80,
      render: (_, m) => (
        <Space size={6}>
          <Button size="small" onClick={() => openEdit(m)}>{t('common.edit')}</Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ fontSize: 15, fontWeight: 700 }}>{'// ' + t('nav.models')}</h2>
        <Button onClick={openCreate}>{t('models.newModel')}</Button>
      </div>

      <Table<Model>
        columns={columns}
        dataSource={models}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        pagination={false}
      />

      <Modal
        title={editId ? t('models.editModel') : t('models.newModelTitle')}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        confirmLoading={saving}
        okText={t('common.save')}
        width={480}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 12 }}>
          <div>
            <div style={labelStyle}>{t('models.modelName')}</div>
            <Input
              value={form.model_name}
              onChange={(e) => setForm({ ...form, model_name: e.target.value })}
              placeholder="claude-sonnet-4"
            />
          </div>
          <div>
            <div style={labelStyle}>{t('models.provider')}</div>
            <Input
              value={form.provider}
              onChange={(e) => setForm({ ...form, provider: e.target.value })}
              placeholder="claude / openai / deepseek ..."
            />
          </div>
          <div>
            <div style={labelStyle}>{t('models.displayName')}</div>
            <Input
              value={form.display_name}
              onChange={(e) => setForm({ ...form, display_name: e.target.value })}
              placeholder="Claude 3.5 Sonnet"
            />
          </div>
          <div>
            <div style={labelStyle}>{t('models.rateHint')}</div>
            <Input
              type="number"
              value={form.rate}
              onChange={(e) => setForm({ ...form, rate: Number(e.target.value) })}
              step={0.1}
            />
          </div>
          <div style={{ display: 'flex', gap: 12 }}>
            <div style={{ flex: 1 }}>
              <div style={labelStyle}>{t('models.inputPriceHint')}</div>
              <Input
                type="number"
                value={form.input_price}
                onChange={(e) => setForm({ ...form, input_price: Number(e.target.value) })}
                step={0.01}
                placeholder="15"
              />
            </div>
            <div style={{ flex: 1 }}>
              <div style={labelStyle}>{t('models.outputPriceHint')}</div>
              <Input
                type="number"
                value={form.output_price}
                onChange={(e) => setForm({ ...form, output_price: Number(e.target.value) })}
                step={0.01}
                placeholder="75"
              />
            </div>
          </div>
        </div>
      </Modal>
    </div>
  );
}
