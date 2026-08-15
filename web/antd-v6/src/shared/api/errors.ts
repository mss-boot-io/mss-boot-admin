export interface ApiErrorBody {
  code?: number | string;
  error?: string;
  errorMessage?: string;
  message?: string;
  msg?: string;
}

export interface ApiResponseLike {
  status?: number;
  data?: ApiErrorBody;
}

export interface ApiRequestFailure extends Error {
  response?: ApiResponseLike;
  request?: unknown;
}

export function getRequestStatus(error: unknown): number | undefined {
  return (error as ApiRequestFailure | undefined)?.response?.status;
}

export function getRequestErrorMessage(error: unknown): string {
  const failure = error as ApiRequestFailure;
  const body = failure.response?.data;
  return (
    body?.errorMessage ??
    body?.message ??
    body?.msg ??
    body?.error ??
    failure.message ??
    'Request failed'
  );
}
