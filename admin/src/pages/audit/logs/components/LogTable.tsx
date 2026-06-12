import { ClockCircleOutlined, UserOutlined } from "@ant-design/icons";
import { Avatar, Tag, Tooltip } from "antd";
import { createStyles } from "antd-style";
import React from "react";
import { MODULE_CONFIG } from "../constants";
import type { AuditLogRow } from "../types";

const useStyles = createStyles(() => ({
  container: {
    background: "#fff",
    borderRadius: 12,
    border: "1px solid #f0f0f0",
    overflowX: "auto",
    overflowY: "hidden",
  },

  header: {
    display: "grid",
    minWidth: 1120,
    gridTemplateColumns: "140px 140px 100px 120px 200px 240px 180px",
    gap: 16,
    padding: "16px 24px",
    background: "#fafbfc",
    borderBottom: "1px solid #f0f0f0",
    "@media (max-width: 1200px)": { display: "none" },
  },
  headerCell: {
    fontSize: 14,
    fontWeight: 600,
    color: "#4b5563",
  },
  row: {
    display: "grid",
    minWidth: 1120,
    gridTemplateColumns: "140px 140px 100px 120px 200px 240px 180px",
    gap: 16,
    padding: "16px 24px",
    borderBottom: "1px solid #f5f5f5",
    alignItems: "center",
    transition: "background 0.2s ease",
    "&:last-child": {
      borderBottom: "none",
    },
    "&:hover": {
      background: "#fafbfc",
    },
    "@media (max-width: 1200px)": {
      display: "flex",
      flexDirection: "column",
      alignItems: "flex-start",
      gap: 8,
    },
  },
  cell: {
    fontSize: 14,
    color: "#1f2937",
  },
  timeCell: {
    display: "flex",
    alignItems: "center",
    gap: 8,
    color: "#6b7280",
    fontSize: 13,
  },
  timeIcon: {
    color: "#9ca3af",
  },
  operatorCell: {
    display: "flex",
    alignItems: "center",
    gap: 10,
  },
  avatar: {
    background: "linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%)",
    color: "#3b82f6",
  },
  operatorName: {
    fontSize: 14,
    fontWeight: 500,
    color: "#1f2937",
  },
  roleTag: {
    fontSize: 12,
    padding: "2px 8px",
    borderRadius: 4,
    border: "none",
    background: "#fef3c7",
    color: "#d97706",
  },
  moduleTag: {
    fontSize: 12,
    padding: "2px 8px",
    borderRadius: 4,
    border: "none",
    cursor: "pointer",
  },
  operationTag: {
    fontSize: 12,
    padding: "2px 8px",
    borderRadius: 4,
    border: "none",
    background: "#eff6ff",
    color: "#3b82f6",
  },
  targetCell: {
    fontSize: 14,
    color: "#1f2937",
    fontWeight: 500,
    overflow: "hidden",
    textOverflow: "ellipsis",
    whiteSpace: "nowrap",
  },

  detailsCell: {
    fontSize: 13,
    color: "#6b7280",
    overflow: "hidden",
    textOverflow: "ellipsis",
    whiteSpace: "nowrap",
  },
  mobileLabel: {
    display: "none",
    fontSize: 12,
    color: "#9ca3af",
    marginRight: 8,
    "@media (max-width: 1200px)": {
      display: "inline",
    },
  },
  emptyState: {
    padding: "60px 24px",
    textAlign: "center",
    color: "#9ca3af",
  },
}));

interface LogTableProps {
  logs: AuditLogRow[];
  t: (
    id: string,
    defaultMessage: string,
    values?: Record<string, any>
  ) => string;
  onSelect?: (log: AuditLogRow) => void;
}

