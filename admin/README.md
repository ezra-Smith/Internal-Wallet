# Ant Design Pro

This project is initialized with [Ant Design Pro](https://pro.ant.design). Follow is the quick guide for how to use.

## Environment Prepare

This repo enforces `bun` as the package manager.

Install deps:

```bash
bun install
```

## Provided Scripts

Ant Design Pro provides some useful script to help you quick start and build with web project, code style check and test.

Scripts provided in `package.json`. It's safe to modify or add additional script:

### Start project

```bash
bun run start:dev
```

### Build project

```bash
bun run build
```

### Check code style

```bash
bun run lint
```

### Test code

```bash
bun run test
```

## RBAC + Dynamic Menu

- Post-login navigation can be rendered from RBAC menu data (`/api/v1/admin/me` → `rbac.user_menu_tree`).
- RBAC management pages are under `/rbac/*` (roles / permissions / menus / admins).
- Canonical permission codes for RBAC follow `<domain>.<resource>.<action>` (e.g. `rbac.role.create`).
- The admin UI fetches `/api/v1/admin/rbac/permission-map` to avoid hard-coding permission strings per page.

## More

You can view full document on our [official website](https://pro.ant.design). And welcome any feedback in our [github](https://github.com/ant-design/ant-design-pro).
