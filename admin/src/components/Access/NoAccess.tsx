import { Result } from 'antd';
import React from 'react';

type NoAccessProps = {
  title?: string;
  subTitle?: string;
};

const NoAccess: React.FC<NoAccessProps> = ({
  title = '403',
  subTitle = '无权限访问',
}) => {
  return <Result status="403" title={title} subTitle={subTitle} />;
};

export default NoAccess;

