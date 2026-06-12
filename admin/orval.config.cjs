/** @type {import('orval').ConfigExternal} */
const DEFAULT_ADMIN_SWAGGER_URL =
  "http://43.198.24.199:13001/swagger/admin.swagger.json";

module.exports = {
  admin: {
    input: {
      // Swagger v2 spec
      // - 默认使用远程 swagger（admin-web 依赖前置启动 swagger 服务）
      // - 如需覆盖（例如 Docker 构建/离线环境），设置：
      //   ORVAL_ADMIN_SWAGGER_TARGET=/abs/path/to/admin.swagger.json
      //   ORVAL_ADMIN_SWAGGER_TARGET=./admin.swagger.json
      target: process.env.ORVAL_ADMIN_SWAGGER_TARGET || DEFAULT_ADMIN_SWAGGER_URL,
      // NOTE: `validation` enables IBM/Spectral ruleset validation. Keep it off here to
      // avoid toolchain/node version surprises; Swagger parsing still happens.
      validation: false,
    },
    output: {
      // Keep generated code in a dedicated folder (do not hand-edit).
      mode: "tags",
      target: "./src/api/generated",
      schemas: "./src/api/generated/schemas",
      indexFiles: true,
      client: "axios-functions",
      clean: true,
      tsconfig: "./tsconfig.json",
      override: {
        // Reuse Umi Max request (auth + error handling + baseURL are already configured globally).
        mutator: {
          path: "./src/api/http/umiRequest.ts",
          name: "umiRequest",
        },
        useTypeOverInterfaces: true,
        useNamedParameters: true,
        enumGenerationType: "union",
      },
    },
  },
};
