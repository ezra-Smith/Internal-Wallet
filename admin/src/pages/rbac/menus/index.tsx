import type { ProColumns } from '@ant-design/pro-components';
import {
  ModalForm,
  PageContainer,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProFormTreeSelect,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { App, Button, Form, Popconfirm, Space, Tag } from 'antd';
import type { FormInstance } from 'antd';
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  adminCreateMenu,
  adminDeleteMenu,
  adminGetMenuTree,
  adminUpdateMenu,
} from '@/api/generated/rbac';
import type {
  AdminAdminMenu,
  AdminCreateMenuRequest,
  AdminUpdateMenuBody,
} from '@/api/generated/schemas';
import IconPickerInput from '@/components/IconPicker/IconPickerInput';
import { renderIconValuePreview } from '@/components/IconPicker/iconUtils';
import { useRbac } from '@/hooks/useRbac';

type TreeOption = {
  title: string;
  value: string;
  children?: TreeOption[];
};

function buildTreeOptions(menus: AdminAdminMenu[], excludeId?: string): TreeOption[] {
  return (menus || [])
    .filter((m) => !!m.id && m.id !== excludeId)
    .map((m) => ({
      title: m.name || m.path || m.id || '-',
      value: m.id as string,
      children: m.children?.length ? buildTreeOptions(m.children, excludeId) : undefined,
    }));
}

function toStatusTag(status: number | undefined, t: (id: string, defaultMessage: string) => string) {
  if (status === 1) return <Tag color="green">{t('pages.rbac.menus.status.enabled', 'Enabled')}</Tag>;
  if (status === 0) return <Tag>{t('pages.rbac.menus.status.disabled', 'Disabled')}</Tag>;
  return <Tag>{t('pages.rbac.menus.status.unknown', 'Unknown')}</Tag>;
}

const PERM = {
  create: 'CreateMenu',
  update: 'UpdateMenu',
  delete: 'DeleteMenu',
};

