import { adminListCurrencies } from "@/api/generated/assets";
import type {
  AdminCurrencyItem,
  CustomApiV1AdminTransferImportPOSTBody,
  CustomApiV1AdminTransferImportPreviewPOSTBody,
} from "@/api/generated/schemas";
import {
  customApiV1AdminTransferImportPOST,
  customApiV1AdminTransferImportPreviewPOST,
} from "@/api/generated/transfers";
import {
  DeleteOutlined,
  DownloadOutlined,
  InboxOutlined,
  UploadOutlined,
} from "@ant-design/icons";
import type { UploadFile, UploadProps } from "antd";
import {
  Alert,
  App,
  Button,
  Input,
  Modal,
  Radio,
  Select,
  Steps,
  Table,
  Tag,
  Typography,
  Upload,
} from "antd";
import { createStyles } from "antd-style";
import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

const { Text } = Typography;
const { Dragger } = Upload;

const MAX_FILE_MB = 5;
const MAX_ROWS = 1000;

type AddMethod = "manual" | "excel";
type TransferType = "normal" | "airdrop";

type ManualRow = {
  key: string;
  uid: string;
  currency: string;
  amount: string;
  note: string;
};

type PreviewRow = {
  key: string;
  row: number;
  uid: string;
  currency: string;
  amount: string;
  note: string;
  errors: string[];
};

const useStyles = createStyles(() => ({
  subtitle: {
    fontSize: 14,
    color: "#6b7280",
    marginTop: -8,
    marginBottom: 16,
  },
  steps: {
    marginBottom: 16,
  },
  section: {
    marginBottom: 16,
  },
  sectionTitle: {
    fontSize: 14,
    fontWeight: 600,
    color: "#374151",
    marginBottom: 10,
  },
  grid2: {
    display: "grid",
    gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
    gap: 12,
    "@media (max-width: 576px)": {
      gridTemplateColumns: "1fr",
    },
  },
  helpBox: {
    background: "#f9fafb",
    border: "1px solid #e5e7eb",
    borderRadius: 8,
    padding: 12,
  },
  dragger: {
    ".ant-upload-drag": {
      borderRadius: 8,
      border: "2px dashed #d1d5db",
      background: "#fafafa",
      "&:hover": {
        borderColor: "#3b82f6",
      },
    },
  },
  previewTable: {
    border: "1px solid #f0f0f0",
    borderRadius: 10,
    overflow: "hidden",
  },
  errText: {
    color: "#ef4444",
    fontSize: 12,
    lineHeight: 1.5,
  },
  summaryRow: {
    display: "flex",
    gap: 12,
    flexWrap: "wrap",
    marginBottom: 12,
    color: "#6b7280",
    fontSize: 13,
  },
}));

function csvEscape(v: string): string {
  const s = String(v ?? "");
  if (
    s.includes('"') ||
    s.includes(",") ||
    s.includes("\n") ||
    s.includes("\r")
  ) {
    return `"${s.replaceAll('"', '""')}"`;
  }
  return s;
}

function buildCsv(
  headers: { uid: string; currency: string; amount: string; note: string },
  rows: { uid: string; currency: string; amount: string; note: string }[]
): string {
  const header = [headers.uid, headers.currency, headers.amount, headers.note]
    .map(csvEscape)
    .join(",");
  const lines = rows.map((r) =>
    [
      csvEscape(r.uid),
      csvEscape(r.currency),
      csvEscape(r.amount),
      csvEscape(r.note),
    ].join(",")
  );
  return [header, ...lines].join("\n");
}

function toNumber(v: any): number {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

interface CreateBatchModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated?: (batchId: string) => void;
  canImport: boolean;
  t: (
    id: string,
    defaultMessage: string,
    values?: Record<string, any>
  ) => string;
}

