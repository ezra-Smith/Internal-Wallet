import { customApiV1AdminCurrenciesIconUploadPOST } from "@/api/generated/assets";
import {
  App,
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Space,
  Upload,
  type UploadProps,
} from "antd";
import React, { useEffect, useMemo } from "react";

export type EditCurrencyFormValues = {
  asset_code: string;
  asset_name: string;
  precision: number;
  icon_url: string;
  status?: number;
};

type Props = {
  open: boolean;
  confirmLoading?: boolean;
  assetCode: string;
  initialValues?: EditCurrencyFormValues;
  canUpdate: boolean;
  onCancel: () => void;
  onSubmit: (values: EditCurrencyFormValues) => void | Promise<void>;
  t: (
    id: string,
    defaultMessage: string,
    values?: Record<string, any>
  ) => string;
};

const EditCurrencyModal: React.FC<Props> = ({
  open,
  confirmLoading,
  assetCode,
  initialValues,
  canUpdate,
  onCancel,
  onSubmit,
  t,
}) => {
  const { message } = App.useApp();
  const [form] = Form.useForm<EditCurrencyFormValues>();

  const isValidHttpUrl = (value: string): boolean => {
    const v = (value || "").trim();
    if (!v) return true;
    try {
      const u = new URL(v);
      return u.protocol === "http:" || u.protocol === "https:";
    } catch {
      return false;
    }
  };

  useEffect(() => {
    if (!open) return;
    form.setFieldsValue({
      asset_code: initialValues?.asset_code || assetCode || "",
      asset_name: initialValues?.asset_name || "",
      precision: Number(initialValues?.precision ?? 0),
      icon_url: initialValues?.icon_url || "",
      status: initialValues?.status ?? 2,
    });
  }, [open, assetCode, initialValues, form]);

  const uploadProps: UploadProps = useMemo(
    () => ({
      accept: "image/png,image/jpeg,image/webp,image/gif",
      showUploadList: false,
      maxCount: 1,
      disabled: !canUpdate,
      beforeUpload: (file) => {
        const maxMB = 2;
        if (file.size > maxMB * 1024 * 1024) {
          message.error(
            t(
              "pages.assets.currencies.messages.fileTooLarge",
              "File too large (max {max}MB)",
              { max: maxMB }
            )
          );
          return false;
        }
        const allowed = ["image/png", "image/jpeg", "image/webp", "image/gif"];
        if (file.type && !allowed.includes(file.type)) {
          message.error(
            t(
              "pages.assets.currencies.messages.unsupportedFileType",
              "Unsupported file type"
            )
          );
          return false;
        }
        const status = form.getFieldValue("status");
        if (status === 1) {
          message.error(
            t(
              "pages.assets.currencies.messages.onlyEditWhenDisabled",
              "Only editable when disabled"
            )
          );
          return false;
        }
        return true;
      },
      customRequest: async ({ file, onSuccess, onError }) => {
        try {
          const status = form.getFieldValue("status");
          if (status === 1) {
            throw new Error(
              t(
                "pages.assets.currencies.messages.onlyEditWhenDisabled",
                "Only editable when disabled"
              )
            );
          }
          if (!assetCode) throw new Error("asset_code missing");
          const res = await customApiV1AdminCurrenciesIconUploadPOST(
            { file: file as Blob, asset_code: assetCode },
            { skipErrorHandler: true }
          );
          const url = res?.data?.url || "";
          if (!res?.success || !url) throw new Error(res?.message || "");
          form.setFieldValue("icon_url", url);
          message.success(
            t("pages.assets.currencies.messages.uploaded", "Uploaded")
          );
          onSuccess?.({ url }, undefined as any);
        } catch (e: any) {
          message.error(
            e?.message ||
              t(
                "pages.assets.currencies.messages.uploadFailed",
                "Upload failed"
              )
          );
          onError?.(e);
        }
      },
    }),
    [assetCode, canUpdate, form, message, t]
  );

  return (
    <Modal
      open={open}
      title={t("pages.assets.currencies.edit.title.cn", "修改弹窗")}
      onCancel={onCancel}
      confirmLoading={confirmLoading}
      footer={null}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="asset_name"
          label={t("pages.assets.currencies.columns.name.cn", "名称")}
          rules={[
            {
              required: true,
              message: t(
                "pages.assets.currencies.messages.nameRequired.cn",
                "请输入名称"
              ),
            },
          ]}
        >
          <Input disabled={!canUpdate} maxLength={64} />
        </Form.Item>

        <Form.Item
          name="precision"
          label={t("pages.assets.currencies.columns.precision.cn", "精度")}
          rules={[
            {
              required: true,
              message: t(
                "pages.assets.currencies.messages.precisionRequired.cn",
                "请输入精度"
              ),
            },
          ]}
        >
          <InputNumber<number>
            disabled={!canUpdate}
            min={0}
            max={36}
            step={1}
            precision={0}
            style={{ width: "100%" }}
            formatter={(v) => String(v ?? "").replace(/[^\d]/g, "")}
            parser={(v) => {
              const s = String(v ?? "").replace(/[^\d]/g, "");
              return s === "" ? 0 : Number(s);
            }}
            controls
          />
        </Form.Item>

        <Form.Item
          name="icon_url"
          label={t("pages.assets.currencies.columns.iconUrl.cn", "图标URL")}
          rules={[
            {
              validator: async (_, v) => {
                if (!isValidHttpUrl(String(v || ""))) {
                  throw new Error(
                    t(
                      "pages.assets.currencies.messages.invalidIconUrl.cn",
                      "图标URL格式不正确（仅支持http/https）"
                    )
                  );
                }
              },
            },
          ]}
        >
          <Input
            disabled={!canUpdate}
            placeholder="https://..."
            addonAfter={
              <Upload {...uploadProps}>
                <Button size="small" disabled={!canUpdate}>
                  {t("pages.assets.currencies.actions.uploadIcon.cn", "上传")}
                </Button>
              </Upload>
            }
          />
        </Form.Item>

        <Form.Item name="asset_code" hidden>
          <Input />
        </Form.Item>
        <Form.Item name="status" hidden>
          <Input />
        </Form.Item>
      </Form>

      <Space style={{ width: "100%", justifyContent: "flex-end" }}>
        <Button onClick={onCancel}>{t("common.cancel.cn", "取消")}</Button>
        <Button
          type="primary"
          loading={confirmLoading}
          disabled={!canUpdate}
          onClick={async () => {
            const values = await form.validateFields();
            const iconUrl = (values.icon_url || "").trim();
            if (!isValidHttpUrl(iconUrl)) {
              message.error(
                t(
                  "pages.assets.currencies.messages.invalidIconUrl.cn",
                  "图标URL格式不正确（仅支持http/https）"
                )
              );
              return;
            }
            const status = values.status ?? form.getFieldValue("status");
            if (status === 1) {
              message.error(
                t(
                  "pages.assets.currencies.messages.onlyEditWhenDisabled.cn",
                  "仅禁用状态可修改"
                )
              );
              return;
            }
            await onSubmit({
              ...values,
              asset_code: assetCode || values.asset_code,
              precision: Number(values.precision ?? 0),
              icon_url: iconUrl,
            });
          }}
        >
          {t("common.save.cn", "保存")}
        </Button>
      </Space>
    </Modal>
  );
};

export default EditCurrencyModal;
