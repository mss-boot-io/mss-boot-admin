export interface ApiErrorBody {
  code?: number | string;
  error?: string;
  errorCode?: string;
  errorMessage?: string;
  message?: string;
  msg?: string;
}

export interface ApiResponseLike {
  status?: number;
  data?: ApiErrorBody;
}

export interface ApiRequestFailure extends Error {
  data?: ApiErrorBody;
  response?: ApiResponseLike;
  request?: unknown;
}

function getRequestErrorBody(error: unknown): ApiErrorBody | undefined {
  const failure = error as ApiRequestFailure | undefined;
  return failure?.data ?? failure?.response?.data;
}

export function getRequestStatus(error: unknown): number | undefined {
  return (error as ApiRequestFailure | undefined)?.response?.status;
}

export function getRequestErrorCode(error: unknown): string | undefined {
  const value = getRequestErrorBody(error)?.errorCode;
  return typeof value === 'string' && value.trim() ? value : undefined;
}

export function getRequestErrorMessage(error: unknown): string {
  const failure = error as ApiRequestFailure;
  const body = getRequestErrorBody(error);
  return (
    body?.errorMessage ??
    body?.message ??
    body?.msg ??
    body?.error ??
    failure.message ??
    'Request failed'
  );
}
