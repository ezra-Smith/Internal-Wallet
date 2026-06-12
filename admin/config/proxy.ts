/**
 * @name 代理的配置
 * @see 在生产环境 代理是无法生效的，所以这里没有生产环境的配置
 * -------------------------------
 * The agent cannot take effect in the production environment
 * so there is no configuration of the production environment
 * For details, please see
 * https://pro.ant.design/docs/deploy
 *
 * @doc https://umijs.org/docs/guides/proxy
 */
const gatewayTarget = process.env.INTERNAL_WALLET_GATEWAY_URL || 'http://43.198.24.199:13001';

export default {
  /**
   * Local dev:
   * - Run backend via `deploy/admin/docker-compose.yml` (api-gateway host port: 13001).
   * - Or run backend locally (default port 8080) and set:
   *     INTERNAL_WALLET_GATEWAY_URL=http://localhost:8080
   * - Run this frontend via `npm run start:dev` (UMI_ENV=dev).
   *
   * Keep browser requests same-origin (to the dev server), proxying API calls to the gateway.
   */
  dev: {
    '/api/': {
      target: gatewayTarget,
      // target: 'http://43.198.24.199:13001',
      changeOrigin: true,
    },
    '/docs/': {
      target: gatewayTarget,
      changeOrigin: true,
    },
    '/swagger/': {
      target: gatewayTarget,
      changeOrigin: true,
    },
  },
  /**
   * @name 详细的代理配置
   * @doc https://github.com/chimurai/http-proxy-middleware
   */
  test: {
    // localhost:8000/api/** -> https://preview.pro.ant.design/api/**
    '/api/': {
      target: 'https://proapi.azurewebsites.net',
      changeOrigin: true,
      pathRewrite: { '^': '' },
    },
  },
  pre: {
    '/api/': {
      target: 'your pre url',
      changeOrigin: true,
      pathRewrite: { '^': '' },
    },
  },
};
