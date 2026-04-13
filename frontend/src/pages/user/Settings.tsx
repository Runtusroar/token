import { useState } from 'react';
import { Input, Button, message, Form } from 'antd';
import { useTranslation } from 'react-i18next';
import { userAPI } from '../../api/user';

interface PasswordForm {
  old_password: string;
  new_password: string;
  confirm_password: string;
}

export default function Settings() {
  const [form] = Form.useForm<PasswordForm>();
  const [loading, setLoading] = useState(false);
  const { t } = useTranslation();

  async function handleSubmit(values: PasswordForm) {
    if (values.new_password !== values.confirm_password) {
      message.error(t('settings.passwordMismatch'));
      return;
    }
    setLoading(true);
    try {
      await userAPI.changePassword(values.old_password, values.new_password);
      message.success(t('settings.passwordChanged'));
      form.resetFields();
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { message?: string } } };
      const errMsg = axiosErr?.response?.data?.message ?? t('settings.passwordChangeFailed');
      message.error(errMsg);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div>
      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 16 }}>{'// ' + t('nav.settings')}</h2>

      <div style={{
        maxWidth: 400,
        border: '1px solid var(--border-color)',
        padding: 24,
        backgroundColor: 'var(--bg-card)',
      }}>
        <div style={{
          fontSize: 11,
          textTransform: 'uppercase',
          letterSpacing: 1,
          color: 'var(--text-muted)',
          marginBottom: 16,
        }}>
          {t('settings.changePassword')}
        </div>

        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          style={{ fontFamily: 'var(--font-mono)' }}
        >
          <Form.Item
            name="old_password"
            label={<span style={{ fontSize: 12 }}>{t('settings.oldPassword')}</span>}
            rules={[{ required: true, message: t('common.required') }]}
          >
            <Input.Password />
          </Form.Item>

          <Form.Item
            name="new_password"
            label={<span style={{ fontSize: 12 }}>{t('settings.newPassword')}</span>}
            rules={[{ required: true, message: t('common.required') }, { min: 6, message: t('settings.minLength') }]}
          >
            <Input.Password />
          </Form.Item>

          <Form.Item
            name="confirm_password"
            label={<span style={{ fontSize: 12 }}>{t('settings.confirmPassword')}</span>}
            rules={[{ required: true, message: t('common.required') }]}
          >
            <Input.Password />
          </Form.Item>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" loading={loading} block>
              {t('settings.changePasswordBtn')}
            </Button>
          </Form.Item>
        </Form>
      </div>
    </div>
  );
}
