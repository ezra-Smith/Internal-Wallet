declare module 'slash2';
declare module '*.css';
declare module '*.less';
declare module '*.scss';
declare module '*.sass';
declare module '*.svg';
declare module '*.png';
declare module '*.jpg';
declare module '*.jpeg';
declare module '*.gif';
declare module '*.bmp';
declare module '*.tiff';
declare module 'omit.js';
declare module 'numeral';
declare module 'mockjs';
declare module 'react-fittext';

declare module '@iconify/react' {
  import type React from 'react';

  export type IconProps = {
    icon: string;
    width?: string | number;
    height?: string | number;
    className?: string;
    style?: React.CSSProperties;
  };

  export const Icon: React.FC<IconProps>;
}

declare const REACT_APP_ENV: 'test' | 'dev' | 'pre' | false;
