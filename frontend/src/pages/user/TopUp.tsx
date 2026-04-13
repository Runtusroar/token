import { useState } from 'react';
import { Input, Button, message, Alert } from 'antd';
import { useTranslation } from 'react-i18next';
import { userAPI } from '../../api/user';

export default function TopUp() {
  const [code, setCode] = useState('');
  const [loading, setLoading] = useState(false);
  const [successMsg, setSuccessMsg] = useState('');
  const { t } = useTranslation();

  async function handleRedeem() {
    const trimmed = code.trim();
    if (!trimmed) {
      message.error(t('billing.enterCode'));
      return;
    }
    setLoading(true);
    setSuccessMsg('');
    try {
      await userAPI.redeem(trimmed);
      setSuccessMsg(`"${trimmed}" ${t('billing.redeemSuccess')}`);
      setCode('');
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { message?: string } } };
      const errMsg = axiosErr?.response?.data?.message ?? t('billing.redeemSuccess');
      message.error(errMsg);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.topUp')}</h2>

      <div style={{
        maxWidth: 480,
        border: '1px solid var(--border-color)',
        padding: 24,
        backgroundColor: 'var(--bg-card)',
      }}>
        <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 12, textTransform: 'uppercase', letterSpacing: 1 }}>
          {t('billing.redeemCode')}
        </div>

        {successMsg && (
          <Alert
            type="success"
            message={successMsg}
            style={{ marginBottom: 16 }}
            showIcon
          />
        )}

        <Input
          placeholder={t('billing.enterCode')}
          value={code}
          onChange={(e) => setCode(e.target.value)}
          onPressEnter={handleRedeem}
          style={{ marginBottom: 12, fontFamily: 'var(--font-mono)' }}
        />

        <Button
          type="primary"
          loading={loading}
          onClick={handleRedeem}
          block
        >
          {t('billing.redeem')}
        </Button>
      </div>
    </div>
  );
}
