import { LockOutlined, SafetyCertificateOutlined, UserOutlined } from '@ant-design/icons';
import { LoginForm, ProFormText } from '@ant-design/pro-components';
import { Helmet, history, useIntl, useModel } from '@umijs/max';
import { Alert, App, Button, Divider, Typography } from 'antd';
import { createStyles } from 'antd-style';
import React, { useEffect, useState } from 'react';
import { flushSync } from 'react-dom';
import { Footer, SelectLang } from '@/components';
import {
  adminAdminMFABindTwoFA,
  adminAdminMFACaptchaGenerate,
  adminAdminMFACaptchaValidate,
  adminAdminMFAGetTwoFAQrCode,
  adminAdminMFALogin,
  adminAdminMFAVerifyTwoFA,
} from '@/api/generated/auth';
import { setAdminToken } from '@/utils/auth';
import Settings from '../../../../config/defaultSettings';

const motionStyles = `
@keyframes login-rise {
  0% { transform: translateY(18px); opacity: 0; }
  100% { transform: translateY(0); opacity: 1; }
}
@keyframes login-sheen {
  0% { background-position: 0% 50%; }
  100% { background-position: 100% 50%; }
}
@keyframes login-float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-6px); }
}
`;

const useStyles = createStyles(({ token }) => ({
  lang: {
    width: 40,
    height: 40,
    lineHeight: '40px',
    position: 'fixed',
    right: 24,
    top: 24,
    zIndex: 5,
    borderRadius: 12,
    display: 'grid',
    placeItems: 'center',
    color: '#e2e8f0',
    background: 'rgba(15, 23, 42, 0.6)',
    border: '1px solid rgba(148, 163, 184, 0.35)',
    boxShadow: '0 10px 30px rgba(15, 23, 42, 0.25)',
    backdropFilter: 'blur(10px)',
    transition: 'all 0.2s ease',
    ':hover': {
      transform: 'translateY(-1px)',
      backgroundColor: 'rgba(30, 41, 59, 0.75)',
    },
  },
  container: {
    position: 'relative',
    minHeight: '100vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    background:
      'radial-gradient(1200px 800px at 10% 5%, rgba(59, 130, 246, 0.18), transparent 60%), radial-gradient(900px 600px at 100% 15%, rgba(251, 191, 36, 0.15), transparent 55%), linear-gradient(120deg, #0b1120 0%, #0f172a 45%, #111827 100%)',
    fontFamily: '"Space Grotesk", "Sora", "Noto Sans SC", sans-serif',
    '&::before': {
      content: '""',
      position: 'absolute',
      inset: 0,
      background:
        'radial-gradient(circle at 75% 20%, rgba(14, 116, 144, 0.2) 0%, rgba(14, 116, 144, 0) 55%)',
      pointerEvents: 'none',
    },
    '&::after': {
      content: '""',
      position: 'absolute',
      inset: 0,
      backgroundImage:
        'linear-gradient(rgba(255, 255, 255, 0.06) 1px, transparent 1px), linear-gradient(90deg, rgba(255, 255, 255, 0.05) 1px, transparent 1px)',
      backgroundSize: '120px 120px, 120px 120px',
      opacity: 0.18,
      pointerEvents: 'none',
    },
  },
  shell: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '72px 24px 40px',
    position: 'relative',
    zIndex: 1,
  },
  content: {
    width: '100%',
    maxWidth: 1200,
    display: 'grid',
    gridTemplateColumns: 'minmax(0, 1.1fr) minmax(0, 480px)',
    gap: 56,
    alignItems: 'center',
    '@media (max-width: 1024px)': {
      gridTemplateColumns: '1fr',
      gap: 36,
    },
  },
  hero: {
    color: '#e2e8f0',
    display: 'flex',
    flexDirection: 'column',
    gap: 18,
    animation: 'login-rise 0.7s ease both',
    '@media (max-width: 1024px)': {
      order: 2,
      textAlign: 'left',
    },
  },
  heroBadge: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 8,
    padding: '6px 14px',
    fontSize: 12,
    letterSpacing: 0.8,
    textTransform: 'uppercase',
    borderRadius: 999,
    background: 'rgba(15, 23, 42, 0.7)',
    border: '1px solid rgba(148, 163, 184, 0.35)',
    width: 'fit-content',
    animation: 'login-float 6s ease-in-out infinite',
  },
  heroTitle: {
    fontSize: 46,
    fontWeight: 600,
    lineHeight: 1.05,
    letterSpacing: 1,
    fontFamily: '"Teko", "Bebas Neue", "Noto Sans SC", sans-serif',
    '@media (max-width: 640px)': {
      fontSize: 34,
    },
  },
  heroSubtitle: {
    fontSize: 16,
    color: 'rgba(226, 232, 240, 0.78)',
    maxWidth: 520,
  },
  heroDescription: {
    color: 'rgba(226, 232, 240, 0.64)',
    fontSize: 13,
    maxWidth: 520,
  },
  heroChips: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 10,
  },
  heroChip: {
    padding: '6px 12px',
    borderRadius: 999,
    fontSize: 12,
    border: '1px solid rgba(148, 163, 184, 0.25)',
    background: 'rgba(30, 41, 59, 0.55)',
    color: 'rgba(226, 232, 240, 0.9)',
  },
  heroStats: {
    display: 'grid',
    gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
    gap: 12,
    '@media (max-width: 1024px)': {
      gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
    },
    '@media (max-width: 640px)': {
      gridTemplateColumns: '1fr',
    },
  },
  heroStat: {
    padding: '14px 16px',
    borderRadius: 16,
    background: 'rgba(15, 23, 42, 0.65)',
    border: '1px solid rgba(148, 163, 184, 0.2)',
    boxShadow: '0 12px 24px rgba(15, 23, 42, 0.25)',
  },
  heroStatValue: {
    fontSize: 15,
    fontWeight: 600,
    color: '#f8fafc',
  },
  heroStatLabel: {
    fontSize: 12,
    color: 'rgba(226, 232, 240, 0.6)',
    marginTop: 4,
  },
  heroFooter: {
    fontSize: 12,
    color: 'rgba(226, 232, 240, 0.55)',
  },
  formPanel: {
    animation: 'login-rise 0.7s ease 0.1s both',
  },
  formCard: {
    position: 'relative',
    padding: '28px 30px 32px',
    borderRadius: 24,
    background: 'rgba(248, 250, 252, 0.96)',
    border: '1px solid rgba(148, 163, 184, 0.25)',
    boxShadow: '0 28px 60px rgba(15, 23, 42, 0.28)',
    backdropFilter: 'blur(10px)',
    '@media (max-width: 640px)': {
      padding: '24px 20px',
    },
  },
  loginForm: {
    width: '100%',
    ':global(.ant-pro-form-login-container)': {
      background: 'transparent',
      boxShadow: 'none',
      padding: 0,
    },
    ':global(.ant-pro-form-login-main)': {
      width: '100%',
      minWidth: 0,
      maxWidth: 'none',
    },
    ':global(.ant-pro-form-login-title)': {
      fontSize: 26,
      fontWeight: 600,
      color: '#0f172a',
      letterSpacing: 0.3,
      fontFamily: '"Space Grotesk", "Sora", "Noto Sans SC", sans-serif',
    },
    ':global(.ant-pro-form-login-desc)': {
      fontSize: 13,
      color: '#475569',
      marginTop: 6,
    },
    ':global(.ant-pro-form-login-logo img)': {
      width: 34,
      height: 34,
    },
    ':global(.ant-input-affix-wrapper)': {
      borderRadius: 12,
      padding: '8px 12px',
      borderColor: 'rgba(148, 163, 184, 0.4)',
      background: '#f8fafc',
    },
    ':global(.ant-input-affix-wrapper:focus, .ant-input-affix-wrapper-focused)': {
      borderColor: token.colorPrimary,
      boxShadow: `0 0 0 2px ${token.colorPrimaryBg}`,
    },
    ':global(.ant-btn-primary)': {
      height: 44,
      borderRadius: 12,
      fontWeight: 600,
      background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 100%)',
      border: 'none',
      boxShadow: '0 12px 28px rgba(15, 23, 42, 0.25)',
      backgroundSize: '160% 160%',
      animation: 'login-sheen 8s ease infinite',
    },
    ':global(.ant-btn-primary:hover)': {
      opacity: 0.92,
    },
  },
  captchaRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    padding: '10px 12px',
    borderRadius: 12,
    background: 'rgba(15, 23, 42, 0.04)',
    border: '1px dashed rgba(148, 163, 184, 0.35)',
    marginBottom: 12,
  },
  captchaImage: {
    height: 32,
    borderRadius: 6,
    background: '#fff',
  },
  captchaMeta: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
  },
  twoFaBox: {
    padding: '12px 14px',
    borderRadius: 14,
    background: 'rgba(15, 23, 42, 0.04)',
    border: '1px solid rgba(148, 163, 184, 0.25)',
    marginBottom: 14,
  },
  footer: {
    position: 'relative',
    zIndex: 1,
    padding: '8px 24px 24px',
    color: 'rgba(226, 232, 240, 0.85)',
    ':global(.ant-layout-footer)': {
      background: 'transparent !important',
    },
    ':global(.ant-pro-global-footer)': {
      color: 'rgba(226, 232, 240, 0.85) !important',
      background: 'transparent !important',
    },
    ':global(.ant-pro-global-footer-copyright)': {
      color: 'rgba(226, 232, 240, 0.85) !important',
    },
    ':global(.ant-pro-global-footer a)': {
      color: 'rgba(226, 232, 240, 0.92) !important',
    },
  },
}));

