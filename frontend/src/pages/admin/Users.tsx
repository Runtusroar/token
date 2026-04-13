import { useEffect, useState } from 'react';
import { Table, Tag, Button, Input, Modal, InputNumber, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { adminAPI } from '../../api/admin';

interface User {
  id: number;
  email: string;
  role: string;
  balance: number;
  status: string;
  created_at: string;
}

export default function Users() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const pageSize = 20;
  const { t } = useTranslation();

  const [topUpModal, setTopUpModal] = useState(false);
  const [topUpUser, setTopUpUser] = useState<User | null>(null);
  const [topUpAmount, setTopUpAmount] = useState<number>(10);
  const [topUpLoading, setTopUpLoading] = useState(false);

  function fetchUsers(p = page, s = search) {
    setLoading(true);
    adminAPI.listUsers(p, pageSize, s || undefined)
      .then((res) => {
        const d = res.data.data;
        setUsers(Array.isArray(d) ? d : (d?.list ?? []));
        setTotal(d?.total ?? (Array.isArray(d) ? d.length : 0));
      })
      .finally(() => setLoading(false));
  }

  useEffect(() => { fetchUsers(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  function handleSearch() {
    setPage(1);
    fetchUsers(1, search);
  }

  function handleBanToggle(user: User) {
    const newStatus = user.status === 'active' ? 'disabled' : 'active';
    adminAPI.updateUser(user.id, { status: newStatus })
      .then(() => {
        message.success(newStatus === 'active' ? t('users.unbanSuccess') : t('users.banSuccess'));
        fetchUsers();
      })
      .catch(() => message.error(t('users.operationFailed')));
  }

  function handleTopUpOpen(user: User) {
    setTopUpUser(user);
    setTopUpAmount(10);
    setTopUpModal(true);
  }

  function handleTopUpConfirm() {
    if (!topUpUser) return;
    setTopUpLoading(true);
    adminAPI.topUp(topUpUser.id, topUpAmount)
      .then(() => {
        message.success(t('users.topUpSuccess'));
        setTopUpModal(false);
        fetchUsers();
      })
      .catch(() => message.error(t('users.topUpFailed')))
      .finally(() => setTopUpLoading(false));
  }

  const columns: ColumnsType<User> = [
    { title: t('users.id'), dataIndex: 'id', key: 'id', width: 60 },
    { title: t('users.email'), dataIndex: 'email', key: 'email' },
    {
      title: t('users.role'),
      dataIndex: 'role',
      key: 'role',
      width: 80,
      render: (r: string) => (
        <Tag color={r === 'admin' ? 'purple' : 'default'}>{r}</Tag>
      ),
    },
    {
      title: t('users.balance'),
      dataIndex: 'balance',
      key: 'balance',
      width: 100,
      render: (v: number) => `$${(v ?? 0).toFixed(4)}`,
    },
    {
      title: t('common.status'),
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (s: string) => {
        const color = s === 'active' ? 'success' : 'default';
        return <Tag color={color}>{s}</Tag>;
      },
    },
    {
      title: t('users.createdAt'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 140,
      render: (v: string) => new Date(v).toLocaleDateString(),
    },
    {
      title: t('common.actions'),
      key: 'actions',
      width: 180,
      render: (_, user) => (
        <span style={{ display: 'flex', gap: 6 }}>
          <Button
            size="small"
            danger={user.status === 'active'}
            onClick={() => handleBanToggle(user)}
          >
            {user.status === 'active' ? t('common.ban') : t('common.unban')}
          </Button>
          <Button size="small" onClick={() => handleTopUpOpen(user)}>
            {t('billing.topUp')}
          </Button>
        </span>
      ),
    },
  ];

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.users')}</h2>

      <div style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
        <Input
          placeholder={t('users.searchPlaceholder')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onPressEnter={handleSearch}
          style={{ width: 280 }}
        />
        <Button onClick={handleSearch}>{t('common.search')}</Button>
      </div>

      <Table<User>
        columns={columns}
        dataSource={users}
        rowKey="id"
        loading={loading}
        bordered
        size="small"
        pagination={{
          current: page,
          pageSize,
          total,
          onChange: (p) => { setPage(p); fetchUsers(p); },
        }}
      />

      <Modal
        title={t('users.topUpTitle')}
        open={topUpModal}
        onOk={handleTopUpConfirm}
        onCancel={() => setTopUpModal(false)}
        confirmLoading={topUpLoading}
        okText={t('common.confirm')}
      >
        <div style={{ marginBottom: 8, color: 'var(--text-muted)', fontSize: 12 }}>
          {t('users.userLabel')}: {topUpUser?.email}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span>{t('billing.amountUSD')}:</span>
          <InputNumber
            min={0.01}
            step={1}
            value={topUpAmount}
            onChange={(v) => setTopUpAmount(v ?? 10)}
            style={{ width: 160 }}
          />
        </div>
      </Modal>
    </div>
  );
}
