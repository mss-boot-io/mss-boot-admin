export interface UserSnapshot {
  id: string;
  user_id: string;
  email: string;
  user_role: string;
  models: string;
  max_budget: number | null;
  budget_duration: string | null;
  budget_reset_at: string | null;
  spend: number;
  synced_at: string;
}

export interface KeySnapshot {
  id: string;
  key_hash_prefix: string;
  alias: string | null;
  user_id: string;
  user_email: string;
  max_budget: number | null;
  spend: number;
  tpm_limit: number | null;
  rpm_limit: number | null;
  max_parallel_requests: number | null;
  expires: string | null;
  is_session_key: boolean;
  synced_at: string;
}

export interface Page<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface UserDetail {
  user: UserSnapshot;
  keys: KeySnapshot[];
}

export interface SyncReport {
  users: number;
  keys: number;
  users_retired: number;
  keys_retired: number;
  synced_at: string;
}

export interface SpendLog {
  request_id: string;
  model: string;
  status: string;
  spend: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  start_time: string;
}

export interface BillsPage {
  items: SpendLog[];
  total: number;
  spend_sum: number;
  token_sum: number;
  page: number;
  page_size: number;
}

export interface UserPageParams {
  page: number;
  page_size: number;
  email?: string;
  user_role?: string;
}

export interface KeyPageParams {
  page: number;
  page_size: number;
  alias?: string;
  user_email?: string;
  is_session_key?: string;
}

export interface BillsParams {
  page: number;
  page_size: number;
  model?: string;
  status?: string;
  key_hash_prefix?: string;
  since?: string;
  until?: string;
}

export function parseModels(models: string): string[] {
  try {
    const parsed = JSON.parse(models);
    return Array.isArray(parsed) ? parsed.filter((m) => typeof m === 'string') : [];
  } catch {
    return [];
  }
}

export function formatUsd(value: number | null | undefined, digits = 4): string {
  if (value === null || value === undefined) return '—';
  return `$${value.toFixed(digits)}`;
}

export function formatInt(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—';
  return new Intl.NumberFormat('en-US').format(value);
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}
