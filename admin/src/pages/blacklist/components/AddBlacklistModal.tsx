import React, { useEffect } from 'react';
import { Alert, Form } from 'antd';
import { ModalForm, ProFormText, ProFormSelect, ProFormTextArea } from '@ant-design/pro-components';
import { PlusOutlined } from '@ant-design/icons';
import { createStyles } from 'antd-style';
import { RISK_LEVEL_CONFIG, SOURCE_CONFIG } from '../constants';
import type { NetworkType, RiskLevel, SourceType } from '../types';

const useStyles = createStyles(() => ({
  modalContent: {
    '.ant-pro-form': {
      paddingTop: 8,
    },
  },
  subtitle: {
    fontSize: 14,
    color: '#6b7280',
    marginBottom: 24,
  },
  formRow: {
    display: 'grid',
    gridTemplateColumns: '1fr 1fr',
    gap: 16,
    '@media (max-width: 576px)': {
      gridTemplateColumns: '1fr',
    },
  },
  alertWrapper: {
    marginTop: 16,
    marginBottom: 8,
  },
  alert: {
    borderRadius: 8,
    background: '#fffbeb',
    border: '1px solid #fde68a',
  },
  alertTitle: {
    fontWeight: 600,
    color: '#d97706',
    marginBottom: 4,
  },
  alertContent: {
    color: '#92400e',
    fontSize: 13,
  },
}));

interface AddBlacklistModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (values: AddBlacklistFormValues) => Promise<boolean>;
  networks: { value: string; label: string }[];
  defaultNetwork?: string;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

export interface AddBlacklistFormValues {
  address: string;
  network: NetworkType;
  riskLevel: RiskLevel;
  source: SourceType;
  reason: string;
}

const AddBlacklistModal: React.FC<AddBlacklistModalProps> = ({
  open,
  onOpenChange,
  onSubmit,
  networks,
  defaultNetwork,
  t,
}) => {
  const { styles } = useStyles();
  const [form] = Form.useForm<AddBlacklistFormValues>();

  useEffect(() => {
    if (!open) return;
    const current = (form.getFieldValue('network') as string | undefined) || '';
    if (current) return;
    const first = defaultNetwork || networks[0]?.value || '';
    if (first) {
      form.setFieldsValue({ network: first } as any);
    }
  }, [defaultNetwork, form, networks, open]);

  const riskLevelOptions = Object.entries(RISK_LEVEL_CONFIG).map(([key, config]) => ({
    value: key,
    label: t(`pages.blacklist.riskLevel.${key}`, config.label),
  }));

  const sourceOptions = Object.entries(SOURCE_CONFIG).map(([key, config]) => ({
    value: key,
    label: t(`pages.blacklist.source.${key}`, config.label),
  }));

  return (
    <ModalForm<AddBlacklistFormValues>
      title={
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <PlusOutlined style={{ color: '#ef4444' }} />
          {t('pages.blacklist.modal.addTitle', '添加黑名单地址')}
        </span>
      }
      open={open}
      onOpenChange={onOpenChange}
      width={520}
      form={form}
      modalProps={{
        destroyOnClose: true,
        maskClosable: false,
      }}
      submitter={{
        searchConfig: {
          submitText: t('pages.blacklist.modal.submitBtn', '添加到黑名单'),
          resetText: t('common.cancel', '取消'),
        },
        submitButtonProps: {
          style: {
            background: '#ef4444',
            borderColor: '#ef4444',
          },
        },
      }}
      onFinish={onSubmit}
      initialValues={{
        network: defaultNetwork || networks[0]?.value,
        riskLevel: 'high',
        source: 'user_report',
      }}
    >
      <div className={styles.modalContent}>
        <div className={styles.subtitle}>
          {t('pages.blacklist.modal.addSubtitle', '添加需要监测的链上敏感地址')}
        </div>

        <ProFormText
          name="address"
          label={t('pages.blacklist.form.address', '钱包地址')}
          placeholder={t('pages.blacklist.form.addressPlaceholder', '输入完整的钱包地址')}
          rules={[
            {
              required: true,
              message: t('pages.blacklist.form.addressRequired', '请输入钱包地址'),
            },
            {
              validator: async (_, value) => {
                const v = String(value ?? '').trim();
                if (!v) return;
                if (v.length > 255) {
                  throw new Error(t('pages.blacklist.form.addressTooLong', '地址过长'));
                }
                if (/\s/.test(v)) {
                  throw new Error(t('pages.blacklist.form.addressHasSpace', '地址不能包含空格'));
                }
              },
            },
          ]}
        />

        <div className={styles.formRow}>
          <ProFormSelect
            name="network"
            label={t('pages.blacklist.form.network', '区块链网络')}
            options={networks}
            rules={[
              {
                required: true,
                message: t('pages.blacklist.form.networkRequired', '请选择区块链网络'),
              },
            ]}
          />

          <ProFormSelect
            name="riskLevel"
            label={t('pages.blacklist.form.riskLevel', '风险等级')}
            options={riskLevelOptions}
            rules={[
              {
                required: true,
                message: t('pages.blacklist.form.riskLevelRequired', '请选择风险等级'),
              },
            ]}
          />
        </div>

        <ProFormSelect
          name="source"
          label={t('pages.blacklist.form.source', '信息来源')}
          options={sourceOptions}
        />

        <ProFormTextArea
          name="reason"
          label={t('pages.blacklist.form.reason', '标记原因')}
          placeholder={t('pages.blacklist.form.reasonPlaceholder', '详细说明将此地址加入黑名单的原因')}
          rules={[
            {
              required: true,
              message: t('pages.blacklist.form.reasonRequired', '请输入标记原因'),
            },
          ]}
          fieldProps={{
            rows: 4,
          }}
        />

        <div className={styles.alertWrapper}>
          <Alert
            className={styles.alert}
            type="warning"
            showIcon
            message={
              <div>
                <div className={styles.alertTitle}>
                  {t('pages.blacklist.modal.alertTitle', '监测提示')}
                </div>
                <div className={styles.alertContent}>
                  {t(
                    'pages.blacklist.modal.alertContent',
                    '添加后将自动开启监测，系统会实时检测用户交易中是否涉及该地址，并在发现时发出警告。',
                  )}
                </div>
              </div>
            }
          />
        </div>
      </div>
    </ModalForm>
  );
};

export default AddBlacklistModal;
