import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Modal, Alert, Upload, Button, Typography, Steps, Select, Input, Table, App, Checkbox } from 'antd';
import { UploadOutlined, InboxOutlined, DownloadOutlined } from '@ant-design/icons';
import type { UploadFile, UploadProps } from 'antd';
import { createStyles } from 'antd-style';
import type {
  AdminBatchCreateBlacklistAddressItem as BatchCreateBlacklistAddressItem,
  GatewayUnifiedResponseAdminBatchCreateBlacklistAddressesResponse as BatchCreateBlacklistAddressesResponse,
} from '@/api/generated/schemas';
import { MONITOR_STATUS_CONFIG, RISK_LEVEL_CONFIG, SOURCE_CONFIG } from '../constants';
import type { MonitorStatus, RiskLevel, SourceType } from '../types';

const { Text } = Typography;
const { Dragger } = Upload;
const { TextArea } = Input;

const MAX_ROWS = 2000;

const useStyles = createStyles(() => ({
  modalContent: {
    paddingTop: 8,
  },
  subtitle: {
    fontSize: 14,
    color: '#6b7280',
    marginBottom: 24,
  },
  section: {
    marginBottom: 20,
  },
  sectionTitle: {
    fontSize: 14,
    fontWeight: 600,
    color: '#374151',
    marginBottom: 12,
  },
  formatBox: {
    background: '#f9fafb',
    border: '1px solid #e5e7eb',
    borderRadius: 8,
    padding: 16,
  },
  formatLine: {
    fontSize: 13,
    color: '#374151',
    marginBottom: 8,
  },
  exampleLabel: {
    fontSize: 12,
    color: '#9ca3af',
    marginTop: 12,
    marginBottom: 4,
  },
  exampleLine: {
    fontSize: 12,
    color: '#6b7280',
    fontFamily: 'monospace',
    lineHeight: 1.8,
  },
  uploadSection: {
    marginTop: 20,
    marginBottom: 16,
  },
  dragger: {
    '.ant-upload-drag': {
      borderRadius: 8,
      border: '2px dashed #d1d5db',
      background: '#fafafa',
      '&:hover': {
        borderColor: '#f59e0b',
      },
    },
  },
  uploadIcon: {
    fontSize: 48,
    color: '#f59e0b',
    marginBottom: 8,
  },
  uploadText: {
    fontSize: 14,
    color: '#374151',
    marginBottom: 4,
  },
  uploadHint: {
    fontSize: 12,
    color: '#9ca3af',
  },
  alert: {
    borderRadius: 8,
    marginTop: 16,
  },
  alertTitle: {
    fontWeight: 600,
    color: '#1d4ed8',
    marginBottom: 4,
  },
  alertContent: {
    color: '#1e40af',
    fontSize: 13,
  },
  fileInfo: {
    marginTop: 12,
    padding: 12,
    background: '#f0fdf4',
    border: '1px solid #86efac',
    borderRadius: 8,
  },
  fileName: {
    color: '#166534',
    fontWeight: 500,
  },
  steps: {
    marginBottom: 16,
  },
  defaultsGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
    gap: 12,
    '@media (max-width: 576px)': {
      gridTemplateColumns: '1fr',
    },
  },
  fieldLabel: {
    fontSize: 13,
    color: '#6b7280',
    fontWeight: 500,
    marginBottom: 6,
  },
  previewSummary: {
    display: 'flex',
    gap: 12,
    flexWrap: 'wrap',
    marginBottom: 12,
    color: '#6b7280',
    fontSize: 13,
  },
  previewTable: {
    marginTop: 12,
    border: '1px solid #f0f0f0',
    borderRadius: 10,
    overflow: 'hidden',
  },
  rowError: {
    '& > td': {
      background: '#fff1f2 !important', // rose-50
    },
  },
  rowOk: {
    '& > td': {
      background: '#f0fdf4 !important', // green-50
    },
  },
  fullCell: {
    wordBreak: 'break-all',
    whiteSpace: 'pre-wrap',
  },
  errorSummary: {
    marginTop: 12,
    padding: 12,
    borderRadius: 10,
    border: '1px solid #fecaca',
    background: '#fff1f2',
  },
  errorSummaryTitle: {
    fontWeight: 600,
    color: '#991b1b',
    marginBottom: 8,
  },
}));

