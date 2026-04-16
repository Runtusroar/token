import { useEffect, useState } from 'react';
import { Button, Input, Switch, InputNumber, message } from 'antd';
import { useTranslation } from 'react-i18next';
import { adminAPI } from '../../api/admin';

interface SettingsState {
  site_name: string;
  register_enabled: boolean;
  default_balance: number;
}

const labelStyle: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--text-muted)',
  textTransform: 'uppercase',
  letterSpacing: 1,
  marginBottom: 4,
};

const rowStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 4,
  paddingBottom: 16,
  borderBottom: '1px solid var(--border-light)',
};

export default function AdminSettings() {
  const [settings, setSettings] = useState<SettingsState>({
    site_name: '',
    register_enabled: true,
    default_balance: 0,
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const { t } = useTranslation();

  useEffect(() => {
    adminAPI.getSettings()
      .then((res) => {
        const data: Record<string, string> = res.data.data ?? {};
        setSettings({
          site_name: data.site_name ?? '',
          register_enabled: data.register_enabled !== 'false',
          default_balance: Number(data.default_balance ?? 0),
        });
      })
      .finally(() => setLoading(false));
  }, []);

  function handleSave() {
    const payload: Record<string, unknown> = {
      site_name: settings.site_name,
      register_enabled: String(settings.register_enabled),
      default_balance: String(settings.default_balance),
    };
    setSaving(true);
    adminAPI.updateSettings(payload)
      .then(() => message.success(t('settings.saveSuccess')))
      .catch(() => message.error(t('settings.saveFailed')))
      .finally(() => setSaving(false));
  }

  if (loading) {
    return (
      <div>
        <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.settings')}</h2>
        <div style={{ color: 'var(--text-muted)' }}>{t('common.loading')}</div>
      </div>
    );
  }

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.settings')}</h2>

      <div
        style={{
          maxWidth: 480,
          border: '1px solid var(--border-color)',
          padding: 20,
          backgroundColor: 'var(--bg-card)',
          display: 'flex',
          flexDirection: 'column',
          gap: 16,
        }}
      >
        <div style={rowStyle}>
          <div style={labelStyle}>{t('settings.siteName')}</div>
          <Input
            value={settings.site_name}
            onChange={(e) => setSettings({ ...settings, site_name: e.target.value })}
            placeholder="AI Token Relay"
          />
        </div>

        <div style={{ ...rowStyle, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <div style={labelStyle}>{t('settings.registerEnabled')}</div>
            <div style={{ fontSize: 11, color: 'var(--text-secondary)' }}>
              {t('settings.registerEnabledDesc')}
            </div>
          </div>
          <Switch
            checked={settings.register_enabled}
            onChange={(v) => setSettings({ ...settings, register_enabled: v })}
          />
        </div>

        <div style={{ ...rowStyle, borderBottom: 'none' }}>
          <div style={labelStyle}>{t('settings.defaultBalance')}</div>
          <InputNumber
            min={0}
            step={1}
            value={settings.default_balance}
            onChange={(v) => setSettings({ ...settings, default_balance: v ?? 0 })}
            style={{ width: '100%' }}
          />
        </div>

        <Button
          type="primary"
          onClick={handleSave}
          loading={saving}
          style={{ alignSelf: 'flex-start' }}
        >
          {t('settings.saveSettings')}
        </Button>
      </div>
    </div>
  );
}
