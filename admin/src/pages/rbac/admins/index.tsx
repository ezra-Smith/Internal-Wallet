import type { ProColumns } from '@ant-design/pro-components';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App, Button, Divider, Input, Select, Space, Spin, Tag } from 'antd';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  adminCreateAdmin,
  adminDisableAdmin,
  adminListAdmins,
  adminResetAdminPassword,
  adminResetAdminTwoFA,
  adminUpdateAdmin,
} from '@/api/generated/admins';
import { adminListRoles } from '@/api/generated/rbac';
import type {
  AdminAdminUserItem,
  AdminCreateAdminRequest,
  AdminUpdateAdminBody,
} from '@/api/generated/schemas';
import { useRbac } from '@/hooks/useRbac';
import { getRequestErrorMessage } from '@/utils/requestError';

const PERM = {
  create: 'CreateAdmin',
  update: 'UpdateAdmin',
  disable: 'DisableAdmin',
  resetPassword: 'ResetAdminPassword',
  reset2fa: 'ResetAdminTwoFA',
};

type SelectOption = { label: string; value: string };

const ROLE_CODE_PAGE_SIZE = 50;
const ROLE_CODE_SCROLL_THRESHOLD_PX = 24;

function toStatusTag(status: string | undefined, t: (id: string, defaultMessage: string) => string) {
  const normalized = (status || '').toLowerCase();
  if (!normalized) return <Tag>{t('pages.rbac.admins.status.unknown', 'Unknown')}</Tag>;
  if (normalized === 'active') return <Tag color="green">{t('pages.rbac.admins.status.active', 'Active')}</Tag>;
  if (normalized === 'disabled') return <Tag>{t('pages.rbac.admins.status.disabled', 'Disabled')}</Tag>;
  return <Tag>{status}</Tag>;
}

