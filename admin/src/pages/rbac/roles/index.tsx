import type { ProColumns } from '@ant-design/pro-components';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App, Button, Drawer, Space, Spin, Tabs, Transfer, Tree } from 'antd';
import React, { useCallback, useMemo, useRef, useState } from 'react';
import {
  adminAssignMenusToRole,
  adminAssignPermissionsToRole,
  adminCreateRole,
  adminDeleteRole,
  adminGetMenuTree,
  adminGetRoleMenus,
  adminGetRolePermissions,
  adminListPermissions,
  adminListRoles,
  adminUpdateRole,
} from '@/api/generated/rbac';
import type {
  AdminAdminMenu,
  AdminAdminPermission,
  AdminAdminRole,
  AdminCreateRoleRequest,
  AdminUpdateRoleBody,
} from '@/api/generated/schemas';
import { useRbac } from '@/hooks/useRbac';

type TransferItem = {
  key: string;
  title: string;
  description?: string;
};

function toTreeData(menus: AdminAdminMenu[] | undefined): any[] {
  return (menus || [])
    .filter((m) => !!m.id)
    .map((m) => ({
      key: String(m.id),
      title: m.name || m.path || m.id,
      children: toTreeData(m.children),
    }));
}

function flattenPermissionTransferItems(perms: AdminAdminPermission[]): TransferItem[] {
  return (perms || [])
    .filter((p) => !!p.id)
    .map((p) => ({
      key: String(p.id),
      title: p.name || p.code || p.id || '-',
      description: p.code || p.description,
    }));
}

function uniqStrings(values: Array<string | undefined | null>): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const value of values) {
    if (!value) continue;
    const normalized = String(value);
    if (seen.has(normalized)) continue;
    seen.add(normalized);
    out.push(normalized);
  }
  return out;
}

function mergePermissionsForTransfer(
  all: AdminAdminPermission[],
  selected: AdminAdminPermission[],
): AdminAdminPermission[] {
  const byId = new Map<string, AdminAdminPermission>();
  for (const perm of all) {
    if (!perm.id) continue;
    if (byId.has(perm.id)) continue;
    byId.set(perm.id, perm);
  }
  for (const perm of selected) {
    if (!perm.id) continue;
    if (byId.has(perm.id)) continue;
    byId.set(perm.id, perm);
  }
  return Array.from(byId.values());
}

