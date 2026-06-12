import * as AntIcons from '@ant-design/icons';
import { Icon as IconifyIcon } from '@iconify/react';
import { useIntl } from '@umijs/max';
import { Button, Input, Modal, Segmented, Space, Tooltip, Typography } from 'antd';
import React, { useMemo, useState } from 'react';
import { ANTD_ICON_NAMES } from './antdIconCatalog';
import { ICONIFY_LUCIDE, ICONIFY_MDI, type IconifyCatalogItem } from './iconifyCatalog';
import { renderIconValuePreview } from './iconUtils';

const { AppstoreOutlined, CloseCircleFilled, SearchOutlined } = AntIcons;

type IconSource = 'antd' | 'iconify';
type IconifySet = 'lucide' | 'mdi';

export type IconPickerValue = string;

export type IconPickerInputProps = {
  value?: IconPickerValue;
  onChange?: (value: IconPickerValue) => void;
  placeholder?: string;
};

// 使用静态图标列表，避免 tree-shaking 导致动态导入为空
function buildAntdIconNames(): string[] {
  return ANTD_ICON_NAMES;
}

function normalizeQuery(q: string): string {
  return q.trim().toLowerCase();
}

export const IconPickerInput: React.FC<IconPickerInputProps> = ({
  value,
  onChange,
  placeholder,
}) => {
  const intl = useIntl();
  const t = (id: string, defaultMessage: string) =>
    intl.formatMessage({ id, defaultMessage });
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<IconSource>('antd');
  const [iconifySet, setIconifySet] = useState<IconifySet>('lucide');
  const [query, setQuery] = useState('');

  const antdIconNames = useMemo(() => buildAntdIconNames(), []);
  const q = normalizeQuery(query);

  const iconifyItems: IconifyCatalogItem[] = useMemo(() => {
    return iconifySet === 'mdi' ? ICONIFY_MDI : ICONIFY_LUCIDE;
  }, [iconifySet]);

  const filteredAntd = useMemo(() => {
    if (!q) return antdIconNames;
    return antdIconNames.filter((name) => name.toLowerCase().includes(q));
  }, [antdIconNames, q]);

  const filteredIconify = useMemo(() => {
    if (!q) return iconifyItems;
    return iconifyItems.filter((it) => `${it.label} ${it.icon}`.toLowerCase().includes(q));
  }, [iconifyItems, q]);

  const pick = (v: string) => {
    onChange?.(v);
    setOpen(false);
  };

  const preview = renderIconValuePreview(value);
  const counts = {
    antd: antdIconNames.length,
    lucide: ICONIFY_LUCIDE.length,
    mdi: ICONIFY_MDI.length,
  };

  return (
    <>
      <Input
        value={value}
        placeholder={placeholder ?? t('pages.rbac.menus.iconPicker.input.placeholder', 'SafetyOutlined / iconify:lucide:user')}
        onChange={(e) => onChange?.(e.target.value)}
        prefix={preview ? <span style={{ display: 'inline-flex' }}>{preview}</span> : undefined}
        suffix={
          <Space size={6}>
            {!!value && (
              <Tooltip title={t('pages.rbac.menus.iconPicker.actions.clear', 'Clear')}>
                <Button
                  type="text"
                  size="small"
                  icon={<CloseCircleFilled />}
                  onClick={() => onChange?.('')}
                />
              </Tooltip>
            )}
            <Tooltip title={t('pages.rbac.menus.iconPicker.actions.pick', 'Pick icon')}>
              <Button
                type="text"
                size="small"
                icon={<AppstoreOutlined />}
                onClick={() => setOpen(true)}
              />
            </Tooltip>
          </Space>
        }
      />

      <Modal
        title={t('pages.rbac.menus.iconPicker.title', 'Pick an icon')}
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        width={960}
        destroyOnClose
      >
        <Space direction="vertical" style={{ width: '100%' }} size={12}>
          <Space wrap style={{ justifyContent: 'space-between', width: '100%' }}>
            <Segmented
              value={source}
              onChange={(v) => setSource(v as IconSource)}
              options={[
                {
                  label: `${t('pages.rbac.menus.iconPicker.source.antd', 'Ant Design')} (${counts.antd})`,
                  value: 'antd',
                },
                {
                  label: t('pages.rbac.menus.iconPicker.source.iconify', 'Open-source (Iconify)'),
                  value: 'iconify',
                },
              ]}
            />
            {source === 'iconify' && (
              <Segmented
                value={iconifySet}
                onChange={(v) => setIconifySet(v as IconifySet)}
                options={[
                  { label: `Lucide (${counts.lucide})`, value: 'lucide' },
                  { label: `MDI (${counts.mdi})`, value: 'mdi' },
                ]}
              />
            )}
          </Space>

          <Input
            allowClear
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            prefix={<SearchOutlined />}
            placeholder={t('pages.rbac.menus.iconPicker.search.placeholder', 'Search icon name…')}
          />

          <div
            style={{
              maxHeight: 520,
              overflow: 'auto',
              padding: 4,
              border: '1px solid rgba(5, 5, 5, 0.08)',
              borderRadius: 8,
            }}
          >
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
                gap: 10,
              }}
            >
              {source === 'antd'
                ? filteredAntd.slice(0, 1200).map((name) => {
                    // 将 PascalCase 转换为 kebab-case 用于 Iconify
                    // 例如: AccountBookOutlined -> account-book-outlined
                    const kebabName = name
                      .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
                      .toLowerCase();
                    const iconifyName = `ant-design:${kebabName}`;
                    return (
                      <Tooltip key={name} title={name}>
                        <Button
                          onClick={() => pick(name)}
                          style={{ height: 44, display: 'flex', alignItems: 'center', justifyContent: 'flex-start' }}
                        >
                          <Space size={10}>
                            <span style={{ display: 'inline-flex', width: 18, justifyContent: 'center' }}>
                              <IconifyIcon icon={iconifyName} />
                            </span>
                            <Typography.Text ellipsis style={{ maxWidth: 120, fontSize: 12 }}>
                              {name}
                            </Typography.Text>
                          </Space>
                        </Button>
                      </Tooltip>
                    );
                  })
                : filteredIconify.slice(0, 1200).map((it) => {
                    const stored = `iconify:${it.icon}`;
                    return (
                      <Tooltip key={stored} title={stored}>
                        <Button
                          onClick={() => pick(stored)}
                          style={{ height: 44, display: 'flex', alignItems: 'center', justifyContent: 'flex-start' }}
                        >
                          <Space size={10}>
                            <span style={{ display: 'inline-flex', width: 18, justifyContent: 'center' }}>
                              <IconifyIcon icon={it.icon} />
                            </span>
                            <Typography.Text ellipsis style={{ maxWidth: 120, fontSize: 12 }}>
                              {it.label}
                            </Typography.Text>
                          </Space>
                        </Button>
                      </Tooltip>
                    );
                  })}
            </div>
          </div>

          <Space>
            <Button onClick={() => pick('')}>
              {t('pages.rbac.menus.iconPicker.actions.clearIcon', 'Clear icon')}
            </Button>
            <Button
              onClick={() => {
                Modal.info({
                  title: t('pages.rbac.menus.iconPicker.format.title', 'Icon field formats'),
                  content: (
                    <div>
                      <div>
                        {t('pages.rbac.menus.iconPicker.format.antd', 'Ant Design')}: <code>SafetyOutlined</code>
                      </div>
                      <div>
                        {t('pages.rbac.menus.iconPicker.format.iconify', 'Iconify')}: <code>iconify:lucide:user</code>
                      </div>
                      <div>
                        {t('pages.rbac.menus.iconPicker.format.iconfont', 'Iconfont')}: <code>icon-xxx</code>
                      </div>
                      <div>
                        {t('pages.rbac.menus.iconPicker.format.image', 'Image')}: <code>https://…/x.png</code> /{' '}
                        <code>/x.png</code>
                      </div>
                    </div>
                  ),
                });
              }}
            >
              {t('pages.rbac.menus.iconPicker.actions.formatHelp', 'Format help')}
            </Button>
          </Space>
        </Space>
      </Modal>
    </>
  );
};

export default IconPickerInput;