type RawFields = {
  address?: string;
  network?: string;
  risk_level?: string;
  source?: string;
  reason?: string;
  monitor_status?: string;
};

type PreviewRow = {
  key: string;
  line: number;
  address: string;
  network: string;
  risk_level: string;
  source: string;
  reason: string;
  monitor_status: string;
  errors: string[];
};

const HEADER_ALIASES: Record<string, keyof RawFields> = {
  address: 'address',
  wallet_address: 'address',
  地址: 'address',
  network: 'network',
  chain: 'network',
  网络: 'network',
  risk_level: 'risk_level',
  risklevel: 'risk_level',
  风险等级: 'risk_level',
  source: 'source',
  来源: 'source',
  reason: 'reason',
  remark: 'reason',
  标记原因: 'reason',
  原因: 'reason',
  monitor_status: 'monitor_status',
  monitorstatus: 'monitor_status',
  status: 'monitor_status',
  监测状态: 'monitor_status',
};

function parseCSVLine(line: string): string[] {
  const out: string[] = [];
  let current = '';
  let inQuotes = false;

  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (inQuotes) {
      if (ch === '"') {
        if (line[i + 1] === '"') {
          current += '"';
          i++;
          continue;
        }
        inQuotes = false;
        continue;
      }
      current += ch;
      continue;
    }

    if (ch === ',') {
      out.push(current.trim());
      current = '';
      continue;
    }
    if (ch === '"') {
      inQuotes = true;
      continue;
    }
    current += ch;
  }

  out.push(current.trim());
  return out;
}

function normalizeLower(v: string): string {
  return v.trim().toLowerCase();
}

function normalizeRiskLevel(raw: string): RiskLevel | null {
  const v = normalizeLower(raw);
  switch (v) {
    case 'high':
    case 'high risk':
    case '高风险':
    case '高':
      return 'high';
    case 'medium':
    case 'medium risk':
    case '中风险':
    case '中':
      return 'medium';
    case 'low':
    case 'low risk':
    case '低风险':
    case '低':
      return 'low';
    default:
      return null;
  }
}

function normalizeSource(raw: string): SourceType | null {
  const v = normalizeLower(raw);
  switch (v) {
    case 'user_report':
    case '用户举报':
      return 'user_report';
    case 'third_party':
    case 'third-party':
    case '第三方':
    case '第三方数据源':
      return 'third_party';
    case 'auto_detect':
    case 'auto-detect':
    case '自动检测':
      return 'auto_detect';
    case 'association':
    case '关联分析':
      return 'association';
    default:
      return null;
  }
}

function normalizeMonitorStatus(raw: string): MonitorStatus | null {
  const v = normalizeLower(raw);
  switch (v) {
    case 'active':
    case 'enabled':
    case '监测中':
    case '启用':
      return 'active';
    case 'stopped':
    case 'disabled':
    case '已停止':
    case '停止':
      return 'stopped';
    default:
      return null;
  }
}

function detectHeaderMapping(cols: string[]): (keyof RawFields | undefined)[] | null {
  const mapping = cols.map((c) => {
    const raw = c.trim();
    if (!raw) return undefined;
    const k = raw.toLowerCase();
    return HEADER_ALIASES[k] || HEADER_ALIASES[raw];
  });
  if (!mapping.includes('address')) return null;
  const recognized = mapping.filter(Boolean).length;
  if (recognized < 1) return null;
  return mapping;
}

function parseLines(text: string): { line: number; cols: string[] }[] {
  const lines = text.split(/\r?\n/);
  const out: { line: number; cols: string[] }[] = [];
  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i]?.trim() ?? '';
    if (!raw) continue;
    if (raw.startsWith('#')) continue;
    const cols = parseCSVLine(raw);
    if (cols.every((c) => !c.trim())) continue;
    out.push({ line: i + 1, cols });
  }
  return out;
}

