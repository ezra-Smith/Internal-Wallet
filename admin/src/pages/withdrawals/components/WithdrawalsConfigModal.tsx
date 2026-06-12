import React from 'react';
import { ModalForm, ProFormDigit, ProFormSwitch } from '@ant-design/pro-components';
import {
  SettingOutlined,
  DownloadOutlined,
  SwapOutlined,
  SafetyCertificateOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { Alert } from 'antd';
import { createStyles } from 'antd-style';
import type { ApprovalRuleConfig } from '../types';

const useStyles = createStyles(() => ({
  modalSubtitle: {
    fontSize: 14,
    color: '#6b7280',
    fontWeight: 400,
    marginTop: 4,
  },
  section: {
    borderRadius: 12,
    border: '1px solid #e5e7eb',
    padding: '20px',
    marginBottom: 16,
  },
  sectionHeader: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: 20,
  },
  sectionHeaderLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  },
  sectionIcon: {
    width: 40,
    height: 40,
    borderRadius: 10,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 18,
  },
  sectionIconWithdrawal: {
    background: 'linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)',
    color: '#3b82f6',
  },
  sectionIconTransfer: {
    background: 'linear-gradient(135deg, #f5f3ff 0%, #ede9fe 100%)',
    color: '#8b5cf6',
  },
  sectionIconWhitelist: {
    background: 'linear-gradient(135deg, #ecfdf5 0%, #d1fae5 100%)',
    color: '#10b981',
  },
  sectionTitleWrap: {
    display: 'flex',
    flexDirection: 'column',
  },
  sectionTitle: {
    fontSize: 15,
    fontWeight: 600,
    color: '#1f2937',
  },
  sectionSubtitle: {
    fontSize: 13,
    color: '#9ca3af',
    marginTop: 2,
  },
  formItem: {
    marginBottom: 16,
  },
  formLabel: {
    fontSize: 14,
    fontWeight: 500,
    color: '#374151',
    marginBottom: 8,
    display: 'block',
  },
  formHint: {
    fontSize: 12,
    color: '#f59e0b',
    marginTop: 6,
  },
  inputWrapper: {
    background: '#f9fafb',
    borderRadius: 8,
    padding: '12px 16px',
    fontSize: 15,
    color: '#1f2937',
    border: '1px solid #e5e7eb',
  },
  noticeBox: {
    marginTop: 8,
  },
  noticeTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 14,
    fontWeight: 600,
    color: '#d97706',
    marginBottom: 8,
  },
  noticeList: {
    margin: 0,
    paddingLeft: 16,
    fontSize: 13,
    color: '#d97706',
    lineHeight: 1.8,
  },
}));

