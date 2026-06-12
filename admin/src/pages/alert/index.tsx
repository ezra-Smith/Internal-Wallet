import {
  adminCreateAlertConfig,
  adminDeleteAlertConfig,
  adminGetAlertConfig,
  adminGetAlertConfigWithStats,
  adminListAlertConfigs,
  adminToggleAlertConfig,
  adminUpdateAlertConfig,
} from "@/api/generated/alert";
import { useRbac } from "@/hooks/useRbac";
import type { ActionType, ProColumns } from "@ant-design/pro-components";
import {
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from "@ant-design/pro-components";
import { useIntl } from "@umijs/max";
import {
  App,
  Button,
  Descriptions,
  Divider,
  Form,
  Grid,
  Progress,
  Space,
  Switch,
  Tag,
  Typography,
} from "antd";
import React, { useMemo, useRef, useState } from "react";

type AlertConfigItem = {
  id?: string;
  name?: string;
  description?: string;
  alert_type?: string;
  threshold_usd?: string;
  time_window_seconds?: number;
  monitor_web3_withdraw?: boolean;
  monitor_web2_withdraw?: boolean;
  monitor_internal_transfer?: boolean;
  enabled?: boolean;
  test_mode?: boolean;
  cooldown_seconds?: number;
  created_at?: string;
  updated_at?: string;
};

type AlertStats = {
  current_window_amount?: string;
  current_window_tx_count?: number;
  progress_percentage?: string;
  is_in_cooldown?: boolean;
  cooldown_remaining_seconds?: number;
};

const PERM = {
  list: "AlertListAlertConfigs",
  create: "AlertCreateAlertConfig",
  detail: "AlertGetAlertConfig",
  detailWithStats: "AlertGetAlertConfigWithStats",
  update: "AlertUpdateAlertConfig",
  del: "AlertDeleteAlertConfig",
  toggle: "AlertToggleAlertConfig",
} as const;

const AlertConfigsPage: React.FC = () => {
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string) =>
    intl.formatMessage({ id, defaultMessage });

  const { useBreakpoint } = Grid;
  const screens = useBreakpoint();

  const actionRef = useRef<ActionType | null>(null);

  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);

  const [editLoading, setEditLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);

  const [current, setCurrent] = useState<AlertConfigItem | null>(null);
  const [detailConfig, setDetailConfig] = useState<AlertConfigItem | null>(
    null
  );
  const [detailStats, setDetailStats] = useState<AlertStats | null>(null);

  const modalWidth = screens.xl ? 860 : screens.md ? 720 : "100%";
  const descCols = screens.md ? 2 : 1;

  const refresh = () => actionRef.current?.reload?.();

  const toId = (v: any) => (v === undefined || v === null ? "" : String(v));
  const switchItemStyle: React.CSSProperties = { marginBottom: 8 };
  const alertTypeValueEnum = {
    platform_transaction: {
      text: t(
        "pages.alert.configs.alertType.platformTransaction",
        "Platform Transaction Total"
      ),
    },
  } as const;
  const alertTypeText = (v?: string) => {
    const key = (v || "").trim();
    if (!key) return "-";
    const map: Record<string, string> = {
      platform_transaction: t(
        "pages.alert.configs.alertType.platformTransaction",
        "Platform Transaction Total"
      ),
    };
    return map[key] || key;
  };

  const openEdit = async (row: AlertConfigItem) => {
    if (!canRpc(PERM.update)) return;
    const id = row.id;
    if (!id) return;
    setEditLoading(true);
    try {
      const res = await adminGetAlertConfig(
        { configId: toId(id) },
        { skipErrorHandler: true }
      );
      if (!res?.success) throw new Error(res?.message || "");
      const cfg = (res as any)?.data?.config as AlertConfigItem | undefined;
      setCurrent(cfg || row);
      setEditOpen(true);
    } catch (e: any) {
      message.error(
        e?.message ||
          t(
            "pages.alert.configs.messages.loadDetailFailed",
            "Failed to load detail"
          )
      );
    } finally {
      setEditLoading(false);
    }
  };

  const openDetail = async (row: AlertConfigItem) => {
    if (!canRpc(PERM.detailWithStats) && !canRpc(PERM.detail)) return;
    const id = row.id;
    if (!id) return;
    setDetailLoading(true);
    try {
      if (canRpc(PERM.detailWithStats)) {
        const res = await adminGetAlertConfigWithStats(
          { configId: toId(id) },
          { skipErrorHandler: true }
        );
        if (!res?.success) throw new Error(res?.message || "");
        setDetailConfig(((res as any)?.data?.config as AlertConfigItem) || row);
        setDetailStats(((res as any)?.data?.stats as AlertStats) || null);
      } else {
        const res = await adminGetAlertConfig(
          { configId: toId(id) },
          { skipErrorHandler: true }
        );
        if (!res?.success) throw new Error(res?.message || "");
        setDetailConfig(((res as any)?.data?.config as AlertConfigItem) || row);
        setDetailStats(null);
      }
      setDetailOpen(true);
    } catch (e: any) {
      message.error(
        e?.message ||
          t(
            "pages.alert.configs.messages.loadDetailFailed",
            "Failed to load detail"
          )
      );
    } finally {
      setDetailLoading(false);
    }
  };

  const confirmDelete = (row: AlertConfigItem) => {
    if (!canRpc(PERM.del)) return;
    const id = row.id;
    if (!id) return;
    modal.confirm({
      title: t("pages.alert.configs.actions.deleteTitle", "Delete config"),
      content: (
        <Space direction="vertical" size={4}>
          <Typography.Text>
            {t(
              "pages.alert.configs.actions.deleteConfirm",
              "This action cannot be undone."
            )}
          </Typography.Text>
          <Typography.Text type="secondary">
            {t("pages.alert.configs.columns.name", "Name")}: {row.name || "-"}
          </Typography.Text>
        </Space>
      ),
      okButtonProps: { danger: true },
      okText: t("pages.alert.configs.actions.delete", "Delete"),
      cancelText: t("pages.alert.configs.actions.cancel", "Cancel"),
      onOk: async () => {
        const res = await adminDeleteAlertConfig(
          { configId: toId(id) },
          { skipErrorHandler: true }
        );
        if (!res?.success) {
          message.error(
            res?.message ||
              t("pages.alert.configs.messages.deleteFailed", "Failed to delete")
          );
          return;
        }
        message.success(
          t("pages.alert.configs.messages.deleteSuccess", "Deleted")
        );
        refresh();
      },
    });
  };

  const confirmToggle = (row: AlertConfigItem, nextEnabled: boolean) => {
    if (!canRpc(PERM.toggle)) return;
    const id = row.id;
    if (!id) return;
    modal.confirm({
      title: nextEnabled
        ? t("pages.alert.configs.actions.enableTitle", "Enable config")
        : t("pages.alert.configs.actions.disableTitle", "Disable config"),
      content: (
        <Space direction="vertical" size={4}>
          <Typography.Text>
            {nextEnabled
              ? t(
                  "pages.alert.configs.actions.enableConfirm",
                  "Enable this config?"
                )
              : t(
                  "pages.alert.configs.actions.disableConfirm",
                  "Disable this config?"
                )}
          </Typography.Text>
          <Typography.Text type="secondary">
            {t("pages.alert.configs.columns.name", "Name")}: {row.name || "-"}
          </Typography.Text>
        </Space>
      ),
      okText: t("pages.alert.configs.actions.confirm", "Confirm"),
      cancelText: t("pages.alert.configs.actions.cancel", "Cancel"),
      onOk: async () => {
        const res = await adminToggleAlertConfig(
          { configId: toId(id) },
          { enabled: nextEnabled },
          { skipErrorHandler: true }
        );
        if (!res?.success) {
          message.error(
            res?.message ||
              t(
                "pages.alert.configs.messages.toggleFailed",
                "Failed to update status"
              )
          );
          return;
        }
        message.success(
          t("pages.alert.configs.messages.toggleSuccess", "Status updated")
        );
        refresh();
      },
    });
  };

  const renderMonitors = (row: AlertConfigItem) => {
    const tags: React.ReactNode[] = [];
    if (row.monitor_web3_withdraw) tags.push(<Tag key="w3">Web3</Tag>);
    if (row.monitor_web2_withdraw) tags.push(<Tag key="w2">Web2</Tag>);
    if (row.monitor_internal_transfer)
      tags.push(
        <Tag key="it">
          {t("pages.alert.configs.monitor.internal", "Internal")}
        </Tag>
      );
    if (!tags.length)
      return <Typography.Text type="secondary">-</Typography.Text>;
    return (
      <Space size={4} wrap>
        {tags}
      </Space>
    );
  };

  const columns: ProColumns<AlertConfigItem>[] = useMemo(() => {
    return [
      {
        title: t("pages.alert.configs.columns.id", "ID"),
        dataIndex: "id",
        width: 120,
        align: "center",
        copyable: true,
        ellipsis: true,
        render: (_, row) =>
          row.id ? (
            <Typography.Text code>{row.id}</Typography.Text>
          ) : (
            <Typography.Text type="secondary">-</Typography.Text>
          ),
        responsive: ["md"],
      },
      {
        title: t("pages.alert.configs.columns.name", "Name"),
        dataIndex: "name",
        width: 220,
        align: "center",
        ellipsis: true,
      },
      {
        title: t("pages.alert.configs.columns.alertType", "Alert Type"),
        dataIndex: "alert_type",
        width: 180,
        align: "center",
        valueEnum: alertTypeValueEnum as any,
        render: (_, row) => {
          const v = (row.alert_type || "").trim();
          if (!v) return <Typography.Text type="secondary">-</Typography.Text>;
          const text = (alertTypeValueEnum as any)[v]?.text || v;
          return <Tag>{text}</Tag>;
        },
        responsive: ["md"],
      },
      {
        title: t("pages.alert.configs.columns.thresholdUsd", "Threshold (USD)"),
        dataIndex: "threshold_usd",
        width: 150,
        align: "center",
        render: (_, row) =>
          row.threshold_usd ? (
            <Typography.Text>{row.threshold_usd}</Typography.Text>
          ) : (
            <Typography.Text type="secondary">-</Typography.Text>
          ),
      },
      {
        title: t("pages.alert.configs.columns.timeWindow", "Window (s)"),
        dataIndex: "time_window_seconds",
        width: 120,
        align: "center",
        render: (_, row) =>
          typeof row.time_window_seconds === "number" ? (
            <Typography.Text>{row.time_window_seconds}</Typography.Text>
          ) : (
            <Typography.Text type="secondary">-</Typography.Text>
          ),
        responsive: ["md"],
      },
      {
        title: t("pages.alert.configs.columns.monitors", "Monitors"),
        key: "monitors",
        width: 220,
        align: "center",
        render: (_, row) => renderMonitors(row),
        responsive: ["lg"],
      },
      {
        title: t("pages.alert.configs.columns.cooldown", "Cooldown (s)"),
        dataIndex: "cooldown_seconds",
        width: 130,
        align: "center",
        render: (_, row) =>
          typeof row.cooldown_seconds === "number" ? (
            <Typography.Text>{row.cooldown_seconds}</Typography.Text>
          ) : (
            <Typography.Text type="secondary">-</Typography.Text>
          ),
        responsive: ["lg"],
      },
      {
        title: t("pages.alert.configs.columns.testMode", "Test Mode"),
        dataIndex: "test_mode",
        width: 110,
        align: "center",
        render: (_, row) =>
          row.test_mode ? <Tag color="blue">ON</Tag> : <Tag>OFF</Tag>,
        responsive: ["md"],
      },
      {
        title: t("pages.alert.configs.columns.enabled", "Enabled"),
        dataIndex: "enabled",
        width: 120,
        align: "center",
        render: (_, row) => (
          <Switch
            checked={!!row.enabled}
            disabled={!canRpc(PERM.toggle)}
            onChange={(checked) => confirmToggle(row, checked)}
          />
        ),
      },
      {
        title: t("pages.alert.configs.columns.updatedAt", "Updated At"),
        dataIndex: "updated_at",
        valueType: "dateTime",
        width: 180,
        responsive: ["lg"],
        align: "center",
      },
      {
        title: t("pages.alert.configs.columns.actions", "Actions"),
        valueType: "option",
        width: 200,
        fixed: "right",
        align: "center",
        render: (_, row) => (
          <div
            style={{
              display: "flex",
              justifyContent: "center",
              alignItems: "center",
              height: "100%",
            }}
          >
            <Space size={12}>
              <Typography.Link
                disabled={!canRpc(PERM.detailWithStats) && !canRpc(PERM.detail)}
                onClick={() => openDetail(row)}
              >
                {t("pages.alert.configs.actions.detail", "Detail")}
              </Typography.Link>
              <Typography.Link
                disabled={!canRpc(PERM.update)}
                onClick={() => openEdit(row)}
              >
                {t("pages.alert.configs.actions.edit", "Edit")}
              </Typography.Link>
              <Typography.Link
                disabled={!canRpc(PERM.del)}
                onClick={() => confirmDelete(row)}
              >
                {t("pages.alert.configs.actions.delete", "Delete")}
              </Typography.Link>
            </Space>
          </div>
        ),
      },
      {
        title: t("pages.alert.configs.search.enabledOnly", "Enabled Only"),
        dataIndex: "enabled_only",
        hideInTable: true,
        valueType: "switch",
        search: { transform: (v) => ({ enabled_only: v }) },
      },
    ];
  }, [intl, canRpc, screens.lg, screens.md, screens.xl]);

  const formItems = (
    <>
      <ProFormText
        name="name"
        label={t("pages.alert.configs.form.name", "Name")}
        rules={[{ required: true }]}
        fieldProps={{ maxLength: 128 }}
      />
      <ProFormTextArea
        name="description"
        label={t("pages.alert.configs.form.description", "Description")}
        fieldProps={{ autoSize: { minRows: 2, maxRows: 6 }, maxLength: 500 }}
      />
      <ProFormText
        name="threshold_usd"
        label={t("pages.alert.configs.form.thresholdUsd", "Threshold (USD)")}
        rules={[{ required: true }]}
      />
      <ProFormDigit
        name="time_window_seconds"
        label={t(
          "pages.alert.configs.form.timeWindow",
          "Time Window (seconds)"
        )}
        rules={[{ required: true }]}
        min={0}
        fieldProps={{ precision: 0 }}
      />

      <Divider />

      <Form.Item
        label={t(
          "pages.alert.configs.form.monitorWeb3",
          "Monitor Web3 Withdraw"
        )}
        name="monitor_web3_withdraw"
        valuePropName="checked"
        style={{ marginBottom: 12 }}
      >
        <Switch />
      </Form.Item>

      <Form.Item
        label={t(
          "pages.alert.configs.form.monitorWeb2",
          "Monitor Web2 Withdraw"
        )}
        name="monitor_web2_withdraw"
        valuePropName="checked"
        style={{ marginBottom: 12 }}
      >
        <Switch />
      </Form.Item>

      <Form.Item
        label={t(
          "pages.alert.configs.form.monitorInternal",
          "Monitor Internal Transfer"
        )}
        name="monitor_internal_transfer"
        valuePropName="checked"
        style={{ marginBottom: 12 }}
      >
        <Switch />
      </Form.Item>

      <Divider />

      <Form.Item
        label={t("pages.alert.configs.form.enabled", "Enabled")}
        name="enabled"
        valuePropName="checked"
        style={{ marginBottom: 12 }}
      >
        <Switch />
      </Form.Item>

      <Form.Item
        label={t("pages.alert.configs.form.testMode", "Test Mode")}
        name="test_mode"
        valuePropName="checked"
        style={{ marginBottom: 12 }}
      >
        <Switch />
      </Form.Item>

      <ProFormDigit
        name="cooldown_seconds"
        label={t("pages.alert.configs.form.cooldown", "Cooldown (seconds)")}
        min={0}
        fieldProps={{ precision: 0 }}
      />
    </>
  );

  const detailProgress = (() => {
    const raw = (detailStats?.progress_percentage || "").trim();
    const pct = Number(raw);
    const value = Number.isFinite(pct)
      ? Math.max(0, Math.min(100, pct))
      : undefined;
    return value;
  })();

  return (
    <PageContainer>
      <ProTable<AlertConfigItem>
        actionRef={actionRef}
        rowKey={(row) => row.id || JSON.stringify(row)}
        columns={columns}
        scroll={{ x: 1400 }}
        // search={{ labelWidth: screens.md ? 140 : 110 }}
        search={false}
        toolBarRender={() => [
          <Typography.Text
            key="hint"
            type="secondary"
            style={{ display: screens.md ? "inline" : "none" }}
          >
            {t(
              "pages.alert.configs.hint",
              "Manage alert thresholds and monitor scopes."
            )}
          </Typography.Text>,
          <Button
            key="create"
            type="primary"
            onClick={() => setCreateOpen(true)}
            disabled={!canRpc(PERM.create)}
          >
            {t("pages.alert.configs.actions.create", "New")}
          </Button>,
        ]}
        request={async (params) => {
          if (!canRpc(PERM.list)) return { success: true, data: [], total: 0 };
          try {
            const res = await adminListAlertConfigs(
              {
                page: params.current as any,
                page_size: params.pageSize as any,
                enabled_only: (params as any).enabled_only,
              },
              { skipErrorHandler: true }
            );
            if (!res?.success) throw new Error(res?.message || "");
            const list =
              ((res as any)?.data?.configs as AlertConfigItem[]) || [];
            const total = Number((res as any)?.data?.total ?? 0);
            return { success: true, data: list, total };
          } catch (e: any) {
            message.error(
              e?.message ||
                t("pages.alert.configs.messages.loadFailed", "Failed to load")
            );
            return { success: false, data: [], total: 0 };
          }
        }}
      />

      <ModalForm
        title={t("pages.alert.configs.create.title", "Create Alert Config")}
        open={createOpen}
        modalProps={{
          onCancel: () => setCreateOpen(false),
          destroyOnClose: true,
          width: modalWidth,
        }}
        onOpenChange={setCreateOpen}
        layout={screens.md ? "horizontal" : "vertical"}
        labelCol={screens.md ? { span: 7 } : undefined}
        wrapperCol={screens.md ? { span: 17 } : undefined}
        initialValues={{
          enabled: true,
          test_mode: false,
          monitor_web3_withdraw: true,
          monitor_web2_withdraw: true,
          monitor_internal_transfer: true,
          time_window_seconds: 0,
          cooldown_seconds: 0,
        }}
        submitter={{
          searchConfig: {
            submitText: t("pages.alert.configs.actions.create", "New"),
            resetText: t("pages.alert.configs.actions.cancel", "Cancel"),
          },
          resetButtonProps: { onClick: () => setCreateOpen(false) },
          submitButtonProps: { disabled: !canRpc(PERM.create) },
        }}
        onFinish={async (values) => {
          if (!canRpc(PERM.create)) return true;
          try {
            const res = await adminCreateAlertConfig(values as any, {
              skipErrorHandler: true,
            });
            if (!res?.success) throw new Error(res?.message || "");
            message.success(
              t("pages.alert.configs.messages.createSuccess", "Created")
            );
            setCreateOpen(false);
            refresh();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.alert.configs.messages.createFailed",
                  "Failed to create"
                )
            );
            return false;
          }
        }}
      >
        {formItems}
      </ModalForm>

      <ModalForm
        title={t("pages.alert.configs.edit.title", "Edit Alert Config")}
        open={editOpen}
        modalProps={{
          onCancel: () => setEditOpen(false),
          destroyOnClose: true,
          width: modalWidth,
          confirmLoading: editLoading,
        }}
        onOpenChange={setEditOpen}
        layout={screens.md ? "horizontal" : "vertical"}
        labelCol={screens.md ? { span: 7 } : undefined}
        wrapperCol={screens.md ? { span: 17 } : undefined}
        initialValues={current || undefined}
        submitter={{
          searchConfig: {
            submitText: t("pages.alert.configs.actions.save", "Save"),
            resetText: t("pages.alert.configs.actions.cancel", "Cancel"),
          },
          resetButtonProps: { onClick: () => setEditOpen(false) },
          submitButtonProps: { disabled: !canRpc(PERM.update) },
        }}
        onFinish={async (values) => {
          if (!canRpc(PERM.update)) return true;
          const id = current?.id;
          if (!id) return false;
          try {
            const res = await adminUpdateAlertConfig(
              { configId: toId(id) },
              values as any,
              { skipErrorHandler: true }
            );
            if (!res?.success) throw new Error(res?.message || "");
            message.success(
              t("pages.alert.configs.messages.updateSuccess", "Updated")
            );
            setEditOpen(false);
            setCurrent(null);
            refresh();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.alert.configs.messages.updateFailed",
                  "Failed to update"
                )
            );
            return false;
          }
        }}
      >
        {formItems}
      </ModalForm>

      <ModalForm
        title={t("pages.alert.configs.detail.title", "Alert Config Detail")}
        open={detailOpen}
        modalProps={{
          onCancel: () => setDetailOpen(false),
          destroyOnClose: true,
          width: modalWidth,
          footer: null,
        }}
        onOpenChange={setDetailOpen}
        submitter={false}
      >
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Descriptions
            size="small"
            bordered
            column={descCols}
            items={[
              {
                key: "id",
                label: t("pages.alert.configs.detail.id", "ID"),
                children: detailConfig?.id ? (
                  <Typography.Text code>{detailConfig.id}</Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "name",
                label: t("pages.alert.configs.detail.name", "Name"),
                children: detailConfig?.name || "-",
              },
              {
                key: "alert_type",
                label: t("pages.alert.configs.detail.alertType", "Alert Type"),
                children: alertTypeText(detailConfig?.alert_type),
              },

              {
                key: "enabled",
                label: t("pages.alert.configs.detail.enabled", "Enabled"),
                children: detailConfig?.enabled ? (
                  <Tag color="green">ON</Tag>
                ) : (
                  <Tag>OFF</Tag>
                ),
              },
              {
                key: "threshold_usd",
                label: t(
                  "pages.alert.configs.detail.thresholdUsd",
                  "Threshold (USD)"
                ),
                children: detailConfig?.threshold_usd || "-",
              },
              {
                key: "time_window_seconds",
                label: t(
                  "pages.alert.configs.detail.timeWindow",
                  "Time Window (seconds)"
                ),
                children:
                  typeof detailConfig?.time_window_seconds === "number"
                    ? detailConfig?.time_window_seconds
                    : "-",
              },
              {
                key: "cooldown_seconds",
                label: t(
                  "pages.alert.configs.detail.cooldown",
                  "Cooldown (seconds)"
                ),
                children:
                  typeof detailConfig?.cooldown_seconds === "number"
                    ? detailConfig?.cooldown_seconds
                    : "-",
              },
              {
                key: "test_mode",
                label: t("pages.alert.configs.detail.testMode", "Test Mode"),
                children: detailConfig?.test_mode ? (
                  <Tag color="blue">ON</Tag>
                ) : (
                  <Tag>OFF</Tag>
                ),
              },
              {
                key: "monitors",
                label: t("pages.alert.configs.detail.monitors", "Monitors"),
                children: renderMonitors(detailConfig || {}),
              },
              {
                key: "created_at",
                label: t("pages.alert.configs.detail.createdAt", "Created At"),
                children: detailConfig?.created_at || "-",
              },
              {
                key: "updated_at",
                label: t("pages.alert.configs.detail.updatedAt", "Updated At"),
                children: detailConfig?.updated_at || "-",
              },
              {
                key: "description",
                label: t(
                  "pages.alert.configs.detail.description",
                  "Description"
                ),
                children: detailConfig?.description || "-",
              },
            ]}
          />

          {detailLoading ? (
            <Typography.Text type="secondary">
              {t("pages.alert.configs.detail.loading", "Loading...")}
            </Typography.Text>
          ) : detailStats ? (
            <Descriptions
              size="small"
              bordered
              column={descCols}
              items={[
                {
                  key: "current_window_amount",
                  label: t(
                    "pages.alert.configs.stats.currentWindowAmount",
                    "Current Window Amount"
                  ),
                  children: detailStats.current_window_amount || "-",
                },
                {
                  key: "current_window_tx_count",
                  label: t(
                    "pages.alert.configs.stats.currentWindowTxCount",
                    "Current Window Tx Count"
                  ),
                  children:
                    typeof detailStats.current_window_tx_count === "number"
                      ? detailStats.current_window_tx_count
                      : "-",
                },
                {
                  key: "progress",
                  label: t("pages.alert.configs.stats.progress", "Progress"),
                  children:
                    detailProgress === undefined ? (
                      <Typography.Text type="secondary">-</Typography.Text>
                    ) : (
                      <Progress percent={detailProgress} size="small" />
                    ),
                },
                {
                  key: "is_in_cooldown",
                  label: t(
                    "pages.alert.configs.stats.inCooldown",
                    "In Cooldown"
                  ),
                  children: detailStats.is_in_cooldown ? (
                    <Tag color="orange">YES</Tag>
                  ) : (
                    <Tag>NO</Tag>
                  ),
                },
                {
                  key: "cooldown_remaining_seconds",
                  label: t(
                    "pages.alert.configs.stats.cooldownRemaining",
                    "Cooldown Remaining (s)"
                  ),
                  children:
                    typeof detailStats.cooldown_remaining_seconds === "number"
                      ? detailStats.cooldown_remaining_seconds
                      : "-",
                },
              ]}
            />
          ) : (
            <Typography.Text type="secondary">
              {t(
                "pages.alert.configs.stats.unavailable",
                "Realtime stats not available."
              )}
            </Typography.Text>
          )}
        </Space>
      </ModalForm>
    </PageContainer>
  );
};

export default AlertConfigsPage;
