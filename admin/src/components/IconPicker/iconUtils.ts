import { Icon as IconifyIcon } from '@iconify/react';
import React from 'react';

export function isUrl(value: string): boolean {
  return /^(https?:)?\/\//i.test(value) || /^mailto:/i.test(value) || /^tel:/i.test(value);
}

export function isDataUri(value: string): boolean {
  return /^data:/i.test(value);
}

export function isImagePath(value: string): boolean {
  return /\.(png|jpe?g|gif|svg|webp|ico)(\?.*)?$/i.test(value);
}

export function parseIconifyValue(value: string): string | null {
  const raw = value.trim();
  if (!raw.toLowerCase().startsWith('iconify:')) return null;
  const rest = raw.slice('iconify:'.length).trim();
  const [collection, name, ...extra] = rest.split(':').filter(Boolean);
  if (!collection || !name || extra.length) return null;
  return `${collection}:${name}`;
}

function toIconifyAntDesign(name: string): string {
  const kebabName = name
    .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
    .toLowerCase();
  return `ant-design:${kebabName}`;
}

function isAntDesignIconName(value: string): boolean {
  return /^[A-Z][a-zA-Z0-9]*(Outlined|Filled|TwoTone)$/.test(value);
}

/**
 * Lightweight preview for a stored icon string.
 * - Ant icons: uses Iconify ant-design collection
 * - iconify: uses Iconify <Icon/>
 * - urls/images: renders <img/>
 */
export function renderIconValuePreview(value?: string): React.ReactNode {
  if (!value) return null;
  const raw = value.trim();
  if (!raw) return null;

  // iconify:collection:name 格式
  const iconify = parseIconifyValue(raw);
  if (iconify) return React.createElement(IconifyIcon, { icon: iconify });

  // Ant Design 图标名称 (如 FireFilled, HomeOutlined)
  if (isAntDesignIconName(raw)) {
    const iconifyName = toIconifyAntDesign(raw);
    return React.createElement(IconifyIcon, { icon: iconifyName });
  }

  // URL 或图片路径
  if (isUrl(raw) || isDataUri(raw) || isImagePath(raw) || (raw.startsWith('/') && isImagePath(raw))) {
    return React.createElement('img', {
      alt: 'icon',
      src: raw,
      style: { width: 16, height: 16, objectFit: 'contain' },
    });
  }

  return null;
}