const RolesPage: React.FC = () => {
  const { message, modal } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = useCallback(
    (id: string, defaultMessage: string, values?: Record<string, any>) =>
      intl.formatMessage({ id, defaultMessage }, values),
    [intl],
  );

  const actionRef = useRef<ActionType | null>(null);
  const assignRequestIdRef = useRef(0);

  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<AdminAdminRole | null>(null);

  const [assignOpen, setAssignOpen] = useState(false);
  const [assignLoading, setAssignLoading] = useState(false);
  const [assignRole, setAssignRole] = useState<AdminAdminRole | null>(null);
  const [menuTree, setMenuTree] = useState<AdminAdminMenu[]>([]);
  const [checkedMenuIds, setCheckedMenuIds] = useState<string[]>([]);
  const [allPermissions, setAllPermissions] = useState<AdminAdminPermission[]>([]);
  const [selectedPermissionIds, setSelectedPermissionIds] = useState<string[]>([]);

  const fetchAllPermissions = useCallback(async () => {
    const pageSize = 1000;
    const maxPages = 200;

    const collected: AdminAdminPermission[] = [];
    for (let page = 1; page <= maxPages; page += 1) {
      const res = await adminListPermissions(
        { page, page_size: pageSize },
        { skipErrorHandler: true },
      );
      if (!res?.success) {
        throw new Error(
          res?.message ||
            t('pages.rbac.roles.messages.loadPermissionsFailed', 'Failed to load permissions'),
        );
      }

      collected.push(...(res?.data?.list ?? []));

      const hasNext = res?.data?.pagination?.has_next;
      const totalPages = res?.data?.pagination?.total_pages;
      const listLength = res?.data?.list?.length ?? 0;

      if (hasNext === false) break;
      if (typeof hasNext === 'undefined' && typeof totalPages === 'number' && page >= totalPages) break;
      if (listLength === 0) break;
    }

    return collected;
  }, [t]);

  const openAssignDrawer = useCallback(async (role: AdminAdminRole) => {
    if (!role.id) return;
    assignRequestIdRef.current += 1;
    const requestId = assignRequestIdRef.current;
    setAssignRole(role);
    setAssignOpen(true);
    setAssignLoading(true);
    try {
      const [treeRes, roleMenusRes, rolePermsRes, perms] = await Promise.all([
        adminGetMenuTree({ root_pid: '0' }, { skipErrorHandler: true }),
        adminGetRoleMenus({ roleId: role.id }, { skipErrorHandler: true }),
        adminGetRolePermissions({ roleId: role.id }, { skipErrorHandler: true }),
        fetchAllPermissions(),
      ]);

      if (assignRequestIdRef.current !== requestId) return;

      setMenuTree(treeRes?.data?.list ?? []);
      setCheckedMenuIds(uniqStrings((roleMenusRes?.data?.list ?? []).map((m: AdminAdminMenu) => m.id)));

      const rolePerms = rolePermsRes?.data?.list ?? [];
      const mergedPerms = mergePermissionsForTransfer(perms, rolePerms);
      setAllPermissions(mergedPerms);
      setSelectedPermissionIds(uniqStrings(rolePerms.map((p: AdminAdminPermission) => p.id)));
    } catch (e: any) {
      if (assignRequestIdRef.current !== requestId) return;
      message.error(
        e?.message || t('pages.rbac.roles.messages.loadBindingsFailed', 'Failed to load role bindings'),
      );
    } finally {
      if (assignRequestIdRef.current === requestId) setAssignLoading(false);
    }
  }, [fetchAllPermissions, message, t]);

  const permissionItems = useMemo(
    () => flattenPermissionTransferItems(allPermissions),
    [allPermissions],
  );

  const columns: ProColumns<AdminAdminRole>[] = [
    { title: t('pages.rbac.roles.columns.name', 'Name'), dataIndex: 'name' },
    { title: t('pages.rbac.roles.columns.code', 'Code'), dataIndex: 'code', copyable: true },
    { title: t('pages.rbac.roles.columns.description', 'Description'), dataIndex: 'description', ellipsis: true },
    { title: t('pages.rbac.roles.columns.status', 'Status'), dataIndex: 'status', width: 80 },
    {
      title: t('pages.rbac.roles.columns.actions', 'Actions'),
      valueType: 'option',
      width: 360,
      render: (_, row, __, action) => {
        const canUpdate = canRpc('UpdateRole');
        const canDelete = canRpc('DeleteRole');
        const canAssignMenus = canRpc('AssignMenusToRole');
        const canAssignPermissions = canRpc('AssignPermissionsToRole');

        return (
          <Space>
            <Button size="small" disabled={!canUpdate} onClick={() => setEditing(row)}>
              {t('pages.rbac.roles.actions.edit', 'Edit')}
            </Button>
            <Button
              size="small"
              disabled={!canAssignMenus && !canAssignPermissions}
              onClick={() => void openAssignDrawer(row)}
            >
              {t('pages.rbac.roles.actions.assign', 'Assign')}
            </Button>
            <Button
              size="small"
              danger
              disabled={!canDelete}
              onClick={() => {
                const roleId = row.id;
                if (!roleId) return;
                modal.confirm({
                  title: t('pages.rbac.roles.delete.title', 'Delete this role?'),
                  okButtonProps: { danger: true, disabled: !canDelete },
                  onOk: async () => {
                    try {
                      const res = await adminDeleteRole(
                        { id: roleId },
                        { force: false },
                        { skipErrorHandler: true },
                      );
                      if (!res?.success)
                        throw new Error(res?.message || t('pages.rbac.roles.messages.deleteFailed', 'Delete failed'));
                      message.success(t('pages.rbac.roles.messages.deleted', 'Deleted'));
                      action?.reload();
                    } catch (e: any) {
                      message.error(e?.message || t('pages.rbac.roles.messages.deleteFailed', 'Delete failed'));
                    }
                  },
                });
              }}
            >
              {t('pages.rbac.roles.actions.delete', 'Delete')}
            </Button>
          </Space>
        );
      },
    },
  ];

  return (
    <PageContainer>
      <ProTable<AdminAdminRole>
        actionRef={actionRef}
        rowKey={(row) => row.id || row.code || JSON.stringify(row)}
        columns={columns}
        request={async (params) => {
          try {
            const res = await adminListRoles(
              { page: params.current, page_size: params.pageSize },
              { skipErrorHandler: true },
            );
            return {
              success: !!res?.success,
              data: res.data?.list ?? [],
              total: Number(res.data?.pagination?.total ?? 0),
            };
          } catch (e: any) {
            message.error(e?.message || t('pages.rbac.roles.messages.loadFailed', 'Failed to load roles'));
            return { success: false, data: [], total: 0 };
          }
        }}
        search={false}
        toolbar={{
          actions: [
            <Button
              key="create"
              type="primary"
              disabled={!canRpc('CreateRole')}
              onClick={() => setCreateOpen(true)}
            >
              {t('pages.rbac.roles.actions.create', 'Create Role')}
            </Button>,
          ],
        }}
      />

      <ModalForm<AdminCreateRoleRequest>
        title={t('pages.rbac.roles.create.title', 'Create Role')}
        open={createOpen}
        onOpenChange={setCreateOpen}
        onFinish={async (values) => {
          try {
            const res = await adminCreateRole(values, { skipErrorHandler: true });
            if (!res?.success)
              throw new Error(res?.message || t('pages.rbac.roles.messages.createFailed', 'Create failed'));
            message.success(t('pages.rbac.roles.messages.created', 'Created'));
            setCreateOpen(false);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(e?.message || t('pages.rbac.roles.messages.createFailed', 'Create failed'));
            return false;
          }
        }}
      >
        <ProFormText name="name" label={t('pages.rbac.roles.form.name', 'Name')} rules={[{ required: true }]} />
        <ProFormText name="code" label={t('pages.rbac.roles.form.code', 'Code')} rules={[{ required: true }]} />
        <ProFormDigit
          name="status"
          label={t('pages.rbac.roles.form.status', 'Status')}
          min={0}
          max={1}
          fieldProps={{ precision: 0 }}
        />
        <ProFormTextArea name="description" label={t('pages.rbac.roles.form.description', 'Description')} fieldProps={{ rows: 3 }} />
      </ModalForm>

      <ModalForm<AdminUpdateRoleBody>
        title={t('pages.rbac.roles.edit.title', 'Edit Role')}
        open={!!editing}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
        initialValues={{
          name: editing?.name,
          code: editing?.code,
          description: editing?.description,
          status: editing?.status,
        }}
        onFinish={async (values) => {
          if (!editing?.id) return false;
          try {
            const res = await adminUpdateRole(
              { id: editing.id },
              values,
              { skipErrorHandler: true },
            );
            if (!res?.success)
              throw new Error(res?.message || t('pages.rbac.roles.messages.updateFailed', 'Update failed'));
            message.success(t('pages.rbac.roles.messages.updated', 'Updated'));
            setEditing(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(e?.message || t('pages.rbac.roles.messages.updateFailed', 'Update failed'));
            return false;
          }
        }}
      >
        <ProFormText name="name" label={t('pages.rbac.roles.form.name', 'Name')} rules={[{ required: true }]} />
        <ProFormText name="code" label={t('pages.rbac.roles.form.code', 'Code')} rules={[{ required: true }]} />
        <ProFormDigit
          name="status"
          label={t('pages.rbac.roles.form.status', 'Status')}
          min={0}
          max={1}
          fieldProps={{ precision: 0 }}
        />
        <ProFormTextArea name="description" label={t('pages.rbac.roles.form.description', 'Description')} fieldProps={{ rows: 3 }} />
      </ModalForm>

      <Drawer
        title={
          assignRole?.name
            ? t('pages.rbac.roles.assign.titleWithRole', 'Assign: {role}', { role: assignRole.name })
            : t('pages.rbac.roles.assign.title', 'Assign')
        }
        open={assignOpen}
        width={820}
        onClose={() => {
          assignRequestIdRef.current += 1;
          setAssignOpen(false);
          setAssignLoading(false);
          setAssignRole(null);
          setMenuTree([]);
          setCheckedMenuIds([]);
          setAllPermissions([]);
          setSelectedPermissionIds([]);
        }}
        destroyOnClose
      >
        <Spin spinning={assignLoading}>
          <Tabs
            items={[
              {
                key: 'menus',
                label: t('pages.rbac.roles.assign.menus', 'Menus'),
                children: (
                  <>
                    <div style={{ marginBottom: 12 }}>
                      <Button
                        type="primary"
                        disabled={!canRpc('AssignMenusToRole') || !assignRole?.id}
                        onClick={async () => {
                          if (!assignRole?.id) return;
                          try {
                            const res = await adminAssignMenusToRole(
                              { roleId: assignRole.id },
                              { menu_ids: checkedMenuIds, replace: true },
                              { skipErrorHandler: true },
                            );
                            if (!res?.success)
                              throw new Error(res?.message || t('pages.rbac.roles.messages.assignMenusFailed', 'Assign menus failed'));
                            message.success(t('pages.rbac.roles.messages.menusUpdated', 'Menus updated'));
                            actionRef.current?.reload();
                          } catch (e: any) {
                            message.error(e?.message || t('pages.rbac.roles.messages.assignMenusFailed', 'Assign menus failed'));
                          }
                        }}
                      >
                        {t('pages.rbac.roles.actions.saveMenus', 'Save Menus')}
                      </Button>
                    </div>
                    <Tree
                      checkable
                      defaultExpandAll
                      checkedKeys={checkedMenuIds}
                      treeData={toTreeData(menuTree)}
                      onCheck={(keys) => {
                        const checkedKeys = Array.isArray(keys) ? keys : ((keys as any)?.checked ?? []);
                        setCheckedMenuIds(uniqStrings((checkedKeys as any[]).map(String)));
                      }}
                    />
                  </>
                ),
              },
              {
                key: 'permissions',
                label: t('pages.rbac.roles.assign.permissions', 'Permissions'),
                children: (
                  <>
                    <div style={{ marginBottom: 12 }}>
                      <Button
                        type="primary"
                        disabled={
                          !canRpc('AssignPermissionsToRole') || !assignRole?.id
                        }
                        onClick={async () => {
                          if (!assignRole?.id) return;
                          try {
                            const res = await adminAssignPermissionsToRole(
                              { roleId: assignRole.id },
                              { permission_ids: selectedPermissionIds, replace: true },
                              { skipErrorHandler: true },
                            );
                            if (!res?.success)
                              throw new Error(
                                res?.message || t('pages.rbac.roles.messages.assignPermissionsFailed', 'Assign permissions failed'),
                              );
                            message.success(t('pages.rbac.roles.messages.permissionsUpdated', 'Permissions updated'));
                            actionRef.current?.reload();
                          } catch (e: any) {
                            message.error(
                              e?.message || t('pages.rbac.roles.messages.assignPermissionsFailed', 'Assign permissions failed'),
                            );
                          }
                        }}
                      >
                        {t('pages.rbac.roles.actions.savePermissions', 'Save Permissions')}
                      </Button>
                    </div>
                    <Transfer
                      rowKey={(item) => item.key}
                      dataSource={permissionItems}
                      titles={[
                        t('pages.rbac.roles.transfer.available', 'Available'),
                        t('pages.rbac.roles.transfer.selected', 'Selected'),
                      ]}
                      showSearch
                      targetKeys={selectedPermissionIds}
                      onChange={(nextKeys) => setSelectedPermissionIds(uniqStrings(nextKeys.map(String)))}
                      render={(item) => item.title}
                      listStyle={{ width: 360, height: 520 }}
                    />
                  </>
                ),
              },
            ]}
          />
        </Spin>
      </Drawer>
    </PageContainer>
  );
};

export default RolesPage;
