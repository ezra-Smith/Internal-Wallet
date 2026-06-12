import type {
  AdminAdminCreateSwapConfigRequest,
  AdminAdminSwapConfigItem,
  AdminUpdateSwapConfigDetailBody,
} from "@/api/generated/schemas";
import {
  adminCreateSwapConfig,
  adminDeleteSwapConfig,
  adminGetSwapConfigDetail,
  adminListSwapConfigs,
  adminListSwapProvidersForDropdown,
  adminUpdateSwapConfigDetail,
} from "@/api/generated/swap";
import { useRbac } from "@/hooks/useRbac";
import type {
  ActionType,
  ProColumns,
  ProFormInstance,
} from "@ant-design/pro-components";
import {
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from "@ant-design/pro-components";
import { App, Button, Modal, Switch } from "antd";
import React, { useEffect, useMemo, useRef, useState } from "react";

const PERM = {
  list: "ListSwapConfigs",
  create: "CreateSwapConfig",
  update: "UpdateSwapConfig",
  delete: "DeleteSwapConfig",
};

const BODY_STYLE: React.CSSProperties = {
  maxHeight: 560,
  overflowY: "auto",
  paddingRight: 8,
};

const n2s = (v: any) => {
  if (v === null || v === undefined || v === "") return undefined;
  return typeof v === "number" ? String(v) : String(v);
};

type ModalMode = "create" | "edit";

type FormValues = {
  provider_id?: string;
  provider_name?: string;
  chain_id?: string;
  chain_name?: string;
  chain_symbol?: string;
  token_symbol?: string;
  token_name?: string;
  contract_address?: string;
  decimals?: number;
  icon_url?: string;
  is_enabled?: boolean;
  priority?: number;
  router_address?: string;
  min_swap_amount_usd?: number;
  max_swap_amount_usd?: number;
  default_slippage?: number;
  max_slippage?: number;
  fee_rate?: number;
  config_json?: string;
};

const SwapConfigPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();

  const actionRef = useRef<ActionType | null>(null);
  const formRef = useRef<ProFormInstance<FormValues> | undefined>(undefined);

  const [providerOptions, setProviderOptions] = useState<
    { label: string; value: string }[]
  >([]);
  const [modalOpen, setModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<ModalMode>("create");
  const [currentRow, setCurrentRow] = useState<AdminAdminSwapConfigItem | null>(
    null
  );
  const [currentId, setCurrentId] = useState<string | null>(null);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [togglingId, setTogglingId] = useState<string | null>(null);

  const canList = canRpc(PERM.list);
  const canCreate = canRpc(PERM.create);
  const canUpdate = canRpc(PERM.update);
  const canDelete = canRpc(PERM.delete);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await adminListSwapProvidersForDropdown({
          skipErrorHandler: true,
        });
        if (!res?.success) throw new Error(res?.message || "加载服务商失败");
        const list = res.data?.providers ?? [];
        const opts = list
          .filter((x: any) => x?.id)
          .map((x: any) => ({ label: x.provider_name ?? x.id, value: x.id }));
        if (!cancelled) setProviderOptions(opts);
      } catch (e: any) {
        if (!cancelled) message.error(e?.message || "加载服务商失败");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [message]);

  const providerIdToName = useMemo(() => {
    const m = new Map<string, string>();
    providerOptions.forEach((o) => m.set(o.value, o.label));
    return m;
  }, [providerOptions]);

  const openCreate = () => {
    if (!canCreate) return;
    setModalMode("create");
    setCurrentId(null);
    setCurrentRow(null);
    setModalOpen(true);
    setTimeout(() => formRef.current?.resetFields(), 0);
  };

  const openEdit = (row: AdminAdminSwapConfigItem) => {
    if (!canUpdate) return;
    if (!row.id) return;
    setModalMode("edit");
    setCurrentId(row.id);
    setCurrentRow(row);
    setModalOpen(true);
  };

  useEffect(() => {
    const id = currentId;
    if (!modalOpen || modalMode !== "edit" || !id) return;

    let cancelled = false;
    (async () => {
      setLoadingDetail(true);
      try {
        const res = await adminGetSwapConfigDetail(
          { id },
          { skipErrorHandler: true }
        );
        if (!res?.success) throw new Error(res?.message || "加载详情失败");
        const cfg = res.data?.config;
        if (!cfg) throw new Error("加载详情失败");
        if (cancelled) return;

        formRef.current?.setFieldsValue({
          provider_id: cfg.provider_id,
          provider_name: cfg.provider_name,
          chain_name: cfg.chain_name,
          chain_symbol: cfg.chain_symbol,
          token_name: cfg.token_name,
          icon_url: cfg.icon_url,
          is_enabled: cfg.is_enabled,
          priority: cfg.priority,
          router_address: cfg.router_address,
          min_swap_amount_usd: cfg.min_swap_amount_usd
            ? Number(cfg.min_swap_amount_usd)
            : undefined,
          max_swap_amount_usd: cfg.max_swap_amount_usd
            ? Number(cfg.max_swap_amount_usd)
            : undefined,
          default_slippage: cfg.default_slippage
            ? Number(cfg.default_slippage)
            : undefined,
          max_slippage: cfg.max_slippage ? Number(cfg.max_slippage) : undefined,
          fee_rate: cfg.fee_rate ? Number(cfg.fee_rate) : undefined,
          config_json: cfg.config_json,
        });
      } catch (e: any) {
        if (!cancelled) message.error(e?.message || "加载详情失败");
      } finally {
        if (!cancelled) setLoadingDetail(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [modalOpen, modalMode, currentId, message]);

  const toggleEnabled = async (
    row: AdminAdminSwapConfigItem,
    nextEnabled: boolean
  ) => {
    if (!canUpdate) return;
    if (!row.id) return;

    Modal.confirm({
      title: nextEnabled ? "确认启用" : "确认停用",
      content: nextEnabled
        ? "确定要启用该 Swap 配置吗？"
        : "确定要停用该 Swap 配置吗？",
      okText: "确认",
      cancelText: "取消",
      onOk: async () => {
        try {
          setTogglingId(row.id as string);
          const body: AdminUpdateSwapConfigDetailBody = {
            provider_name: row.provider_name,
            chain_name: row.chain_name,
            chain_symbol: row.chain_symbol,
            token_name: row.token_name,
            icon_url: row.icon_url,
            is_enabled: nextEnabled,
            priority: row.priority,
            router_address: row.router_address,
            min_swap_amount_usd: row.min_swap_amount_usd,
            max_swap_amount_usd: row.max_swap_amount_usd,
            default_slippage: row.default_slippage,
            max_slippage: row.max_slippage,
            fee_rate: row.fee_rate,
            config_json: row.config_json,
          };
          const res = await adminUpdateSwapConfigDetail(
            { id: row.id as string },
            body,
            { skipErrorHandler: true }
          );
          if (!res?.success) throw new Error(res?.message || "更新失败");
          message.success("更新成功");
          actionRef.current?.reload();
        } catch (e: any) {
          message.error(e?.message || "更新失败");
        } finally {
          setTogglingId(null);
        }
      },
    });
  };

  const columns = useMemo(() => {
    return [
      {
        title: "服务商",
        dataIndex: "provider_id",
        hideInTable: true,
        valueType: "select",
        fieldProps: {
          options: providerOptions,
          showSearch: true,
          optionFilterProp: "label",
        },
      },
      { title: "链ID", dataIndex: "chain_id", hideInTable: true },
      { title: "代币符号", dataIndex: "token_symbol", hideInTable: true },
      {
        title: "启用",
        dataIndex: "is_enabled",
        hideInTable: true,
        valueType: "select",
        valueEnum: { true: { text: "是" }, false: { text: "否" } },
      },

      {
        title: "ID",
        dataIndex: "id",
        width: 160,
        copyable: true,
        fixed: "left",
        hideInSearch: true,
      },
      {
        title: "服务商",
        dataIndex: "provider_name",
        width: 160,
        hideInSearch: true,
      },
      {
        title: "合约地址",
        dataIndex: "contract_address",
        width: 160,
        hideInSearch: true,
      },
      { title: "链", dataIndex: "chain_name", width: 140, hideInSearch: true },
      {
        title: "代币",
        dataIndex: "token_symbol",
        width: 120,
        hideInSearch: true,
      },
      {
        title: "启用",
        dataIndex: "is_enabled",
        width: 120,
        hideInSearch: true,
        render: (_, r) => (
          <Switch
            checked={!!r.is_enabled}
            loading={togglingId === r.id}
            disabled={!canUpdate}
            onChange={(checked) => toggleEnabled(r, checked)}
          />
        ),
      },
      {
        title: "优先级",
        dataIndex: "priority",
        width: 100,
        hideInSearch: true,
      },
      { title: "费率", dataIndex: "fee_rate", width: 120, hideInSearch: true },
      {
        title: "默认滑点",
        dataIndex: "default_slippage",
        width: 140,
        hideInSearch: true,
      },
      {
        title: "最大滑点",
        dataIndex: "max_slippage",
        width: 140,
        hideInSearch: true,
      },
      {
        title: "最小金额(USD)",
        dataIndex: "min_swap_amount_usd",
        width: 140,
        hideInSearch: true,
      },
      {
        title: "最大金额(USD)",
        dataIndex: "max_swap_amount_usd",
        width: 140,
        hideInSearch: true,
      },
      {
        title: "更新时间",
        dataIndex: "updated_at",
        width: 180,
        hideInSearch: true,
      },

      {
        title: "操作",
        valueType: "option",
        width: 150,
        fixed: "right",
        render: (_, row) => {
          const nodes: React.ReactNode[] = [];
          nodes.push(
            <a key="edit" onClick={() => openEdit(row)}>
              编辑
            </a>
          );
          nodes.push(
            <a
              key="delete"
              style={{ color: canDelete ? undefined : "rgba(0,0,0,0.25)" }}
              onClick={() => {
                if (!canDelete) return;
                if (!row.id) return;
                Modal.confirm({
                  title: "确认删除",
                  content: "确定要删除该 Swap 配置吗？",
                  okText: "删除",
                  cancelText: "取消",
                  okButtonProps: { danger: true },
                  onOk: async () => {
                    try {
                      const res = await adminDeleteSwapConfig(
                        { id: row.id as string },
                        { skipErrorHandler: true }
                      );
                      if (!res?.success)
                        throw new Error(res?.message || "删除失败");
                      message.success("删除成功");
                      actionRef.current?.reload();
                    } catch (e: any) {
                      message.error(e?.message || "删除失败");
                    }
                  },
                });
              }}
            >
              删除
            </a>
          );
          return nodes;
        },
      },
    ] as ProColumns<AdminAdminSwapConfigItem>[];
  }, [providerOptions, canDelete, canUpdate, togglingId]);

  const onFinish = async (values: FormValues) => {
    try {
      if (modalMode === "create") {
        if (!canCreate) return false;

        const decimals = values.decimals;
        if (
          decimals === null ||
          decimals === undefined ||
          Number.isNaN(Number(decimals)) ||
          !Number.isInteger(Number(decimals))
        ) {
          message.error("精度必须是整数");
          return false;
        }

        const body: AdminAdminCreateSwapConfigRequest = {
          provider_id: values.provider_id as string,
          chain_id: values.chain_id as string,
          chain_name: values.chain_name as string,
          chain_symbol: values.chain_symbol as string,
          token_symbol: values.token_symbol as string,
          token_name: values.token_name as string,
          contract_address: values.contract_address as string,
          decimals: Math.trunc(Number(values.decimals)),
          icon_url: values.icon_url,
          is_enabled: !!values.is_enabled,
          priority: values.priority ? Math.trunc(Number(values.priority)) : 0,
          router_address: values.router_address,
          min_swap_amount_usd: n2s(values.min_swap_amount_usd),
          max_swap_amount_usd: n2s(values.max_swap_amount_usd),
          default_slippage: n2s(values.default_slippage),
          max_slippage: n2s(values.max_slippage),
          fee_rate: n2s(values.fee_rate),
          config_json: values.config_json,
        };

        const res = await adminCreateSwapConfig(body, {
          skipErrorHandler: true,
        });
        if (!res?.success) throw new Error(res?.message || "创建失败");
        message.success("创建成功");
      } else {
        if (!canUpdate) return false;
        const id = currentId;
        if (!id) return false;

        const providerName =
          values.provider_name ||
          (values.provider_id
            ? providerIdToName.get(values.provider_id)
            : undefined) ||
          currentRow?.provider_name;

        const body: AdminUpdateSwapConfigDetailBody = {
          provider_name: providerName,
          chain_name: values.chain_name,
          chain_symbol: values.chain_symbol,
          token_name: values.token_name,
          icon_url: values.icon_url,
          is_enabled: values.is_enabled,
          priority:
            values.priority !== undefined
              ? Math.trunc(Number(values.priority))
              : undefined,
          router_address: values.router_address,
          min_swap_amount_usd:
            values.min_swap_amount_usd !== undefined
              ? n2s(values.min_swap_amount_usd)
              : undefined,
          max_swap_amount_usd:
            values.max_swap_amount_usd !== undefined
              ? n2s(values.max_swap_amount_usd)
              : undefined,
          default_slippage:
            values.default_slippage !== undefined
              ? n2s(values.default_slippage)
              : undefined,
          max_slippage:
            values.max_slippage !== undefined
              ? n2s(values.max_slippage)
              : undefined,
          fee_rate:
            values.fee_rate !== undefined ? n2s(values.fee_rate) : undefined,
          config_json: values.config_json,
        };

        const res = await adminUpdateSwapConfigDetail({ id }, body, {
          skipErrorHandler: true,
        });
        if (!res?.success) throw new Error(res?.message || "更新失败");
        message.success("更新成功");
      }

      setModalOpen(false);
      setCurrentId(null);
      setCurrentRow(null);
      formRef.current?.resetFields();
      actionRef.current?.reload();
      return true;
    } catch (e: any) {
      message.error(
        e?.message || (modalMode === "create" ? "创建失败" : "更新失败")
      );
      return false;
    }
  };

  return (
    <PageContainer>
      <ProTable<AdminAdminSwapConfigItem>
        actionRef={actionRef}
        rowKey={(row) =>
          row.id ||
          `${row.provider_id || ""}-${row.chain_id || ""}-${
            row.token_symbol || ""
          }`
        }
        columns={columns}
        scroll={{ x: 1500 }}
        search={{ labelWidth: 110 }}
        toolBarRender={() => [
          <Button
            key="create"
            type="primary"
            disabled={!canCreate}
            onClick={openCreate}
          >
            新增配置
          </Button>,
        ]}
        request={async (params) => {
          if (!canList) return { success: true, data: [], total: 0 };
          try {
            const res = await adminListSwapConfigs(
              {
                page: params.current,
                page_size: params.pageSize,
                provider_id: (params as any).provider_id,
                chain_id: (params as any).chain_id,
                token_symbol: (params as any).token_symbol,
                is_enabled:
                  (params as any).is_enabled === undefined
                    ? undefined
                    : String((params as any).is_enabled) === "true"
                    ? true
                    : String((params as any).is_enabled) === "false"
                    ? false
                    : undefined,
              },
              { skipErrorHandler: true }
            );
            if (!res?.success) throw new Error(res?.message || "加载失败");
            return {
              success: true,
              data: res.data?.configs ?? [],
              total: Number(res.data?.pagination?.total ?? 0),
            };
          } catch (e: any) {
            message.error(e?.message || "加载失败");
            return { success: false, data: [], total: 0 };
          }
        }}
      />

      <ModalForm<FormValues>
        formRef={formRef}
        title={modalMode === "create" ? "新增 Swap 配置" : "编辑 Swap 配置"}
        open={modalOpen}
        onOpenChange={(open) => {
          setModalOpen(open);
          if (!open) {
            setCurrentId(null);
            setCurrentRow(null);
            formRef.current?.resetFields();
          }
        }}
        modalProps={{
          destroyOnClose: true,
          okText: modalMode === "create" ? "创建" : "保存",
          cancelText: "取消",
          bodyStyle: BODY_STYLE,
          confirmLoading: loadingDetail,
        }}
        onFinish={onFinish}
      >
        <ProFormSelect
          name="provider_id"
          label="服务商"
          // rules={modalMode === 'create' ? [{ required: true, message: '请选择服务商' }] : undefined}
          rules={[{ required: true, message: "请选择服务商" }]}
          fieldProps={{
            options: providerOptions,
            showSearch: true,
            optionFilterProp: "label",
            onChange: (v) => {
              const name = v ? providerIdToName.get(String(v)) : undefined;
              formRef.current?.setFieldsValue({ provider_name: name });
            },
          }}
        />
        <ProFormText name="provider_name" hidden />

        {modalMode === "create" ? (
          <>
            <ProFormText
              name="chain_id"
              label="链ID"
              rules={[{ required: true, message: "请输入链ID" }]}
            />
            <ProFormText
              name="chain_name"
              label="链名称"
              rules={[{ required: true, message: "请输入链名称" }]}
            />
            <ProFormText
              name="chain_symbol"
              label="链符号"
              rules={[{ required: true, message: "请输入链符号" }]}
            />
            <ProFormText
              name="token_symbol"
              label="代币符号"
              rules={[{ required: true, message: "请输入代币符号" }]}
            />
            <ProFormText
              name="token_name"
              label="代币名称"
              rules={[{ required: true, message: "请输入代币名称" }]}
            />
            <ProFormText
              name="contract_address"
              label="合约地址"
              rules={[{ required: true, message: "请输入合约地址" }]}
            />
            <ProFormDigit
              name="decimals"
              label="精度"
              min={0}
              max={30}
              fieldProps={{ precision: 0 }}
              rules={[
                { required: true, message: "请输入精度" },
                {
                  validator: async (_: any, v: any) => {
                    if (v === undefined || v === null) return Promise.resolve();
                    if (Number.isInteger(Number(v))) return Promise.resolve();
                    return Promise.reject(new Error("精度必须是整数"));
                  },
                },
              ]}
            />
          </>
        ) : (
          <>
            <ProFormText
              name="chain_name"
              label="链名称"
              rules={[{ required: true, message: "请输入链名称" }]}
            />
            <ProFormText
              name="chain_symbol"
              label="链符号"
              rules={[{ required: true, message: "请输入链符号" }]}
            />
            <ProFormText
              name="token_name"
              label="代币名称"
              rules={[{ required: true, message: "请输入代币符号" }]}
            />
          </>
        )}

        <ProFormText name="icon_url" label="图标URL" />
        <ProFormSwitch name="is_enabled" label="是否启用" />
        <ProFormDigit
          name="priority"
          label="优先级"
          min={0}
          fieldProps={{ precision: 0 }}
        />
        <ProFormText name="router_address" label="路由合约地址" />
        <ProFormDigit
          name="min_swap_amount_usd"
          label="最小金额(USD)"
          min={0}
          fieldProps={{ precision: 8 }}
        />
        <ProFormDigit
          name="max_swap_amount_usd"
          label="最大金额(USD)"
          min={0}
          fieldProps={{ precision: 8 }}
        />
        <ProFormDigit
          name="fee_rate"
          label="费率"
          min={0}
          fieldProps={{ precision: 8 }}
        />
        <ProFormDigit
          name="default_slippage"
          label="默认滑点"
          min={0}
          fieldProps={{ precision: 8 }}
        />
        <ProFormDigit
          name="max_slippage"
          label="最大滑点"
          min={0}
          fieldProps={{ precision: 8 }}
        />
        <ProFormTextArea
          name="config_json"
          label="配置JSON"
          fieldProps={{ rows: 6 }}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default SwapConfigPage;