const LogTable: React.FC<LogTableProps> = ({ logs, t, onSelect }) => {
  const { styles } = useStyles();

  const getModuleStyle = (module: string) => {
    const config =
      MODULE_CONFIG[module as keyof typeof MODULE_CONFIG] ||
      MODULE_CONFIG.system;
    return {
      background: `${config.color}15`,
      color: config.color,
    };
  };

  const getOperationStyle = (status: "success" | "failed") => {
    if (status === "failed") {
      return {
        background: "#fef2f2",
        color: "#ef4444",
      };
    }
    return {
      background: "#eff6ff",
      color: "#3b82f6",
    };
  };

  // 格式化角色显示
  const formatRole = (role?: string): string => {
    if (!role) return "-";
    const roleLower = role.toLowerCase().trim();
    // 如果是 super_admin，使用国际化翻译
    if (roleLower === "super_admin") {
      return t("pages.users.roles.super_admin", "super admin");
    }
    // 其他角色：将下划线替换为空格，并转换为小写（英文情况）
    // 如果国际化文件中有对应的翻译，会使用翻译；否则使用格式化后的值
    const formatted = role.replace(/_/g, " ").toLowerCase();
    const i18nKey = `pages.users.roles.${roleLower}`;
    const translated = t(i18nKey, formatted);
    // 如果翻译结果和格式化结果相同，说明没有对应的翻译，直接返回格式化结果
    return translated !== formatted ? translated : formatted;
  };

  if (logs.length === 0) {
    return (
      <div className={styles.container}>
        <div className={styles.emptyState}>
          {t("common.noData", "暂无数据")}
        </div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      {/* 表头 */}
      <div className={styles.header}>
        <div className={styles.headerCell}>
          {t("pages.audit.logs.table.time", "时间")}
        </div>
        <div className={styles.headerCell}>
          {t("pages.audit.logs.table.operator", "操作人")}
        </div>
        <div className={styles.headerCell}>
          {t("pages.audit.logs.table.role", "角色")}
        </div>
        <div className={styles.headerCell}>
          {t("pages.audit.logs.table.module", "模块")}
        </div>
        <div className={styles.headerCell}>
          {t("pages.audit.logs.table.operation", "操作")}
        </div>
        <div className={styles.headerCell}>
          {t("pages.audit.logs.table.target", "目标")}
        </div>
        <div className={styles.headerCell}>
          {t("pages.audit.logs.table.details", "详情")}
        </div>
      </div>

      {/* 数据行 */}
      {logs.map((log) => (
        <div
          key={log.id}
          className={styles.row}
          onClick={() => onSelect?.(log)}
          style={{ cursor: onSelect ? "pointer" : "default" }}
        >
          {/* 时间 */}
          <div className={styles.timeCell}>
            <ClockCircleOutlined className={styles.timeIcon} />
            <span>{log.createdAt}</span>
          </div>

          {/* 操作人 */}
          <div className={styles.operatorCell}>
            <Avatar
              size={32}
              icon={<UserOutlined />}
              className={styles.avatar}
            />
            <Tooltip
              title={
                log.operator.username
                  ? `${log.operator.name} (${log.operator.username})`
                  : log.operator.name
              }
            >
              <span className={styles.operatorName}>{log.operator.name}</span>
            </Tooltip>
          </div>

          {/* 角色 */}
          <div className={styles.cell}>
            <Tag className={styles.roleTag}>{formatRole(log.operator.role)}</Tag>
          </div>

          {/* 模块 */}
          <div className={styles.cell}>
            <Tag
              className={styles.moduleTag}
              style={getModuleStyle(log.module)}
            >
              {t(
                `pages.audit.logs.modules.${log.module}`,
                MODULE_CONFIG[log.module as keyof typeof MODULE_CONFIG]
                  ?.label || log.module
              )}
            </Tag>
          </div>

          {/* 操作 */}
          <div className={styles.cell}>
            <Tag
              className={styles.operationTag}
              style={getOperationStyle(log.status)}
            >
              {log.action}
            </Tag>
          </div>

          {/* 目标 */}
          <Tooltip title={log.target}>
            <div
              className={styles.targetCell}
              onClick={(e) => e.stopPropagation()}
            >
              {log.target}
            </div>
          </Tooltip>

          {/* 详情 */}
          <Tooltip title={log.description}>
            <div
              className={styles.detailsCell}
              onClick={(e) => e.stopPropagation()}
            >
              {log.description}
            </div>
          </Tooltip>
        </div>
      ))}
    </div>
  );
};

export default LogTable;
