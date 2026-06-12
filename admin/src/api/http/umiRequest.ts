import { request } from '@umijs/max';
import type { RequestOptions } from '@@/plugin-request/request';

type OrvalRequestConfig = RequestOptions & { url: string };

function isPlainObject(value: unknown): value is Record<string, any> {
  return (
    typeof value === 'object' &&
    value !== null &&
    Object.prototype.toString.call(value) === '[object Object]'
  );
}

/**
 * Orval mutator:
 * - Routes requests through Umi Max `request`
 * - Reuses existing baseURL, auth header injection, and error handling
 *
 * The generated client calls this with an axios-like config: { url, method, params, data, headers, ... }.
 */
export function umiRequest<T>(config: OrvalRequestConfig, options?: RequestOptions) {
  const { url, headers: configHeaders, params: configParams, data: configData, ...restConfig } =
    config;

  const mergedHeaders = {
    ...(isPlainObject(configHeaders) ? configHeaders : {}),
    ...(isPlainObject(options?.headers) ? options.headers : {}),
  };

  const mergedParams =
    isPlainObject(configParams) && isPlainObject(options?.params)
      ? { ...configParams, ...options.params }
      : options?.params ?? configParams;

  const mergedData =
    isPlainObject(configData) && isPlainObject(options?.data)
      ? { ...configData, ...options.data }
      : options?.data ?? configData;

  return request<T>(url, {
    ...restConfig,
    ...(options || {}),
    ...(Object.keys(mergedHeaders).length ? { headers: mergedHeaders } : {}),
    ...(mergedParams !== undefined ? { params: mergedParams } : {}),
    ...(mergedData !== undefined ? { data: mergedData } : {}),
  });
}

