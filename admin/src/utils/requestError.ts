function isPlainObject(value: unknown): value is Record<string, any> {
  return (
    typeof value === 'object' &&
    value !== null &&
    Object.prototype.toString.call(value) === '[object Object]'
  );
}

function messageFromPayload(payload: unknown): string | undefined {
  if (typeof payload === 'string') return payload;
  if (!isPlainObject(payload)) return undefined;

  const msg =
    payload.message ??
    payload.errorMessage ??
    payload.error ??
    payload.msg;

  return typeof msg === 'string' && msg.trim() ? msg : undefined;
}

export function getServerErrorMessage(error: unknown): string | undefined {
  if (!isPlainObject(error)) return undefined;

  const infoMessage = messageFromPayload(error.info);
  if (infoMessage) return infoMessage;

  const dataMessage = messageFromPayload(error.data);
  if (dataMessage) return dataMessage;

  const responseMessage = messageFromPayload(error.response?.data);
  if (responseMessage) return responseMessage;

  const infoDataMessage = messageFromPayload(error.info?.data);
  if (infoDataMessage) return infoDataMessage;

  return undefined;
}

export function getRequestErrorMessage(error: unknown): string | undefined {
  const serverMessage = getServerErrorMessage(error);
  if (serverMessage) return serverMessage;

  if (isPlainObject(error) && typeof error.message === 'string' && error.message.trim()) {
    return error.message;
  }

  return undefined;
}

export function getServerFieldViolations(error: unknown): Array<{ field: string; description: string }> {
  if (!isPlainObject(error)) return [];

  const payload = (error.data ?? error.response?.data) as unknown;
  if (!isPlainObject(payload)) return [];

  const details = payload.details as unknown;
  if (!isPlainObject(details)) return [];

  const violations = details.field_violations as unknown;
  if (!Array.isArray(violations)) return [];

  return violations
    .filter((v): v is Record<string, any> => isPlainObject(v))
    .map((v) => ({
      field: typeof v.field === 'string' ? v.field : '',
      description: typeof v.description === 'string' ? v.description : '',
    }))
    .filter((v) => v.field && v.description);
}
