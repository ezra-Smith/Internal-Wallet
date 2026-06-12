// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 获取当前管理员信息 GET /api/v1/admin/me */
export async function currentUser(options?: { [key: string]: any }) {
  return request<{
    success: boolean;
    message?: string;
    data?: {
      user_info?: API.AdminUserInfo;
      rbac?: API.GetMyRBACData;
    };
  }>('/api/v1/admin/me', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 退出登录接口 POST /api/v1/admin/auth/logout */
export async function outLogin(options?: { [key: string]: any }) {
  return request<API.LogoutResponse>('/api/v1/admin/auth/logout', {
    method: 'POST',
    ...(options || {}),
  });
}

// ==================== Admin MFA Login Flow ====================

export async function adminMFACaptchaGenerate(options?: { [key: string]: any }) {
  return request<API.AdminMFACaptchaGenerateResponse>(
    '/api/v1/admin/auth/captcha/generate',
    {
      method: 'POST',
      ...(options || {}),
    },
  );
}

export async function adminMFACaptchaValidate(
  body: API.AdminMFACaptchaValidateRequest,
  options?: { [key: string]: any },
) {
  return request<API.AdminMFACaptchaValidateResponse>(
    '/api/v1/admin/auth/captcha/validate',
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      data: body,
      ...(options || {}),
    },
  );
}

export async function adminMFALogin(
  body: API.AdminMFALoginRequest,
  options?: { [key: string]: any },
) {
  return request<API.AdminMFALoginResponse>('/api/v1/admin/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function adminMFAGetTwoFAQrCode(
  body: API.AdminMFAGetTwoFAQrCodeRequest,
  options?: { [key: string]: any },
) {
  return request<API.AdminMFAGetTwoFAQrCodeResponse>(
    '/api/v1/admin/auth/2fa/qrcode',
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      data: body,
      ...(options || {}),
    },
  );
}

export async function adminMFABindTwoFA(
  body: API.AdminMFABindTwoFARequest,
  options?: { [key: string]: any },
) {
  return request<API.AdminMFABindTwoFAResponse>('/api/v1/admin/auth/2fa/bind', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function adminMFAVerifyTwoFA(
  body: API.AdminMFAVerifyTwoFARequest,
  options?: { [key: string]: any },
) {
  return request<API.AdminMFAVerifyTwoFAResponse>(
    '/api/v1/admin/auth/2fa/verify',
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      data: body,
      ...(options || {}),
    },
  );
}

/** 此处后端没有提供注释 GET /api/notices */
export async function getNotices(options?: { [key: string]: any }) {
  return request<API.NoticeIconList>('/api/notices', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 获取规则列表 GET /api/rule */
export async function rule(
  params: {
    // query
    /** 当前的页码 */
    current?: number;
    /** 页面的容量 */
    pageSize?: number;
  },
  options?: { [key: string]: any },
) {
  return request<API.RuleList>('/api/rule', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 更新规则 PUT /api/rule */
export async function updateRule(options?: { [key: string]: any }) {
  return request<API.RuleListItem>('/api/rule', {
    method: 'POST',
    data: {
      method: 'update',
      ...(options || {}),
    },
  });
}

/** 新建规则 POST /api/rule */
export async function addRule(options?: { [key: string]: any }) {
  return request<API.RuleListItem>('/api/rule', {
    method: 'POST',
    data: {
      method: 'post',
      ...(options || {}),
    },
  });
}

/** 删除规则 DELETE /api/rule */
export async function removeRule(options?: { [key: string]: any }) {
  return request<Record<string, any>>('/api/rule', {
    method: 'POST',
    data: {
      method: 'delete',
      ...(options || {}),
    },
  });
}
