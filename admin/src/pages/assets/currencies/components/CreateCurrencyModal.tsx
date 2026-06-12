import type { UploadProps } from 'antd';
import { App, Avatar, Button, Form, Input, InputNumber, Modal, Radio, Space, Upload } from 'antd';
import React, { useEffect, useMemo } from 'react';
import { customApiV1AdminCurrenciesIconUploadPOST } from '@/api/generated/assets';
import type { I18nT } from '../types';

export type CreateCurrencyFormValues = {
  asset_code: string;
  asset_name?: string;
  precision?: number;
  status?: number;
  icon_url?: string;
};

type Props = {
  open: boolean;
  confirmLoading: boolean;
  canUpload: boolean;
  onCancel: () => void;
  onSubmit: (values: CreateCurrencyFormValues) => void | Promise<void>;
  t: I18nT;
};

const isValidHttpUrl = (value: string): boolean => {
  const v = (value || '').trim();
  if (!v) return true;
  try {
    const u = new URL(v);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
};

const normalizeAssetCode = (value: string): string => (value || '').trim().toUpperCase();

const CreateCurrencyModal: React.FC<Props> = ({ open, confirmLoading, canUpload, onCancel, onSubmit, t }) => {
  const { message } = App.useApp();
  const [form] = Form.useForm<CreateCurrencyFormValues>();

  const iconUrl = Form.useWatch('icon_url', form);
  const assetCode = Form.useWatch('asset_code', form);

  useEffect(() => {
    if (!open) return;
    form.setFieldsValue({ precision: 18, status: 2, icon_url: '' });
  }, [open, form]);

  const uploadProps: UploadProps = useMemo(
    () => ({
      accept: 'image/png,image/jpeg,image/webp,image/gif',
      showUploadList: false,
      maxCount: 1,
      beforeUpload: (file) => {
        const maxMB = 2;
        if (file.size > maxMB * 1024 * 1024) {
          message.error(t('pages.assets.currencies.messages.fileTooLarge', 'File too large (max {max}MB)', { max: maxMB }));
          return false;
        }
        const allowed = ['image/png', 'image/jpeg', 'image/webp', 'image/gif'];
        if (file.type && !allowed.includes(file.type)) {
          message.error(t('pages.assets.currencies.messages.unsupportedFileType', 'Unsupported file type'));
          return false;
        }
        return true;
      },
      customRequest: async ({ file, onSuccess, onError }) => {
        try {
          const code = normalizeAssetCode(String(assetCode || ''));
          const res = await customApiV1AdminCurrenciesIconUploadPOST(
            { file: file as Blob, ...(code ? { asset_code: code } : {}) },
            { skipErrorHandler: true },
          );
          const url = res?.data?.url || '';
          if (!res?.success || !url) throw new Error(res?.message || '');

          form.setFieldValue('icon_url', url);
          onSuccess?.({ url }, undefined as any);
        } catch (e: any) {
          message.error(e?.message || t('pages.assets.currencies.messages.uploadFailed', 'Upload failed'));
          onError?.(e);
        }
      },
    }),
    [assetCode, form, message, t],
  );

  return (
    <Modal
      open={open}
      title={t('pages.assets.currencies.create.title', 'Create currency')}
      confirmLoading={confirmLoading}
      okText={t('pages.assets.currencies.create.ok', 'Create')}
      cancelText={t('pages.assets.currencies.create.cancel', 'Cancel')}
      onCancel={() => {
        form.resetFields();
        onCancel();
      }}
      onOk={() => form.submit()}
      destroyOnClose
    >
      <Form
        form={form}
        layout="vertical"
        onFinish={async (values) => {
          const payload: CreateCurrencyFormValues = {
            asset_code: normalizeAssetCode(values.asset_code),
            asset_name: (values.asset_name || '').trim(),
            precision: values.precision,
            status: values.status,
            icon_url: (values.icon_url || '').trim(),
          };
          if (!payload.asset_name) delete payload.asset_name;
          if (!payload.icon_url) delete payload.icon_url;
          await onSubmit(payload);
        }}
      >
        <Form.Item
          name="asset_code"
          label={t('pages.assets.currencies.create.assetCode', 'Asset code')}
          rules={[
            { required: true, message: t('pages.assets.currencies.create.assetCodeRequired', 'Asset code is required') },
            { pattern: /^[A-Za-z0-9_-]{1,32}$/, message: t('pages.assets.currencies.create.assetCodeInvalid', '1-32 chars: A-Z 0-9 _ -') },
          ]}
        >
          <Input placeholder="e.g. USDT" maxLength={32} />
        </Form.Item>

        <Form.Item
          name="asset_name"
          label={t('pages.assets.currencies.create.assetName', 'Name')}
          rules={[{ max: 64, message: t('pages.assets.currencies.create.assetNameTooLong', 'Max 64 chars') }]}
        >
          <Input placeholder={t('pages.assets.currencies.create.assetNamePlaceholder', 'Optional, defaults to code')} maxLength={64} />
        </Form.Item>

        <Form.Item
          name="precision"
          label={t('pages.assets.currencies.create.precision', 'Precision')}
          rules={[{ required: true, message: t('pages.assets.currencies.create.precisionRequired', 'Precision is required') }]}
        >
          <InputNumber min={0} max={30} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item
          name="status"
          label={t('pages.assets.currencies.create.status', 'Status')}
          rules={[{ required: true, message: t('pages.assets.currencies.create.statusRequired', 'Status is required') }]}
        >
          <Radio.Group>
            <Radio value={2}>{t('pages.assets.currencies.status.disabled', 'Disabled')}</Radio>
            <Radio value={1}>{t('pages.assets.currencies.status.enabled', 'Enabled')}</Radio>
          </Radio.Group>
        </Form.Item>

        <Form.Item
          name="icon_url"
          label={t('pages.assets.currencies.create.iconUrl', 'Icon URL')}
          rules={[
            {
              validator: async (_, value) => {
                if (!isValidHttpUrl(String(value || ''))) {
                  throw new Error(t('pages.assets.currencies.messages.invalidIconUrl', 'Invalid icon URL (http/https only)'));
                }
              },
            },
          ]}
        >
          <Input placeholder="https://..." maxLength={2048} />
        </Form.Item>

        <Space style={{ width: '100%', justifyContent: 'space-between' }}>
          <Space>
            <Avatar shape="square" size={32} src={String(iconUrl || '').trim() || undefined}>
              {(normalizeAssetCode(String(assetCode || '')) || '?').slice(0, 1)}
            </Avatar>
            {canUpload && (
              <Upload {...uploadProps}>
                <Button size="small">{t('pages.assets.currencies.actions.uploadIcon', 'Upload')}</Button>
              </Upload>
            )}
          </Space>
        </Space>
      </Form>
    </Modal>
  );
};

export default CreateCurrencyModal;
