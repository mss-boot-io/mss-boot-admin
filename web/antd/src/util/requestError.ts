export const getResponseErrorMessage = (response?: {
  status?: number;
  statusText?: string;
  data?: unknown;
}) => {
  const fallback = response?.status ? `HTTP ${response.status}` : 'Request failed';
  const statusText = response?.statusText || fallback;
  const data = response?.data;
  let detail = '';

  if (typeof data === 'string') {
    detail = data;
  } else if (data && typeof data === 'object') {
    const body = data as Record<string, unknown>;
    const value =
      body.msg || body.errorMessage || body.message || body.error || body.detail || body.reason;
    if (typeof value === 'string') {
      detail = value;
    } else if (typeof value === 'number') {
      detail = String(value);
    }
  }

  return detail ? `${statusText}: ${detail}` : statusText;
};
