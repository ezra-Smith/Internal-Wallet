import type { ProColumns } from '@ant-design/pro-components';
import {
  type ActionType,
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormText,
  ProFormTextArea,
  ProFormTreeSelect,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App, Button, Popconfirm, Space, Tag } from 'antd';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  adminCreatePermission,
  adminDeletePermission,
  adminGetMenuTree,
  adminListPermissions,
  adminUpdatePermission,
} from '@/api/generated/rbac';
import type {
  AdminAdminMenu,
  AdminAdminPermission,
  AdminCreatePermissionRequest,
  AdminUpdatePermissionBody,
} from '@/api/generated/schemas';
import { useRbac } from '@/hooks/useRbac';

type TreeOption = {
  title: string;
  value: string;
  children?: TreeOption[];
};

function buildTreeOptions(menus: AdminAdminMenu[]): TreeOption[] {
  return (menus || [])
    .filter((m) => !!m.id)
    .map((m) => ({
      title: m.name || m.path || m.id || '-',
      value: m.id as string,
      children: m.children?.length ? buildTreeOptions(m.children) : undefined,
    }));
}

function toStatusTag(status: number | undefined, t: (id: string, defaultMessage: string) => string) {
  if (status === 1) return <Tag color="green">{t('pages.rbac.permissions.status.enabled', 'Enabled')}</Tag>;
  if (status === 0) return <Tag>{t('pages.rbac.permissions.status.disabled', 'Disabled')}</Tag>;
  return <Tag>{t('pages.rbac.permissions.status.unknown', 'Unknown')}</Tag>;
}

const PERM = {
  create: 'CreatePermission',
  update: 'UpdatePermission',
  delete: 'DeletePermission',
};

const PermissionsPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string, values?: Record<string, any>) =>
    intl.formatMessage({ id, defaultMessage }, values);

  const actionRef = useRef<ActionType | null>(null);

  const [menuTree, setMenuTree] = useState<AdminAdminMenu[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<AdminAdminPermission | null>(null);

  const loadMenus = useCallback(async () => {
    try {
      const res = await adminGetMenuTree({ root_pid: '0' }, { skipErrorHandler: true });
      if (res?.success) setMenuTree(res.data?.list ?? []);
    } catch (e: any) {
      message.error(e?.message || t('pages.rbac.permissions.messages.loadMenusFailed', 'Failed to load menus'));
    }
  }, [message]);

  useEffect(() => {
    void loadMenus();
  }, [loadMenus]);

  const menuOptions = useMemo(() => buildTreeOptions(menuTree), [menuTree]);

  const columns: ProColumns<AdminAdminPermission>[] = [
    { title: t('pages.rbac.permissions.columns.name', 'Name'), dataIndex: 'name' },
    { title: t('pages.rbac.permissions.columns.code', 'Code'), dataIndex: 'code', copyable: true },
    { title: t('pages.rbac.permissions.columns.menuId', 'Menu ID'), dataIndex: 'menu_id', copyable: true },
    { title: t('pages.rbac.permissions.columns.description', 'Description'), dataIndex: 'description', ellipsis: true },
    {
      title: t('pages.rbac.permissions.columns.status', 'Status'),
      dataIndex: 'status',
      width: 110,
      render: (_, row) => toStatusTag(row.status, t),
    },
    {
      title: t('pages.rbac.permissions.columns.actions', 'Actions'),
      valueType: 'option',
      width: 220,
      render: (_, row, __, action) => {
        const canUpdate = canRpc(PERM.update);
        const canDelete = canRpc(PERM.delete);
        return (
          <Space>
            <Button size="small" disabled={!canUpdate} onClick={() => setEditing(row)}>
              {t('pages.rbac.permissions.actions.edit', 'Edit')}
            </Button>
            <Popconfirm
              title={t('pages.rbac.permissions.delete.title', 'Delete this permission?')}
              okButtonProps={{ danger: true, disabled: !canDelete }}
              onConfirm={async () => {
                if (!canDelete) return;
                if (!row.id) return;
                try {
                  const res = await adminDeletePermission({ id: row.id }, undefined, {
                    skipErrorHandler: true,
                  });
                  if (!res?.success)
                    throw new Error(res?.message || t('pages.rbac.permissions.messages.deleteFailed', 'Delete failed'));
                  message.success(t('pages.rbac.permissions.messages.deleted', 'Deleted'));
                  action?.reload();
                } catch (e: any) {
                  message.error(e?.message || t('pages.rbac.permissions.messages.deleteFailed', 'Delete failed'));
                }
              }}
            >
              <Button size="small" danger disabled={!canDelete}>
                {t('pages.rbac.permissions.actions.delete', 'Delete')}
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  return (
    <PageContainer>
      <ProTable<AdminAdminPermission>
        actionRef={actionRef}
        rowKey={(row) => row.id || row.code || JSON.stringify(row)}
        columns={columns}
        request={async (params) => {
          try {
            const res = await adminListPermissions(
              {
                page: params.current,
                page_size: params.pageSize,
                menu_id: (params as any).menu_id,
              },
              { skipErrorHandler: true },
            );
            return {
              success: !!res?.success,
              data: res.data?.list ?? [],
              total: Number(res.data?.pagination?.total ?? 0),
            };
          } catch (e: any) {
            message.error(e?.message || t('pages.rbac.permissions.messages.loadFailed', 'Failed to load permissions'));
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
              {t('pages.rbac.permissions.actions.create', 'Create Permission')}
            </Button>,
          ],
        }}
      />

      <ModalForm<AdminCreatePermissionRequest>
        title={t('pages.rbac.permissions.create.title', 'Create Permission')}
        open={createOpen}
        onOpenChange={setCreateOpen}
        onFinish={async (values) => {
          try {
            const res = await adminCreatePermission(values, { skipErrorHandler: true });
            if (!res?.success)
              throw new Error(res?.message || t('pages.rbac.permissions.messages.createFailed', 'Create failed'));
            message.success(t('pages.rbac.permissions.messages.created', 'Created'));
            setCreateOpen(false);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(e?.message || t('pages.rbac.permissions.messages.createFailed', 'Create failed'));
            return false;
          }
        }}
      >
        <ProFormText name="name" label={t('pages.rbac.permissions.form.name', 'Name')} rules={[{ required: true }]} />
        <ProFormText name="code" label={t('pages.rbac.permissions.form.code', 'Code')} rules={[{ required: true }]} />
        <ProFormTreeSelect
          name="menu_id"
          label={t('pages.rbac.permissions.form.menu', 'Menu')}
          fieldProps={{
            treeData: menuOptions,
            allowClear: true,
            showSearch: true,
          }}
        />
        <ProFormDigit name="status" label={t('pages.rbac.permissions.form.status', 'Status')} min={0} max={1} fieldProps={{ precision: 0 }} />
        <ProFormTextArea name="description" label={t('pages.rbac.permissions.form.description', 'Description')} fieldProps={{ rows: 3 }} />
      </ModalForm>

      <ModalForm<AdminUpdatePermissionBody>
        title={t('pages.rbac.permissions.edit.title', 'Edit Permission')}
        open={!!editing}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
        initialValues={{
          name: editing?.name,
          code: editing?.code,
          menu_id: editing?.menu_id,
          description: editing?.description,
          status: editing?.status,
        }}
        onFinish={async (values) => {
          if (!editing?.id) return false;
          try {
            const res = await adminUpdatePermission(
              { id: editing.id },
              values,
              { skipErrorHandler: true },
            );
            if (!res?.success)
              throw new Error(res?.message || t('pages.rbac.permissions.messages.updateFailed', 'Update failed'));
            message.success(t('pages.rbac.permissions.messages.updated', 'Updated'));
            setEditing(null);
            actionRef.current?.reload();
            return true;
          } catch (e: any) {
            message.error(e?.message || t('pages.rbac.permissions.messages.updateFailed', 'Update failed'));
            return false;
          }
        }}
      >
        <ProFormText name="name" label={t('pages.rbac.permissions.form.name', 'Name')} rules={[{ required: true }]} />
        <ProFormText name="code" label={t('pages.rbac.permissions.form.code', 'Code')} rules={[{ required: true }]} />
        <ProFormTreeSelect
          name="menu_id"
          label={t('pages.rbac.permissions.form.menu', 'Menu')}
          fieldProps={{
            treeData: menuOptions,
            allowClear: true,
            showSearch: true,
          }}
        />
        <ProFormDigit name="status" label={t('pages.rbac.permissions.form.status', 'Status')} min={0} max={1} fieldProps={{ precision: 0 }} />
        <ProFormTextArea name="description" label={t('pages.rbac.permissions.form.description', 'Description')} fieldProps={{ rows: 3 }} />
      </ModalForm>
    </PageContainer>
  );
};

export default PermissionsPage;