interface WithdrawalsConfigModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialValues: ApprovalRuleConfig;
  onFinish: (values: ApprovalRuleConfig) => Promise<boolean>;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const WithdrawalsConfigModal: React.FC<WithdrawalsConfigModalProps> = ({
  open,
  onOpenChange,
  initialValues,
  onFinish,
  t,
}) => {
  const { styles } = useStyles();

  return (
    <ModalForm<ApprovalRuleConfig>
      title={
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <SettingOutlined style={{ color: '#3b82f6' }} />
            <span>{t('pages.approval.configModal.title', '出金审核配置')}</span>
          </div>
          <div className={styles.modalSubtitle}>
            {t('pages.approval.configModal.subtitle', '配置提现和转账的审核开关及触发阈值')}
          </div>
        </div>
      }
      open={open}
      onOpenChange={onOpenChange}
      initialValues={initialValues}
      onFinish={onFinish}
      width={560}
      modalProps={{
        destroyOnClose: true,
        maskClosable: false,
        styles: {
          body: {
            maxHeight: 'calc(80vh - 110px)',
            overflowY: 'auto',
          },
        },
      }}
      submitter={{
        searchConfig: {
          submitText: t('pages.approval.configModal.save', '保存配置'),
          resetText: t('common.cancel', '取消'),
        },
      }}
    >
      {/* 提现审核配置 */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <div className={styles.sectionHeaderLeft}>
            <div className={`${styles.sectionIcon} ${styles.sectionIconWithdrawal}`}>
              <DownloadOutlined />
            </div>
            <div className={styles.sectionTitleWrap}>
              <div className={styles.sectionTitle}>
                {t('pages.approval.configModal.withdrawal.title', '提现审核')}
              </div>
              <div className={styles.sectionSubtitle}>
                {t('pages.approval.configModal.withdrawal.subtitle', '用户提现申请审核')}
              </div>
            </div>
          </div>
          <ProFormSwitch
            name="withdrawalEnabled"
            noStyle
            fieldProps={{
              style: { marginBottom: 0 },
            }}
          />
        </div>

        <div className={styles.formItem}>
          <span className={styles.formLabel}>
            {t('pages.approval.configModal.singleThreshold', '单笔金额审核阈值（USDT）')}
          </span>
          <ProFormDigit
            name="withdrawalThreshold"
            noStyle
            fieldProps={{
              precision: 0,
              min: 0,
              style: { width: '100%' },
              className: styles.inputWrapper,
            }}
            rules={[{ required: true, message: t('common.required', '请输入') }]}
          />
          <div className={styles.formHint}>
            {t(
              'pages.approval.configModal.withdrawal.singleHint',
              '提现单笔金额（折合USDT）大于等于此值时需要审核',
            )}
          </div>
        </div>

        <div className={styles.formItem}>
          <span className={styles.formLabel}>
            {t('pages.approval.configModal.dailyThreshold', '单日总额审核阈值（USDT）')}
          </span>
          <ProFormDigit
            name="withdrawalDailyThreshold"
            noStyle
            fieldProps={{
              precision: 0,
              min: 0,
              style: { width: '100%' },
              className: styles.inputWrapper,
            }}
            rules={[{ required: true, message: t('common.required', '请输入') }]}
          />
          <div className={styles.formHint}>
            {t(
              'pages.approval.configModal.withdrawal.dailyHint',
              '用户单日提现总额（折合USDT）大于等于此值时需要审核',
            )}
          </div>
        </div>
      </div>

      {/* 转账审核配置 */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <div className={styles.sectionHeaderLeft}>
            <div className={`${styles.sectionIcon} ${styles.sectionIconTransfer}`}>
              <SwapOutlined />
            </div>
            <div className={styles.sectionTitleWrap}>
              <div className={styles.sectionTitle}>
                {t('pages.approval.configModal.transfer.title', '转账审核')}
              </div>
              <div className={styles.sectionSubtitle}>
                {t('pages.approval.configModal.transfer.subtitle', '用户内部转账审核')}
              </div>
            </div>
          </div>
          <ProFormSwitch
            name="transferEnabled"
            noStyle
            fieldProps={{
              style: { marginBottom: 0 },
            }}
          />
        </div>

        <div className={styles.formItem}>
          <span className={styles.formLabel}>
            {t('pages.approval.configModal.singleThreshold', '单笔金额审核阈值（USDT）')}
          </span>
          <ProFormDigit
            name="transferThreshold"
            noStyle
            fieldProps={{
              precision: 0,
              min: 0,
              style: { width: '100%' },
              className: styles.inputWrapper,
            }}
            rules={[{ required: true, message: t('common.required', '请输入') }]}
          />
          <div className={styles.formHint}>
            {t(
              'pages.approval.configModal.transfer.singleHint',
              '转账单笔金额（折合USDT）大于等于此值时需要审核',
            )}
          </div>
        </div>

        <div className={styles.formItem}>
          <span className={styles.formLabel}>
            {t('pages.approval.configModal.dailyThreshold', '单日总额审核阈值（USDT）')}
          </span>
          <ProFormDigit
            name="transferDailyThreshold"
            noStyle
            fieldProps={{
              precision: 0,
              min: 0,
              style: { width: '100%' },
              className: styles.inputWrapper,
            }}
            rules={[{ required: true, message: t('common.required', '请输入') }]}
          />
          <div className={styles.formHint}>
            {t(
              'pages.approval.configModal.transfer.dailyHint',
              '用户单日转账总额（折合USDT）大于等于此值时需要审核',
            )}
          </div>
        </div>
      </div>

      {/* 白名单免审配置 */}
      <div className={styles.section}>
        <div className={styles.sectionHeader}>
          <div className={styles.sectionHeaderLeft}>
            <div className={`${styles.sectionIcon} ${styles.sectionIconWhitelist}`}>
              <SafetyCertificateOutlined />
            </div>
            <div className={styles.sectionTitleWrap}>
              <div className={styles.sectionTitle}>
                {t('pages.approval.configModal.whitelist.title', '白名单免审配置')}
              </div>
              <div className={styles.sectionSubtitle}>
                {t('pages.approval.configModal.whitelist.subtitle', '白名单用户提现免审阈值设置')}
              </div>
            </div>
          </div>
          <ProFormSwitch
            name="whitelistEnabled"
            noStyle
            fieldProps={{
              style: { marginBottom: 0 },
            }}
          />
        </div>

        <div className={styles.formItem}>
          <span className={styles.formLabel}>
            {t('pages.approval.configModal.whitelist.threshold', '白名单免审阈值（USDT）')}
          </span>
          <ProFormDigit
            name="whitelistThreshold"
            noStyle
            fieldProps={{
              precision: 0,
              min: 0,
              style: { width: '100%' },
              className: styles.inputWrapper,
            }}
            rules={[{ required: true, message: t('common.required', '请输入') }]}
          />
          <div className={styles.formHint}>
            {t(
              'pages.approval.configModal.whitelist.thresholdHint',
              '白名单用户单笔提现金额（折合USDT）小于此值时自动审核通过',
            )}
          </div>
        </div>

        <div className={styles.formItem}>
          <span className={styles.formLabel}>
            {t('pages.approval.configModal.whitelist.dailyLimit', '白名单免审总额（USDT）')}
          </span>
          <ProFormDigit
            name="whitelistDailyLimit"
            noStyle
            fieldProps={{
              precision: 0,
              min: 0,
              style: { width: '100%' },
              className: styles.inputWrapper,
            }}
            rules={[{ required: true, message: t('common.required', '请输入') }]}
          />
          <div className={styles.formHint}>
            {t(
              'pages.approval.configModal.whitelist.dailyLimitHint',
              '白名单用户单日免审总额上限，超出后需要人工审核',
            )}
          </div>
        </div>
      </div>

      {/* 注意事项 */}
      <Alert
        className={styles.noticeBox}
        type="warning"
        showIcon={false}
        message={
          <div>
            <div className={styles.noticeTitle}>
              <WarningOutlined />
              {t('pages.approval.configModal.notice.title', '注意事项')}
            </div>
            <ul className={styles.noticeList}>
              <li>
                {t(
                  'pages.approval.configModal.notice.item1',
                  '关闭审核开关后，该类型的所有申请将自动通过',
                )}
              </li>
              <li>
                {t(
                  'pages.approval.configModal.notice.item2',
                  '阈值设置为0时，所有金额都需要审核',
                )}
              </li>
              <li>
                {t(
                  'pages.approval.configModal.notice.item3',
                  '金额换算以实时汇率折合为USDT进行比较',
                )}
              </li>
              <li>
                {t(
                  'pages.approval.configModal.notice.item4',
                  '白名单用户提现会优先判断免审条件，满足条件则自动审核通过',
                )}
              </li>
              <li>
                {t(
                  'pages.approval.configModal.notice.item5',
                  '白名单免审仍会产生审核记录，并标记为"白名单免审"',
                )}
              </li>
            </ul>
          </div>
        }
      />
    </ModalForm>
  );
};

export default WithdrawalsConfigModal;