const CreateBatchModal: React.FC<CreateBatchModalProps> = ({
  open,
  onOpenChange,
  onCreated,
  canImport,
  t,
}) => {
  const { styles } = useStyles();
  const { message } = App.useApp();

  const [step, setStep] = useState(0);
  const [addMethod, setAddMethod] = useState<AddMethod>("excel");

  const [transferType, setTransferType] = useState<TransferType>("normal");
  const [batchName, setBatchName] = useState("");
  const [description, setDescription] = useState("");

  const [assetsLoading, setAssetsLoading] = useState(false);
  const [assetOptions, setAssetOptions] = useState<
    { value: string; label: string }[]
  >([]);

  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const lastNotifiedFileUidRef = useRef<string | null>(null);

  const [manualRows, setManualRows] = useState<ManualRow[]>([]);

  const [parsing, setParsing] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const [previewRows, setPreviewRows] = useState<PreviewRow[]>([]);
  const [previewSummary, setPreviewSummary] = useState<{
    total: number;
    valid: number;
    invalid: number;
    totalAmount: string;
  }>({
    total: 0,
    valid: 0,
    invalid: 0,
    totalAmount: "0.000000",
  });
  const [previewCurrencyStats, setPreviewCurrencyStats] = useState<
    { currency: string; validRows: number; totalAmount: string }[]
  >([]);
  const [dirty, setDirty] = useState(false);

  const invalidCount = useMemo(
    () => previewRows.filter((r) => r.errors.length > 0).length,
    [previewRows]
  );
  const csvHeaders = useMemo(
    () => ({
      uid: t("pages.transfers.batches.import.header.uid", "用户UID"),
      currency: t("pages.transfers.batches.import.header.currency", "币种"),
      amount: t("pages.transfers.batches.import.header.amount", "金额"),
      note: t("pages.transfers.batches.import.header.note", "备注"),
    }),
    [t]
  );

  const loadOptions = useCallback(async () => {
    setAssetsLoading(true);
    try {
      const currenciesRes = await adminListCurrencies(
        { page: 1, page_size: 200, status: 1 },
        { skipErrorHandler: true }
      );
      const currs = (currenciesRes?.data?.currencies ??
        []) as unknown as AdminCurrencyItem[];
      const currOpts = currs
        .map((c) => {
          const code = String(c.asset_code || "")
            .trim()
            .toUpperCase();
          if (!code) return null;
          return { value: code, label: code };
        })
        .filter(Boolean) as { value: string; label: string }[];
      currOpts.sort((a, b) => a.label.localeCompare(b.label));
      setAssetOptions(currOpts);
    } catch {
    } finally {
      setAssetsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    void loadOptions();
  }, [loadOptions, open]);

  useEffect(() => {
    if (open) return;
    setStep(0);
    setAddMethod("excel");
    setTransferType("normal");
    setBatchName("");
    setDescription("");
    setFileList([]);
    setManualRows([]);
    setParsing(false);
    setSubmitting(false);
    setPreviewRows([]);
    setPreviewSummary({
      total: 0,
      valid: 0,
      invalid: 0,
      totalAmount: "0.000000",
    });
    setPreviewCurrencyStats([]);
    setDirty(false);
    lastNotifiedFileUidRef.current = null;
  }, [open]);

  const uploadProps: UploadProps = {
    name: "file",
    multiple: false,
    accept: ".csv,.xlsx",
    fileList,
    maxCount: 1,
    beforeUpload: (file) => {
      const name = String(file?.name || "");
      const ext = name.toLowerCase().split(".").pop() || "";
      if (!["csv", "xlsx"].includes(ext)) {
        message.error(
          t(
            "pages.transfers.batches.import.fileTypeInvalid",
            "仅支持 .csv / .xlsx 文件"
          )
        );
        return Upload.LIST_IGNORE;
      }
      if (file.size > MAX_FILE_MB * 1024 * 1024) {
        message.error(
          t(
            "pages.transfers.batches.import.fileTooLarge",
            "文件过大（最大 {max}MB）",
            { max: MAX_FILE_MB }
          )
        );
        return Upload.LIST_IGNORE;
      }
      return true;
    },
    onChange: (info) => {
      const list = info.fileList.slice(-1);
      setFileList(list);
      const f = list[0];
      if (f?.uid && lastNotifiedFileUidRef.current !== f.uid) {
        lastNotifiedFileUidRef.current = f.uid;
        message.success(
          t(
            "pages.transfers.batches.import.fileSelected",
            "已选择文件：{name}",
            { name: f.name || "" }
          )
        );
      }
    },
    onRemove: () => {
      setFileList([]);
      lastNotifiedFileUidRef.current = null;
    },
  };

  const canGoNext = useMemo(() => {
    if (!canImport) return false;
    if (!batchName.trim()) return false;
    if (addMethod === "excel") return fileList.length > 0;
    if (manualRows.length === 0) return false;
    return manualRows.some(
      (r) =>
        !!(
          r.uid.trim() ||
          r.currency.trim() ||
          r.amount.trim() ||
          r.note.trim()
        )
    );
  }, [addMethod, batchName, canImport, fileList.length, manualRows]);

  const makePreviewRequestBody =
    async (): Promise<CustomApiV1AdminTransferImportPreviewPOSTBody | null> => {
      const name = batchName.trim();
      if (!name) return null;

      const base: Omit<CustomApiV1AdminTransferImportPreviewPOSTBody, "file"> =
        {
          name,
          ...(description.trim() ? { description: description.trim() } : {}),
          transfer_type: transferType,
        };

      if (addMethod === "excel") {
        const f = fileList[0]?.originFileObj;
        if (!f) return null;
        return { ...base, file: f as Blob };
      }

      if (manualRows.length > MAX_ROWS) {
        message.error(
          t(
            "pages.transfers.batches.import.tooManyRows",
            "最多支持 {max} 行数据",
            { max: MAX_ROWS }
          )
        );
        return null;
      }

      const csv = buildCsv(
        csvHeaders,
        manualRows.map((r) => ({
          uid: r.uid,
          currency: r.currency,
          amount: r.amount,
          note: r.note,
        }))
      );
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
      return { ...base, file: blob };
    };

  const updatePreviewFromResponse = (res: any) => {
    const items = (res?.data?.preview_items ?? []) as any[];
    const mapped: PreviewRow[] = items.map((it) => ({
      key: String(it?.row ?? `${Math.random()}`),
      row: toNumber(it?.row),
      uid: String(it?.uid || ""),
      currency: String(it?.currency || ""),
      amount: String(it?.amount || ""),
      note: String(it?.note || ""),
      errors: Array.isArray(it?.errors)
        ? it.errors.map((e: any) => String(e)).filter(Boolean)
        : [],
    }));

    setPreviewRows(mapped);
    const r = res?.data?.import_result;
    setPreviewSummary({
      total: toNumber(r?.total_rows),
      valid: toNumber(r?.valid_rows),
      invalid: toNumber(r?.invalid_rows),
      totalAmount: String(r?.total_amount || "0.000000"),
    });
    const stats = (r?.currency_stats ?? []) as any[];
    setPreviewCurrencyStats(
      stats
        .map((s) => ({
          currency: String(s?.currency || "")
            .trim()
            .toUpperCase(),
          validRows: toNumber(s?.valid_rows),
          totalAmount: String(s?.total_amount || "0.000000"),
        }))
        .filter((s) => !!s.currency)
    );
    setDirty(false);
  };

  const handleNext = async () => {
    if (parsing) return;
    if (!canGoNext) return;

    const body = await makePreviewRequestBody();
    if (!body) {
      message.error(
        t(
          "pages.transfers.batches.import.missingParams",
          "请完善批次信息与导入数据"
        )
      );
      return;
    }

    setParsing(true);
    try {
      const res = await customApiV1AdminTransferImportPreviewPOST(body, {
        skipErrorHandler: true,
      });
      if (!res) throw new Error("preview failed");
      updatePreviewFromResponse(res);
      setStep(1);
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.transfers.batches.import.previewFailed", "预览失败")
      );
    } finally {
      setParsing(false);
    }
  };

  const handleRevalidate = async () => {
    if (parsing) return;
    if (!batchName.trim()) {
      message.error(
        t(
          "pages.transfers.batches.import.missingParams",
          "请完善批次信息与导入数据"
        )
      );
      return;
    }
    if (previewRows.length === 0) {
      message.error(
        t("pages.transfers.batches.import.empty", "未识别到有效数据")
      );
      return;
    }

    setParsing(true);
    try {
      const csv = buildCsv(
        csvHeaders,
        previewRows.map((r) => ({
          uid: r.uid,
          currency: r.currency,
          amount: r.amount,
          note: r.note,
        }))
      );
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });

      const body: CustomApiV1AdminTransferImportPreviewPOSTBody = {
        file: blob,
        name: batchName.trim(),
        ...(description.trim() ? { description: description.trim() } : {}),
        transfer_type: transferType,
      };

      const res = await customApiV1AdminTransferImportPreviewPOST(body, {
        skipErrorHandler: true,
      });
      if (!res) throw new Error("preview failed");

      updatePreviewFromResponse(res);
      message.success(
        t("pages.transfers.batches.import.revalidated", "已重新校验")
      );
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.transfers.batches.import.previewFailed", "预览失败")
      );
    } finally {
      setParsing(false);
    }
  };

  const handleSubmit = async () => {
    if (submitting || parsing) return;
    if (!canImport) return;
    if (dirty) {
      message.error(
        t(
          "pages.transfers.batches.import.dirtyBlocked",
          "存在未校验修改，请先重新校验"
        )
      );
      return;
    }
    if (invalidCount > 0) {
      message.error(
        t(
          "pages.transfers.batches.import.blocked",
          "存在错误数据，请修正后再提交"
        )
      );
      return;
    }
    if (previewRows.length === 0) {
      message.error(
        t("pages.transfers.batches.import.empty", "未识别到有效数据")
      );
      return;
    }

    setSubmitting(true);
    try {
      const csv = buildCsv(
        csvHeaders,
        previewRows.map((r) => ({
          uid: r.uid,
          currency: r.currency,
          amount: r.amount,
          note: r.note,
        }))
      );
      const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
      const body: CustomApiV1AdminTransferImportPOSTBody = {
        file: blob,
        name: batchName.trim(),
        ...(description.trim() ? { description: description.trim() } : {}),
        transfer_type: transferType,
      };
      const res = await customApiV1AdminTransferImportPOST(body, {
        skipErrorHandler: true,
      });
      if (!res) throw new Error("import failed");

      if (!res?.success) {
        if (Array.isArray((res as any)?.data?.preview_items))
          updatePreviewFromResponse(res);
        message.error(
          res?.message ||
            t("pages.transfers.batches.import.submitFailed", "提交失败")
        );
        return;
      }

      const batchId = String(res?.data?.batch_id || "").trim();
      message.success(
        t("pages.transfers.batches.import.created", "批次创建成功")
      );
      onOpenChange(false);
      if (batchId) onCreated?.(batchId);
    } catch (e: any) {
      message.error(
        e?.message ||
          t("pages.transfers.batches.import.submitFailed", "提交失败")
      );
    } finally {
      setSubmitting(false);
    }
  };

  const getAssetOptionsWithFallback = useCallback(
    (value: string) => {
      const v = String(value || "")
        .trim()
        .toUpperCase();
      if (!v) return assetOptions;
      if (assetOptions.some((o) => o.value === v)) return assetOptions;
      return [
        {
          value: v,
          label: `${v} (${t("common.invalid", "无效")})`,
          disabled: true,
        },
        ...assetOptions,
      ];
    },
    [assetOptions, t]
  );

  const handleDownloadSample = () => {
    const rows = [
      {
        uid: "10001",
        currency: "USDT",
        amount: "10.5",
        note: t("pages.transfers.batches.import.sample.note1", "普通发放"),
      },
      {
        uid: "10001",
        currency: "USDC",
        amount: "3.25",
        note: t("pages.transfers.batches.import.sample.note2", "空投"),
      },
    ];
    const csv = buildCsv(csvHeaders, rows);
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "transfer-batch-import-sample.csv";
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  const handleAddEmptyManualRow = () => {
    if (manualRows.length >= MAX_ROWS) {
      message.error(
        t(
          "pages.transfers.batches.import.tooManyRows",
          "最多支持 {max} 行数据",
          { max: MAX_ROWS }
        )
      );
      return;
    }
    const row: ManualRow = {
      key: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
      uid: "",
      currency: assetOptions[0]?.value || "",
      amount: "",
      note: "",
    };
    setManualRows((prev) => [...prev, row]);
  };

  const manualColumns = useMemo(
    () => [
      {
        title: t("pages.transfers.batches.create.table.uid", "用户UID"),
        dataIndex: "uid",
        width: 180,
        render: (_: any, record: ManualRow) => (
          <Input
            value={record.uid}
            onChange={(e) => {
              const v = e.target.value;
              setManualRows((prev) =>
                prev.map((r) => (r.key === record.key ? { ...r, uid: v } : r))
              );
            }}
            placeholder="UID"
          />
        ),
      },
      {
        title: t("pages.transfers.batches.create.table.currency", "币种"),
        dataIndex: "currency",
        width: 160,
        render: (_: any, record: ManualRow) => (
          <Select
            value={record.currency ? record.currency.toUpperCase() : undefined}
            options={getAssetOptionsWithFallback(record.currency)}
            loading={assetsLoading}
            onChange={(v) =>
              setManualRows((prev) =>
                prev.map((r) =>
                  r.key === record.key ? { ...r, currency: String(v || "") } : r
                )
              )
            }
            style={{ width: "100%" }}
            placeholder={t(
              "pages.transfers.batches.create.form.currencyPlaceholder",
              "请选择币种"
            )}
            showSearch
            optionFilterProp="label"
            allowClear
          />
        ),
      },
      {
        title: t("pages.transfers.batches.create.table.amount", "转账金额"),
        dataIndex: "amount",
        width: 160,
        render: (_: any, record: ManualRow) => (
          <Input
            value={record.amount}
            onChange={(e) => {
              const v = e.target.value;
              setManualRows((prev) =>
                prev.map((r) =>
                  r.key === record.key ? { ...r, amount: v } : r
                )
              );
            }}
            placeholder="0.00"
          />
        ),
      },
      {
        title: t("pages.transfers.batches.create.table.note", "备注"),
        dataIndex: "note",
        render: (_: any, record: ManualRow) => (
          <Input
            value={record.note}
            onChange={(e) => {
              const v = e.target.value;
              setManualRows((prev) =>
                prev.map((r) => (r.key === record.key ? { ...r, note: v } : r))
              );
            }}
            placeholder={t(
              "pages.transfers.batches.create.form.notePlaceholder",
              "可选"
            )}
          />
        ),
      },
      {
        title: t("common.actions", "操作"),
        key: "actions",
        width: 80,
        render: (_: any, record: ManualRow) => (
          <Button
            type="link"
            danger
            size="small"
            icon={<DeleteOutlined />}
            onClick={() =>
              setManualRows((prev) => prev.filter((x) => x.key !== record.key))
            }
          />
        ),
      },
    ],
    [assetsLoading, getAssetOptionsWithFallback, t]
  );

  const previewColumns = useMemo(
    () => [
      { title: "#", dataIndex: "row", width: 60 },
      {
        title: t("pages.transfers.batches.create.table.uid", "用户UID"),
        dataIndex: "uid",
        render: (_: any, record: PreviewRow) => (
          <Input
            value={record.uid}
            onChange={(e) => {
              const v = e.target.value;
              setPreviewRows((prev) =>
                prev.map((r) => (r.key === record.key ? { ...r, uid: v } : r))
              );
              setDirty(true);
            }}
          />
        ),
      },
      {
        title: t("pages.transfers.batches.create.table.currency", "币种"),
        dataIndex: "currency",
        width: 160,
        render: (_: any, record: PreviewRow) => (
          <Select
            value={record.currency ? record.currency.toUpperCase() : undefined}
            options={getAssetOptionsWithFallback(record.currency)}
            loading={assetsLoading}
            onChange={(v) => {
              const next = String(v || "");
              setPreviewRows((prev) =>
                prev.map((r) =>
                  r.key === record.key ? { ...r, currency: next } : r
                )
              );
              setDirty(true);
            }}
            style={{ width: "100%" }}
            placeholder={t(
              "pages.transfers.batches.create.form.currencyPlaceholder",
              "请选择币种"
            )}
            showSearch
            optionFilterProp="label"
            allowClear
          />
        ),
      },
      {
        title: t("pages.transfers.batches.create.table.amount", "转账金额"),
        dataIndex: "amount",
        width: 160,
        render: (_: any, record: PreviewRow) => (
          <Input
            value={record.amount}
            onChange={(e) => {
              const v = e.target.value;
              setPreviewRows((prev) =>
                prev.map((r) =>
                  r.key === record.key ? { ...r, amount: v } : r
                )
              );
              setDirty(true);
            }}
          />
        ),
      },
      {
        title: t("pages.transfers.batches.create.table.note", "备注"),
        dataIndex: "note",
        render: (_: any, record: PreviewRow) => (
          <Input
            value={record.note}
            onChange={(e) => {
              const v = e.target.value;
              setPreviewRows((prev) =>
                prev.map((r) => (r.key === record.key ? { ...r, note: v } : r))
              );
              setDirty(true);
            }}
          />
        ),
      },
      {
        title: t("pages.transfers.batches.create.table.errors", "错误"),
        dataIndex: "errors",
        width: 220,
        render: (_: any, record: PreviewRow) =>
          record.errors?.length ? (
            <div className={styles.errText}>
              {record.errors.map((e, idx) => (
                <div key={idx}>{e}</div>
              ))}
            </div>
          ) : (
            <Text type="secondary">-</Text>
          ),
      },
      {
        title: t("common.actions", "操作"),
        key: "actions",
        width: 80,
        render: (_: any, record: PreviewRow) => (
          <Button
            type="link"
            danger
            size="small"
            icon={<DeleteOutlined />}
            onClick={() => {
              setPreviewRows((prev) =>
                prev.filter((x) => x.key !== record.key)
              );
              setDirty(true);
            }}
          />
        ),
      },
    ],
    [assetsLoading, getAssetOptionsWithFallback, styles.errText, t]
  );

  const peopleStats = useMemo(() => {
    const m = new Map<string, Set<string>>();
    previewRows.forEach((r) => {
      if (r.errors?.length) return;
      const c = String(r.currency || "")
        .trim()
        .toUpperCase();
      const u = String(r.uid || "").trim();
      if (!c || !u) return;
      if (!m.has(c)) m.set(c, new Set<string>());
      m.get(c)!.add(u);
    });
    return Array.from(m.entries())
      .map(([currency, set]) => ({ currency, people: set.size }))
      .sort((a, b) => a.currency.localeCompare(b.currency));
  }, [previewRows]);

  const tokenStats = useMemo(() => {
    return [...previewCurrencyStats].sort((a, b) =>
      a.currency.localeCompare(b.currency)
    );
  }, [previewCurrencyStats]);

  return (
    <Modal
      open={open}
      title={t("pages.transfers.batches.create.title", "创建转账批次")}
      width={1000}
      onCancel={() => onOpenChange(false)}
      footer={
        step === 0
          ? [
              <Button key="cancel" onClick={() => onOpenChange(false)}>
                {t("common.cancel", "取消")}
              </Button>,
              <Button
                key="next"
                type="primary"
                icon={<UploadOutlined />}
                disabled={!canGoNext}
                loading={parsing}
                onClick={() => void handleNext()}
              >
                {t("common.next", "下一步预览")}
              </Button>,
            ]
          : [
              <Button key="back" onClick={() => setStep(0)}>
                {t("common.back", "返回")}
              </Button>,
              <Button
                key="revalidate"
                onClick={() => void handleRevalidate()}
                loading={parsing}
              >
                {t("pages.transfers.batches.import.revalidate", "重新校验")}
              </Button>,
              <Button
                key="submit"
                type="primary"
                disabled={
                  !canImport ||
                  parsing ||
                  submitting ||
                  dirty ||
                  invalidCount > 0 ||
                  previewRows.length === 0
                }
                loading={submitting}
                onClick={() => void handleSubmit()}
              >
                {t("common.submit", "提交")}
              </Button>,
            ]
      }
      destroyOnClose
    >
      <div className={styles.subtitle}>
        {t(
          "pages.transfers.batches.create.subtitle",
          "导入/手动录入 UID + 币种，下一步预览校验后提交"
        )}
      </div>

      <Steps
        className={styles.steps}
        current={step}
        items={[
          {
            title: t(
              "pages.transfers.batches.create.steps.config",
              "配置与导入"
            ),
          },
          {
            title: t(
              "pages.transfers.batches.create.steps.preview",
              "预览与校验"
            ),
          },
        ]}
      />

      {step === 0 ? (
        <>
          <div className={styles.section}>
            <div className={styles.sectionTitle}>
              {t("pages.transfers.batches.create.section.batch", "批次信息")}
            </div>
            <div className={styles.grid2}>
              <div>
                <div
                  style={{ fontSize: 12, color: "#6b7280", marginBottom: 6 }}
                >
                  {t("pages.transfers.batches.create.form.name", "批次名称")}
                </div>
                <Input
                  value={batchName}
                  onChange={(e) => setBatchName(e.target.value)}
                  placeholder={t(
                    "pages.transfers.batches.create.form.namePlaceholder",
                    "请输入批次名称"
                  )}
                />
              </div>
              {/* <div>
                <div
                  style={{ fontSize: 12, color: "#6b7280", marginBottom: 6 }}
                >
                  {t("pages.transfers.batches.create.form.type", "转账类型")}
                </div>
                <Select
                  value={transferType}
                  onChange={(v) => setTransferType(v)}
                  options={[
                    {
                      value: "normal",
                      label: t("pages.transfers.batches.type.normal", "Normal"),
                    },
                    {
                      value: "airdrop",
                      label: t(
                        "pages.transfers.batches.type.airdrop",
                        "Airdrop"
                      ),
                    },
                  ]}
                  style={{ width: "100%" }}
                />
              </div> */}
            </div>
            <div style={{ marginTop: 12 }}>
              <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 6 }}>
                {t("pages.transfers.batches.create.form.description", "Note")}
              </div>
              <Input.TextArea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
                placeholder={t(
                  "pages.transfers.batches.create.form.descriptionPlaceholder",
                  "Optional"
                )}
              />
            </div>
          </div>

          <div className={styles.section}>
            <div className={styles.sectionTitle}>
              {t("pages.transfers.batches.create.section.import", "导入方式")}
            </div>
            <Radio.Group
              value={addMethod}
              onChange={(e) => setAddMethod(e.target.value)}
              buttonStyle="solid"
            >
              <Radio.Button value="excel">
                {t(
                  "pages.transfers.batches.create.method.excel",
                  "Excel/CSV 导入"
                )}
              </Radio.Button>
              <Radio.Button value="manual">
                {t("pages.transfers.batches.create.method.manual", "手动录入")}
              </Radio.Button>
            </Radio.Group>

            <div style={{ marginTop: 12 }}>
              <div className={styles.helpBox}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    gap: 12,
                    alignItems: "center",
                  }}
                >
                  <div>
                    <div style={{ fontSize: 13, color: "#374151" }}>
                      {t(
                        "pages.transfers.batches.import.format",
                        "文件格式：{uid},{currency},{amount},{note}（{noteLabel}可选）",
                        {
                          uid: csvHeaders.uid,
                          currency: csvHeaders.currency,
                          amount: csvHeaders.amount,
                          note: csvHeaders.note,
                          noteLabel: csvHeaders.note,
                        }
                      )}
                    </div>
                    <div
                      style={{ fontSize: 12, color: "#6b7280", marginTop: 4 }}
                    >
                      {t(
                        "pages.transfers.batches.import.hint",
                        "支持 .csv / .xlsx，最多 {max} 行，大小 ≤ {mb}MB",
                        { max: MAX_ROWS, mb: MAX_FILE_MB }
                      )}
                    </div>
                  </div>
                  <Button
                    type="link"
                    icon={<DownloadOutlined />}
                    onClick={handleDownloadSample}
                  >
                    {t(
                      "pages.transfers.batches.import.downloadSample",
                      "下载示例文件"
                    )}
                  </Button>
                </div>
              </div>
            </div>

            {addMethod === "excel" ? (
              <div style={{ marginTop: 12 }}>
                <Dragger {...uploadProps} className={styles.dragger}>
                  <p
                    style={{ fontSize: 40, marginBottom: 6, color: "#3b82f6" }}
                  >
                    <InboxOutlined />
                  </p>
                  <p style={{ marginBottom: 0, color: "#374151" }}>
                    {t(
                      "pages.transfers.batches.import.drag",
                      "点击或拖拽文件到此区域上传"
                    )}
                  </p>
                  <p style={{ fontSize: 12, color: "#9ca3af" }}>
                    {t(
                      "pages.transfers.batches.import.accept",
                      "支持 .csv / .xlsx"
                    )}
                  </p>
                </Dragger>
              </div>
            ) : (
              <div style={{ marginTop: 12 }}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    gap: 12,
                    flexWrap: "wrap",
                  }}
                >
                  <Typography.Text type="secondary">
                    {t(
                      "pages.transfers.batches.create.manual.hint",
                      "点击“新增一行”，在表格内填写 UID/币种/金额"
                    )}
                  </Typography.Text>
                  <Button type="primary" onClick={handleAddEmptyManualRow}>
                    {t(
                      "pages.transfers.batches.create.manual.addRow",
                      "新增一行"
                    )}
                  </Button>
                </div>
                <div style={{ marginTop: 12 }}>
                  <Table
                    size="small"
                    rowKey="key"
                    columns={manualColumns as any}
                    dataSource={manualRows}
                    pagination={{ pageSize: 8 }}
                  />
                </div>
              </div>
            )}
          </div>

          {!canImport ? (
            <Alert
              type="warning"
              showIcon
              message={t("pages.transfers.batches.import.noPerm", "无导入权限")}
              description={t(
                "pages.transfers.batches.import.noPermHint",
                "请联系管理员授予 rpc:ImportTransferBatch 权限"
              )}
            />
          ) : null}
        </>
      ) : (
        <>
          <div className={styles.summaryRow}>
            <span>
              {t("pages.transfers.batches.preview.total", "总行数")}:{" "}
              {previewSummary.total}
            </span>
            <span>
              {t("pages.transfers.batches.preview.valid", "有效")}:{" "}
              {previewSummary.valid}
            </span>
            <span
              style={{
                color: previewSummary.invalid > 0 ? "#ef4444" : undefined,
              }}
            >
              {t("pages.transfers.batches.preview.invalid", "错误")}:{" "}
              {previewSummary.invalid}
            </span>
            <span>
              {t("pages.transfers.batches.preview.totalAmount", "总金额")}:{" "}
              {tokenStats.length > 1 ? "-" : previewSummary.totalAmount}
            </span>
            {dirty ? (
              <span style={{ color: "#f59e0b" }}>
                {t(
                  "pages.transfers.batches.preview.dirty",
                  "已修改，需重新校验"
                )}
              </span>
            ) : null}
          </div>

          {peopleStats.length ? (
            <div style={{ marginBottom: 8 }}>
              <Typography.Text type="secondary" style={{ marginRight: 8 }}>
                {t("pages.transfers.batches.preview.peopleStats", "按人数统计")}
                :
              </Typography.Text>
              {peopleStats.map((s) => (
                <Tag key={`p-${s.currency}`} style={{ marginBottom: 8 }}>
                  {s.currency}-{s.people}
                  {t("common.people", "人")}
                </Tag>
              ))}
            </div>
          ) : null}

          {tokenStats.length ? (
            <div style={{ marginBottom: 12 }}>
              <Typography.Text type="secondary" style={{ marginRight: 8 }}>
                {t("pages.transfers.batches.preview.tokenStats", "按Token统计")}
                :
              </Typography.Text>
              {tokenStats.map((s) => (
                <Tag key={`t-${s.currency}`} style={{ marginBottom: 8 }}>
                  {s.currency}: {s.totalAmount}
                </Tag>
              ))}
            </div>
          ) : null}

          {invalidCount > 0 ? (
            <Alert
              type="error"
              showIcon
              message={t(
                "pages.transfers.batches.import.blocked",
                "存在错误数据，请修正后再提交"
              )}
              style={{ marginBottom: 12 }}
            />
          ) : dirty ? (
            <Alert
              type="warning"
              showIcon
              message={t(
                "pages.transfers.batches.import.dirtyBlocked",
                "存在未校验修改，请先重新校验"
              )}
              style={{ marginBottom: 12 }}
            />
          ) : null}

          <div className={styles.previewTable}>
            <Table
              size="small"
              rowKey="key"
              columns={previewColumns as any}
              dataSource={previewRows}
              pagination={{ pageSize: 10 }}
              scroll={{ x: 1000 }}
            />
          </div>
        </>
      )}
    </Modal>
  );
};

export default CreateBatchModal;
