/**
 * @name umi 的路由配置
 * @description 只支持 path,component,routes,redirect,wrappers,name,icon 的配置
 * @param path  path 只支持两种占位符配置，第一种是动态参数 :id 的形式，第二种是 * 通配符，通配符只能出现路由字符串的最后。
 * @param component 配置 location 和 path 匹配后用于渲染的 React 组件路径。可以是绝对路径，也可以是相对路径，如果是相对路径，会从 src/pages 开始找起。
 * @param routes 配置子路由，通常在需要为多个路径增加 layout 组件时使用。
 * @param redirect 配置路由跳转
 * @param wrappers 配置路由组件的包装组件，通过包装组件可以为当前的路由组件组合进更多的功能。 比如，可以用于路由级别的权限校验
 * @param name 配置路由的标题，默认读取国际化文件 menu.ts 中 menu.xxxx 的值，如配置 name 为 login，则读取 menu.ts 中 menu.login 的取值作为标题
 * @param icon 配置路由的图标，取值参考 https://ant.design/components/icon-cn， 注意去除风格后缀和大小写，如想要配置图标为 <StepBackwardOutlined /> 则取值应为 stepBackward 或 StepBackward，如想要配置图标为 <UserOutlined /> 则取值应为 user 或者 User
 * @doc https://umijs.org/docs/guides/routes
 */
export default [
  {
    path: "/user",
    layout: false,
    routes: [
      {
        name: "login",
        path: "/user/login",
        component: "./user/login",
      },
    ],
  },
  {
    path: "/dashboard",
    name: "dashboard",
    icon: "dashboard",
    component: "./dashboard",
  },
  {
    path: "/transfers",
    name: "transfers",
    icon: "send",
    routes: [
      {
        path: "/transfers",
        redirect: "/transfers/batches",
      },
      {
        path: "/transfers/batches",
        name: "batches",
        component: "./transfers/batches",
      },
      {
        path: "/transfers/batches/:batchId",
        name: "batch-detail",
        component: "./transfers/batches/detail",
        hideInMenu: true,
      },
    ],
  },
  {
    path: "/assets",
    name: "assets",
    icon: "wallet",
    routes: [
      {
        path: "/assets",
        redirect: "/assets/deposit-addresses",
      },
      {
        path: "/assets/deposit-addresses",
        name: "deposit-addresses",
        component: "./assets/deposit-addresses",
      },
      {
        path: "/assets/currencies",
        name: "currencies",
        component: "./assets/currencies",
      },
    ],
  },
  {
    path: "/accounting",
    name: "accounting",
    icon: "accountBook",
    routes: [
      {
        path: "/accounting",
        redirect: "/accounting/account-types",
      },
      {
        path: "/accounting/account-types",
        name: "account-types",
        component: "./accounting/account-types",
      },
      {
        path: "/accounting/user-ledger",
        name: "user-ledger",
        component: "./accounting/user-ledger",
      },
      {
        path: "/accounting/system-setup",
        name: "system-setup",
        component: "./accounting/system-setup",
      },
      {
        path: "/accounting/user-flow",
        name: "user-flow",
        component: "./accounting/user-flow",
      },
      {
        path: "/accounting/ledger",
        name: "ledger",
        component: "./accounting/ledger",
      },
    ],
  },
  {
    path: "/deposits",
    name: "deposits",
    icon: "download",
    component: "./deposits",
  },

  {
    path: "/withdrawals",
    name: "withdrawals",
    icon: "audit",
    component: "./withdrawals",
  },
  {
    path: "/internal-transfers",
    name: "internal-transfers",
    icon: "swap",
    component: "./internal-transfers",
  },
  {
    path: "/users",
    name: "users",
    icon: "team",
    component: "./users",
  },
  {
    path: "/web3-users",
    name: "web3-users",
    icon: "cloud",
    routes: [
      {
        path: "/web3-users",
        redirect: "/web3-users/list",
      },
      {
        path: "/web3-users/list",
        name: "list",
        component: "./web3-users",
      },
    ],
  },
  {
    path: "/web3-wallets",
    name: "web3-wallets",
    icon: "wallet",
    component: "./web3-wallets",
  },
  {
    path: "/vault",
    name: "vault",
    icon: "bank",
    routes: [
      {
        path: "/vault",
        redirect: "/vault/funds",
      },
      {
        path: "/vault/funds",
        name: "funds",
        component: "./vault/funds",
      },
      {
        path: "/vault/addresses",
        name: "addresses",
        component: "./vault/addresses",
      },
      {
        path: "/vault/chains/:chainId",
        name: "chain",
        component: "./vault/chains/detail",
        hideInMenu: true,
      },
      {
        path: "/vault/sub-addresses",
        name: "sub-addresses",
        component: "./vault/sub-addresses",
        hideInMenu: true,
      },
    ],
  },
  {
    path: "/fund-flow",
    name: "fund-flow",
    icon: "transaction",
    component: "./fund-flow",
  },
  // {
  //   path: '/blacklist',
  //   name: 'blacklist',
  //   icon: 'stop',
  //   component: './blacklist',
  // },
  {
    path: "/swap",
    name: "swap",
    icon: "swap",
    routes: [
      {
        path: "/swap",
        redirect: "/swap/config",
      },
      {
        path: "/swap/config",
        name: "config",
        component: "./swap/config",
      },
      {
        path: "/swap/providers",
        name: "providers",
        component: "./swap/providers",
      },
      {
        path: "/swap/pairs",
        name: "pairs",
        component: "./swap/pairs",
      },
      {
        path: "/swap/tokens",
        name: "tokens",
        component: "./swap/tokens",
      },
      {
        path: "/swap/projects",
        name: "projects",
        component: "./swap/projects",
      },
    ],
  },
  {
    path: "/audit",
    name: "audit",
    icon: "fileSearch",
    routes: [
      {
        path: "/audit",
        redirect: "/audit/logs",
      },
      {
        path: "/audit/logs",
        name: "logs",
        component: "./audit/logs",
      },
    ],
  },
  {
    path: "/settings",
    name: "settings",
    icon: "setting",
    routes: [
      {
        path: "/settings",
        redirect: "/settings/security",
      },
      {
        path: "/settings/security",
        name: "security",
        component: "./settings/security",
      },
    ],
  },
  {
    path: "/alert",
    name: "alert",
    icon: "alert",
    component: "./alert",
  },
  {
    path: "/rbac",
    name: "rbac",
    icon: "safety",
    routes: [
      {
        path: "/rbac",
        redirect: "/rbac/roles",
      },
      {
        path: "/rbac/roles",
        name: "roles",
        component: "./rbac/roles",
      },
      {
        path: "/rbac/permissions",
        name: "permissions",
        component: "./rbac/permissions",
      },
      {
        path: "/rbac/menus",
        name: "menus",
        component: "./rbac/menus",
      },
      {
        path: "/rbac/admins",
        name: "admins",
        component: "./rbac/admins",
      },
    ],
  },
  {
    path: "/",
    redirect: "/dashboard",
  },
  {
    path: "*",
    layout: false,
    component: "./404",
  },
];