const Lang = () => {
  const { styles } = useStyles();
  return (
    <div className={styles.lang} data-lang>
      {SelectLang && <SelectLang />}
    </div>
  );
};

const LoginMessage: React.FC<{ content: string }> = ({ content }) => {
  return (
    <Alert
      style={{
        marginBottom: 24,
      }}
      message={content}
      type="error"
      showIcon
    />
  );
};

type LoginStage = 'password' | '2fa_required' | '2fa_not_bound';

function toDataUrl(maybeBase64: string | undefined): string | undefined {
  if (!maybeBase64) return undefined;
  if (maybeBase64.startsWith('data:')) return maybeBase64;
  return `data:image/png;base64,${maybeBase64}`;
}

function normalizeTotpCode(code: string | undefined): string {
  if (!code) return '';
  return code.replace(/[^\d]/g, '');
}

function getAuthErrorMessage(
  error: any,
  fallback: string,
  stage: LoginStage | undefined,
  t: (id: string, defaultMessage: string) => string,
) {
  const status = error?.response?.status;

  // Prefer structured API error payload when available.
  const payload = (error?.data ?? error?.info ?? error?.response?.data) as
    | {
        code?: number | string;
        message?: string;
        details?: {
          reason?: string;
          metadata?: {
            code?: number | string;
            http_status?: number | string;
          };
        };
      }
    | undefined;

  const reason = payload?.details?.reason;
  const bizCode = Number(payload?.code ?? payload?.details?.metadata?.code);

  if (reason === 'AUTH_ACCOUNT_DISABLED')
    return t('pages.login.errors.accountDisabled', 'Account disabled, please contact the administrator');
  if (reason === 'AUTH_ACCOUNT_LOCKED' || status === 423)
    return t('pages.login.errors.accountLocked', 'Account locked, please try again later');

  if (reason === 'AUTH_2FA_CODE_INVALID' || bizCode === 11006)
    return t('pages.login.errors.twoFaCodeInvalid', 'Invalid 2FA code');
  if (reason === 'AUTH_2FA_SESSION_EXPIRED')
    return t('pages.login.errors.twoFaSessionExpired', '2FA session expired, please log in again');
  if (reason === 'AUTH_2FA_TOO_MANY_ATTEMPTS')
    return t('pages.login.errors.twoFaTooManyAttempts', 'Too many 2FA attempts, please try again later');

  if (status === 401 && stage === 'password')
    return t('pages.login.errors.invalidCredentials', 'Incorrect email or password');
  if (status === 403) return t('pages.login.errors.forbidden', 'Access denied, please contact the administrator');
  if (typeof status === 'number' && status >= 500)
    return t('pages.login.errors.serverError', 'Server error, please try again later');

  const bizMessage = payload?.message;
  if (typeof bizMessage === 'string' && bizMessage.trim()) return bizMessage;

  if (typeof error?.message === 'string' && error.message.trim()) return error.message;

  return fallback;
}

