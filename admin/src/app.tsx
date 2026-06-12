import { adminGetMyRBAC } from "@/api/generated/rbac";
import { adminGetMe } from "@/api/generated/settings";
import { AvatarDropdown, AvatarName, Footer, SelectLang } from "@/components";
import PermissionBootstrap from "@/components/Rbac/PermissionBootstrap";
import { clearAdminToken } from "@/utils/auth";
import { buildMenuDataFromApi } from "@/utils/menu";
import { normalizePermissionCodes } from "@/utils/permission";
import type { Settings as LayoutSettings } from "@ant-design/pro-components";
import { SettingDrawer } from "@ant-design/pro-components";
import "@ant-design/v5-patch-for-react-19";
import type { RequestConfig, RunTimeLayoutConfig } from "@umijs/max";
import { history } from "@umijs/max";
import defaultSettings from "../config/defaultSettings";
import { errorConfig } from "./requestErrorConfig";

const isDev = process.env.NODE_ENV === "development";
const loginPath = "/user/login";

/**
 * @see https://umijs.org/docs/api/runtime-config#getinitialstate
 * */
export async function getInitialState(): Promise<{
  settings?: Partial<LayoutSettings>;
  currentUser?: API.CurrentUser;
  loading?: boolean;
  rbacPermissionMap?: Record<string, string>;
  rbacPermissionMapVersion?: string;
  fetchUserInfo?: () => Promise<API.CurrentUser | undefined>;
}> {
  const fetchUserInfo = async () => {
    try {
      const msg = await adminGetMe({
        skipErrorHandler: true,
      });
      const me = msg?.data;
      if (!msg?.success || !me?.user_info) {
        throw new Error(msg?.message || "Failed to fetch user info");
      }
      let rbac = me.rbac;
      // Only backfill when RBAC payload is missing (empty arrays can be valid).
      const needsRbacBackfill =
        !rbac ||
        !Array.isArray(rbac.user_permission_codes) ||
        !Array.isArray(rbac.user_menu_tree);
      if (needsRbacBackfill) {
        try {
          const rbacRes = await adminGetMyRBAC({ skipErrorHandler: true });
          const rbacData = rbacRes?.data;
          if (rbacRes?.success && rbacData) {
            rbac = rbacData;
          }
        } catch {
          // Ignore backfill errors; keep `adminGetMe` payload.
        }
      }
      return {
        ...me.user_info,
        rbac,
      } as API.CurrentUser;
    } catch (_error) {
      clearAdminToken();
      history.push(loginPath);
    }
    return undefined;
  };
  // 如果不是登录页面，执行
  const { location } = history;
  if (
    ![loginPath, "/user/register", "/user/register-result"].includes(
      location.pathname
    )
  ) {
    const currentUser = await fetchUserInfo();
    return {
      fetchUserInfo,
      currentUser,
      settings: defaultSettings as Partial<LayoutSettings>,
    };
  }
  return {
    fetchUserInfo,
    settings: defaultSettings as Partial<LayoutSettings>,
  };
}

// ProLayout 支持的api https://procomponents.ant.design/components/layout
export const layout: RunTimeLayoutConfig = ({
  initialState,
  setInitialState,
}) => {
  const role = (initialState?.currentUser?.role || "").trim().toLowerCase();
  const permissionCodes = normalizePermissionCodes([
    ...(initialState?.currentUser?.permissions ?? []),
    ...(initialState?.currentUser?.rbac?.user_permission_codes ?? []),
  ]);
  const isSuperAdmin =
    role === "admin" || role === "super_admin" || permissionCodes.includes("*");

  return {
    actionsRender: () => [
      // <Question key="doc" />,
      <SelectLang key="SelectLang" />,
    ],
    avatarProps: {
      src: initialState?.currentUser?.avatar,
      title: <AvatarName />,
      render: (_, avatarChildren) => {
        return <AvatarDropdown>{avatarChildren}</AvatarDropdown>;
      },
    },
    waterMarkProps: {
      content: initialState?.currentUser?.name,
    },
    footerRender: () => <Footer />,
    onPageChange: () => {
      const { location } = history;
      // 如果没有登录，重定向到 login
      if (!initialState?.currentUser && location.pathname !== loginPath) {
        history.push(loginPath);
      }
    },
    bgLayoutImgList: [
      {
        src: "https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/D2LWSqNny4sAAAAAAAAAAAAAFl94AQBr",
        left: 85,
        bottom: 100,
        height: "303px",
      },
      {
        src: "https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/C2TWRpJpiC0AAAAAAAAAAAAAFl94AQBr",
        bottom: -68,
        right: -45,
        height: "303px",
      },
      {
        src: "https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/F6vSTbj8KpYAAAAAAAAAAAAAFl94AQBr",
        bottom: 0,
        left: 0,
        width: "331px",
      },
    ],
    links: isDev
      ? [
          // <Link key="openapi" to="/umi/plugin/openapi" target="_blank">
          //   <LinkOutlined />
          //   <span>OpenAPI 文档</span>
          // </Link>,
        ]
      : [],
    menuHeaderRender: undefined,
    menuDataRender: (menuData) => {
      const menuTree = initialState?.currentUser?.rbac?.user_menu_tree as any;
      const apiMenu = buildMenuDataFromApi(menuTree, {
        permissionCodes,
        isSuperAdmin,
        rpcMethodPermissions: initialState?.rbacPermissionMap,
      });
      if (!apiMenu.length) return menuData;

      // Bootstrap: ensure super admin can always reach RBAC management pages,
      // even if backend menus were not seeded yet.
      if (isSuperAdmin) {
        const hasRbac = apiMenu.some((m) => m.path === "/rbac");
        if (!hasRbac) {
          const rbacItem = (menuData || []).find((m) => m?.path === "/rbac");
          if (rbacItem) return [...apiMenu, rbacItem];
        }
      }

      return apiMenu;
    },
    // 自定义 403 页面
    // unAccessible: <div>unAccessible</div>,
    // 增加一个 loading 的状态
    childrenRender: (children) => {
      // if (initialState?.loading) return <PageLoading />;
      return (
        <>
          {children}
          <PermissionBootstrap />
          {isDev && (
            <SettingDrawer
              disableUrlParams
              enableDarkTheme
              settings={initialState?.settings}
              onSettingChange={(settings) => {
                setInitialState((preInitialState) => ({
                  ...preInitialState,
                  settings,
                }));
              }}
            />
          )}
        </>
      );
    },
    ...initialState?.settings,
  };
};

/**
 * @name request 配置，可以配置错误处理
 * 它基于 axios 和 ahooks 的 useRequest 提供了一套统一的网络请求和错误处理方案。
 * @doc https://umijs.org/docs/max/request#配置
 */
export const request: RequestConfig = {
  // Use same-origin by default:
  // - local dev server proxies to api-gateway via `config/proxy.ts`
  // - docker compose serves the SPA behind nginx, which proxies `/api/*` to api-gateway
  baseURL: process.env.REACT_APP_API_BASE || "",
  ...errorConfig,
};
