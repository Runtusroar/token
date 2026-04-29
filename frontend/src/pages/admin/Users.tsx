import { useEffect, useState, useCallback } from 'react';
import { Table, Tag, Button, Input, Modal, InputNumber, Drawer, Tooltip, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { adminAPI } from '../../api/admin';

interface User {
  id: number;
  email: string;
  role: string;
  balance: number;
  rate_multiplier: number;
  note: string;
  status: string;
  created_at: string;
  request_count: number;
  total_tokens: number;
  total_cost: number;
}

interface BalanceLog {
  id: number;
  type: string;
  amount: number;
  balance_after: number;
  description: string;
  created_at: string;
}

interface RequestLog {
  id: number;
  model: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost: number;
  status: string;
  duration_ms: number;
  created_at: string;
}

interface DailyStat {
  date: string;
  requests: number;
  total_tokens: number;
  cost: number | string;
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

  const [deductModal, setDeductModal] = useState(false);
  const [deductUser, setDeductUser] = useState<User | null>(null);
  const [deductAmount, setDeductAmount] = useState<number>(1);
  const [deductReason, setDeductReason] = useState<string>('');
  const [deductLoading, setDeductLoading] = useState(false);

  const [rateModal, setRateModal] = useState(false);
  const [rateUser, setRateUser] = useState<User | null>(null);
  const [rateValue, setRateValue] = useState<number>(1);
  const [rateLoading, setRateLoading] = useState(false);

  const [noteModal, setNoteModal] = useState(false);
  const [noteUser, setNoteUser] = useState<User | null>(null);
  const [noteValue, setNoteValue] = useState<string>('');
  const [noteLoading, setNoteLoading] = useState(false);

  // Drawer state
  const [drawerUser, setDrawerUser] = useState<User | null>(null);
  const [balanceLogs, setBalanceLogs] = useState<BalanceLog[]>([]);
  const [balanceTotal, setBalanceTotal] = useState(0);
  const [balancePage, setBalancePage] = useState(1);
  const [balanceLoading, setBalanceLoading] = useState(false);
  const [requestLogs, setRequestLogs] = useState<RequestLog[]>([]);
  const [requestTotal, setRequestTotal] = useState(0);
  const [requestPage, setRequestPage] = useState(1);
  const [requestLoading, setRequestLoading] = useState(false);
  const [dailyStats, setDailyStats] = useState<DailyStat[]>([]);
  const [dailyLoading, setDailyLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<'balance' | 'requests' | 'daily'>('balance');

  function fetchUsers(p = page, s = search) {
    setLoading(true);
    adminAPI.listUsers(p, pageSize, s || undefined)
      .then((res) => {
        const d = res.data.data;
        setUsers(Array.isArray(d) ? d : (d?.items ?? []));
        setTotal(d?.total ?? (Array.isArray(d) ? d.length : 0));
      })
      .finally(() => setLoading(false));
  }

  useEffect(() => { fetchUsers(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const fetchBalanceLogs = useCallback((userId: number, p: number) => {
    setBalanceLoading(true);
    adminAPI.userBalanceLogs(userId, p, 10)
      .then((res) => {
        const d = res.data.data;
        setBalanceLogs(d?.items ?? []);
        setBalanceTotal(d?.total ?? 0);
      })
      .finally(() => setBalanceLoading(false));
  }, []);

  const fetchRequestLogs = useCallback((userId: number, p: number) => {
    setRequestLoading(true);
    adminAPI.userRequestLogs(userId, p, 10)
      .then((res) => {
        const d = res.data.data;
        setRequestLogs(d?.items ?? []);
        setRequestTotal(d?.total ?? 0);
      })
      .finally(() => setRequestLoading(false));
  }, []);

  const fetchDailyStats = useCallback((userId: number) => {
    setDailyLoading(true);
    adminAPI.userDailyStats(userId, 30)
      .then((res) => setDailyStats(res.data.data ?? []))
      .finally(() => setDailyLoading(false));
  }, []);

  function openDrawer(user: User) {
    setDrawerUser(user);
    setActiveTab('balance');
    setBalancePage(1);
    setRequestPage(1);
    fetchBalanceLogs(user.id, 1);
    fetchRequestLogs(user.id, 1);
    fetchDailyStats(user.id);
  }

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

  function handleRateOpen(user: User) {
    setRateUser(user);
    setRateValue(Number(user.rate_multiplier ?? 1));
    setRateModal(true);
  }

  function handleRateConfirm() {
    if (!rateUser) return;
    setRateLoading(true);
    adminAPI.updateUser(rateUser.id, { rate_multiplier: rateValue })
      .then(() => {
        message.success(t('users.rateUpdateSuccess'));
        setRateModal(false);
        fetchUsers();
      })
      .catch(() => message.error(t('users.operationFailed')))
      .finally(() => setRateLoading(false));
  }

  function handleNoteOpen(user: User) {
    setNoteUser(user);
    setNoteValue(user.note ?? '');
    setNoteModal(true);
  }

  function handleNoteConfirm() {
    if (!noteUser) return;
    setNoteLoading(true);
    adminAPI.updateUser(noteUser.id, { note: noteValue })
      .then(() => {
        message.success(t('users.noteUpdateSuccess'));
        setNoteModal(false);
        if (drawerUser?.id === noteUser.id) {
          setDrawerUser({ ...drawerUser, note: noteValue });
        }
        fetchUsers();
      })
      .catch(() => message.error(t('users.operationFailed')))
      .finally(() => setNoteLoading(false));
  }

  function handleTopUpConfirm() {
    if (!topUpUser) return;
    setTopUpLoading(true);
    adminAPI.topUp(topUpUser.id, topUpAmount)
      .then(() => {
        message.success(t('users.topUpSuccess'));
        setTopUpModal(false);
        fetchUsers();
        // Refresh drawer if open for this user.
        if (drawerUser?.id === topUpUser.id) {
          fetchBalanceLogs(topUpUser.id, 1);
        }
      })
      .catch(() => message.error(t('users.topUpFailed')))
      .finally(() => setTopUpLoading(false));
  }

  function handleDeductOpen(user: User) {
    setDeductUser(user);
    setDeductAmount(1);
    setDeductReason('');
    setDeductModal(true);
  }

  function handleDeductConfirm() {
    if (!deductUser) return;
    setDeductLoading(true);
    adminAPI.deduct(deductUser.id, deductAmount, deductReason)
      .then(() => {
        message.success(t('users.deductSuccess'));
        setDeductModal(false);
        fetchUsers();
        if (drawerUser?.id === deductUser.id) {
          fetchBalanceLogs(deductUser.id, 1);
        }
      })
      .catch(() => message.error(t('users.deductFailed')))
      .finally(() => setDeductLoading(false));
  }

  const typeColor = (type: string) => {
    if (['redeem', 'topup'].includes(type)) return 'success';
    if (['deduct', 'admin_deduct'].includes(type)) return 'error';
    return 'default';
  };

  const columns: ColumnsType<User> = [
    { title: t('users.id'), dataIndex: 'id', key: 'id', width: 60 },
    {
      title: t('users.email'),
      dataIndex: 'email',
      key: 'email',
      render: (email: string, user) => (
        <a onClick={() => openDrawer(user)} style={{ cursor: 'pointer', color: 'var(--text-primary)', textDecoration: 'underline' }}>
          {email}
        </a>
      ),
    },
    {
      title: t('users.role'), dataIndex: 'role', key: 'role', width: 80,
      render: (r: string) => <Tag color={r === 'admin' ? 'purple' : 'default'}>{r}</Tag>,
    },
    {
      title: t('users.balance'), dataIndex: 'balance', key: 'balance', width: 100,
      render: (v: number) => `$${Number(v ?? 0).toFixed(4)}`,
    },
    {
      title: t('users.rateMultiplier'), dataIndex: 'rate_multiplier', key: 'rate_multiplier', width: 90,
      render: (v: number) => `${Number(v ?? 1).toFixed(2)}x`,
    },
    {
      title: t('users.requests'), dataIndex: 'request_count', key: 'request_count', width: 90,
      render: (v: number) => (v ?? 0).toLocaleString(),
    },
    {
      title: t('users.consumed'), dataIndex: 'total_cost', key: 'total_cost', width: 100,
      render: (v: number) => `$${Number(v ?? 0).toFixed(4)}`,
    },
    {
      title: t('users.note'), dataIndex: 'note', key: 'note', width: 140,
      render: (v: string) => {
        const text = (v ?? '').trim();
        if (!text) return <span style={{ color: 'var(--text-muted)' }}>-</span>;
        return (
          <Tooltip title={text} placement="topLeft">
            <span style={{
              display: 'inline-block', maxWidth: 120,
              overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              verticalAlign: 'middle',
            }}>{text}</span>
          </Tooltip>
        );
      },
    },
    {
      title: t('common.status'), dataIndex: 'status', key: 'status', width: 90,
      render: (s: string) => <Tag color={s === 'active' ? 'success' : 'default'}>{s}</Tag>,
    },
    {
      title: t('common.actions'), key: 'actions', width: 380,
      render: (_, user) => (
        <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
          <Button size="small" onClick={() => openDrawer(user)}>{t('users.detail')}</Button>
          <Button size="small" danger={user.status === 'active'} onClick={() => handleBanToggle(user)}>
            {user.status === 'active' ? t('common.ban') : t('common.unban')}
          </Button>
          <Button size="small" onClick={() => handleTopUpOpen(user)}>{t('billing.topUp')}</Button>
          <Button size="small" danger onClick={() => handleDeductOpen(user)}>{t('users.deduct')}</Button>
          <Button size="small" onClick={() => handleRateOpen(user)}>{t('users.rateMultiplier')}</Button>
          <Button size="small" onClick={() => handleNoteOpen(user)}>{t('users.note')}</Button>
        </span>
      ),
    },
  ];

  const balanceColumns: ColumnsType<BalanceLog> = [
    {
      title: t('logs.type'), dataIndex: 'type', key: 'type', width: 80,
      render: (tp: string) => <Tag color={typeColor(tp)}>{tp}</Tag>,
    },
    {
      title: t('billing.amount'), dataIndex: 'amount', key: 'amount', width: 100, align: 'right',
      render: (v: number) => (
        <span style={{ color: v >= 0 ? 'var(--accent-green)' : '#e53e3e', fontWeight: 600 }}>
          {v >= 0 ? '+' : ''}{Number(v).toFixed(4)}
        </span>
      ),
    },
    {
      title: t('logs.balanceAfter'), dataIndex: 'balance_after', key: 'balance_after', width: 100, align: 'right',
      render: (v: number) => `$${Number(v).toFixed(4)}`,
    },
    {
      title: t('logs.description'), dataIndex: 'description', key: 'description',
      render: (v: string) => <span style={{ fontSize: 11 }}>{v || '-'}</span>,
    },
    {
      title: t('logs.time'), dataIndex: 'created_at', key: 'created_at', width: 150,
      render: (v: string) => new Date(v).toLocaleString(),
    },
  ];

  const requestColumns: ColumnsType<RequestLog> = [
    { title: t('logs.model'), dataIndex: 'model', key: 'model' },
    {
      title: 'Tokens', key: 'tokens', width: 100, align: 'right',
      render: (_, r) => `${r.prompt_tokens}+${r.completion_tokens}`,
    },
    {
      title: t('logs.cost'), dataIndex: 'cost', key: 'cost', width: 90, align: 'right',
      render: (v: number) => `$${Number(v ?? 0).toFixed(6)}`,
    },
    {
      title: t('common.status'), dataIndex: 'status', key: 'status', width: 70,
      render: (s: string) => <Tag color={s === 'success' ? 'success' : 'error'}>{s}</Tag>,
    },
    {
      title: t('logs.time'), dataIndex: 'created_at', key: 'created_at', width: 150,
      render: (v: string) => new Date(v).toLocaleString(),
    },
  ];

  const dailyColumns: ColumnsType<DailyStat> = [
    { title: t('dashboard.date'), dataIndex: 'date', key: 'date', width: 110 },
    {
      title: t('dashboard.requests'), dataIndex: 'requests', key: 'requests',
      width: 90, align: 'right',
      render: (v: number) => (v ?? 0).toLocaleString(),
    },
    {
      title: t('dashboard.totalTokens'), dataIndex: 'total_tokens', key: 'total_tokens',
      width: 110, align: 'right',
      render: (v: number) => (v ?? 0).toLocaleString(),
    },
    {
      title: t('users.consumed'), dataIndex: 'cost', key: 'cost',
      width: 110, align: 'right',
      render: (v: number | string) => `$${Number(v ?? 0).toFixed(4)}`,
    },
  ];

  const tabStyle = (active: boolean): React.CSSProperties => ({
    padding: '6px 16px',
    fontSize: 12,
    fontFamily: 'var(--font-mono)',
    border: '1px solid var(--border-color)',
    borderBottom: active ? 'none' : '1px solid var(--border-color)',
    background: active ? 'var(--bg-card)' : 'transparent',
    color: active ? 'var(--text-primary)' : 'var(--text-muted)',
    cursor: 'pointer',
    marginBottom: -1,
    position: 'relative' as const,
    zIndex: active ? 1 : 0,
  });

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
          current: page, pageSize, total,
          onChange: (p) => { setPage(p); fetchUsers(p); },
        }}
      />

      {/* Top-up modal */}
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
          <InputNumber min={0.01} step={1} value={topUpAmount} onChange={(v) => setTopUpAmount(v ?? 10)} style={{ width: 160 }} />
        </div>
      </Modal>

      {/* Deduct modal */}
      <Modal
        title={t('users.deductTitle')}
        open={deductModal}
        onOk={handleDeductConfirm}
        onCancel={() => setDeductModal(false)}
        confirmLoading={deductLoading}
        okText={t('common.confirm')}
        okButtonProps={{ danger: true }}
      >
        <div style={{ marginBottom: 8, color: 'var(--text-muted)', fontSize: 12 }}>
          {t('users.userLabel')}: {deductUser?.email}
          <span style={{ marginLeft: 12 }}>
            {t('users.balance')}: ${Number(deductUser?.balance ?? 0).toFixed(4)}
          </span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
          <span>{t('billing.amountUSD')}:</span>
          <InputNumber min={0.01} step={1} value={deductAmount} onChange={(v) => setDeductAmount(v ?? 1)} style={{ width: 160 }} />
        </div>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
          <span style={{ paddingTop: 4 }}>{t('users.deductReason')}:</span>
          <Input.TextArea
            value={deductReason}
            onChange={(e) => setDeductReason(e.target.value)}
            rows={2}
            maxLength={200}
            placeholder={t('users.deductReasonPlaceholder')}
            style={{ flex: 1 }}
          />
        </div>
        <div style={{ marginTop: 8, color: 'var(--text-muted)', fontSize: 11 }}>
          {t('users.deductHint')}
        </div>
      </Modal>

      {/* Rate multiplier modal */}
      <Modal
        title={t('users.rateMultiplierTitle')}
        open={rateModal}
        onOk={handleRateConfirm}
        onCancel={() => setRateModal(false)}
        confirmLoading={rateLoading}
        okText={t('common.confirm')}
      >
        <div style={{ marginBottom: 8, color: 'var(--text-muted)', fontSize: 12 }}>
          {t('users.userLabel')}: {rateUser?.email}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span>{t('users.rateMultiplier')}:</span>
          <InputNumber
            min={0}
            step={0.1}
            precision={2}
            value={rateValue}
            onChange={(v) => setRateValue(v ?? 1)}
            style={{ width: 160 }}
          />
          <span style={{ color: 'var(--text-muted)', fontSize: 12 }}>
            {t('users.rateMultiplierHint')}
          </span>
        </div>
      </Modal>

      {/* Note modal */}
      <Modal
        title={t('users.noteTitle')}
        open={noteModal}
        onOk={handleNoteConfirm}
        onCancel={() => setNoteModal(false)}
        confirmLoading={noteLoading}
        okText={t('common.confirm')}
      >
        <div style={{ marginBottom: 8, color: 'var(--text-muted)', fontSize: 12 }}>
          {t('users.userLabel')}: {noteUser?.email}
        </div>
        <Input.TextArea
          value={noteValue}
          onChange={(e) => setNoteValue(e.target.value)}
          rows={4}
          maxLength={500}
          showCount
          placeholder={t('users.notePlaceholder')}
        />
      </Modal>

      {/* User detail drawer */}
      <Drawer
        title={drawerUser?.email}
        open={!!drawerUser}
        onClose={() => setDrawerUser(null)}
        width={680}
      >
        {drawerUser && (
          <div>
            {/* User summary */}
            <div style={{
              display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12, marginBottom: 20,
              fontSize: 12, fontFamily: 'var(--font-mono)',
            }}>
              <div style={{ border: '1px solid var(--border-color)', padding: 12 }}>
                <div style={{ color: 'var(--text-muted)', fontSize: 10, marginBottom: 4 }}>{t('users.balance')}</div>
                <div style={{ fontSize: 18, fontWeight: 700, color: 'var(--accent-green)' }}>
                  ${Number(drawerUser.balance ?? 0).toFixed(4)}
                </div>
              </div>
              <div style={{ border: '1px solid var(--border-color)', padding: 12 }}>
                <div style={{ color: 'var(--text-muted)', fontSize: 10, marginBottom: 4 }}>{t('users.requests')}</div>
                <div style={{ fontSize: 18, fontWeight: 700 }}>{(drawerUser.request_count ?? 0).toLocaleString()}</div>
              </div>
              <div style={{ border: '1px solid var(--border-color)', padding: 12 }}>
                <div style={{ color: 'var(--text-muted)', fontSize: 10, marginBottom: 4 }}>{t('users.consumed')}</div>
                <div style={{ fontSize: 18, fontWeight: 700, color: '#e53e3e' }}>
                  ${Number(drawerUser.total_cost ?? 0).toFixed(4)}
                </div>
              </div>
            </div>

            {/* Admin note */}
            <div style={{
              border: '1px solid var(--border-color)', padding: 12, marginBottom: 20,
              fontSize: 12, fontFamily: 'var(--font-mono)',
            }}>
              <div style={{
                display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                color: 'var(--text-muted)', fontSize: 10, marginBottom: 6,
              }}>
                <span>{t('users.note')}</span>
                <a onClick={() => handleNoteOpen(drawerUser)} style={{ cursor: 'pointer' }}>
                  {t('common.edit')}
                </a>
              </div>
              <div style={{ whiteSpace: 'pre-wrap', color: drawerUser.note ? 'var(--text-primary)' : 'var(--text-muted)' }}>
                {drawerUser.note || '-'}
              </div>
            </div>

            {/* Tabs */}
            <div style={{ display: 'flex', gap: 0, marginBottom: 0 }}>
              <button style={tabStyle(activeTab === 'balance')} onClick={() => setActiveTab('balance')}>
                {t('users.balanceHistory')} ({balanceTotal})
              </button>
              <button style={tabStyle(activeTab === 'requests')} onClick={() => setActiveTab('requests')}>
                {t('users.requestHistory')} ({requestTotal})
              </button>
              <button style={tabStyle(activeTab === 'daily')} onClick={() => setActiveTab('daily')}>
                {t('users.dailyHistory')}
              </button>
            </div>

            {/* Tab content */}
            <div style={{ border: '1px solid var(--border-color)', borderTop: '1px solid var(--border-color)' }}>
              {activeTab === 'balance' && (
                <Table<BalanceLog>
                  columns={balanceColumns}
                  dataSource={balanceLogs}
                  rowKey="id"
                  loading={balanceLoading}
                  size="small"
                  pagination={{
                    current: balancePage, pageSize: 10, total: balanceTotal, size: 'small',
                    onChange: (p) => { setBalancePage(p); fetchBalanceLogs(drawerUser.id, p); },
                  }}
                />
              )}
              {activeTab === 'requests' && (
                <Table<RequestLog>
                  columns={requestColumns}
                  dataSource={requestLogs}
                  rowKey="id"
                  loading={requestLoading}
                  size="small"
                  pagination={{
                    current: requestPage, pageSize: 10, total: requestTotal, size: 'small',
                    onChange: (p) => { setRequestPage(p); fetchRequestLogs(drawerUser.id, p); },
                  }}
                />
              )}
              {activeTab === 'daily' && (
                <Table<DailyStat>
                  columns={dailyColumns}
                  dataSource={dailyStats}
                  rowKey="date"
                  loading={dailyLoading}
                  size="small"
                  pagination={false}
                />
              )}
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}
