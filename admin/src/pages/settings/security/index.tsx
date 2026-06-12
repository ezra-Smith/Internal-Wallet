import { ModalForm, PageContainer, ProFormText } from '@ant-design/pro-components';
import { history, useIntl, useLocation } from '@umijs/max';
import { App, Alert, Button, Card, Descriptions, Form, Space, Tag, Typography } from 'antd';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  adminChangeMyPassword,
  adminConfirmTwoFASetup,
  adminDisableTwoFA,
  adminGetMySecuritySettings,
  adminStartTwoFARebind,
  adminStartTwoFASetup,
} from '@/api/generated/settings';
import type {
  AdminChangeMyPasswordRequest,
  AdminConfirmTwoFASetupRequest,
  AdminDisableTwoFARequest,
  AdminGetMySecuritySettingsData,
  AdminStartTwoFARebindRequest,
  AdminTwoFASetupPayload,
} from '@/api/generated/schemas';
import { getRequestErrorMessage } from '@/utils/requestError';
import { clearAdminToken, setAdminToken } from '@/utils/auth';

const SecurityPage: React.FC = () => {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const location = useLocation();
  const t = useCallback(
    (id: string, defaultMessage: string, values?: Record<string, any>) =>
      intl.formatMessage({ id, defaultMessage }, values),
    [intl],
  );

  const [loading, setLoading] = useState(true);
  const [settings, setSettings] = useState<AdminGetMySecuritySettingsData | null>(null);

  const twoFA = settings?.two_fa;
  const passwordPolicy = settings?.password_policy;
  const passwordMinLen = useMemo(() => {
    const v = passwordPolicy?.min_length;
    return typeof v === 'number' && v > 0 ? v : 10;
  }, [passwordPolicy?.min_length]);

  const [pwdOpen, setPwdOpen] = useState(false);

  const [setupOpen, setSetupOpen] = useState(false);
  const [setupPayload, setSetupPayload] = useState<AdminTwoFASetupPayload | null>(null);
  const [setupPayloadLoading, setSetupPayloadLoading] = useState(false);

  const [rebindOpen, setRebindOpen] = useState(false);
  const [rebindStep, setRebindStep] = useState<1 | 2>(1);
  const [rebindPayload, setRebindPayload] = useState<AdminTwoFASetupPayload | null>(null);
  const [rebindPassword, setRebindPassword] = useState('');
  const [rebindForm] = Form.useForm();

  const [disableOpen, setDisableOpen] = useState(false);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const res = await adminGetMySecuritySettings({ skipErrorHandler: true });
      if (!res?.success) {
        throw new Error(
          res?.message || t('pages.settings.security.messages.loadFailed', 'Failed to load security settings'),
        );
      }
      setSettings(res.data ?? null);
    } catch (e: any) {
      message.error(
        getRequestErrorMessage(e) || t('pages.settings.security.messages.loadFailed', 'Failed to load security settings'),
      );
    } finally {
      setLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const force = useMemo(() => {
    const sp = new URLSearchParams(location.search);
    return (sp.get('force') || '').toLowerCase();
  }, [location.search]);

  const forceHandledRef = useRef<string>('');

  const validateNewPassword = useCallback(
    async (_: unknown, value: string | undefined) => {
      const password = (value ?? '').trim();
      if (!password) return Promise.resolve();

      if (password.length < passwordMinLen) {
        return Promise.reject(
          new Error(
            t('pages.settings.security.password.validation.minLen', 'At least {min} characters', {
              min: passwordMinLen,
            }),
          ),
        );
      }

      const hasUpper = /[A-Z]/.test(password);
      const hasLower = /[a-z]/.test(password);
      const hasDigit = /\d/.test(password);
      const hasSpecial = /[^A-Za-z0-9]/.test(password);
      if (!hasUpper || !hasLower || !hasDigit || !hasSpecial) {
        return Promise.reject(
          new Error(
            t(
              'pages.settings.security.password.validation.complexity',
              'Must include uppercase, lowercase, number, and special character',
            ),
          ),
        );
      }

      return Promise.resolve();
    },
    [passwordMinLen, t],
  );

  const showBackupCodesModal = useCallback(
    async (codes: string[] | undefined) => {
      if (!codes?.length) return;
      await new Promise<void>((resolve) => {
        modal.confirm({
          title: t('pages.login.twoFa.backupCodesTitle', '2FA backup codes (keep them safe)'),
          width: 520,
          centered: true,
          maskClosable: false,
          closable: false,
          cancelButtonProps: { style: { display: 'none' } },
          okText: t('pages.login.twoFa.backupCodesOk', 'I have saved them'),
          content: (
            <Typography.Paragraph>
              <pre style={{ whiteSpace: 'pre-wrap' }}>{codes.join('\n')}</pre>
            </Typography.Paragraph>
          ),
          onOk: () => resolve(),
        });
      });
    },
    [modal, t],
  );

  const openSetupModal = useCallback(async () => {
    setSetupOpen(true);
    setSetupPayload(null);
    setSetupPayloadLoading(true);
    try {
      const res = await adminStartTwoFASetup({}, { skipErrorHandler: true });
      if (!res?.success) {
        throw new Error(res?.message || t('pages.settings.security.twoFa.messages.setupFailed', 'Failed to start setup'));
      }
      setSetupPayload(res.data ?? null);
    } catch (e: any) {
      message.error(
        getRequestErrorMessage(e) ||
          t('pages.settings.security.twoFa.messages.setupFailed', 'Failed to start setup'),
      );
      setSetupOpen(false);
    } finally {
      setSetupPayloadLoading(false);
    }
  }, [message, t]);

  useEffect(() => {
    if (!force) return;
    if (forceHandledRef.current === force) return;
    forceHandledRef.current = force;

    if (force === 'password') {
      setPwdOpen(true);
      return;
    }
    if (force === '2fa') {
      if (twoFA?.enabled) return;
      void openSetupModal();
    }
  }, [force, openSetupModal, twoFA?.enabled]);

  const openRebindModal = useCallback(() => {
    setRebindOpen(true);
    setRebindStep(1);
    setRebindPayload(null);
    setRebindPassword('');
    rebindForm.resetFields();
  }, [rebindForm]);

  return (
    <PageContainer>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <Card
          title={t('pages.settings.security.twoFa.title', 'Two-Factor Authentication (2FA)')}
          extra={
            <Space>
              <Button size="small" onClick={() => void reload()} loading={loading}>
                {t('pages.settings.security.actions.reload', 'Reload')}
              </Button>
              {twoFA?.enabled ? (
                <>
                  <Button size="small" onClick={openRebindModal}>
                    {t('pages.settings.security.twoFa.actions.rebind', 'Rebind')}
                  </Button>
                  <Button
                    size="small"
                    danger
                    disabled={!!twoFA?.required}
                    onClick={() => setDisableOpen(true)}
                  >
                    {t('pages.settings.security.twoFa.actions.disable', 'Disable')}
                  </Button>
                </>
              ) : (
                <Button size="small" type="primary" onClick={() => void openSetupModal()}>
                  {twoFA?.has_pending_setup
                    ? t('pages.settings.security.twoFa.actions.continue', 'Continue Setup')
                    : t('pages.settings.security.twoFa.actions.enable', 'Enable 2FA')}
                </Button>
              )}
            </Space>
          }
          loading={loading}
        >
          <Space direction="vertical" style={{ width: '100%' }}>
            <Alert
              type="info"
              showIcon
              message={t('pages.settings.security.twoFa.what.title', 'What is 2FA?')}
              description={t(
                'pages.settings.security.twoFa.what.desc',
                'Two-Factor Authentication (2FA) adds an extra layer of protection. Even if your password is leaked, attackers cannot access your account without the one-time code from your authenticator app.',
              )}
            />

            <Descriptions size="small" column={1}>
              <Descriptions.Item label={t('pages.settings.security.twoFa.status', 'Status')}>
                <Space size="small">
                  {twoFA?.enabled ? (
                    <Tag color="success">{t('pages.settings.security.twoFa.status.enabled', 'Enabled')}</Tag>
                  ) : (
                    <Tag color="warning">{t('pages.settings.security.twoFa.status.disabled', 'Not enabled')}</Tag>
                  )}
                  {twoFA?.required ? (
                    <Tag color="processing">{t('pages.settings.security.twoFa.status.required', 'Required')}</Tag>
                  ) : null}
                </Space>
              </Descriptions.Item>
              {twoFA?.enabled ? (
                <Descriptions.Item label={t('pages.settings.security.twoFa.boundAt', 'Bound at')}>
                  {twoFA?.bound_at || '-'}
                </Descriptions.Item>
              ) : null}
            </Descriptions>

            {twoFA?.required ? (
              <Typography.Text type="secondary">
                {t(
                  'pages.settings.security.twoFa.requiredHint',
                  '2FA is required by security policy. You can rebind, but cannot disable it.',
                )}
              </Typography.Text>
            ) : null}
          </Space>
        </Card>

        <Card
          title={t('pages.settings.security.password.title', 'Password')}
          extra={
            <Button type="primary" onClick={() => setPwdOpen(true)}>
              {t('pages.settings.security.password.actions.change', 'Change Password')}
            </Button>
          }
        >
          <Space direction="vertical" style={{ width: '100%' }}>
            <Alert
              type="warning"
              showIcon
              message={t('pages.settings.security.password.hints.title', 'Password security tips')}
              description={
                <ul style={{ margin: 0, paddingInlineStart: 18 }}>
                  <li>
                    {t('pages.settings.security.password.hints.minLen', 'At least {min} characters', {
                      min: passwordMinLen,
                    })}
                  </li>
                  <li>
                    {t(
                      'pages.settings.security.password.hints.complexity',
                      'Include uppercase, lowercase, number, and special character',
                    )}
                  </li>
                  <li>
                    {t(
                      'pages.settings.security.password.hints.noSame',
                      'Do not reuse your current password',
                    )}
                  </li>
                  <li>
                    {t(
                      'pages.settings.security.password.hints.noUsername',
                      'Do not include your username in the password',
                    )}
                  </li>
                  <li>{t('pages.settings.security.password.hints.noReuse', 'Avoid reusing recent passwords')}</li>
                </ul>
              }
            />
            <Descriptions size="small" column={1}>
              <Descriptions.Item label={t('pages.settings.security.password.changedAt', 'Last changed')}>
                {settings?.password_changed_at || '-'}
              </Descriptions.Item>
              <Descriptions.Item label={t('pages.settings.security.password.nextExpiry', 'Next expiry')}>
                {settings?.next_password_expiry || '-'}
              </Descriptions.Item>
              <Descriptions.Item label={t('pages.settings.security.lastLogin', 'Last login')}>
                {settings?.last_login_at ? `${settings.last_login_at} (${settings?.last_login_ip || '-'})` : '-'}
              </Descriptions.Item>
            </Descriptions>
          </Space>
        </Card>
      </Space>

      {/* Enable / Continue setup */}
      <ModalForm<AdminConfirmTwoFASetupRequest>
        title={t('pages.settings.security.twoFa.modal.title', 'Enable 2FA')}
        open={setupOpen}
        onOpenChange={(open) => {
          setSetupOpen(open);
          if (!open) {
            setSetupPayload(null);
            setSetupPayloadLoading(false);
          }
        }}
        onFinish={async (values) => {
          try {
            const res = await adminConfirmTwoFASetup(values, { skipErrorHandler: true });
            if (!res?.success) {
              throw new Error(res?.message || t('pages.settings.security.twoFa.messages.confirmFailed', 'Enable 2FA failed'));
            }
            await showBackupCodesModal(res.data?.backup_codes);
            message.success(t('pages.settings.security.twoFa.messages.enabled', '2FA enabled'));
            setSetupOpen(false);
            await reload();
            if (force === '2fa') history.replace('/rbac/roles');
            return true;
          } catch (e: any) {
            message.error(
              getRequestErrorMessage(e) ||
                t('pages.settings.security.twoFa.messages.confirmFailed', 'Enable 2FA failed'),
            );
            return false;
          }
        }}
      >
        {setupPayloadLoading ? (
          <Alert type="info" showIcon message={t('pages.settings.security.twoFa.loading', 'Loading setup...')} />
        ) : setupPayload?.qr_code ? (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Alert
              type="success"
              showIcon
              message={t('pages.settings.security.twoFa.modal.hint', 'Scan QR code with your authenticator app')}
            />
            <div style={{ display: 'flex', justifyContent: 'center' }}>
              <img
                src={setupPayload.qr_code}
                alt="2fa-qr"
                style={{ width: 180, height: 180, borderRadius: 8, border: '1px solid rgba(0,0,0,0.06)' }}
              />
            </div>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>{t('pages.settings.security.twoFa.secret', 'Secret')}:</Typography.Text>{' '}
              <Typography.Text code copyable>
                {setupPayload.secret || ''}
              </Typography.Text>
            </Typography.Paragraph>
            <Typography.Paragraph style={{ marginBottom: 0 }}>
              <Typography.Text strong>{t('pages.settings.security.twoFa.otpauthUrl', 'otpauth_url')}:</Typography.Text>{' '}
              <Typography.Text code copyable>
                {setupPayload.otpauth_url || ''}
              </Typography.Text>
            </Typography.Paragraph>
          </Space>
        ) : (
          <Alert
            type="error"
            showIcon
            message={t('pages.settings.security.twoFa.messages.setupMissing', 'Setup data is missing, please retry')}
          />
        )}

        <ProFormText.Password
          name="current_password"
          label={t('pages.settings.security.form.currentPassword', 'Current Password')}
          rules={[{ required: true }]}
        />
        <ProFormText
          name="totp_code"
          label={t('pages.settings.security.twoFa.form.code', 'Authenticator code')}
          rules={[
            { required: true },
            { pattern: /^\d{6}$/, message: t('pages.settings.security.twoFa.form.codeInvalid', 'Enter 6 digits') },
          ]}
        />
      </ModalForm>

      {/* Rebind (two-step) */}
      <ModalForm
        form={rebindForm}
        title={t('pages.settings.security.twoFa.rebind.title', 'Rebind 2FA')}
        open={rebindOpen}
        onOpenChange={(open) => {
          setRebindOpen(open);
          if (!open) {
            setRebindStep(1);
            setRebindPayload(null);
            setRebindPassword('');
            rebindForm.resetFields();
          }
        }}
        onFinish={async (values) => {
          try {
            if (rebindStep === 1) {
              const v = values as unknown as AdminStartTwoFARebindRequest;
              const res = await adminStartTwoFARebind(v, { skipErrorHandler: true });
              if (!res?.success) {
                throw new Error(res?.message || t('pages.settings.security.twoFa.rebind.startFailed', 'Rebind failed'));
              }
              setRebindPassword(v.current_password || '');
              setRebindPayload(res.data ?? null);
              setRebindStep(2);
              rebindForm.resetFields(['new_totp_code']);
              return false;
            }

            const code = String((values as any)?.new_totp_code || '').trim();
            const req: AdminConfirmTwoFASetupRequest = {
              current_password: rebindPassword,
              totp_code: code,
            };
            const res = await adminConfirmTwoFASetup(req, { skipErrorHandler: true });
            if (!res?.success) {
              throw new Error(res?.message || t('pages.settings.security.twoFa.rebind.confirmFailed', 'Rebind failed'));
            }
            message.success(t('pages.settings.security.twoFa.messages.rebound', '2FA rebound'));
            setRebindOpen(false);
            await reload();
            return true;
          } catch (e: any) {
            message.error(
              getRequestErrorMessage(e) || t('pages.settings.security.twoFa.rebind.confirmFailed', 'Rebind failed'),
            );
            return false;
          }
        }}
      >
        {rebindStep === 1 ? (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Alert
              type="warning"
              showIcon
              message={t('pages.settings.security.twoFa.rebind.verifyTitle', 'Verify your current 2FA')}
              description={t(
                'pages.settings.security.twoFa.rebind.verifyDesc',
                'Enter your password and the current code from your authenticator app.',
              )}
            />
            <ProFormText.Password
              name="current_password"
              label={t('pages.settings.security.form.currentPassword', 'Current Password')}
              rules={[{ required: true }]}
            />
            <ProFormText
              name="totp_code"
              label={t('pages.settings.security.twoFa.form.currentCode', 'Current authenticator code')}
              rules={[
                { required: true },
                { pattern: /^\d{6}$/, message: t('pages.settings.security.twoFa.form.codeInvalid', 'Enter 6 digits') },
              ]}
            />
          </Space>
        ) : (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Alert
              type="success"
              showIcon
              message={t('pages.settings.security.twoFa.rebind.scanTitle', 'Scan new QR code')}
              description={t(
                'pages.settings.security.twoFa.rebind.scanDesc',
                'Scan the new QR code with your authenticator app, then enter the new 6-digit code.',
              )}
            />
            {rebindPayload?.qr_code ? (
              <>
                <div style={{ display: 'flex', justifyContent: 'center' }}>
                  <img
                    src={rebindPayload.qr_code}
                    alt="2fa-qr"
                    style={{ width: 180, height: 180, borderRadius: 8, border: '1px solid rgba(0,0,0,0.06)' }}
                  />
                </div>
                <Typography.Paragraph style={{ marginBottom: 0 }}>
                  <Typography.Text strong>{t('pages.settings.security.twoFa.secret', 'Secret')}:</Typography.Text>{' '}
                  <Typography.Text code copyable>
                    {rebindPayload.secret || ''}
                  </Typography.Text>
                </Typography.Paragraph>
                <Typography.Paragraph style={{ marginBottom: 0 }}>
                  <Typography.Text strong>{t('pages.settings.security.twoFa.otpauthUrl', 'otpauth_url')}:</Typography.Text>{' '}
                  <Typography.Text code copyable>
                    {rebindPayload.otpauth_url || ''}
                  </Typography.Text>
                </Typography.Paragraph>
              </>
            ) : (
              <Alert
                type="error"
                showIcon
                message={t('pages.settings.security.twoFa.messages.setupMissing', 'Setup data is missing, please retry')}
              />
            )}
            <ProFormText
              name="new_totp_code"
              label={t('pages.settings.security.twoFa.form.newCode', 'New authenticator code')}
              rules={[
                { required: true },
                { pattern: /^\d{6}$/, message: t('pages.settings.security.twoFa.form.codeInvalid', 'Enter 6 digits') },
              ]}
            />
            <Button
              type="link"
              onClick={() => {
                setRebindStep(1);
                setRebindPayload(null);
                setRebindPassword('');
                rebindForm.resetFields();
              }}
              style={{ paddingInline: 0 }}
            >
              {t('pages.settings.security.twoFa.rebind.back', 'Back')}
            </Button>
          </Space>
        )}
      </ModalForm>

      {/* Disable 2FA */}
      <ModalForm<AdminDisableTwoFARequest>
        title={t('pages.settings.security.twoFa.disable.title', 'Disable 2FA')}
        open={disableOpen}
        onOpenChange={setDisableOpen}
        onFinish={async (values) => {
          try {
            const res = await adminDisableTwoFA(values, { skipErrorHandler: true });
            if (!res?.success) {
              throw new Error(res?.message || t('pages.settings.security.twoFa.disable.failed', 'Disable failed'));
            }
            message.success(t('pages.settings.security.twoFa.disable.success', '2FA disabled'));
            setDisableOpen(false);
            await reload();
            return true;
          } catch (e: any) {
            message.error(getRequestErrorMessage(e) || t('pages.settings.security.twoFa.disable.failed', 'Disable failed'));
            return false;
          }
        }}
      >
        <Alert
          type="warning"
          showIcon
          message={t('pages.settings.security.twoFa.disable.warnTitle', 'Disabling 2FA reduces account security')}
          description={t(
            'pages.settings.security.twoFa.disable.warnDesc',
            'You will lose the additional protection provided by 2FA.',
          )}
        />
        <ProFormText.Password
          name="current_password"
          label={t('pages.settings.security.form.currentPassword', 'Current Password')}
          rules={[{ required: true }]}
        />
        <ProFormText
          name="totp_code"
          label={t('pages.settings.security.twoFa.form.currentCode', 'Current authenticator code')}
          rules={[
            { required: true },
            { pattern: /^\d{6}$/, message: t('pages.settings.security.twoFa.form.codeInvalid', 'Enter 6 digits') },
          ]}
        />
      </ModalForm>

      {/* Change password */}
      <ModalForm<AdminChangeMyPasswordRequest>
        title={t('pages.settings.security.password.modalTitle', 'Change Password')}
        open={pwdOpen}
        onOpenChange={setPwdOpen}
        onFinish={async (values) => {
          try {
            const res = await adminChangeMyPassword(values, { skipErrorHandler: true });
            if (!res?.success)
              throw new Error(
                res?.message || t('pages.settings.security.messages.changePasswordFailed', 'Change password failed'),
              );

            const nextToken = res.data?.auth_token || '';
            if (nextToken) {
              setAdminToken(nextToken);
            } else {
              clearAdminToken();
              message.success(
                t('pages.settings.security.messages.passwordChangedRelogin', 'Password changed, please log in again'),
              );
              history.replace('/user/login');
              return true;
            }

            message.success(t('pages.settings.security.messages.passwordChanged', 'Password changed'));
            setPwdOpen(false);
            await reload();

            if (twoFA?.required && !twoFA?.enabled) {
              history.replace('/settings/security?force=2fa');
            } else {
              history.replace('/rbac/roles');
            }
            return true;
          } catch (e: any) {
            message.error(
              getRequestErrorMessage(e) ||
                t('pages.settings.security.messages.changePasswordFailed', 'Change password failed'),
            );
            return false;
          }
        }}
      >
        <ProFormText.Password
          name="current_password"
          label={t('pages.settings.security.form.currentPassword', 'Current Password')}
          rules={[{ required: true }]}
        />
        <ProFormText.Password
          name="new_password"
          label={t('pages.settings.security.form.newPassword', 'New Password')}
          dependencies={['current_password']}
          rules={[
            { required: true },
            ({ getFieldValue }) => ({
              validator: async (_, value) => {
                const next = String(value ?? '').trim();
                const current = String(getFieldValue('current_password') ?? '').trim();
                if (next && current && next === current) {
                  return Promise.reject(
                    new Error(
                      t(
                        'pages.settings.security.password.validation.noChange',
                        'New password must be different from current password',
                      ),
                    ),
                  );
                }
                return validateNewPassword(_, value);
              },
            }),
          ]}
        />
        <ProFormText.Password
          name="confirm_password"
          label={t('pages.settings.security.form.confirmPassword', 'Confirm Password')}
          dependencies={['new_password']}
          rules={[
            { required: true },
            ({ getFieldValue }) => ({
              validator: async (_, value) => {
                const v = String(value ?? '');
                const expected = String(getFieldValue('new_password') ?? '');
                if (!v || !expected) return Promise.resolve();
                if (v !== expected) {
                  return Promise.reject(
                    new Error(t('pages.settings.security.password.validation.mismatch', 'Passwords do not match')),
                  );
                }
                return Promise.resolve();
              },
            }),
          ]}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default SecurityPage;