const Login: React.FC = () => {
  const { styles } = useStyles();
  const [stage, setStage] = useState<LoginStage>('password');
  const [loginError, setLoginError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);

  const [captchaKey, setCaptchaKey] = useState<string>();
  const [captchaImage, setCaptchaImage] = useState<string>();
  const [captchaLoading, setCaptchaLoading] = useState(false);

  const [sessionIdentifier, setSessionIdentifier] = useState<string>();
  const [authKey, setAuthKey] = useState<string>();
  const [qrCode, setQrCode] = useState<string>();
  const [secretKey, setSecretKey] = useState<string>();

  const { initialState, setInitialState } = useModel('@@initialState');
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string, values?: Record<string, any>) =>
    intl.formatMessage({ id, defaultMessage }, values);

  const fetchUserInfo = async () => {
    const userInfo = await initialState?.fetchUserInfo?.();
    if (userInfo) {
      flushSync(() => {
        setInitialState((s) => ({
          ...s,
          currentUser: userInfo,
        }));
      });
    }
    return userInfo;
  };

  const redirectToHome = () => {
    const urlParams = new URL(window.location.href).searchParams;
    const redirect = urlParams.get('redirect');
    const next = redirect?.startsWith('/') ? redirect : '/rbac/roles';
    history.replace(next);
  };

  const redirectAfterLogin = async () => {
    const user = await fetchUserInfo();
    if (user?.require_password_change) {
      history.replace('/settings/security?force=password');
      return;
    }
    redirectToHome();
  };

  const showBackupCodesModal = async (codes: string[] | undefined) => {
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
  };

  const refreshCaptcha = async () => {
    setCaptchaLoading(true);
    try {
      const res = await adminAdminMFACaptchaGenerate({}, { skipErrorHandler: true });
      const payload = res?.data;
      if (!payload?.captcha_key || !payload?.captcha_image) {
        throw new Error(t('pages.login.errors.captchaLoadFailed', 'Failed to load captcha'));
      }
      setCaptchaKey(payload.captcha_key);
      setCaptchaImage(toDataUrl(payload.captcha_image));
    } catch (e: any) {
      setLoginError(
        getAuthErrorMessage(e, t('pages.login.errors.captchaLoadFailed', 'Failed to load captcha'), stage, t),
      );
    } finally {
      setCaptchaLoading(false);
    }
  };

  useEffect(() => {
    void refreshCaptcha();
  }, []);

  const resetLogin = async () => {
    setStage('password');
    setSessionIdentifier(undefined);
    setAuthKey(undefined);
    setQrCode(undefined);
    setSecretKey(undefined);
    setLoginError(undefined);
    await refreshCaptcha();
  };

  const handlePasswordLogin = async (values: {
    email: string;
    password: string;
    captcha_answer: string;
  }) => {
    try {
      setLoginError(undefined);
      if (!captchaKey) {
        await refreshCaptcha();
        throw new Error(t('pages.login.errors.captchaNotReady', 'Captcha not ready, please retry'));
      }

      const validate = await adminAdminMFACaptchaValidate(
        {
          captcha_key: captchaKey,
          captcha_answer: values.captcha_answer,
        },
        { skipErrorHandler: true },
      );
      const validatePayload = validate?.data;
      if (!validatePayload?.temp_key)
        throw new Error(validate?.message || t('pages.login.errors.invalidCaptcha', 'Invalid captcha'));

      const loginRes = await adminAdminMFALogin(
        {
          temp_key: validatePayload.temp_key,
          email: values.email,
          password: values.password,
        },
        { skipErrorHandler: true },
      );
      const loginPayload = loginRes?.data;

      if (loginPayload?.status === 'login_successful' && loginPayload.auth_token) {
        setAdminToken(loginPayload.auth_token);
        message.success(t('pages.login.success', 'Login successful!'));
        await redirectAfterLogin();
        return;
      }

      if (loginPayload?.status === '2fa_required' && loginPayload.session_identifier) {
        setSessionIdentifier(loginPayload.session_identifier);
        setStage('2fa_required');
        message.info(t('pages.login.messages.twoFaRequired', '2FA verification required'));
        return;
      }

      if (loginPayload?.status === '2fa_not_bound' && loginPayload.auth_key) {
        setAuthKey(loginPayload.auth_key);
        const qr = await adminAdminMFAGetTwoFAQrCode(
          { auth_key: loginPayload.auth_key },
          { skipErrorHandler: true },
        );
        const qrPayload = qr?.data;
        setQrCode(toDataUrl(qrPayload?.qr_code));
        setSecretKey(qrPayload?.secret_key);
        setStage('2fa_not_bound');
        message.info(t('pages.login.messages.twoFaBindRequired', 'Please bind 2FA first'));
        return;
      }

      throw new Error(t('pages.login.errors.unsupportedState', 'Unsupported login state'));
    } catch (error: any) {
      await refreshCaptcha();
      setLoginError(
        getAuthErrorMessage(error, t('pages.login.failure', 'Login failed, please try again!'), 'password', t),
      );
    }
  };

  const handleTwoFAVerify = async (values: { totp_code: string }) => {
    try {
      setLoginError(undefined);
      if (!sessionIdentifier)
        throw new Error(t('pages.login.errors.missingSession', 'Missing session identifier'));

      const totpCode = normalizeTotpCode(values.totp_code);
      const res = await adminAdminMFAVerifyTwoFA(
        {
          session_identifier: sessionIdentifier,
          totp_code: totpCode,
        },
        { skipErrorHandler: true },
      );
      const payload = res?.data;
      if (!payload?.auth_token) throw new Error(t('pages.login.errors.twoFaVerifyFailed', '2FA verify failed'));

      setAdminToken(payload.auth_token);
      message.success(t('pages.login.success', 'Login successful!'));
      await redirectAfterLogin();
    } catch (error: any) {
      setLoginError(
        getAuthErrorMessage(
          error,
          t('pages.login.errors.twoFaVerifyRetry', '2FA verification failed, please try again!'),
          '2fa_required',
          t,
        ),
      );
    }
  };

  const handleTwoFABind = async (values: { totp_code: string }) => {
    try {
      setLoginError(undefined);
      if (!authKey) throw new Error(t('pages.login.errors.missingAuthKey', 'Missing auth key'));
      if (!secretKey) throw new Error(t('pages.login.errors.missingSecretKey', 'Missing secret key'));

      const totpCode = normalizeTotpCode(values.totp_code);
      const res = await adminAdminMFABindTwoFA(
        {
          auth_key: authKey,
          totp_code: totpCode,
          secret_key: secretKey,
        },
        { skipErrorHandler: true },
      );
      const payload = res?.data;
      if (!payload?.auth_token) throw new Error(t('pages.login.errors.twoFaBindFailed', '2FA bind failed'));

      setAdminToken(payload.auth_token);
      await showBackupCodesModal(payload.backup_codes);

      message.success(t('pages.login.success', 'Login successful!'));
      await redirectAfterLogin();
    } catch (error: any) {
      setLoginError(
        getAuthErrorMessage(
          error,
          t('pages.login.errors.twoFaBindRetry', '2FA binding failed, please try again!'),
          '2fa_not_bound',
          t,
        ),
      );
    }
  };

  const heroChips = [
    t('pages.login.hero.chip.mpc', 'MPC custody'),
    t('pages.login.hero.chip.policy', 'Policy approvals'),
    t('pages.login.hero.chip.audit', 'Audit-ready trails'),
  ];

  const heroStats = [
    {
      value: t('pages.login.hero.stat.security.value', 'Multi-factor'),
      label: t('pages.login.hero.stat.security.label', 'Defense in depth'),
    },
    {
      value: t('pages.login.hero.stat.routing.value', 'Global coverage'),
      label: t('pages.login.hero.stat.routing.label', 'Settlement rails'),
    },
    {
      value: t('pages.login.hero.stat.monitoring.value', 'Always on'),
      label: t('pages.login.hero.stat.monitoring.label', 'Risk monitoring'),
    },
  ];

  return (
    <div className={styles.container}>
      <style>{motionStyles}</style>
      <Helmet>
        <title>
          {intl.formatMessage({
            id: 'menu.login',
            defaultMessage: '登录页',
          })}
          {Settings.title && ` - ${Settings.title}`}
        </title>
      </Helmet>
      <Lang />
      <div className={styles.shell}>
        <div className={styles.content}>
          <section className={styles.hero}>
            <div className={styles.heroBadge}>
              <SafetyCertificateOutlined />
              <span>{t('pages.login.hero.kicker', 'Security-first operations')}</span>
            </div>
            <div className={styles.heroTitle}>{t('pages.login.title', 'Internal Wallet')}</div>
            <div className={styles.heroSubtitle}>{t('pages.login.subtitle', 'Secure treasury operations console')}</div>
            <Typography.Text className={styles.heroDescription}>
              {t(
                'pages.login.hero.description',
                'MPC custody, policy approvals, and real-time audit trails keep every transfer accountable.',
              )}
            </Typography.Text>
            <div className={styles.heroChips}>
              {heroChips.map((chip) => (
                <span key={chip} className={styles.heroChip}>
                  {chip}
                </span>
              ))}
            </div>
            <div className={styles.heroStats}>
              {heroStats.map((stat) => (
                <div key={stat.label} className={styles.heroStat}>
                  <div className={styles.heroStatValue}>{stat.value}</div>
                  <div className={styles.heroStatLabel}>{stat.label}</div>
                </div>
              ))}
            </div>
            <div className={styles.heroFooter}>
              {t('pages.login.hero.footer', 'Authorized personnel only · All actions are logged')}
            </div>
          </section>

          <section className={styles.formPanel}>
            <div className={styles.formCard}>
              <LoginForm
                className={styles.loginForm}
                contentStyle={{
                  width: '100%',
                  minWidth: 0,
                }}
                logo={<img alt="logo" src="/logo.svg" />}
                title={t('pages.login.title', 'Internal Wallet')}
                subTitle={t('pages.login.subtitle', 'Secure treasury operations console')}
                submitter={{
                  submitButtonProps: {
                    loading: submitting,
                    size: 'large',
                    block: true,
                  },
                }}
                onFinish={async (values: any) => {
                  setSubmitting(true);
                  try {
                    if (stage === 'password') {
                      await handlePasswordLogin(values);
                      return true;
                    }
                    if (stage === '2fa_required') {
                      await handleTwoFAVerify(values);
                      return true;
                    }
                    if (stage === '2fa_not_bound') {
                      await handleTwoFABind(values);
                      return true;
                    }
                    return true;
                  } finally {
                    setSubmitting(false);
                  }
                }}
              >
                {loginError && <LoginMessage content={loginError} />}

                {stage === 'password' && (
                  <>
                    <ProFormText
                      name="email"
                      fieldProps={{
                        size: 'large',
                        prefix: <UserOutlined />,
                        autoComplete: 'username',
                      }}
                      placeholder={t('pages.login.placeholders.email', 'Admin email')}
                      rules={[
                        {
                          required: true,
                          message: t('pages.login.validation.emailRequired', 'Please enter your email'),
                        },
                        { type: 'email', message: t('pages.login.validation.emailInvalid', 'Invalid email format') },
                      ]}
                    />

                    <ProFormText.Password
                      name="password"
                      fieldProps={{
                        size: 'large',
                        prefix: <LockOutlined />,
                        autoComplete: 'current-password',
                      }}
                      placeholder={t('pages.login.placeholders.password', 'Password')}
                      rules={[
                        { required: true, message: t('pages.login.validation.passwordRequired', 'Please enter your password') },
                      ]}
                    />

                    <div className={styles.captchaRow}>
                      <div className={styles.captchaMeta}>
                        <Typography.Text type="secondary">
                          {t('pages.login.captcha.label', 'Captcha:')}
                        </Typography.Text>
                        {captchaImage ? (
                          <img
                            alt="captcha"
                            src={captchaImage}
                            className={styles.captchaImage}
                            style={{ cursor: 'pointer' }}
                            onClick={() => void refreshCaptcha()}
                          />
                        ) : (
                          <Typography.Text type="secondary">
                            {t('pages.login.captcha.loading', 'Loading...')}
                          </Typography.Text>
                        )}
                      </div>
                      <Button size="small" onClick={() => void refreshCaptcha()} loading={captchaLoading}>
                        {t('pages.login.captcha.refresh', 'Refresh')}
                      </Button>
                    </div>

                    <ProFormText
                      name="captcha_answer"
                      fieldProps={{
                        size: 'large',
                        autoComplete: 'off',
                      }}
                      placeholder={t('pages.login.captcha.placeholder', 'Enter captcha')}
                      rules={[
                        { required: true, message: t('pages.login.validation.captchaRequired', 'Please enter captcha') },
                      ]}
                    />
                  </>
                )}

                {(stage === '2fa_required' || stage === '2fa_not_bound') && (
                  <>
                    <Divider style={{ margin: '12px 0 16px' }} />

                    {stage === '2fa_not_bound' && (
                      <div className={styles.twoFaBox}>
                        <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
                          {t(
                            'pages.login.twoFa.bindHint',
                            'Use an authenticator app (Google Authenticator / 1Password / Authy, etc.) to scan the QR code or enter the key manually.',
                          )}
                        </Typography.Paragraph>
                        {qrCode && (
                          <div style={{ marginBottom: 12 }}>
                            <img
                              alt="2fa qrcode"
                              src={qrCode}
                              style={{ width: 180, height: 180, borderRadius: 10 }}
                            />
                          </div>
                        )}
                        {secretKey && (
                          <Typography.Paragraph copyable style={{ marginBottom: 0 }}>
                            {t('pages.login.twoFa.secretLabel', 'Secret')}: {secretKey}
                          </Typography.Paragraph>
                        )}
                      </div>
                    )}

                    <ProFormText
                      name="totp_code"
                      fieldProps={{
                        size: 'large',
                        autoComplete: 'one-time-code',
                      }}
                      placeholder={t('pages.login.twoFa.placeholder', 'Enter 2FA code (6 digits)')}
                      rules={[
                        { required: true, message: t('pages.login.validation.twoFaRequired', 'Please enter 2FA code') },
                      ]}
                    />

                    <Button type="link" onClick={() => void resetLogin()}>
                      {t('pages.login.actions.backToLogin', 'Back to login')}
                    </Button>
                  </>
                )}
              </LoginForm>
            </div>
          </section>
        </div>
      </div>
      <div className={styles.footer}>
        <Footer />
      </div>
    </div>
  );
};

export default Login;