const AdminsPage: React.FC = () => {
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string, values?: Record<string, any>) =>
    intl.formatMessage({ id, defaultMessage }, values);

  const actionRef = useRef<ActionType | null>(null);

  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<AdminAdminUserItem | null>(null);

  const [roleCodeOptions, setRoleCodeOptions] = useState<SelectOption[]>([]);
  const [roleCodeLoading, setRoleCodeLoading] = useState(false);
  const roleCodePageRef = useRef(0);
  const roleCodeHasNextRef = useRef(true);
  const roleCodeLoadingRef = useRef(false);

  const statusOptions = useMemo(
    () => [
      { label: t('pages.rbac.admins.status.active', 'Active'), value: 'active' },
      { label: t('pages.rbac.admins.status.disabled', 'Disabled'), value: 'disabled' },
    ],
    [t],
  );

  const mergeOptionsByValue = useCallback((prev: SelectOption[], next: SelectOption[]) => {
    if (!next.length) return prev;
    const map = new Map(prev.map((o) => [o.value, o]));
    for (const option of next) map.set(option.value, option);
    return Array.from(map.values());
  }, []);

  const loadRoleCodePage = useCallback(
    async (page: number) => {
      if (roleCodeLoadingRef.current) return;
      roleCodeLoadingRef.current = true;
      setRoleCodeLoading(true);
      try {
        const res = await adminListRoles(
          { page, page_size: ROLE_CODE_PAGE_SIZE, status: 1 },
          { skipErrorHandler: true },
        );
        if (!res?.success)
          throw new Error(res?.message || t('pages.rbac.admins.messages.loadRolesFailed', 'Failed to load roles'));

        const list = res.data?.list ?? [];
        const nextOptions: SelectOption[] = list
          .filter((r) => !!r.code)
          .map((r) => ({ label: `${r.name || r.code} (${r.code})`, value: r.code as string }))
          .filter((o) => !!o.value);

        setRoleCodeOptions((prev) => mergeOptionsByValue(prev, nextOptions));
        roleCodePageRef.current = page;

        const hasNext =
          typeof res.data?.pagination?.has_next === 'boolean'
            ? res.data.pagination.has_next
            : typeof res.data?.pagination?.total_pages === 'number'
              ? page < res.data.pagination.total_pages
              : nextOptions.length >= ROLE_CODE_PAGE_SIZE;

        roleCodeHasNextRef.current = hasNext;
      } catch (e: any) {
        message.error(e?.message || t('pages.rbac.admins.messages.loadRolesFailed', 'Failed to load roles'));
      } finally {
        roleCodeLoadingRef.current = false;
        setRoleCodeLoading(false);
      }
    },
    [mergeOptionsByValue, message],
  );

  const ensureRoleCodeFirstPageLoaded = useCallback(() => {
    if (roleCodePageRef.current > 0) return;
    void loadRoleCodePage(1);
  }, [loadRoleCodePage]);

  const loadRoleCodeNextPage = useCallback(() => {
    if (!roleCodeHasNextRef.current) return;
    const nextPage = roleCodePageRef.current > 0 ? roleCodePageRef.current + 1 : 1;
    void loadRoleCodePage(nextPage);
  }, [loadRoleCodePage]);

  const onRoleCodeDropdownVisibleChange = useCallback(
    (open: boolean) => {
      if (!open) return;
      ensureRoleCodeFirstPageLoaded();
    },
    [ensureRoleCodeFirstPageLoaded],
  );

  const onRoleCodePopupScroll = useCallback(
    (e: React.UIEvent<HTMLDivElement>) => {
      const target = e.target as HTMLDivElement;
      if (!target) return;
      if (target.scrollTop + target.offsetHeight < target.scrollHeight - ROLE_CODE_SCROLL_THRESHOLD_PX) return;
      loadRoleCodeNextPage();
    },
    [loadRoleCodeNextPage],
  );

  useEffect(() => {
    if (createOpen || !!editing) ensureRoleCodeFirstPageLoaded();
  }, [createOpen, editing, ensureRoleCodeFirstPageLoaded]);

  const roleCodeOptionsWithEditing = useMemo(() => {
    const currentRoleCode = editing?.role;
    if (!currentRoleCode) return roleCodeOptions;
    if (roleCodeOptions.some((o) => o.value === currentRoleCode)) return roleCodeOptions;
    return [{ label: currentRoleCode, value: currentRoleCode }, ...roleCodeOptions];
  }, [editing?.role, roleCodeOptions]);

  const columns: ProColumns<AdminAdminUserItem>[] = [
    { title: t('pages.rbac.admins.columns.adminId', 'Admin ID'), dataIndex: 'admin_id', copyable: true, width: 160 },
    { title: t('pages.rbac.admins.columns.username', 'Username'), dataIndex: 'username' },
    { title: t('pages.rbac.admins.columns.name', 'Name'), dataIndex: 'name' },
    {
      title: t('pages.rbac.admins.columns.roleLegacy', 'Role (legacy)'),
      dataIndex: 'role',
      copyable: true,
      renderFormItem: (_, config) => (
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder={t('pages.rbac.admins.form.rolePlaceholder', 'Select a role')}
          options={roleCodeOptions}
          loading={roleCodeLoading}
          onDropdownVisibleChange={onRoleCodeDropdownVisibleChange}
          onPopupScroll={onRoleCodePopupScroll}
          dropdownRender={(menu) => (
            <>
              {menu}
              {roleCodeLoading ? (
                <div style={{ padding: 8, textAlign: 'center' }}>
                  <Spin size="small" />
                </div>
              ) : null}
            </>
          )}
          value={config.value}
          onChange={(value) => config.onChange?.(value)}
        />
      ),
    },
    {
      title: t('pages.rbac.admins.columns.twoFa', '2FA'),
      dataIndex: 'two_factor_enabled',
      width: 80,
      render: (_, row) =>
        row.two_factor_enabled ? (
          <Tag color="green">{t('pages.rbac.admins.values.on', 'On')}</Tag>
        ) : (
          <Tag>{t('pages.rbac.admins.values.off', 'Off')}</Tag>
        ),
    },
    {
      title: t('pages.rbac.admins.columns.status', 'Status'),
      dataIndex: 'status',
      width: 100,
      renderFormItem: (_, config) => (
        <Select
          allowClear
          placeholder={t('pages.rbac.admins.form.status', 'Status')}
          options={statusOptions}
          value={config.value}
          onChange={(value) => config.onChange?.(value)}
        />
      ),
      render: (_, row) => toStatusTag(row.status, t),
    },
    { title: t('pages.rbac.admins.columns.lastLogin', 'Last Login'), dataIndex: 'last_login_at', valueType: 'dateTime' },
    {
      title: t('pages.rbac.admins.columns.actions', 'Actions'),
      valueType: 'option',
      width: 460,
      render: (_, row, __, action) => {
        const canUpdate = canRpc(PERM.update);
        const canDisable = canRpc(PERM.disable);
        const canResetPassword = canRpc(PERM.resetPassword);
        const canReset2fa = canRpc(PERM.reset2fa);

        return (
          <Space wrap>
            <Button size="small" disabled={!canUpdate} onClick={() => setEditing(row)}>
              {t('pages.rbac.admins.actions.edit', 'Edit')}
            </Button>
            <Button
              size="small"
              disabled={!canResetPassword}
              onClick={() => {
                const adminId = row.admin_id;
                if (!adminId) return;
                modal.confirm({
                  title: t('pages.rbac.admins.resetPassword.title', 'Reset password?'),
                  content: (
                    <div>
                      <div>{t('pages.rbac.admins.resetPassword.content', 'This will create a new temporary password.')}</div>
                    </div>
                  ),
                  onOk: async () => {
                    try {
                      const res = await adminResetAdminPassword(
                        { adminId },
                        { send_email: false, require_change: true },
                        { skipErrorHandler: true },
                      );
                      if (!res?.success) {
                        throw new Error(res?.message || t('pages.rbac.admins.messages.resetFailed', 'Reset failed'));
                      }

                      const tempPassword = res.data?.temp_password;

                      if (tempPassword) {
                        modal.info({
                          title: t('pages.rbac.admins.resetPassword.tempTitle', 'Temporary password'),
                          content: (
                            <div>
                              <div>
                                {t('pages.rbac.admins.columns.adminId', 'Admin ID')}: {adminId}
                              </div>
                              <Divider />
                              <div style={{ marginBottom: 8 }}>
                                {t(
                                  'pages.rbac.admins.resetPassword.tempHint',
                                  'Please copy it and send via a secure channel. It will not be shown again.',
                                )}
                              </div>
                              <Input.TextArea value={tempPassword} readOnly autoSize />
                            </div>
                          ),
                        });
                      } else {
                        message.success(t('pages.rbac.admins.messages.passwordResetRequested', 'Password reset requested'));
                      }
                    } catch (e: any) {
                      message.error(getRequestErrorMessage(e) || t('pages.rbac.admins.messages.resetFailed', 'Reset failed'));
                    }
                  },
                });
              }}
            >
              {t('pages.rbac.admins.actions.resetPassword', 'Reset Password')}
            </Button>
            <Button
              size="small"
              disabled={!canReset2fa}
              onClick={() => {
                const adminId = row.admin_id;
                if (!adminId) return;
                modal.confirm({
                  title: t('pages.rbac.admins.reset2fa.title', 'Reset 2FA?'),
                  onOk: async () => {
                    try {
                      const res = await adminResetAdminTwoFA(
                        { adminId },
                        { reason: 'reset via admin console' },
                        { skipErrorHandler: true },
                      );
                      if (!res?.success)
                        throw new Error(res?.message || t('pages.rbac.admins.messages.resetFailed', 'Reset failed'));
                      message.success(t('pages.rbac.admins.messages.twoFaResetRequested', '2FA reset requested'));
                      action?.reload();
                    } catch (e: any) {
                      message.error(getRequestErrorMessage(e) || t('pages.rbac.admins.messages.resetFailed', 'Reset failed'));
                    }
                  },
                });
              }}
            >
              {t('pages.rbac.admins.actions.reset2fa', 'Reset 2FA')}
            </Button>
            <Button
              size="small"
              danger
              disabled={!canDisable}
              onClick={() => {
                const adminId = row.admin_id;
                if (!adminId) return;
                modal.confirm({
                  title: t('pages.rbac.admins.disable.title', 'Disable this admin?'),
                  okButtonProps: { danger: true, disabled: !canDisable },
                  onOk: async () => {
                    try {
                      const res = await adminDisableAdmin(
                        { adminId },
                        { reason: 'disabled via admin console' },
                        { skipErrorHandler: true },
                      );
                      if (!res?.success)
                        throw new Error(res?.message || t('pages.rbac.admins.messages.disableFailed', 'Disable failed'));
                      message.success(t('pages.rbac.admins.messages.disabled', 'Disabled'));
                      action?.reload();
                    } catch (e: any) {
                      message.error(getRequestErrorMessage(e) || t('pages.rbac.admins.messages.disableFailed', 'Disable failed'));
                    }
                  },
                });
              }}
            >
              {t('pages.rbac.admins.actions.disable', 'Disable')}
            </Button>
          </Space>
        );
      },
    },
  ];

  return (
    <PageContainer>
      <ProTable<AdminAdminUserItem>
        actionRef={actionRef}
        rowKey={(row) => row.admin_id || row.username || JSON.stringify(row)}
        columns={columns}
        request={async (params) => {
          try {
            const res = await adminListAdmins(
              {
                page: params.current,
                page_size: params.pageSize,
                keyword: (params as any).keyword,
                role: (params as any).role,
                status: (params as any).status,
              },
              { skipErrorHandler: true },
            );
            return {
              success: !!res?.success,
              data: res.data?.admins ?? [],
              total: Number(res.data?.pagination?.total ?? 0),
            };
          } catch (e: any) {
            message.error(getRequestErrorMessage(e) || t('pages.rbac.admins.messages.loadFailed', 'Failed to load admins'));
            return { success: false, data: [], total: 0 };
          }
        }}
        search={{
          labelWidth: 100,
        }}
        toolbar={{
          actions: [
            <Button
              key="create"
              type="primary"
              disabled={!canRpc(PERM.create)}
              onClick={() => setCreateOpen(true)}
            >
              {t('pages.rbac.admins.actions.create', 'Create Admin')}
            </Button>,
          ],
        }}
      />

      <ModalForm<AdminCreateAdminRequest>
        title={t('pages.rbac.admins.create.title', 'Create Admin')}
        open={createOpen}
        onOpenChange={setCreateOpen}
        onFinish={async (values) => {
          try {
            const res = await adminCreateAdmin(values, { skipErrorHandler: true });
            if (!res?.success)
              throw new Error(res?.message || t('pages.rbac.admins.messages.createFailed', 'Create failed'));
            message.success(t('pages.rbac.admins.messages.created', 'Created'));
            if (res.data?.temp_password) {
              modal.info({
                title: t('pages.rbac.admins.create.tempPasswordTitle', 'Temporary password'),
                content: (
                  <div>
                    <div>{t('pages.rbac.admins.create.adminId', 'Admin ID')}: {res.data?.admin_id}</div>
                    <Divider />
                    <Input.TextArea value={res.data.temp_password} readOnly autoSize />
                  </div>
                ),
              });
            }
            setCreateOpen(false);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(getRequestErrorMessage(e) || t('pages.rbac.admins.messages.createFailed', 'Create failed'));
            return false;
          }
        }}
      >
        <ProFormText name="username" label={t('pages.rbac.admins.form.username', 'Username')} rules={[{ required: true }]} />
        <ProFormText name="name" label={t('pages.rbac.admins.form.name', 'Name')} rules={[{ required: true }]} />
        <ProFormText.Password
          name="password"
          label={t('pages.rbac.admins.form.password', 'Password')}
          fieldProps={{
            placeholder: t('pages.rbac.admins.form.passwordPlaceholder', 'Leave blank to auto-generate a temporary password'),
          }}
        />
        <ProFormSelect
          name="role"
          label={t('pages.rbac.admins.form.role', 'Role')}
          options={roleCodeOptions}
          rules={[{ required: true }]}
          fieldProps={{
            showSearch: true,
            allowClear: true,
            optionFilterProp: 'label',
            placeholder: t('pages.rbac.admins.form.rolePlaceholder', 'Select a role'),
            loading: roleCodeLoading,
            onDropdownVisibleChange: onRoleCodeDropdownVisibleChange,
            onPopupScroll: onRoleCodePopupScroll,
            dropdownRender: (menu) => (
              <>
                {menu}
                {roleCodeLoading ? (
                  <div style={{ padding: 8, textAlign: 'center' }}>
                    <Spin size="small" />
                  </div>
                ) : null}
              </>
            ),
          }}
        />
        <ProFormSwitch
          name="require_password_change"
          label={t('pages.rbac.admins.form.requirePasswordChange', 'Require Password Change')}
          initialValue={true}
        />
        {/* NOTE: 按需求隐藏“需要2FA(two_factor_required)”字段，不在创建管理员弹窗中展示 */}
        {/* <ProFormSwitch name="two_factor_required" label={t('pages.rbac.admins.form.twoFaRequired', '2FA Required')} /> */}
      </ModalForm>

      <ModalForm<AdminUpdateAdminBody>
        title={t('pages.rbac.admins.edit.title', 'Edit Admin')}
        open={!!editing}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
        initialValues={{
          name: editing?.name,
          role: editing?.role,
          status: editing?.status,
        }}
        onFinish={async (values) => {
          if (!editing?.admin_id) return false;
          try {
            const res = await adminUpdateAdmin(
              { adminId: editing.admin_id },
              values,
              { skipErrorHandler: true },
            );
            if (!res?.success)
              throw new Error(res?.message || t('pages.rbac.admins.messages.updateFailed', 'Update failed'));
            message.success(t('pages.rbac.admins.messages.updated', 'Updated'));
            setEditing(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(getRequestErrorMessage(e) || t('pages.rbac.admins.messages.updateFailed', 'Update failed'));
            return false;
          }
        }}
      >
        <ProFormText name="name" label={t('pages.rbac.admins.form.name', 'Name')} />
        <ProFormSelect
          name="role"
          label={t('pages.rbac.admins.form.legacyRole', 'Legacy Role')}
          options={roleCodeOptionsWithEditing}
          fieldProps={{
            showSearch: true,
            allowClear: true,
            optionFilterProp: 'label',
            placeholder: t('pages.rbac.admins.form.rolePlaceholder', 'Select a role'),
            loading: roleCodeLoading,
            onDropdownVisibleChange: onRoleCodeDropdownVisibleChange,
            onPopupScroll: onRoleCodePopupScroll,
            dropdownRender: (menu) => (
              <>
                {menu}
                {roleCodeLoading ? (
                  <div style={{ padding: 8, textAlign: 'center' }}>
                    <Spin size="small" />
                  </div>
                ) : null}
              </>
            ),
          }}
        />
        <ProFormText name="status" label={t('pages.rbac.admins.form.status', 'Status')} />
      </ModalForm>
    </PageContainer>
  );
};

export default AdminsPage;
