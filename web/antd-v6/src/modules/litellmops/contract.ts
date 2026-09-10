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
  db_status: string;
  checked_at: string;
  message?: string;
}

export interface GatewayModel {
  id: string;
  name: string;
  provider?: string;
  mode?: string;
  base_model?: string;
  healthy?: boolean | null;
  blocked?: boolean | null;
  manageable: boolean;
  last_checked_at?: string;
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

export type ManagementCommandStatus =
  | 'executing'
  | 'result_unverified'
  | 'completed'
  | 'failed'
  | 'resolved_applied'
  | 'resolved_not_applied';

export interface ManagementExpectedSummary {
  kind: string;
  affected_fields: string[];
  max_budget_usd_micro?: number;
  spend_usd_micro?: number;
  blocked?: boolean;
  deleted?: boolean;
  manual_reason?: string;
}

export interface ManagementCommand {
  id: string;
  created_at: string;
  updated_at: string;
  resolved_at?: string | null;
  user_id: string;
  user_email: string;
  target_type: string;
  target_id: string;
  action: string;
  status: ManagementCommandStatus;
  last_error_code: string;
  requires_manual_review: boolean;
  last_observed_at?: string | null;
  operator: string;
  resolved_by: string;
  resolution_reason: string;
  version: number;
  expected: ManagementExpectedSummary;
}

export interface ManagementCommandParams extends Omit<OpsListParams, 'query'> {
  user_id?: string;
  status?: ManagementCommandStatus;
}

export interface ManagementPendingResponse {
  code: 'management_reconcile_required';
  error: string;
  command: ManagementCommand;
}

export type ManagementAware<T> = T | ManagementPendingResponse;

export interface ManagementResolveRequest {
  resolution: 'applied' | 'not_applied';
  reason: string;
  confirm_authoritative_state: true;
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
  amount_usd_micro: number;
  before_budget_usd_micro?: number | null;
  target_after_usd_micro?: number | null;
  raise_keys: boolean;
  keys_updated: string;
  operator: string;
  reason: string;
  business_reference?: string;
  idempotency_key?: string;
  source?: string;
  source_ref?: string;
  status: RechargeStatus | string;
  last_error_code?: string;
  uncertain_since?: string | null;
  version?: number;
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
  raise_keys: boolean;
  enabled: boolean;
  auto_apply: boolean;
  version: number;
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
  raise_keys: boolean;
  enabled: boolean;
  auto_apply: boolean;
}

export interface SalesProductPatch {
  title: string;
  price_cny_fen: number;
  credit_usd_micro: number;
  raise_keys: boolean;
  enabled: boolean;
  auto_apply: boolean;
  version: number;
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

export interface SalesProductParams extends Omit<OpsListParams, 'query'> {
  channel?: string;
  shop?: string;
  enabled?: 'true' | 'false';
}

export interface SalesOrderParams extends Omit<OpsListParams, 'query'> {
  channel?: string;
  shop?: string;
  status?: SalesOrderStatus;
  external_order_id?: string;
  user_email?: string;
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
    return Array.isArray(decoded)
      ? decoded.filter((item): item is string => typeof item === 'string')
      : [];
  } catch {
    return value
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
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
  return new Intl.NumberFormat(undefined, { style: 'currency', currency: 'CNY' }).format(
    value / 100,
  );
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
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(
    parsed,
  );
}
