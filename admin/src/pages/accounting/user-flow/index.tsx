import {
  adminAccountingGetUserTransactionRecordDetail,
  adminAccountingListUserTransactionRecords,
} from "@/api/generated/accounting";
import { adminListCurrencies } from "@/api/generated/assets";
import type { AdminCurrencyItem as CurrencyItem } from "@/api/generated/schemas";
import { useRbac } from "@/hooks/useRbac";
import type { ActionType, ProColumns } from "@ant-design/pro-components";
import { PageContainer, ProTable } from "@ant-design/pro-components";
import { useIntl } from "@umijs/max";
import {
  App,
  Button,
  Descriptions,
  Grid,
  Modal,
  Space,
  Tag,
  Typography,
} from "antd";
import dayjs from "dayjs";
import React, { useEffect, useMemo, useRef, useState } from "react";

const PERM = {
  list: "AccountingListUserTransactionRecords",
  detail: "AccountingGetUserTransactionRecordDetail",
} as const;

type TxItem = {
  id?: string;
  user_id?: string;
  tx_type?: number;
  asset_code?: string;
  chain_code?: string;
  amount_decimal?: string;
  fee_decimal?: string;
  status?: string;
  memo?: string;
  from_address?: string;
  to_address?: string;
  tx_hash?: string;
  timestamp?: string;
  freeze_ledger_tx_id?: string;
  settle_ledger_tx_id?: string;
  confirm_ledger_tx_id?: string;
  biz_ref?: string;
  idempotency_key?: string;
  created_at?: string;
  updated_at?: string;
};

const toUpper = (v?: string) => (v || "").trim().toUpperCase();
const emptyText = <Typography.Text type="secondary">-</Typography.Text>;

const formatUnixSeconds = (v?: string) => {
  const s = (v || "").trim();
  if (!s) return "";
  const n = Number(s);
  if (!Number.isFinite(n) || n <= 0) return "";
  return dayjs(n * 1000).format("YYYY-MM-DD HH:mm:ss");
};

