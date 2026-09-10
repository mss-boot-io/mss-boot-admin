export interface OpsPage<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface OpsListParams {
  page: number;
  page_size: number;
  query?: string;
}

export type GatewayReadiness = 'ready' | 'degraded' | 'unavailable';

export interface GatewayHealth {
  ready: boolean;
  status: GatewayReadiness;
  version?: string;
  checked_at: string;
  message?: string;
  model_count?: number;
  healthy_model_count?: number;
  blocked_model_count?: number;
}

export interface GatewayModel {
  id: string;
  name: string;
  provider?: string;
  mode?: string;
  healthy?: boolean;
  blocked: boolean;
  endpoint_count?: number;
  last_checked_at?: string;
  message?: string;
}

export interface ManagedUser {
  id: string;
  user_id: string;
  email: string;
  user_role: string;
  models: string[] | string;
  max_budget: number | null;
  budget_duration: string | null;
  budget_reset_at: string | null;
  spend: number;
  tpm_limit: number | null;
  rpm_limit: number | null;
  blocked: boolean;
  synced_at: string;
}

export interface ManagedUserDetail {
  user: ManagedUser;
  keys: ManagedKey[];
}

export interface ManagedUserInput {
  email: string;
  user_role: string;
  models: string[];
  max_budget?: number;
  budget_duration?: string;
  tpm_limit?: number;
  rpm_limit?: number;
  blocked?: boolean;
}

export interface ManagedUserPatch {
  user_role?: string;
  models?: string[];
  max_budget?: number;
  budget_duration?: string | null;
  tpm_limit?: number | null;
  rpm_limit?: number | null;
  blocked?: boolean;
}

export interface ManagedKey {
  id: string;
  key_hash_prefix: string;
  alias: string | null;
  user_id: string;
  user_email: string;
  models?: string[] | string;
  max_budget: number | null;
  spend: number;
  tpm_limit: number | null;
  rpm_limit: number | null;
  max_parallel_requests: number | null;
  expires: string | null;
  blocked: boolean;
  is_session_key: boolean;
  synced_at: string;
}

export interface ManagedKeyInput {
  alias: string;
  user_id?: string;
  user_email?: string;
  models: string[];
  max_budget?: number;
  tpm_limit?: number;
  rpm_limit?: number;
  max_parallel_requests?: number;
  expires?: string;
}

export interface ManagedKeyPatch {
  alias?: string;
  models?: string[];
  max_budget?: number | null;
  tpm_limit?: number | null;
  rpm_limit?: number | null;
  max_parallel_requests?: number | null;
  expires?: string | null;
}

export interface OneTimeKeyResult {
  key?: ManagedKey | string;
  raw_key?: string;
  token?: string;
  record?: ManagedKey;
}

export type RechargeStatus =
  | 'received'
  | 'approved'
  | 'executing'
  | 'applied_unverified'
  | 'completed'
  | 'retryable_failed'
  | 'terminal_failed'
  | 'reconcile_required';

export interface RechargeRecord {
  id: string;
  created_at: string;
  user_id: string;
  email: string;
  amount: number;
  before_budget: number;
  after_budget: number;
  raise_keys: boolean;
  keys_updated: string;
  operator: string;
  reason: string;
  business_reference?: string;
  idempotency_key?: string;
  status: RechargeStatus | string;
}

export interface RechargeRequest {
  amount: number;
  reason: string;
  raise_keys: boolean;
  business_reference: string;
  idempotency_key: string;
}

export interface SalesProduct {
  id: string;
  channel: string;
  shop: string;
  external_item_id: string;
  sku: string;
  title: string;
  price_cny_fen: number;
  credit_usd_micro: number;
  enabled: boolean;
  auto_apply: boolean;
  created_at: string;
  updated_at: string;
}

export interface SalesProductInput {
  channel: string;
  shop: string;
  external_item_id: string;
  sku: string;
  title: string;
  price_cny_fen: number;
  credit_usd_micro: number;
  enabled: boolean;
  auto_apply: boolean;
}

export type SalesOrderStatus =
  | 'received'
  | 'verified_paid'
  | 'mapped'
  | 'approved'
  | 'executing'
  | 'applied_unverified'
  | 'completed'
  | 'retryable_failed'
  | 'terminal_failed'
  | 'reconcile_required'
  | 'refund_review'
  | 'reversed';

export type SalesPaymentStatus = 'paid' | 'unpaid' | 'refunded' | 'cancelled';

export interface SalesOrder {
  id: string;
  channel: string;
  shop: string;
  external_order_id: string;
  adjustment_type: string;
  payload_hash?: string;
  product_id?: string | null;
  product_title?: string;
  user_id?: string | null;
  user_email: string;
  paid_cny_fen: number;
  credit_usd_micro: number;
  source_trust: 'trusted' | 'manual' | 'untrusted' | string;
  payment_status: SalesPaymentStatus;
  status: SalesOrderStatus;
  operator?: string;
  approver?: string;
  error_code?: string;
  last_error_code?: string;
  error_message?: string;
  before_budget?: number | null;
  target_after?: number | null;
  created_at: string;
  updated_at: string;
}

export interface SalesOrderInput {
  channel: 'xianyu' | string;
  shop: string;
  external_order_id: string;
  adjustment_type: 'credit' | string;
  product_id?: string;
  external_item_id?: string;
  sku?: string;
  user_email: string;
  paid_cny_fen: number;
  payment_status: SalesPaymentStatus;
  source_trust: 'manual';
  note?: string;
}

export interface SalesOrderParams extends OpsListParams {
  status?: SalesOrderStatus;
  payment_status?: SalesPaymentStatus;
  source_trust?: string;
}

export interface UserListParams extends OpsListParams {
  email?: string;
  user_role?: string;
}

export interface KeyListParams extends OpsListParams {
  alias?: string;
  user_email?: string;
  is_session_key?: 'true' | 'false';
  blocked?: 'true' | 'false';
}

export function modelsOf(value: string[] | string | undefined): string[] {
  if (Array.isArray(value)) return value.filter(Boolean);
  if (!value) return [];
  try {
    const decoded: unknown = JSON.parse(value);
    return Array.isArray(decoded) ? decoded.filter((item): item is string => typeof item === 'string') : [];
  } catch {
    return value.split(',').map((item) => item.trim()).filter(Boolean);
  }
}

export function formatUsd(value: number | null | undefined, digits = 2): string {
  if (value === null || value === undefined) return '—';
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(value);
}

export function formatCnyFen(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—';
  return new Intl.NumberFormat(undefined, { style: 'currency', currency: 'CNY' }).format(value / 100);
}

export function formatUsdMicro(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—';
  return formatUsd(value / 1_000_000, 2);
}

export function formatInteger(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—';
  return new Intl.NumberFormat().format(value);
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(parsed);
}
