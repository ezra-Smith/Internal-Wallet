import type {
  AdminApproveTransferBatchBody,
  AdminCancelTransferBatchBody,
  AdminExecuteTransferBatchBody,
  AdminGetTransferBatchData,
  AdminRetryTransferBatchBody,
  AdminSubmitTransferBatchBody,
  AdminTransferBatchTimelineItem,
  AdminTransferRecipientItem,
} from "@/api/generated/schemas";
import {
  adminApproveTransferBatch,
  adminCancelTransferBatch,
  adminExecuteTransferBatch,
  adminGetTransferBatch,
  adminRetryTransferBatch,
  adminSubmitTransferBatch,
} from "@/api/generated/transfers";
import { useRbac } from "@/hooks/useRbac";
import { formatLocalDateTime } from "@/utils/date";
import type { ProColumns } from "@ant-design/pro-components";
import {
  type ActionType,
  ModalForm,
  PageContainer,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from "@ant-design/pro-components";
import { history, useIntl, useParams } from "@umijs/max";
import { App, Button, Descriptions, Space, Table, Tag, Typography } from "antd";
import React, { useCallback, useMemo, useRef, useState } from "react";

const PERM = {
  detail: "GetTransferBatch",
  submit: "SubmitTransferBatch",
  approve: "ApproveTransferBatch",
  cancel: "CancelTransferBatch",
  execute: "ExecuteTransferBatch",
  retry: "RetryTransferBatch",
};

type BatchAction =
  | { type: "submit" }
  | { type: "approve" }
  | { type: "execute" }
  | { type: "cancel" }
  | { type: "retry"; transferIds: string[] }
  | null;

const TransferBatchDetailPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (
    id: string,
    defaultMessage: string,
    values?: Record<string, any>
  ) => intl.formatMessage({ id, defaultMessage }, values);
  const { batchId = "" } = useParams<{ batchId: string }>();

  const recipientStatusValueEnum = {
    pending: {
      text: t(
        "pages.transfers.batches.detail.recipients.status.pending",
        "Pending"
      ),
    },
    processing: {
      text: t(
        "pages.transfers.batches.detail.recipients.status.processing",
        "Processing"
      ),
    },
    success: {
      text: t(
        "pages.transfers.batches.detail.recipients.status.success",
        "Success"
      ),
    },
    failed: {
      text: t(
        "pages.transfers.batches.detail.recipients.status.failed",
        "Failed"
      ),
    },
    retrying: {
      text: t(
        "pages.transfers.batches.detail.recipients.status.retrying",
        "Retrying"
      ),
    },
  };

  const recipientStatusLabel = useCallback(
    (raw: any) => {
      const s = String(raw || "")
        .trim()
        .toLowerCase();
      if (!s) return "-";
      const mapped = (recipientStatusValueEnum as any)?.[s]?.text;
      return mapped || s;
    },
    [recipientStatusValueEnum]
  );

  const actionRef = useRef<ActionType | null>(null);
  const [batchAction, setBatchAction] = useState<BatchAction>(null);
  const [headerData, setHeaderData] =
    useState<AdminGetTransferBatchData | null>(null);
  const [selectedFailedTransferIds, setSelectedFailedTransferIds] = useState<
    string[]
  >([]);

  const loadHeader = useCallback(async () => {
    if (!batchId) return;
    if (!canRpc(PERM.detail)) return;
    try {
      const res = await adminGetTransferBatch(
        { batchId },
        { page: 1, page_size: 1 },
        { skipErrorHandler: true }
      );
      if (!res?.success)
        throw new Error(
          res?.message ||
            t(
              "pages.transfers.batches.detail.messages.loadFailed",
              "Failed to load batch"
            )
        );
      setHeaderData(res.data ?? null);
    } catch (e: any) {
      message.error(
        e?.message ||
          t(
            "pages.transfers.batches.detail.messages.loadFailed",
            "Failed to load batch"
          )
      );
    }
  }, [batchId, canRpc, message]);

  const recipientColumns: ProColumns<AdminTransferRecipientItem>[] = [
    {
      key: "id-column",
      title: t("pages.transfers.batches.detail.recipients.columns.id", "ID"),
      dataIndex: "id",
      copyable: true,
      width: 160,
      search: {
        transform: (value) => ({ id: value }),
      },
    },
    {
      key: "uid-column",
      title: t("pages.transfers.batches.detail.recipients.columns.uid", "UID"),
      dataIndex: "uid",
      copyable: true,
      width: 120,
      render: (_, row) => (row as any)?.uid || "-",
      search: {
        transform: (value) => ({ uid: value }),
      },
    },
    {
      key: "nickname-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.nickname",
        "Nickname"
      ),
      dataIndex: "nickname",
      width: 140,
      render: (_, row) => (row as any)?.nickname || "-",
      search: {
        transform: (value) => ({ nickname: value }),
      },
    },
    {
      key: "email-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.email",
        "Email"
      ),
      dataIndex: "email",
      width: 200,
      render: (_, row) => (row as any)?.email || "-",
      search: {
        transform: (value) => ({ email: value }),
      },
    },
    {
      key: "phone-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.phone",
        "Phone"
      ),
      dataIndex: "phone",
      width: 160,
      render: (_, row) => (row as any)?.phone || "-",
      search: {
        transform: (value) => ({ phone: value }),
      },
    },
    {
      key: "role-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.role",
        "Role"
      ),
      dataIndex: "role",
      width: 110,
      render: (_, row) => {
        const role = String((row as any)?.role || "")
          .trim()
          .toLowerCase();
        if (role === "vip")
          return <Tag color="gold">{t("pages.users.roles.vip", "VIP")}</Tag>;
        if (role === "user")
          return <Tag>{t("pages.users.roles.user", "User")}</Tag>;
        return role || "-";
      },
      search: {
        transform: (value) => ({ role: value }),
      },
    },
    {
      key: "currency-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.currency",
        "Token"
      ),
      dataIndex: "currency",
      width: 120,
      render: (_, row) => (row as any)?.currency || "-",
      search: {
        transform: (value) => ({ currency: value }),
      },
    },
    {
      key: "amount-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.amount",
        "Amount"
      ),
      dataIndex: "amount",
      width: 120,
      search: {
        transform: (value) => ({ amount: value }),
      },
    },
    {
      key: "note-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.note",
        "Note"
      ),
      dataIndex: "note",
      ellipsis: true,
      render: (_, row) => {
        const raw = String((row as any)?.note || "").trim();
        if (!raw) return "-";
        const v = raw.toLowerCase();
        // Historical dirty data: sometimes `note` was filled with transfer type (normal/airdrop). Do not show it as remark.
        if (v === "normal" || v === "airdrop") return "-";
        return raw;
      },
      search: {
        transform: (value) => ({ note: value }),
      },
    },
    {
      key: "status-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.status",
        "Status"
      ),
      dataIndex: "status",
      width: 120,
      valueEnum: recipientStatusValueEnum,
      hideInSearch: true,
      render: (_, row) => recipientStatusLabel((row as any)?.status),
    },
    {
      key: "error-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.error",
        "Error"
      ),
      dataIndex: "error_message",
      ellipsis: true,
      search: {
        transform: (value) => ({ error_message: value }),
      },
    },
    {
      key: "processedAt-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.processedAt",
        "Processed At"
      ),
      dataIndex: "processed_at",
      valueType: "dateTimeRange",
      width: 170,
      search: {
        transform: (value) => {
          if (Array.isArray(value) && value.length === 2) {
            return {
              processed_at_start: value[0],
              processed_at_end: value[1],
            };
          }
          return {};
        },
      },
    },
    {
      key: "status-search",
      title: t(
        "pages.transfers.batches.detail.recipients.search.status",
        "Status"
      ),
      dataIndex: "status",
      hideInTable: true,
      valueEnum: recipientStatusValueEnum,
      search: {
        transform: (value) => ({ status: value }),
      },
    },
    {
      key: "actions-column",
      title: t(
        "pages.transfers.batches.detail.recipients.columns.actions",
        "Actions"
      ),
      valueType: "option",
      width: 110,
      fixed: "right",
      render: (_, row) => {
        const id = String((row as any)?.id || "").trim();
        const st = String((row as any)?.status || "")
          .trim()
          .toLowerCase();
        if (!id) return null;
        if (!canRpc(PERM.retry)) return null;
        if (st !== "failed") return null;
        return (
          <Button
            type="link"
            onClick={() => setBatchAction({ type: "retry", transferIds: [id] })}
          >
            {t(
              "pages.transfers.batches.detail.recipients.actions.retry",
              "Retry"
            )}
          </Button>
        );
      },
    },
  ];

  const timeline: AdminTransferBatchTimelineItem[] = headerData?.timeline ?? [];
  const batch = headerData?.batch;
  const summary = headerData?.summary;
  const currencySummary = (
    ((headerData as any)?.currency_summary ?? []) as any[]
  ).filter(
    (r) =>
      String(r?.currency || "").trim() || r?.total_recipients || r?.total_amount
  );
  const transferType = String(
    (batch as any)?.transfer_type || ""
  ).toLowerCase();
  const transferTypeLabel =
    transferType === "airdrop"
      ? t("pages.transfers.batches.type.airdrop", "Airdrop")
      : transferType === "normal"
      ? t("pages.transfers.batches.type.normal", "Normal")
      : transferType || "-";
  const currencyRaw = String(batch?.currency || "").trim();
  const isMixedCurrency = currencyRaw.toUpperCase() === "MIXED";
  const currencyLabel = isMixedCurrency
    ? t("common.mixedCurrencies", "多币种")
    : currencyRaw || "-";
  const batchStatus = String(batch?.status || "").trim();
  const batchDescription = String((batch as any)?.description || "").trim();

  const timelineStatusLabel = useCallback(
    (raw: any) => {
      const s = String(raw || "")
        .trim()
        .toLowerCase();
      if (!s) return "-";
      const key = `pages.transfers.batches.status.${s}`;
      const fallbackMap: Record<string, string> = {
        draft: "Draft",
        pending: "Pending",
        approved: "Approved",
        processing: "Processing",
        retry: "Retry",
        completed: "Completed",
        cancelled: "Cancelled",
        partial_failed: "Partial Failed",
      };
      return t(key, fallbackMap[s] || s);
    },
    [t]
  );

  const pageTitle = useMemo(
    () =>
      t("pages.transfers.batches.detail.title", "Transfer Batch: {id}", {
        id: batchId || "-",
      }),
    [batchId, t]
  );

  return (
    <PageContainer
      title={pageTitle}
      extra={[
        <Button key="back" onClick={() => history.push("/transfers/batches")}>
          {t("pages.transfers.batches.detail.actions.back", "Back")}
        </Button>,
        <Button key="reload" onClick={() => void loadHeader()}>
          {t("pages.transfers.batches.detail.actions.reload", "Reload")}
        </Button>,
        ...(batchStatus === "draft"
          ? [
              <Button
                key="submit"
                disabled={!canRpc(PERM.submit)}
                onClick={() => setBatchAction({ type: "submit" })}
              >
                {t("pages.transfers.batches.actions.submit", "提交")}
              </Button>,
            ]
          : []),
        ...(batchStatus === "pending"
          ? [
              <Button
                key="approve"
                disabled={!canRpc(PERM.approve)}
                onClick={() => setBatchAction({ type: "approve" })}
              >
                {t("pages.transfers.batches.actions.approve", "审批")}
              </Button>,
              <Button
                key="cancel"
                danger
                disabled={!canRpc(PERM.cancel)}
                onClick={() => setBatchAction({ type: "cancel" })}
              >
                {t("pages.transfers.batches.actions.cancel", "取消")}
              </Button>,
            ]
          : []),
        ...(batchStatus === "approved"
          ? [
              <Button
                key="execute"
                disabled={!canRpc(PERM.execute)}
                onClick={() => setBatchAction({ type: "execute" })}
              >
                {t("pages.transfers.batches.actions.execute", "执行")}
              </Button>,
              <Button
                key="cancel"
                danger
                disabled={!canRpc(PERM.cancel)}
                onClick={() => setBatchAction({ type: "cancel" })}
              >
                {t("pages.transfers.batches.actions.cancel", "取消")}
              </Button>,
            ]
          : []),
      ]}
    >
      <Space direction="vertical" style={{ width: "100%" }} size="large">
        <div>
          <Typography.Title level={5} style={{ marginTop: 0 }}>
            {t(
              "pages.transfers.batches.detail.sections.batchInfo",
              "Batch Info"
            )}
          </Typography.Title>
          <Descriptions column={2} size="small">
            <Descriptions.Item
              label={t(
                "pages.transfers.batches.detail.batch.batchId",
                "Batch ID"
              )}
            >
              {batch?.batch_id || "-"}
            </Descriptions.Item>
            <Descriptions.Item
              label={t("pages.transfers.batches.detail.batch.status", "Status")}
            >
              {timelineStatusLabel(batch?.status)}
            </Descriptions.Item>
            <Descriptions.Item
              label={t("pages.transfers.batches.detail.batch.name", "Name")}
            >
              {batch?.name || "-"}
            </Descriptions.Item>
            <Descriptions.Item
              label={t(
                "pages.transfers.batches.detail.batch.description",
                "Note"
              )}
            >
              {batchDescription || "-"}
            </Descriptions.Item>
            <Descriptions.Item
              label={t("pages.transfers.batches.detail.batch.type", "Type")}
            >
              {transferTypeLabel}
            </Descriptions.Item>
            <Descriptions.Item
              label={t(
                "pages.transfers.batches.detail.batch.currency",
                "Currency"
              )}
            >
              {currencyLabel}
            </Descriptions.Item>
            <Descriptions.Item
              label={t(
                "pages.transfers.batches.detail.batch.createdBy",
                "Created By"
              )}
            >
              {batch?.created_by || "-"}
            </Descriptions.Item>
            <Descriptions.Item
              label={t(
                "pages.transfers.batches.detail.batch.createdAt",
                "Created At"
              )}
            >
              {formatLocalDateTime(batch?.created_at)}
            </Descriptions.Item>
            <Descriptions.Item
              label={t(
                "pages.transfers.batches.detail.batch.submittedAt",
                "Submitted At"
              )}
            >
              {formatLocalDateTime(batch?.submitted_at)}
            </Descriptions.Item>
            <Descriptions.Item
              label={t(
                "pages.transfers.batches.detail.batch.approvedAt",
                "Approved At"
              )}
            >
              {formatLocalDateTime(batch?.approved_at)}
            </Descriptions.Item>
            <Descriptions.Item
              label={t(
                "pages.transfers.batches.detail.batch.completedAt",
                "Completed At"
              )}
            >
              {formatLocalDateTime(batch?.completed_at)}
            </Descriptions.Item>
          </Descriptions>
        </div>

        {summary && (
          <div>
            <Typography.Title level={5} style={{ marginTop: 0 }}>
              {t("pages.transfers.batches.detail.sections.summary", "Summary")}
            </Typography.Title>
            <Descriptions column={2} size="small">
              <Descriptions.Item
                label={t(
                  "pages.transfers.batches.detail.summary.totalAmount",
                  "Total Amount"
                )}
              >
                {isMixedCurrency ? "-" : summary.total_amount ?? "-"}
              </Descriptions.Item>
              <Descriptions.Item
                label={t(
                  "pages.transfers.batches.detail.summary.success",
                  "Success"
                )}
              >
                {summary.success_count ?? "-"}
              </Descriptions.Item>
              <Descriptions.Item
                label={t(
                  "pages.transfers.batches.detail.summary.failed",
                  "Failed"
                )}
              >
                {summary.failed_count ?? "-"}
              </Descriptions.Item>
              <Descriptions.Item
                label={t(
                  "pages.transfers.batches.detail.summary.pending",
                  "Pending"
                )}
              >
                {summary.pending_count ?? "-"}
              </Descriptions.Item>
            </Descriptions>
          </div>
        )}

        {currencySummary.length ? (
          <div>
            <Typography.Title level={5} style={{ marginTop: 0 }}>
              {t(
                "pages.transfers.batches.detail.sections.currencySummary",
                "Token Summary"
              )}
            </Typography.Title>
            <Table
              size="small"
              pagination={false}
              rowKey={(r) =>
                `${String((r as any)?.currency || "")}-${String(
                  (r as any)?.total_amount || ""
                )}`
              }
              columns={[
                {
                  title: t(
                    "pages.transfers.batches.detail.currencySummary.columns.token",
                    "Token"
                  ),
                  dataIndex: "currency",
                  render: (v) => String(v || "").trim() || "-",
                },
                {
                  title: t(
                    "pages.transfers.batches.detail.currencySummary.columns.totalCount",
                    "Total Count"
                  ),
                  dataIndex: "total_recipients",
                  width: 120,
                  render: (v) => (v === 0 || v ? String(v) : "-"),
                },
                {
                  title: t(
                    "pages.transfers.batches.detail.currencySummary.columns.totalAmount",
                    "Total Amount"
                  ),
                  dataIndex: "total_amount",
                  render: (v) => String(v || "").trim() || "-",
                },
              ]}
              dataSource={currencySummary as any}
            />
          </div>
        ) : null}

        {timeline.length ? (
          <div>
            <Typography.Title level={5} style={{ marginTop: 0 }}>
              {t(
                "pages.transfers.batches.detail.sections.timeline",
                "Timeline"
              )}
            </Typography.Title>
            <ProTable<AdminTransferBatchTimelineItem>
              rowKey={(r) =>
                `${r.status || ""}-${r.timestamp || ""}-${r.operator || ""}`
              }
              columns={[
                {
                  title: t(
                    "pages.transfers.batches.detail.timeline.columns.status",
                    "Status"
                  ),
                  dataIndex: "status",
                  width: 150,
                  render: (_, row) => timelineStatusLabel((row as any)?.status),
                },
                {
                  title: t(
                    "pages.transfers.batches.detail.timeline.columns.time",
                    "Time"
                  ),
                  dataIndex: "timestamp",
                  valueType: "dateTime",
                  width: 180,
                },
                {
                  title: t(
                    "pages.transfers.batches.detail.timeline.columns.operator",
                    "Operator"
                  ),
                  dataIndex: "operator",
                  width: 160,
                },
                {
                  title: t(
                    "pages.transfers.batches.detail.timeline.columns.note",
                    "Note"
                  ),
                  dataIndex: "note",
                  render: (_, row) => {
                    const v = String((row as any)?.note || "").trim();
                    if (!v) return "-";
                    return (
                      <Typography.Paragraph
                        style={{ margin: 0 }}
                        ellipsis={{ rows: 3, expandable: true }}
                      >
                        {v}
                      </Typography.Paragraph>
                    );
                  },
                },
              ]}
              dataSource={timeline}
              pagination={false}
              search={false}
              options={{
                reload: false,
                density: false,
                fullScreen: false,
                setting: false,
              }}
            />
          </div>
        ) : null}

        <ProTable<AdminTransferRecipientItem>
          actionRef={actionRef}
          rowKey={(r) => r.id || (r as any)?.uid || JSON.stringify(r)}
          columns={recipientColumns}
          scroll={{ x: 2600 }}
          rowSelection={
            canRpc(PERM.retry)
              ? {
                  selectedRowKeys: selectedFailedTransferIds,
                  preserveSelectedRowKeys: true,
                  getCheckboxProps: (row) => {
                    const st = String((row as any)?.status || "")
                      .trim()
                      .toLowerCase();
                    return { disabled: st !== "failed" };
                  },
                  onChange: (keys) =>
                    setSelectedFailedTransferIds(keys.map((k) => String(k))),
                }
              : undefined
          }
          request={async (params) => {
            if (!batchId) return { success: false, data: [], total: 0 };
            if (!canRpc(PERM.detail))
              return { success: true, data: [], total: 0 };
            try {
              // 提取所有筛选参数
              const filterParams: any = {
                page: params.current,
                page_size: params.pageSize,
              };

              // 状态筛选
              if ((params as any).status) {
                filterParams.status = (params as any).status;
              }

              // ID 筛选
              if ((params as any).id) {
                filterParams.id = String((params as any).id).trim();
              }

              // 用户相关筛选
              if ((params as any).uid) {
                filterParams.uid = String((params as any).uid).trim();
              }
              if ((params as any).nickname) {
                filterParams.nickname = String((params as any).nickname).trim();
              }
              if ((params as any).email) {
                filterParams.email = String((params as any).email).trim();
              }
              if ((params as any).phone) {
                filterParams.phone = String((params as any).phone).trim();
              }
              if ((params as any).role) {
                filterParams.role = String((params as any).role).trim();
              }

              // 交易相关筛选
              if ((params as any).currency) {
                filterParams.currency = String((params as any).currency).trim();
              }
              if ((params as any).amount) {
                filterParams.amount = String((params as any).amount).trim();
              }
              if ((params as any).note) {
                filterParams.note = String((params as any).note).trim();
              }
              if ((params as any).error_message) {
                filterParams.error_message = String(
                  (params as any).error_message
                ).trim();
              }

              // 时间范围筛选
              if ((params as any).processed_at_start) {
                filterParams.processed_at_start = String(
                  (params as any).processed_at_start
                ).trim();
              }
              if ((params as any).processed_at_end) {
                filterParams.processed_at_end = String(
                  (params as any).processed_at_end
                ).trim();
              }

              const res = await adminGetTransferBatch(
                { batchId },
                filterParams,
                { skipErrorHandler: true }
              );
              if (res?.success && res.data) setHeaderData(res.data);
              return {
                success: !!res?.success,
                data: res.data?.recipients ?? [],
                total: Number(res.data?.pagination?.total ?? 0),
              };
            } catch (e: any) {
              message.error(
                e?.message ||
                  t(
                    "pages.transfers.batches.detail.messages.recipientsLoadFailed",
                    "Failed to load recipients"
                  )
              );
              return { success: false, data: [], total: 0 };
            }
          }}
          search={{ labelWidth: 110 }}
          headerTitle={t(
            "pages.transfers.batches.detail.recipients.title",
            "Recipients"
          )}
          toolBarRender={() => {
            if (!canRpc(PERM.retry)) return [];
            return [
              <Button
                key="retrySelected"
                disabled={selectedFailedTransferIds.length === 0}
                onClick={() =>
                  setBatchAction({
                    type: "retry",
                    transferIds: selectedFailedTransferIds,
                  })
                }
              >
                {t(
                  "pages.transfers.batches.detail.actions.retrySelected",
                  "Retry Selected ({count})",
                  {
                    count: selectedFailedTransferIds.length,
                  }
                )}
              </Button>,
            ];
          }}
        />
      </Space>

      <ModalForm<AdminSubmitTransferBatchBody>
        title={t("pages.transfers.batches.submit.title", "Submit Batch")}
        open={batchAction?.type === "submit"}
        onOpenChange={(open) => {
          if (!open) setBatchAction(null);
        }}
        onFinish={async (values) => {
          if (!batchId) return false;
          try {
            const res = await adminSubmitTransferBatch({ batchId }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.transfers.batches.messages.submitFailed",
                    "Submit failed"
                  )
              );
            message.success(
              t("pages.transfers.batches.messages.submitted", "Submitted")
            );
            setBatchAction(null);
            actionRef.current?.reload();
            void loadHeader();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.transfers.batches.messages.submitFailed",
                  "Submit failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="note"
          label={t("pages.transfers.batches.form.note", "Note")}
          fieldProps={{ rows: 3 }}
        />
      </ModalForm>

      <ModalForm<AdminApproveTransferBatchBody>
        title={t("pages.transfers.batches.approve.title", "Approve Batch")}
        open={batchAction?.type === "approve"}
        onOpenChange={(open) => {
          if (!open) setBatchAction(null);
        }}
        initialValues={{ action: "approve" }}
        onFinish={async (values) => {
          if (!batchId) return false;
          try {
            const res = await adminApproveTransferBatch({ batchId }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.transfers.batches.messages.approveFailed",
                    "Approve failed"
                  )
              );
            message.success(
              t("pages.transfers.batches.messages.approved", "Approved")
            );
            setBatchAction(null);
            actionRef.current?.reload();
            void loadHeader();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.transfers.batches.messages.approveFailed",
                  "Approve failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormSelect
          name="action"
          label={t("pages.transfers.batches.form.action", "Action")}
          options={[
            {
              label: t("pages.transfers.batches.actions.approve", "Approve"),
              value: "approve",
            },
            {
              label: t("pages.transfers.batches.actions.reject", "Reject"),
              value: "reject",
            },
          ]}
        />
        <ProFormTextArea
          name="note"
          label={t("pages.transfers.batches.form.note", "Note")}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText
          name="two_fa_code"
          label={t("pages.transfers.batches.form.twoFaCode", "2FA Code")}
          rules={[
            {
              required: true,
              message: t("common.required", "Please enter 2FA Code"),
            },
          ]}
        />
      </ModalForm>

      <ModalForm<AdminExecuteTransferBatchBody>
        title={t("pages.transfers.batches.execute.title", "Execute Batch")}
        open={batchAction?.type === "execute"}
        onOpenChange={(open) => {
          if (!open) setBatchAction(null);
        }}
        onFinish={async (values) => {
          if (!batchId) return false;
          try {
            const res = await adminExecuteTransferBatch({ batchId }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.transfers.batches.messages.executeFailed",
                    "Execute failed"
                  )
              );
            message.success(
              t(
                "pages.transfers.batches.messages.executed",
                "Execution started"
              )
            );
            setBatchAction(null);
            actionRef.current?.reload();
            void loadHeader();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.transfers.batches.messages.executeFailed",
                  "Execute failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="note"
          label={t("pages.transfers.batches.form.note", "Note")}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText
          name="two_fa_code"
          label={t("pages.transfers.batches.form.twoFaCode", "2FA Code")}
          rules={[
            {
              required: true,
              message: t("common.required", "Please enter 2FA Code"),
            },
          ]}
        />
      </ModalForm>

      <ModalForm<AdminRetryTransferBatchBody>
        title={t(
          "pages.transfers.batches.detail.retry.title",
          "Retry Failed Items"
        )}
        open={batchAction?.type === "retry"}
        onOpenChange={(open) => {
          if (!open) setBatchAction(null);
        }}
        onFinish={async (values) => {
          if (!batchId) return false;
          if (batchAction?.type !== "retry") return false;
          const transferIds = (batchAction.transferIds || [])
            .map((v) => String(v || "").trim())
            .filter(Boolean);
          if (transferIds.length === 0) {
            message.error(
              t(
                "pages.transfers.batches.detail.messages.retryEmpty",
                "Select failed recipients first"
              )
            );
            return false;
          }
          try {
            const res = await adminRetryTransferBatch(
              { batchId },
              { ...values, transfer_ids: transferIds },
              { skipErrorHandler: true }
            );
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.transfers.batches.messages.retryFailed",
                    "Retry failed"
                  )
              );
            message.success(
              t(
                "pages.transfers.batches.messages.retryRequested",
                "Retry requested"
              )
            );
            setBatchAction(null);
            setSelectedFailedTransferIds([]);
            actionRef.current?.reload();
            void loadHeader();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.transfers.batches.messages.retryFailed",
                  "Retry failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="note"
          label={t("pages.transfers.batches.form.note", "Note")}
          rules={[{ required: true }]}
          fieldProps={{ rows: 3 }}
        />
        <ProFormText
          name="two_fa_code"
          label={t("pages.transfers.batches.form.twoFaCode", "2FA Code")}
          rules={[
            {
              required: true,
              message: t("common.required", "Please enter 2FA Code"),
            },
          ]}
        />
      </ModalForm>

      <ModalForm<AdminCancelTransferBatchBody>
        title={t("pages.transfers.batches.cancel.title", "Cancel Batch")}
        open={batchAction?.type === "cancel"}
        onOpenChange={(open) => {
          if (!open) setBatchAction(null);
        }}
        submitter={{ submitButtonProps: { danger: true } }}
        onFinish={async (values) => {
          if (!batchId) return false;
          try {
            const res = await adminCancelTransferBatch({ batchId }, values, {
              skipErrorHandler: true,
            });
            if (!res?.success)
              throw new Error(
                res?.message ||
                  t(
                    "pages.transfers.batches.messages.cancelFailed",
                    "Cancel failed"
                  )
              );
            message.success(
              t("pages.transfers.batches.messages.cancelled", "Cancelled")
            );
            setBatchAction(null);
            actionRef.current?.reload();
            void loadHeader();
            return true;
          } catch (e: any) {
            message.error(
              e?.message ||
                t(
                  "pages.transfers.batches.messages.cancelFailed",
                  "Cancel failed"
                )
            );
            return false;
          }
        }}
      >
        <ProFormTextArea
          name="reason"
          label={t("pages.transfers.batches.form.reason", "Reason")}
          rules={[{ required: true }]}
          fieldProps={{ rows: 3 }}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default TransferBatchDetailPage;