const MenusPage: React.FC = () => {
  const { message } = App.useApp();
  const { canRpc } = useRbac();
  const intl = useIntl();
  const t = (id: string, defaultMessage: string, values?: Record<string, any>) =>
    intl.formatMessage({ id, defaultMessage }, values);

  const [loading, setLoading] = useState(false);
  const [menuTree, setMenuTree] = useState<AdminAdminMenu[]>([]);

  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<AdminAdminMenu | null>(null);
  const editFormRef = useRef<FormInstance>(null);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const res = await adminGetMenuTree({ root_pid: '0' }, { skipErrorHandler: true });
      if (!res?.success)
        throw new Error(res?.message || t('pages.rbac.menus.messages.loadFailed', 'Failed to load menus'));
      setMenuTree(res.data?.list ?? []);
    } catch (e: any) {
      message.error(e?.message || t('pages.rbac.menus.messages.loadFailed', 'Failed to load menus'));
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const parentOptions = useMemo(() => buildTreeOptions(menuTree), [menuTree]);
  const editParentOptions = useMemo(
    () => buildTreeOptions(menuTree, editing?.id),
    [menuTree, editing?.id],
  );

  // 当 editing 改变时，更新编辑表单的值
  useEffect(() => {
    if (editing && editFormRef.current) {
      editFormRef.current.setFieldsValue({
        name: editing.name,
        path: editing.path,
        pid: editing.pid,
        icon: editing.icon,
        sort: editing.sort,
        access: editing.access,
        key: editing.key,
        hide_in_menu: editing.hide_in_menu,
        hide_children_in_menu: editing.hide_children_in_menu,
        target: editing.target,
        remark: editing.remark,
        status: editing.status,
      });
    }
  }, [editing]);

  const columns: ProColumns<AdminAdminMenu>[] = [
    { title: t('pages.rbac.menus.columns.name', 'Name'), dataIndex: 'name' },
    { title: t('pages.rbac.menus.columns.path', 'Path'), dataIndex: 'path', copyable: true },
    { title: t('pages.rbac.menus.columns.access', 'Access'), dataIndex: 'access', copyable: true },
    {
      title: t('pages.rbac.menus.columns.icon', 'Icon'),
      dataIndex: 'icon',
      render: (_, row) => {
        const raw = (row.icon as unknown as string | undefined) ?? '';
        const preview = renderIconValuePreview(raw);
        if (!raw) return '-';
        return (
          <Space size={8}>
            {preview ? <span style={{ display: 'inline-flex' }}>{preview}</span> : null}
            <span style={{ fontFamily: 'monospace' }}>{raw}</span>
          </Space>
        );
      },
    },
    { title: t('pages.rbac.menus.columns.sort', 'Sort'), dataIndex: 'sort', width: 80 },
    {
      title: t('pages.rbac.menus.columns.hidden', 'Hidden'),
      dataIndex: 'hide_in_menu',
      width: 90,
      render: (_, row) =>
        row.hide_in_menu ? <Tag>{t('pages.rbac.menus.values.yes', 'Yes')}</Tag> : <Tag color="green">{t('pages.rbac.menus.values.no', 'No')}</Tag>,
    },
    {
      title: t('pages.rbac.menus.columns.status', 'Status'),
      dataIndex: 'status',
      width: 110,
      render: (_, row) => toStatusTag(row.status, t),
    },
    {
      title: t('pages.rbac.menus.columns.actions', 'Actions'),
      valueType: 'option',
      width: 220,
      render: (_, row) => {
        const canUpdate = canRpc(PERM.update);
        const canDelete = canRpc(PERM.delete);
        return (
          <Space>
            <Button
              size="small"
              disabled={!canUpdate}
              onClick={() => setEditing(row)}
            >
              {t('pages.rbac.menus.actions.edit', 'Edit')}
            </Button>
            <Popconfirm
              title={t('pages.rbac.menus.delete.title', 'Delete this menu?')}
              okButtonProps={{ danger: true, disabled: !canDelete }}
              onConfirm={async () => {
                if (!canDelete) return;
                if (!row.id) return;
                try {
                  const res = await adminDeleteMenu({ id: row.id }, { skipErrorHandler: true });
                  if (!res?.success)
                    throw new Error(res?.message || t('pages.rbac.menus.messages.deleteFailed', 'Delete failed'));
                  message.success(t('pages.rbac.menus.messages.deleted', 'Deleted'));
                  await reload();
                } catch (e: any) {
                  message.error(e?.message || t('pages.rbac.menus.messages.deleteFailed', 'Delete failed'));
                }
              }}
            >
              <Button size="small" danger disabled={!canDelete}>
                {t('pages.rbac.menus.actions.delete', 'Delete')}
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  return (
    <PageContainer>
      <ProTable<AdminAdminMenu>
        rowKey={(row) => row.id || row.path || row.key || JSON.stringify(row)}
        loading={loading}
        dataSource={menuTree}
        columns={columns}
        pagination={false}
        search={false}
        options={{ reload: false }}
        toolBarRender={() => {
          const canCreate = canRpc(PERM.create);
          return [
            <Button key="reload" onClick={() => void reload()}>
              {t('pages.rbac.menus.actions.reload', 'Reload')}
            </Button>,
            <Button
              key="create"
              type="primary"
              disabled={!canCreate}
              onClick={() => setCreateOpen(true)}
            >
              {t('pages.rbac.menus.actions.create', 'Create Menu')}
            </Button>,
          ];
        }}
      />

      <ModalForm<AdminCreateMenuRequest>
        title={t('pages.rbac.menus.create.title', 'Create Menu')}
        open={createOpen}
        onOpenChange={setCreateOpen}
        onFinish={async (values) => {
          try {
            const res = await adminCreateMenu(values, { skipErrorHandler: true });
            if (!res?.success)
              throw new Error(res?.message || t('pages.rbac.menus.messages.createFailed', 'Create failed'));
            message.success(t('pages.rbac.menus.messages.created', 'Created'));
            setCreateOpen(false);
            await reload();
            return true;
          } catch (e: any) {
            message.error(e?.message || t('pages.rbac.menus.messages.createFailed', 'Create failed'));
            return false;
          }
        }}
      >
        <ProFormText name="name" label={t('pages.rbac.menus.form.name', 'Name')} rules={[{ required: true }]} />
        <ProFormText name="path" label={t('pages.rbac.menus.form.path', 'Path')} />
        <ProFormTreeSelect
          name="pid"
          label={t('pages.rbac.menus.form.parent', 'Parent')}
          fieldProps={{
            treeData: [{ title: t('pages.rbac.menus.form.root', 'Root'), value: '0', children: parentOptions }],
            allowClear: true,
            showSearch: true,
            placeholder: t('pages.rbac.menus.form.root', 'Root'),
          }}
        />
        <Form.Item name="icon" label={t('pages.rbac.menus.form.icon', 'Icon')}>
          <IconPickerInput />
        </Form.Item>
        <ProFormDigit name="sort" label={t('pages.rbac.menus.form.sort', 'Sort')} min={0} fieldProps={{ precision: 0 }} />
        <ProFormText name="access" label={t('pages.rbac.menus.form.accessCode', 'Access Code')} />
        <ProFormText name="key" label={t('pages.rbac.menus.form.key', 'Key')} />
        <ProFormSelect
          name="target"
          label={t('pages.rbac.menus.form.target', 'Target')}
          options={[
            { label: t('pages.rbac.menus.form.targetSame', 'Same Tab'), value: '' },
            { label: t('pages.rbac.menus.form.targetNew', 'New Tab'), value: '_blank' },
          ]}
        />
        <ProFormSwitch name="hide_in_menu" label={t('pages.rbac.menus.form.hideInMenu', 'Hide In Menu')} />
        <ProFormSwitch
          name="hide_children_in_menu"
          label={t('pages.rbac.menus.form.hideChildrenInMenu', 'Hide Children In Menu')}
        />
        <ProFormDigit name="status" label={t('pages.rbac.menus.form.status', 'Status')} min={0} max={1} fieldProps={{ precision: 0 }} />
        <ProFormTextArea name="remark" label={t('pages.rbac.menus.form.remark', 'Remark')} fieldProps={{ rows: 3 }} />
      </ModalForm>

      <ModalForm<AdminUpdateMenuBody>
        title={t('pages.rbac.menus.edit.title', 'Edit Menu')}
        open={!!editing}
        formRef={editFormRef}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
        onFinish={async (values) => {
          if (!editing?.id) return false;
          try {
            const res = await adminUpdateMenu(
              { id: editing.id },
              values,
              { skipErrorHandler: true },
            );
            if (!res?.success)
              throw new Error(res?.message || t('pages.rbac.menus.messages.updateFailed', 'Update failed'));
            message.success(t('pages.rbac.menus.messages.updated', 'Updated'));
            setEditing(null);
            await reload();
            return true;
          } catch (e: any) {
            message.error(e?.message || t('pages.rbac.menus.messages.updateFailed', 'Update failed'));
            return false;
          }
        }}
      >
        <ProFormText name="name" label={t('pages.rbac.menus.form.name', 'Name')} rules={[{ required: true }]} />
        <ProFormText name="path" label={t('pages.rbac.menus.form.path', 'Path')} />
        <ProFormTreeSelect
          name="pid"
          label={t('pages.rbac.menus.form.parent', 'Parent')}
          fieldProps={{
            treeData: [{ title: t('pages.rbac.menus.form.root', 'Root'), value: '0', children: editParentOptions }],
            allowClear: true,
            showSearch: true,
            placeholder: t('pages.rbac.menus.form.root', 'Root'),
          }}
        />
        <Form.Item name="icon" label={t('pages.rbac.menus.form.icon', 'Icon')}>
          <IconPickerInput />
        </Form.Item>
        <ProFormDigit name="sort" label={t('pages.rbac.menus.form.sort', 'Sort')} min={0} fieldProps={{ precision: 0 }} />
        <ProFormText name="access" label={t('pages.rbac.menus.form.accessCode', 'Access Code')} />
        <ProFormText name="key" label={t('pages.rbac.menus.form.key', 'Key')} />
        <ProFormSelect
          name="target"
          label={t('pages.rbac.menus.form.target', 'Target')}
          options={[
            { label: t('pages.rbac.menus.form.targetSame', 'Same Tab'), value: '' },
            { label: t('pages.rbac.menus.form.targetNew', 'New Tab'), value: '_blank' },
          ]}
        />
        <ProFormSwitch name="hide_in_menu" label={t('pages.rbac.menus.form.hideInMenu', 'Hide In Menu')} />
        <ProFormSwitch
          name="hide_children_in_menu"
          label={t('pages.rbac.menus.form.hideChildrenInMenu', 'Hide Children In Menu')}
        />
        <ProFormDigit name="status" label={t('pages.rbac.menus.form.status', 'Status')} min={0} max={1} fieldProps={{ precision: 0 }} />
        <ProFormTextArea name="remark" label={t('pages.rbac.menus.form.remark', 'Remark')} fieldProps={{ rows: 3 }} />
      </ModalForm>
    </PageContainer>
  );
};

export default MenusPage;
