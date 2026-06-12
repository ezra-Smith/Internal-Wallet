// @ts-ignore
import { TestBrowser } from '@@/testBrowser';
import { fireEvent, render } from '@testing-library/react';
import { act } from 'react';
import * as React from 'react';

import { adminAdminMFACaptchaGenerate, adminAdminMFACaptchaValidate, adminAdminMFALogin } from '@/api/generated/auth';
import { adminGetMe } from '@/api/generated/settings';

jest.mock('@/api/generated/auth', () => ({
  adminAdminMFABindTwoFA: jest.fn(),
  adminAdminMFACaptchaGenerate: jest.fn(),
  adminAdminMFACaptchaValidate: jest.fn(),
  adminAdminMFAGetTwoFAQrCode: jest.fn(),
  adminAdminMFALogin: jest.fn(),
  adminAdminMFAVerifyTwoFA: jest.fn(),
}));

jest.mock('@/api/generated/settings', () => ({
  adminGetMe: jest.fn(),
}));

const waitTime = (time: number = 100) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve(true);
    }, time);
  });
};

describe('Login Page', () => {
  beforeEach(() => {
    (adminAdminMFACaptchaGenerate as jest.Mock).mockResolvedValue({
      success: true,
      captcha_key: 'mock-captcha-key',
      captcha_image:
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+Zq6cAAAAASUVORK5CYII=',
    });

    (adminAdminMFACaptchaValidate as jest.Mock).mockResolvedValue({
      success: true,
      temp_key: 'mock-temp-key',
    });

    (adminAdminMFALogin as jest.Mock).mockResolvedValue({
      success: true,
      status: 'login_successful',
      auth_token: 'mock-auth-token',
      expires_in: '3600',
    });

    (adminGetMe as jest.Mock).mockResolvedValue({
      success: true,
      data: {
        user_info: {
          admin_id: 'mock-admin-id',
          username: 'mock-admin',
          name: 'Mock Admin',
          role: 'admin',
          permissions: ['admin'],
        },
        rbac: {
          admin_id: 'mock-admin-id',
          role: 'admin',
          user_permission_codes: ['admin'],
          user_menu_tree: [],
        },
      },
    });
  });

  it('should show login form', async () => {
    const historyRef = React.createRef<any>();
    const rootContainer = render(
      <TestBrowser
        historyRef={historyRef}
        location={{
          pathname: '/user/login',
        }}
      />,
    );

    await rootContainer.findAllByText('Internal Wallet');

    act(() => {
      historyRef.current?.push('/user/login');
    });

    expect(
      rootContainer.baseElement?.querySelector('.ant-pro-form-login-desc')
        ?.textContent,
    ).toBe(
      '安全可信的资金运营控制台',
    );

    expect(rootContainer.asFragment()).toMatchSnapshot();

    rootContainer.unmount();
  });

  it('should login success', async () => {
    const historyRef = React.createRef<any>();
    const rootContainer = render(
      <TestBrowser
        historyRef={historyRef}
        location={{
          pathname: '/user/login',
        }}
      />,
    );

    await rootContainer.findAllByText('Internal Wallet');

    const emailInput = await rootContainer.findByPlaceholderText('管理员邮箱');

    act(() => {
      fireEvent.change(emailInput, { target: { value: 'admin@example.com' } });
    });

    const passwordInput = await rootContainer.findByPlaceholderText(
      '密码',
    );

    act(() => {
      fireEvent.change(passwordInput, { target: { value: 'password' } });
    });

    const captchaInput = await rootContainer.findByPlaceholderText(/请输入验证码/);

    act(() => {
      fireEvent.change(captchaInput, { target: { value: '1234' } });
    });

    const submitButton = rootContainer.baseElement?.querySelector('button.ant-btn-primary');
    expect(submitButton).toBeTruthy();
    act(() => {
      (submitButton as HTMLButtonElement).click();
    });

    // 等待接口返回结果
    await waitTime(500);

    // Default post-login landing is now RBAC roles.
    await rootContainer.findAllByText(/Create Role|创建角色/);

    expect(rootContainer.asFragment()).toMatchSnapshot();

    await waitTime(200);

    rootContainer.unmount();
  });
});
