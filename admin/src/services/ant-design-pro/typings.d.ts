// @ts-ignore
/* eslint-disable */

declare namespace API {
  type CurrentUser = {
    name?: string;
    avatar?: string;
    userid?: string;
    email?: string;
    signature?: string;
    title?: string;
    group?: string;
    tags?: { key?: string; label?: string }[];
    notifyCount?: number;
    unreadCount?: number;
    country?: string;
    access?: string;
    geographic?: {
      province?: { label?: string; key?: string };
      city?: { label?: string; key?: string };
    };
    address?: string;
    phone?: string;

    // Internal Wallet admin fields (from admin.rpc)
    admin_id?: string;
    username?: string;
    role?: string;
    permissions?: string[];
    require_password_change?: boolean;
    rbac?: GetMyRBACData;
  };

  // ==================== Internal Wallet Admin API (subset) ====================

  type AdminUserInfo = {
    admin_id?: string;
    username?: string;
    name?: string;
    role?: string;
    permissions?: string[];
    require_password_change?: boolean;
  };

  type GetMyRBACData = {
    admin_id?: string;
    role?: string;
    user_permission_codes?: string[];
    user_menu_tree?: any[];
  };

  type LogoutResponse = {
    success?: boolean;
    message?: string;
    request_id?: string;
    timestamp?: string;
  };

  type AdminMFACaptchaGenerateResponse = {
    success?: boolean;
    captcha_image?: string;
    captcha_key?: string;
    message?: string;
    request_id?: string;
    timestamp?: string;
  };

  type AdminMFACaptchaValidateRequest = {
    captcha_key: string;
    captcha_answer: string;
  };

  type AdminMFACaptchaValidateResponse = {
    success?: boolean;
    temp_key?: string;
    message?: string;
    request_id?: string;
    timestamp?: string;
  };

  type AdminMFALoginRequest = {
    temp_key: string;
    email: string;
    password: string;
  };

  type AdminMFALoginResponse = {
    success?: boolean;
    status?: '2fa_not_bound' | '2fa_required' | 'login_successful' | string;
    auth_key?: string;
    session_identifier?: string;
    message?: string;
    request_id?: string;
    timestamp?: string;
    auth_token?: string;
    expires_in?: number;
    user_info?: AdminUserInfo;
  };

  type AdminMFAGetTwoFAQrCodeRequest = {
    auth_key: string;
  };

  type AdminMFAGetTwoFAQrCodeResponse = {
    success?: boolean;
    qr_code?: string;
    secret_key?: string;
    issuer?: string;
    account_name?: string;
    message?: string;
    request_id?: string;
    timestamp?: string;
  };

  type AdminMFABindTwoFARequest = {
    auth_key: string;
    totp_code: string;
    secret_key: string;
  };

  type AdminMFABindTwoFAResponse = {
    success?: boolean;
    status?: string;
    auth_token?: string;
    expires_in?: number;
    backup_codes?: string[];
    message?: string;
    request_id?: string;
    timestamp?: string;
  };

  type AdminMFAVerifyTwoFARequest = {
    session_identifier: string;
    totp_code: string;
  };

  type AdminMFAVerifyTwoFAResponse = {
    success?: boolean;
    status?: string;
    auth_token?: string;
    expires_in?: number;
    user_info?: AdminUserInfo;
    message?: string;
    request_id?: string;
    timestamp?: string;
  };

  type GetMeResponse = {
    success?: boolean;
    message?: string;
    data?: {
      user_info?: AdminUserInfo;
      rbac?: GetMyRBACData;
    };
    request_id?: string;
    timestamp?: string;
  };

  // ==================== Ant Design Pro template types ====================

  type LoginResult = {
    status?: string;
    type?: string;
    currentAuthority?: string;
  };

  type PageParams = {
    current?: number;
    pageSize?: number;
  };

  type RuleListItem = {
    key?: number;
    disabled?: boolean;
    href?: string;
    avatar?: string;
    name?: string;
    owner?: string;
    desc?: string;
    callNo?: number;
    status?: number;
    updatedAt?: string;
    createdAt?: string;
    progress?: number;
  };

  type RuleList = {
    data?: RuleListItem[];
    /** 列表的内容总数 */
    total?: number;
    success?: boolean;
  };

  type FakeCaptcha = {
    code?: number;
    status?: string;
  };

  type LoginParams = {
    username?: string;
    password?: string;
    autoLogin?: boolean;
    type?: string;
  };

  type ErrorResponse = {
    /** 业务约定的错误码 */
    errorCode: string;
    /** 业务上的错误信息 */
    errorMessage?: string;
    /** 业务上的请求是否成功 */
    success?: boolean;
  };

  type NoticeIconList = {
    data?: NoticeIconItem[];
    /** 列表的内容总数 */
    total?: number;
    success?: boolean;
  };

  type NoticeIconItemType = 'notification' | 'message' | 'event';

  type NoticeIconItem = {
    id?: string;
    extra?: string;
    key?: string;
    read?: boolean;
    avatar?: string;
    title?: string;
    status?: string;
    datetime?: string;
    description?: string;
    type?: NoticeIconItemType;
  };
}