interface ImportBlacklistModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  networks: { value: string; label: string }[];
  defaultNetwork?: string;
  networkAliasToValue?: Record<string, string>;
  onSubmit: (
    items: BatchCreateBlacklistAddressItem[],
    opts?: { ignoreExisting?: boolean },
  ) => Promise<BatchCreateBlacklistAddressesResponse | null>;
  onSuccess?: () => void;
  t: (id: string, defaultMessage: string, values?: Record<string, any>) => string;
}

const ImportBlacklistModal: React.FC<ImportBlacklistModalProps> = ({
  open,
  onOpenChange,
  onSubmit,
  onSuccess,
  networks,
  defaultNetwork,
  networkAliasToValue,
  t,
}) => {
  const { styles } = useStyles();
  const { message } = App.useApp();
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [step, setStep] = useState(0); // 0 upload, 1 preview
  const [parsing, setParsing] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [previewRows, setPreviewRows] = useState<PreviewRow[]>([]);
  const [headerUsed, setHeaderUsed] = useState(false);
  const lastNotifiedFileUidRef = useRef<string | null>(null);

  const [defaultNetworkValue, setDefaultNetworkValue] = useState<string>('');
  const [defaultRiskLevel, setDefaultRiskLevel] = useState<RiskLevel>('high');
  const [defaultSource, setDefaultSource] = useState<SourceType>('third_party');
  const [defaultMonitorStatus, setDefaultMonitorStatus] = useState<MonitorStatus>('active');
  const [defaultReason, setDefaultReason] = useState<string>('批量导入');
  const [ignoreExisting, setIgnoreExisting] = useState<boolean>(false);

  const networkOptions = useMemo(() => networks.map((n) => ({ value: n.value, label: n.label })), [networks]);
  const allowedNetworkSet = useMemo(() => new Set(networks.map((n) => n.value.toLowerCase())), [networks]);
  const aliasMap = useMemo(() => networkAliasToValue || {}, [networkAliasToValue]);

  const riskOptions = useMemo(
    () =>
      Object.entries(RISK_LEVEL_CONFIG).map(([key, config]) => ({
        value: key,
        label: t(`pages.blacklist.riskLevel.${key}`, config.label),
      })),
    [t],
  );

  const sourceOptions = useMemo(
    () =>
      Object.entries(SOURCE_CONFIG).map(([key, config]) => ({
        value: key,
        label: t(`pages.blacklist.source.${key}`, config.label),
      })),
    [t],
  );

  const monitorOptions = useMemo(
    () =>
      Object.entries(MONITOR_STATUS_CONFIG).map(([key, config]) => ({
        value: key,
        label: t(`pages.blacklist.monitorStatus.${key}`, config.label),
      })),
    [t],
  );

  useEffect(() => {
    if (!open) return;
    const v = (defaultNetwork || networks[0]?.value || '').trim().toLowerCase();
    if (v) setDefaultNetworkValue(v);
  }, [defaultNetwork, networks, open]);

  useEffect(() => {
    if (open) return;
    setFileList([]);
    setStep(0);
    setParsing(false);
    setSubmitting(false);
    setPreviewRows([]);
    setHeaderUsed(false);
    lastNotifiedFileUidRef.current = null;
    setDefaultRiskLevel('high');
    setDefaultSource('third_party');
    setDefaultMonitorStatus('active');
    setDefaultReason('批量导入');
    setIgnoreExisting(false);
  }, [open]);

  const uploadProps: UploadProps = {
    name: 'file',
    multiple: false,
    accept: '.csv,.txt',
    fileList,
    maxCount: 1,
    beforeUpload: (file) => {
      const name = String(file?.name || '');
      const ext = name.split('.').pop()?.toLowerCase();
      if (ext !== 'csv' && ext !== 'txt') {
        message.error(t('pages.blacklist.message.importInvalidFileType', '文件格式不支持，请选择 .csv 或 .txt'));
        return Upload.LIST_IGNORE;
      }
      return false; // 阻止自动上传（通过 onChange 管理 fileList）
    },
    onChange: (info) => {
      const next = (info.fileList || []).slice(-1);
      setFileList(next);
      const f = next[0];
      if (f?.uid && f.uid !== lastNotifiedFileUidRef.current) {
        lastNotifiedFileUidRef.current = f.uid;
        message.success(
          t('pages.blacklist.message.importFileSelected', '已选择文件：{name}', {
            name: f.name || '',
          }),
        );
      }
    },
    onRemove: () => {
      setFileList([]);
      lastNotifiedFileUidRef.current = null;
    },
  };

  const canNext = fileList.length > 0;

  const parseAndValidate = async () => {
    if (fileList.length === 0) return;
    const file = fileList[0]?.originFileObj;
    if (!file) {
      message.error(t('pages.blacklist.message.importNoFile', '请先选择要导入的文件'));
      return;
    }

    setParsing(true);
    try {
      const text = await file.text();
      const lines = parseLines(text);
      if (lines.length === 0) {
        setPreviewRows([]);
        setHeaderUsed(false);
        message.error(t('pages.blacklist.message.importEmpty', '未识别到有效数据'));
        return;
      }
      if (lines.length > MAX_ROWS) {
        setPreviewRows([
          {
            key: 'too-many',
            line: lines[0]?.line ?? 1,
            address: '',
            network: '',
            risk_level: '',
            source: '',
            reason: '',
            monitor_status: '',
            errors: [t('pages.blacklist.import.tooManyRows', `最多支持 ${MAX_ROWS} 行数据`)],
          },
        ]);
        setHeaderUsed(false);
        return;
      }

      const maybeHeader = detectHeaderMapping(lines[0]?.cols ?? []);
      const header = maybeHeader;
      const dataLines = header ? lines.slice(1) : lines;
      setHeaderUsed(!!header);

      const rows: PreviewRow[] = [];
      const seen = new Map<string, number>(); // key -> first line

      for (const row of dataLines) {
        const raw: RawFields = {};
        const cols = row.cols;

        if (header) {
          for (let i = 0; i < header.length; i++) {
            const field = header[i];
            if (!field) continue;
            raw[field] = cols[i] ?? '';
          }
        } else {
          const address = cols[0] ?? '';
          const network = cols[1] ?? '';
          const risk = cols[2] ?? '';
          const source = cols[3] ?? '';
          const monitor = cols.length >= 6 ? (cols[cols.length - 1] ?? '') : (cols[5] ?? '');
          const reason =
            cols.length >= 6
              ? cols.slice(4, cols.length - 1).join(',') // allow commas in reason without quotes in no-header mode
              : (cols[4] ?? '');
          raw.address = address;
          raw.network = network;
          raw.risk_level = risk;
          raw.source = source;
          raw.reason = reason;
          raw.monitor_status = monitor;
        }

        const errors: string[] = [];

        const address = String(raw.address ?? '').trim();
        if (!address) errors.push(t('pages.blacklist.import.err.addressRequired', '地址不能为空'));
        if (address && address.length > 255) errors.push(t('pages.blacklist.import.err.addressTooLong', '地址过长'));
        if (address && /\s/.test(address)) errors.push(t('pages.blacklist.import.err.addressHasSpace', '地址不能包含空格'));

        const networkInput = String(raw.network ?? '').trim();
        const networkValue = networkInput
          ? aliasMap[normalizeLower(networkInput)] || normalizeLower(networkInput)
          : normalizeLower(defaultNetworkValue);
        if (!networkValue) errors.push(t('pages.blacklist.import.err.networkRequired', '网络不能为空'));
        if (networkValue && allowedNetworkSet.size > 0 && !allowedNetworkSet.has(networkValue)) {
          errors.push(
            t('pages.blacklist.import.err.networkInvalid', '未知网络: {network}', { network: networkInput || networkValue }),
          );
        }

        const riskInput = String(raw.risk_level ?? '').trim();
        const risk = riskInput ? normalizeRiskLevel(riskInput) : defaultRiskLevel;
        if (!risk) errors.push(t('pages.blacklist.import.err.riskInvalid', '风险等级不合法'));

        const sourceInput = String(raw.source ?? '').trim();
        const source = sourceInput ? normalizeSource(sourceInput) : defaultSource;
        if (!source) errors.push(t('pages.blacklist.import.err.sourceInvalid', '来源不合法'));

        const monitorInput = String(raw.monitor_status ?? '').trim();
        const monitor = monitorInput ? normalizeMonitorStatus(monitorInput) : defaultMonitorStatus;
        if (!monitor) errors.push(t('pages.blacklist.import.err.monitorInvalid', '监测状态不合法'));

        const reasonInput = String(raw.reason ?? '').trim();
        const reason = reasonInput || defaultReason.trim();
        if (!reason) errors.push(t('pages.blacklist.import.err.reasonRequired', '原因不能为空'));

        if (address && networkValue) {
          const key = `${networkValue}#${address.toLowerCase()}`;
          const first = seen.get(key);
          if (first) {
            errors.push(t('pages.blacklist.import.err.duplicateInFile', '与第 {line} 行重复', { line: first }));
          } else {
            seen.set(key, row.line);
          }
        }

        rows.push({
          key: String(row.line),
          line: row.line,
          address,
          network: networkValue || '',
          risk_level: risk || '',
          source: source || '',
          reason,
          monitor_status: monitor || '',
          errors,
        });
      }

      setPreviewRows(rows);
      setStep(1);
    } catch (e: any) {
      message.error(e?.message || t('pages.blacklist.message.importFailed', '导入失败'));
    } finally {
      setParsing(false);
    }
  };

  const itemsToSubmit = useMemo(() => {
    const rows = previewRows.filter((r) => r.errors.length === 0 && r.address && r.network);
    return rows.map(
      (r): BatchCreateBlacklistAddressItem => ({
        address: r.address,
        network: r.network,
        risk_level: r.risk_level,
        source: r.source,
        reason: r.reason,
        monitor_status: r.monitor_status,
      }),
    );
  }, [previewRows]);

  const invalidCount = useMemo(() => previewRows.filter((r) => r.errors.length > 0).length, [previewRows]);
  const totalCount = previewRows.length;
  const errorSummaryLines = useMemo(() => {
    const list = previewRows
      .filter((r) => r.errors.length > 0)
      .slice(0, 200)
      .map((r) => `第 ${r.line} 行：${r.errors.join('；')}`);
    return list;
  }, [previewRows]);

  const handleSubmit = async () => {
    if (submitting) return;
    if (invalidCount > 0) {
      message.error(t('pages.blacklist.import.blocked', '存在错误数据，请修正后再提交'));
      return;
    }
    if (itemsToSubmit.length === 0) {
      message.error(t('pages.blacklist.message.importEmpty', '未识别到有效数据'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await onSubmit(itemsToSubmit, { ignoreExisting });
      if (!res) {
        message.error(t('pages.blacklist.message.importFailed', '导入失败'));
        return;
      }
      if (!res?.success) {
        const d = res.data;
        const msg = res?.message || t('pages.blacklist.message.importFailed', '导入失败');
        const serverResults = (res.data as any)?.results as any[] | undefined;
        if (Array.isArray(serverResults) && serverResults.length > 0) {
          const errMap = new Map<string, string[]>();
          for (const r of serverResults) {
            const status = String(r?.status || '').toLowerCase();
            const address = String(r?.address || '').trim();
            const network = String(r?.network || '').trim().toLowerCase();
            const err = String(r?.error || '').trim();
            if (status !== 'failed' || !address || !network || !err) continue;
            const k = `${network}#${address.toLowerCase()}`;
            const list = errMap.get(k) || [];
            list.push(err);
            errMap.set(k, list);
          }
          if (errMap.size > 0) {
            setPreviewRows((prev) =>
              prev.map((row) => {
                const k = `${row.network}#${row.address.toLowerCase()}`;
                const errs = errMap.get(k);
                if (!errs?.length) return row;
                return { ...row, errors: Array.from(new Set([...row.errors, ...errs])) };
              }),
            );
            setStep(1);
          }
        }
        const failed = d?.failed ?? 0;
        const succeeded = d?.succeeded ?? 0;
        const skipped = d?.skipped ?? 0;
        if (succeeded > 0 || skipped > 0) {
          // 部分成功也刷新列表，便于立刻看到新增的记录
          onSuccess?.();
        }

        const allAlreadyExists =
          Array.isArray(serverResults) &&
          serverResults.length > 0 &&
          serverResults.every((r) => String(r?.error || '').toLowerCase().includes('already exists'));

        if (failed > 0 && allAlreadyExists) {
          message.error(
            t(
              'pages.blacklist.import.serverAlreadyExists',
              '导入失败：{failed} 条地址已存在。可勾选“忽略已存在”后重试（已存在将跳过）。',
              { failed },
            ),
          );
          return;
        }

        message.error(msg);
        return;
      }

      const d = res.data;
      if ((d?.failed ?? 0) > 0) {
        message.error(
          t('pages.blacklist.import.serverFailed', '导入失败：存在 {failed} 条失败数据，请检查错误原因', {
            failed: d?.failed ?? 0,
          }),
        );
        if ((d?.succeeded ?? 0) > 0 || (d?.skipped ?? 0) > 0) {
          onSuccess?.();
        }
        return;
      }
      message.success(
        t(
          'pages.blacklist.message.importSuccessWithCount',
          '导入完成：成功 {succeeded}，跳过 {skipped}，失败 {failed}',
          {
            succeeded: d?.succeeded ?? 0,
            skipped: d?.skipped ?? 0,
            failed: d?.failed ?? 0,
          },
        ),
      );
      onOpenChange(false);
      onSuccess?.();
    } finally {
      setSubmitting(false);
    }
  };

  const handleCancel = () => {
    setFileList([]);
    setPreviewRows([]);
    setHeaderUsed(false);
    setStep(0);
    onOpenChange(false);
  };

  const columns = useMemo(
    () => [
      { title: t('pages.blacklist.import.col.line', '行'), dataIndex: 'line', width: 70 },
      {
        title: t('pages.blacklist.import.col.address', '地址'),
        dataIndex: 'address',
        width: 360,
        render: (v: string) => (
          <Text className={styles.fullCell} copyable={{ text: v }}>
            {v || '-'}
          </Text>
        ),
      },
      { title: t('pages.blacklist.import.col.network', '网络'), dataIndex: 'network', width: 120 },
      { title: t('pages.blacklist.import.col.risk', '风险'), dataIndex: 'risk_level', width: 100 },
      { title: t('pages.blacklist.import.col.source', '来源'), dataIndex: 'source', width: 120 },
      { title: t('pages.blacklist.import.col.status', '监测'), dataIndex: 'monitor_status', width: 100 },
      {
        title: t('pages.blacklist.import.col.reason', '原因'),
        dataIndex: 'reason',
        width: 360,
        render: (v: string) => <span className={styles.fullCell}>{v || '-'}</span>,
      },
      {
        title: t('pages.blacklist.import.col.errors', '错误'),
        dataIndex: 'errors',
        width: 480,
        render: (errs: string[]) =>
          errs?.length ? (
            <span className={styles.fullCell} style={{ color: '#ef4444' }}>
              {errs.join('；')}
            </span>
          ) : (
            <span style={{ color: '#10b981' }}>{t('common.ok', '正常')}</span>
          ),
      },
    ],
    [styles.fullCell, t],
  );

  return (
    <Modal
      title={
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <UploadOutlined style={{ color: '#f59e0b' }} />
          {t('pages.blacklist.modal.importTitle', '批量导入黑名单地址')}
        </span>
      }
      open={open}
      onCancel={handleCancel}
      width={1200}
      destroyOnClose
      footer={[
        step === 0 ? (
          <Button key="cancel" onClick={handleCancel}>
            {t('common.cancel', '取消')}
          </Button>
        ) : (
          <Button key="back" onClick={() => setStep(0)}>
            {t('common.back', '返回')}
          </Button>
        ),
        step === 0 ? (
          <Button
            key="next"
            type="primary"
            loading={parsing}
            disabled={!canNext}
            onClick={parseAndValidate}
            style={{ background: '#f59e0b', borderColor: '#f59e0b' }}
          >
            {t('common.next', '下一步')}
          </Button>
        ) : (
          <Button
            key="submit"
            type="primary"
            loading={submitting}
            disabled={invalidCount > 0 || itemsToSubmit.length === 0}
            onClick={handleSubmit}
            style={{ background: '#f59e0b', borderColor: '#f59e0b' }}
          >
            {t('pages.blacklist.modal.confirmImport', '确认导入')}
          </Button>
        ),
      ]}
    >
      <div className={styles.modalContent}>
        <Steps
          className={styles.steps}
          current={step}
          size="small"
          items={[
            { title: t('pages.blacklist.import.step.upload', '上传文件') },
            { title: t('pages.blacklist.import.step.preview', '预览校验') },
          ]}
        />

        <div className={styles.subtitle}>
          {t('pages.blacklist.modal.importSubtitle', '支持CSV格式批量导入，每行一个地址')}
        </div>

        {step === 0 ? (
          <>
            {/* 导入格式说明 */}
            <div className={styles.section}>
              <div className={styles.sectionTitle}>
                {t('pages.blacklist.modal.formatTitle', '导入格式说明')}
              </div>
              <div className={styles.formatBox}>
                <div className={styles.formatLine}>
                  {t(
                    'pages.blacklist.modal.formatDescV2',
                    '支持带表头CSV（推荐）：address,network,risk_level,source,reason,monitor_status（字段可缺省）',
                  )}
                </div>
                <div className={styles.exampleLabel} style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                  {t('pages.blacklist.modal.example', '示例')}:
                  <Button
                    type="link"
                    size="small"
                    icon={<DownloadOutlined />}
                    href="/blacklist-import-sample.csv"
                    target="_blank"
                  >
                    {t('pages.blacklist.import.downloadSample', '下载示例文件')}
                  </Button>
                </div>
                <div className={styles.exampleLine}>
                  address,network,risk_level,source,reason,monitor_status
                </div>
                <div className={styles.exampleLine}>
                  0x1111...1111,ERC20,high,third_party,诈骗地址,active
                </div>
                <div className={styles.exampleLine}>
                  0x2222...2222,ETH,,,,active
                </div>
              </div>
            </div>

            {/* 默认值设置 */}
            <div className={styles.section}>
              <div className={styles.sectionTitle}>
                {t('pages.blacklist.import.defaultsTitle', '缺省字段默认值')}
              </div>
              <div className={styles.defaultsGrid}>
                <div>
                  <div className={styles.fieldLabel}>{t('pages.blacklist.form.network', '区块链网络')}</div>
                  <Select
                    value={defaultNetworkValue}
                    options={networkOptions}
                    onChange={(v) => setDefaultNetworkValue(String(v))}
                    style={{ width: '100%' }}
                    placeholder={t('pages.blacklist.form.networkRequired', '请选择区块链网络')}
                  />
                </div>
                <div>
                  <div className={styles.fieldLabel}>{t('pages.blacklist.form.riskLevel', '风险等级')}</div>
                  <Select
                    value={defaultRiskLevel}
                    options={riskOptions}
                    onChange={(v) => setDefaultRiskLevel(v)}
                    style={{ width: '100%' }}
                  />
                </div>
                <div>
                  <div className={styles.fieldLabel}>{t('pages.blacklist.form.source', '信息来源')}</div>
                  <Select
                    value={defaultSource}
                    options={sourceOptions}
                    onChange={(v) => setDefaultSource(v)}
                    style={{ width: '100%' }}
                  />
                </div>
                <div>
                  <div className={styles.fieldLabel}>{t('pages.blacklist.form.monitorStatus', '监测状态')}</div>
                  <Select
                    value={defaultMonitorStatus}
                    options={monitorOptions}
                    onChange={(v) => setDefaultMonitorStatus(v)}
                    style={{ width: '100%' }}
                  />
                </div>
                <div style={{ gridColumn: '1 / -1' }}>
                  <div className={styles.fieldLabel}>{t('pages.blacklist.form.reason', '标记原因')}</div>
                  <TextArea
                    value={defaultReason}
                    onChange={(e) => setDefaultReason(e.target.value)}
                    rows={3}
                    placeholder={t('pages.blacklist.import.defaultReasonPlaceholder', '用于填充缺省原因字段')}
                  />
                </div>
              </div>
            </div>

            {/* 上传区域 */}
            <div className={styles.uploadSection}>
              <div className={styles.sectionTitle}>
                {t('pages.blacklist.modal.uploadTitle', '上传文件')}
              </div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginBottom: 8 }}>
                <Checkbox checked={ignoreExisting} onChange={(e) => setIgnoreExisting(e.target.checked)}>
                  {t('pages.blacklist.import.ignoreExisting', '忽略已存在（已存在则跳过）')}
                </Checkbox>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {t('pages.blacklist.import.ignoreExistingHint', '常见用于重复导入时避免全量失败')}
                </Text>
              </div>
              <Dragger {...uploadProps} className={styles.dragger}>
                <p className={styles.uploadIcon}>
                  <InboxOutlined />
                </p>
                <p className={styles.uploadText}>
                  {t('pages.blacklist.modal.uploadDragText', '点击或拖拽文件到此区域上传')}
                </p>
                <p className={styles.uploadHint}>
                  {t('pages.blacklist.modal.uploadHint', '支持 .csv 或 .txt 格式文件')}
                </p>
              </Dragger>
            </div>

            {/* 已选文件信息 */}
            {fileList.length > 0 && (
              <div className={styles.fileInfo}>
                <Text className={styles.fileName}>
                  ✓ {t('pages.blacklist.modal.selectedFile', '已选择文件')}: {fileList[0].name}
                </Text>
              </div>
            )}

            {/* 导入提示 */}
            <Alert
              className={styles.alert}
              type="info"
              showIcon
              message={
                <div>
                  <div className={styles.alertTitle}>
                    {t('pages.blacklist.modal.importAlertTitle', '导入提示')}
                  </div>
                  <div className={styles.alertContent}>
                    {t(
                      'pages.blacklist.modal.importAlertContent',
                      '导入前会在前端进行校验与预览；存在任意错误将阻断提交。建议先在测试环境验证。',
                    )}
                  </div>
                </div>
              }
            />
          </>
        ) : (
          <>
            <div className={styles.previewSummary}>
              <span>
                {t('pages.blacklist.import.summary.total', '总行数')}: <b>{totalCount}</b>
              </span>
              <span>
                {t('pages.blacklist.import.summary.invalid', '错误行')}: <b style={{ color: invalidCount ? '#ef4444' : '#10b981' }}>{invalidCount}</b>
              </span>
              <span>
                {t('pages.blacklist.import.summary.header', '表头')}: <b>{headerUsed ? t('common.yes', '是') : t('common.no', '否')}</b>
              </span>
            </div>

            {invalidCount > 0 ? (
              <Alert
                type="error"
                showIcon
                message={t('pages.blacklist.import.blocked', '存在错误数据，请返回修正后再提交')}
              />
            ) : (
              <Alert type="success" showIcon message={t('pages.blacklist.import.ready', '校验通过，可以提交导入')} />
            )}

            <div className={styles.previewTable}>
              <Table<PreviewRow>
                size="small"
                columns={columns as any}
                dataSource={previewRows}
                rowKey="key"
                pagination={{ pageSize: 20, showSizeChanger: true }}
                scroll={{ x: 1600, y: 420 }}
                rowClassName={(record) => (record.errors?.length ? styles.rowError : styles.rowOk)}
              />
            </div>

            {invalidCount > 0 && (
              <div className={styles.errorSummary}>
                <div className={styles.errorSummaryTitle}>
                  {t('pages.blacklist.import.errorSummary', '错误汇总（完整原因）')}
                </div>
                <div className={styles.fullCell} style={{ color: '#991b1b', fontSize: 13 }}>
                  {errorSummaryLines.length ? errorSummaryLines.join('\n') : '-'}
                </div>
                {previewRows.filter((r) => r.errors.length > 0).length > 200 && (
                  <div style={{ marginTop: 8, color: '#7f1d1d', fontSize: 12 }}>
                    {t('pages.blacklist.import.errorSummaryTruncated', '仅展示前 200 条错误，避免页面卡顿')}
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </Modal>
  );
};

export default ImportBlacklistModal;