const UserFlowPage: React.FC = () => {
  const { message } = App.useApp();
  const intl = useIntl();
  const { canRpc } = useRbac();
  const screens = Grid.useBreakpoint();

  const t = (id: string, defaultMessage: string) =>
    intl.formatMessage({ id, defaultMessage });

  const actionRef = useRef<ActionType | null>(null);

  const [assetOptions, setAssetOptions] = useState<
    { label: string; value: string }[]
  >([]);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailItem, setDetailItem] = useState<TxItem | null>(null);

  const modalWidth = useMemo(() => {
    if (screens.xs) return "100%";
    if (screens.sm) return 1000;
    if (screens.md) return 920;
    return 1000;
  }, [screens]);

  const txTypeValueEnum = useMemo(() => {
    return {
      1: { text: t("pages.accounting.userFlow.txType.deposit", "Deposit") },
      2: { text: t("pages.accounting.userFlow.txType.withdraw", "Withdraw") },
      3: {
        text: t(
          "pages.accounting.userFlow.txType.internalTransfer",
          "Internal Transfer"
        ),
      },
    } as any;
  }, [intl]);

  const statusValueEnum = useMemo(() => {
    return {
      pending: {
        text: t("pages.accounting.userFlow.status.pending", "Pending"),
      },
      completed: {
        text: t("pages.accounting.userFlow.status.completed", "Completed"),
      },
      failed: { text: t("pages.accounting.userFlow.status.failed", "Failed") },
    } as any;
  }, [intl]);

  const renderStatusTag = (v?: string) => {
    const s = (v || "").trim().toLowerCase();
    if (!s) return emptyText;
    const label =
      s === "pending"
        ? t("pages.accounting.userFlow.status.pending", "Pending")
        : s === "completed"
        ? t("pages.accounting.userFlow.status.completed", "Completed")
        : s === "failed"
        ? t("pages.accounting.userFlow.status.failed", "Failed")
        : s;
    return <Tag>{label}</Tag>;
  };

  const renderTxTypeTag = (v?: number) => {
    if (v === null || v === undefined) return emptyText;
    const label =
      v === 1
        ? t("pages.accounting.userFlow.txType.deposit", "Deposit")
        : v === 2
        ? t("pages.accounting.userFlow.txType.withdraw", "Withdraw")
        : v === 3
        ? t(
            "pages.accounting.userFlow.txType.internalTransfer",
            "Internal Transfer"
          )
        : String(v);
    return <Tag>{label}</Tag>;
  };

  const loadAssets = async () => {
    try {
      const res = await adminListCurrencies(
        { page: 1, page_size: 500 },
        { skipErrorHandler: true }
      );
      const list: CurrencyItem[] = res?.data?.currencies || [];
      const opts = list
        .map((x) => {
          const code = toUpper(x.asset_code);
          if (!code) return null;
          return {
            value: code,
            label: `${code}${x.asset_name ? ` (${x.asset_name})` : ""}`,
          };
        })
        .filter((x): x is { label: string; value: string } => !!x)
        .sort((a, b) => a.value.localeCompare(b.value));
      setAssetOptions(opts);
    } catch {
      setAssetOptions([]);
    }
  };

  useEffect(() => {
    loadAssets();
  }, []);

  const openDetail = async (row: TxItem) => {
    if (!canRpc(PERM.detail)) return;
    const id = row.id;
    if (!id) return;
    setDetailLoading(true);
    try {
      const res = await adminAccountingGetUserTransactionRecordDetail(
        { id },
        { skipErrorHandler: true }
      );
      if (!res?.success) throw new Error(res?.message || "");
      setDetailItem((res?.data as any)?.item || row);
      setDetailOpen(true);
    } catch (e: any) {
      message.error(
        e?.message ||
          t(
            "pages.accounting.userFlow.messages.detailFailed",
            "Failed to load detail"
          )
      );
    } finally {
      setDetailLoading(false);
    }
  };

  const columns: ProColumns<TxItem>[] = useMemo(() => {
    return [
      {
        title: t("pages.accounting.userFlow.columns.id", "ID"),
        dataIndex: "id",
        width: 120,
        copyable: true,
        hideInSearch: true,
        render: (_, row) => (
          <Typography.Text code>{row.id || "-"}</Typography.Text>
        ),
      },
      {
        title: t("pages.accounting.userFlow.columns.userId", "User ID"),
        dataIndex: "user_id",
        width: 160,
        copyable: true,
        ellipsis: true,
        hideInSearch: true,
      },
      {
        title: t("pages.accounting.userFlow.columns.txType", "Tx Type"),
        dataIndex: "tx_type",
        width: 160,
        hideInSearch: true,
        render: (_, row) => renderTxTypeTag(row.tx_type),
      },
      {
        title: t("pages.accounting.userFlow.columns.asset", "Asset"),
        dataIndex: "asset_code",
        width: 100,
        hideInSearch: true,
        render: (_, row) => (
          <Typography.Text code>
            {toUpper(row.asset_code) || "-"}
          </Typography.Text>
        ),
      },
      {
        title: t("pages.accounting.userFlow.columns.chain", "Chain"),
        dataIndex: "chain_code",
        width: 110,
        responsive: ["md"],
        hideInSearch: true,
        render: (_, row) => (
          <Typography.Text code>
            {toUpper(row.chain_code) || "-"}
          </Typography.Text>
        ),
      },
      {
        title: t("pages.accounting.userFlow.columns.amount", "Amount"),
        dataIndex: "amount_decimal",
        width: 140,
        hideInSearch: true,
        render: (_, row) => (
          <Typography.Text>{row.amount_decimal || "-"}</Typography.Text>
        ),
      },
      {
        title: t("pages.accounting.userFlow.columns.fee", "Fee"),
        dataIndex: "fee_decimal",
        width: 120,
        responsive: ["md"],
        hideInSearch: true,
        render: (_, row) => (
          <Typography.Text>{row.fee_decimal || "-"}</Typography.Text>
        ),
      },
      {
        title: t("pages.accounting.userFlow.columns.status", "Status"),
        dataIndex: "status",
        width: 130,
        hideInSearch: true,
        render: (_, row) => renderStatusTag(row.status),
      },
      {
        title: t("pages.accounting.userFlow.columns.from", "From"),
        dataIndex: "from_address",
        width: 220,
        responsive: ["lg"],
        ellipsis: true,
        hideInSearch: true,
        render: (_, row) =>
          row.from_address ? (
            <Typography.Text copyable ellipsis={{ tooltip: true }}>
              {row.from_address}
            </Typography.Text>
          ) : (
            "-"
          ),
      },
      {
        title: t("pages.accounting.userFlow.columns.to", "To"),
        dataIndex: "to_address",
        width: 220,
        responsive: ["lg"],
        ellipsis: true,
        hideInSearch: true,
        render: (_, row) =>
          row.to_address ? (
            <Typography.Text copyable ellipsis={{ tooltip: true }}>
              {row.to_address}
            </Typography.Text>
          ) : (
            "-"
          ),
      },
      {
        title: t("pages.accounting.userFlow.columns.txHash", "Tx Hash"),
        dataIndex: "tx_hash",
        width: 240,
        responsive: ["xl"],
        ellipsis: true,
        copyable: true,
        hideInSearch: true,
        render: (_, row) =>
          row.tx_hash ? (
            <Typography.Text code>{row.tx_hash}</Typography.Text>
          ) : (
            "-"
          ),
      },
      {
        title: t("pages.accounting.userFlow.columns.timestamp", "Timestamp"),
        dataIndex: "timestamp",
        width: 180,
        responsive: ["md"],
        hideInSearch: true,
        render: (_, row) => {
          const s = formatUnixSeconds(row.timestamp);
          return s ? s : "-";
        },
      },
      {
        title: t("pages.accounting.userFlow.columns.createdAt", "Created At"),
        dataIndex: "created_at",
        width: 180,
        responsive: ["lg"],
        hideInSearch: true,
        render: (_, row) => row.created_at || "-",
      },
      {
        title: t("pages.accounting.userFlow.columns.actions", "Actions"),
        valueType: "option",
        width: 120,
        fixed: screens.xs || screens.sm ? undefined : "right",
        hideInSearch: true,
        render: (_, row) => (
          <Button
            type="link"
            size="small"
            disabled={!canRpc(PERM.detail)}
            onClick={() => openDetail(row)}
          >
            {t("pages.accounting.userFlow.actions.detail", "Detail")}
          </Button>
        ),
      },

      {
        title: t("pages.accounting.userFlow.search.userId", "User ID"),
        key: "search_user_id",
        dataIndex: "user_id",
        hideInTable: true,
        search: { transform: (v) => ({ user_id: v }) },
      },
      {
        title: t("pages.accounting.userFlow.search.txType", "Tx Type"),
        key: "search_tx_type",
        dataIndex: "tx_type",
        hideInTable: true,
        valueType: "select",
        valueEnum: txTypeValueEnum,
        fieldProps: { allowClear: true },
        search: { transform: (v) => ({ tx_type: v }) },
      },
      {
        title: t("pages.accounting.userFlow.search.asset", "Asset"),
        key: "search_asset_code",
        dataIndex: "asset_code",
        hideInTable: true,
        valueType: "select",
        fieldProps: {
          options: assetOptions,
          showSearch: true,
          allowClear: true,
        },
        search: { transform: (v) => ({ asset_code: v }) },
      },
      {
        title: t("pages.accounting.userFlow.search.chain", "Chain"),
        key: "search_chain_code",
        dataIndex: "chain_code",
        hideInTable: true,
        search: { transform: (v) => ({ chain_code: v }) },
      },
      {
        title: t("pages.accounting.userFlow.search.status", "Status"),
        key: "search_status",
        dataIndex: "status",
        hideInTable: true,
        valueType: "select",
        valueEnum: statusValueEnum,
        fieldProps: { allowClear: true },
        search: { transform: (v) => ({ status: v }) },
      },
      {
        title: t("pages.accounting.userFlow.search.timeRange", "Time Range"),
        key: "search_time_range",
        dataIndex: "time_range",
        hideInTable: true,
        valueType: "dateTimeRange",
        search: {
          transform: (v) => {
            const start = Array.isArray(v) ? v?.[0] : undefined;
            const end = Array.isArray(v) ? v?.[1] : undefined;

            const startSec = start ? String(dayjs(start).unix()) : undefined;
            const endSec = end ? String(dayjs(end).unix()) : undefined;

            return {
              start_time: startSec,
              end_time: endSec,
            };
          },
        },
      },
    ];
  }, [assetOptions, canRpc, intl, screens, txTypeValueEnum, statusValueEnum]);

  return (
    <PageContainer>
      <ProTable<TxItem>
        actionRef={actionRef}
        rowKey={(row) => row.id || JSON.stringify(row)}
        columns={columns}
        scroll={{ x: "max-content" }}
        search={{
          labelWidth: 120,
          span: screens.xs ? 24 : screens.sm ? 12 : 8,
        }}
        request={async (params) => {
          if (!canRpc(PERM.list)) return { success: true, data: [], total: 0 };
          try {
            const res = await adminAccountingListUserTransactionRecords(
              {
                user_id: (params as any).user_id,
                tx_type: (params as any).tx_type
                  ? Number((params as any).tx_type)
                  : undefined,
                asset_code: (params as any).asset_code,
                chain_code: (params as any).chain_code,
                status: (params as any).status,
                start_time: (params as any).start_time,
                end_time: (params as any).end_time,
                page: params.current ? Number(params.current) : 1,
                page_size: params.pageSize ? Number(params.pageSize) : 20,
              } as any,
              { skipErrorHandler: true }
            );
            if (!res?.success) throw new Error(res?.message || "");
            const data: any = res?.data || {};
            const items = data?.items ?? [];
            const total = Number(data?.pagination?.total ?? data?.total ?? 0);
            return { success: true, data: items, total };
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.accounting.userFlow.messages.loadFailed",
                  "Failed to load"
                )
            );
            return { success: false, data: [], total: 0 };
          }
        }}
        pagination={{ showSizeChanger: true }}
      />

      <Modal
        title={t(
          "pages.accounting.userFlow.detail.title",
          "Transaction Detail"
        )}
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={null}
        width={modalWidth as any}
        styles={{ body: { maxHeight: "70vh", overflow: "auto" } }}
        destroyOnClose
      >
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Descriptions
            size="small"
            bordered
            column={{ xs: 1, sm: 2, md: 3 }}
            items={[
              {
                key: "id",
                label: t("pages.accounting.userFlow.detail.id", "ID"),
                children: (
                  <Typography.Text code>
                    {detailItem?.id || "-"}
                  </Typography.Text>
                ),
              },
              {
                key: "user_id",
                label: t("pages.accounting.userFlow.detail.userId", "User ID"),
                children: (
                  <Typography.Text copyable>
                    {detailItem?.user_id || "-"}
                  </Typography.Text>
                ),
              },
              {
                key: "tx_type",
                label: t("pages.accounting.userFlow.detail.txType", "Tx Type"),
                children: renderTxTypeTag(detailItem?.tx_type),
              },
              {
                key: "status",
                label: t("pages.accounting.userFlow.detail.status", "Status"),
                children: renderStatusTag(detailItem?.status),
              },
              {
                key: "asset_code",
                label: t("pages.accounting.userFlow.detail.asset", "Asset"),
                children: (
                  <Typography.Text code>
                    {toUpper(detailItem?.asset_code) || "-"}
                  </Typography.Text>
                ),
              },
              {
                key: "chain_code",
                label: t("pages.accounting.userFlow.detail.chain", "Chain"),
                children: (
                  <Typography.Text code>
                    {toUpper(detailItem?.chain_code) || "-"}
                  </Typography.Text>
                ),
              },
              {
                key: "amount_decimal",
                label: t("pages.accounting.userFlow.detail.amount", "Amount"),
                children: detailItem?.amount_decimal || "-",
              },
              {
                key: "fee_decimal",
                label: t("pages.accounting.userFlow.detail.fee", "Fee"),
                children: detailItem?.fee_decimal || "-",
              },
              {
                key: "timestamp",
                label: t(
                  "pages.accounting.userFlow.detail.timestamp",
                  "Timestamp"
                ),
                children: formatUnixSeconds(detailItem?.timestamp) || "-",
              },
              {
                key: "tx_hash",
                label: t("pages.accounting.userFlow.detail.txHash", "Tx Hash"),
                children: detailItem?.tx_hash ? (
                  <Typography.Text copyable ellipsis={{ tooltip: true }}>
                    {detailItem.tx_hash}
                  </Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "from_address",
                label: t(
                  "pages.accounting.userFlow.detail.from",
                  "From Address"
                ),
                children: detailItem?.from_address ? (
                  <Typography.Text copyable ellipsis={{ tooltip: true }}>
                    {detailItem.from_address}
                  </Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "to_address",
                label: t("pages.accounting.userFlow.detail.to", "To Address"),
                children: detailItem?.to_address ? (
                  <Typography.Text copyable ellipsis={{ tooltip: true }}>
                    {detailItem.to_address}
                  </Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "memo",
                label: t("pages.accounting.userFlow.detail.memo", "Memo"),
                children: detailItem?.memo ? (
                  <Typography.Text ellipsis={{ tooltip: true }}>
                    {detailItem.memo}
                  </Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "biz_ref",
                label: t("pages.accounting.userFlow.detail.bizRef", "Biz Ref"),
                children: detailItem?.biz_ref ? (
                  <Typography.Text copyable ellipsis={{ tooltip: true }}>
                    {detailItem.biz_ref}
                  </Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "idempotency_key",
                label: t(
                  "pages.accounting.userFlow.detail.idempotencyKey",
                  "Idempotency Key"
                ),
                children: detailItem?.idempotency_key ? (
                  <Typography.Text copyable ellipsis={{ tooltip: true }}>
                    {detailItem.idempotency_key}
                  </Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "freeze_ledger_tx_id",
                label: t(
                  "pages.accounting.userFlow.detail.freezeLedgerTxId",
                  "Freeze Ledger Tx ID"
                ),
                children: detailItem?.freeze_ledger_tx_id ? (
                  <Typography.Text copyable>
                    {detailItem.freeze_ledger_tx_id}
                  </Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "settle_ledger_tx_id",
                label: t(
                  "pages.accounting.userFlow.detail.settleLedgerTxId",
                  "Settle Ledger Tx ID"
                ),
                children: detailItem?.settle_ledger_tx_id ? (
                  <Typography.Text copyable>
                    {detailItem.settle_ledger_tx_id}
                  </Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "confirm_ledger_tx_id",
                label: t(
                  "pages.accounting.userFlow.detail.confirmLedgerTxId",
                  "Confirm Ledger Tx ID"
                ),
                children: detailItem?.confirm_ledger_tx_id ? (
                  <Typography.Text copyable>
                    {detailItem.confirm_ledger_tx_id}
                  </Typography.Text>
                ) : (
                  "-"
                ),
              },
              {
                key: "created_at",
                label: t(
                  "pages.accounting.userFlow.detail.createdAt",
                  "Created At"
                ),
                children: detailItem?.created_at || "-",
              },
              {
                key: "updated_at",
                label: t(
                  "pages.accounting.userFlow.detail.updatedAt",
                  "Updated At"
                ),
                children: detailItem?.updated_at || "-",
              },
            ]}
          />
          {detailLoading ? (
            <Typography.Text type="secondary">
              {t("pages.accounting.userFlow.detail.loading", "Loading...")}
            </Typography.Text>
          ) : null}
        </Space>
      </Modal>
    </PageContainer>
  );
};

export default UserFlowPage;
