import { getRequestErrorMessage, getRequestStatus } from '@mss-admin-core/shared/api/errors';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  Table,
  type TableColumnsType,
  Tabs,
  Tag,
} from 'antd';
import { useMemo, useState } from 'react';
import {
  availableOrderActions,
  normalizeOptionalSKU,
  type SalesOrderAction,
} from '../business-reference';
import type {
  SalesOrder,
  SalesOrderInput,
  SalesOrderParams,
  SalesProduct,
  SalesProductInput,
  SalesProductParams,
  SalesProductPatch,
} from '../contract';
import { formatCnyFen, formatDateTime, formatUsd, formatUsdMicro } from '../contract';
import { operationsAPI } from '../operations-api';
import { useSalesOrder, useSalesOrders, useSalesProducts } from '../operations-query';
import { buildOpsAccess } from '../permissions';
import { OpsQueryState, OrderStatusTag } from '../ui';

type Translator = (id: string, values?: Record<string, string | number>) => string;

interface ManualOrderForm {
  shop: string;
  external_order_id: string;
  product_id?: string;
  user_email: string;
  paid_cny: number;
  note?: string;
}

interface ProductForm {
  channel: string;
  shop: string;
  external_item_id: string;
  sku?: string;
  title: string;
  price_cny: number;
  credit_usd: number;
  raise_keys: boolean;
  enabled: boolean;
  auto_apply: boolean;
}

function mutationErrorMessage(error: unknown): string {
  return getRequestStatus(error) === 403 ? '403' : getRequestErrorMessage(error);
}

function ProductPanel({ canWrite, t }: { canWrite: boolean; t: Translator }) {
  const { message } = App.useApp();
  const client = useQueryClient();
  const [params, setParams] = useState<SalesProductParams>({ page: 1, page_size: 20 });
  const [editing, setEditing] = useState<SalesProduct | 'new'>();
  const [form] = Form.useForm<ProductForm>();
  const products = useSalesProducts(params);

  const save = useMutation({
    mutationFn: async (values: ProductForm) => {
      const common = {
        title: values.title.trim(),
        price_cny_fen: Math.round(values.price_cny * 100),
        credit_usd_micro: Math.round(values.credit_usd * 1_000_000),
        raise_keys: values.raise_keys,
        enabled: values.enabled,
        auto_apply: values.auto_apply,
      };
      if (editing && editing !== 'new') {
        const patch: SalesProductPatch = { ...common, version: editing.version };
        return operationsAPI.sales.updateProduct(editing.id, patch);
      }
      const payload: SalesProductInput = {
        channel: values.channel.trim(),
        shop: values.shop.trim(),
        external_item_id: values.external_item_id.trim(),
        sku: normalizeOptionalSKU(values.sku),
        ...common,
      };
      return operationsAPI.sales.createProduct(payload);
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ['litellmops', 'sales-products'] });
      message.success(t('litellmops.sales.product.saved'));
      setEditing(undefined);
      form.resetFields();
    },
    onError: (error) => message.error(mutationErrorMessage(error)),
  });

  const openEditor = (product?: SalesProduct) => {
    setEditing(product ?? 'new');
    form.setFieldsValue(
      product
        ? {
            ...product,
            price_cny: product.price_cny_fen / 100,
            credit_usd: product.credit_usd_micro / 1_000_000,
          }
        : {
            channel: 'xianyu',
            enabled: true,
            auto_apply: false,
            raise_keys: true,
            price_cny: 10,
          },
    );
  };

  const columns: TableColumnsType<SalesProduct> = [
    { title: t('litellmops.sales.product.title'), dataIndex: 'title', fixed: 'left', width: 180 },
    { title: t('litellmops.sales.product.shop'), dataIndex: 'shop', width: 140 },
    {
      title: t('litellmops.sales.product.externalItem'),
      dataIndex: 'external_item_id',
      width: 170,
      render: (value) => <code>{value}</code>,
    },
    { title: t('litellmops.sales.product.sku'), dataIndex: 'sku', width: 120 },
    {
      title: t('litellmops.sales.product.price'),
      dataIndex: 'price_cny_fen',
      width: 120,
      render: formatCnyFen,
    },
    {
      title: t('litellmops.sales.product.credit'),
      dataIndex: 'credit_usd_micro',
      width: 120,
      render: formatUsdMicro,
    },
    {
      title: t('litellmops.common.status'),
      dataIndex: 'enabled',
      width: 100,
      render: (enabled) => (
        <Tag color={enabled ? 'success' : 'default'}>
          {t(enabled ? 'litellmops.common.enabled' : 'litellmops.common.disabled')}
        </Tag>
      ),
    },
    {
      title: t('litellmops.sales.product.autoApply'),
      dataIndex: 'auto_apply',
      width: 120,
      render: (enabled) => (enabled ? t('litellmops.common.yes') : t('litellmops.common.no')),
    },
    {
      title: t('litellmops.sales.product.raiseKeys'),
      dataIndex: 'raise_keys',
      width: 120,
      render: (enabled) => (enabled ? t('litellmops.common.yes') : t('litellmops.common.no')),
    },
    {
      title: t('litellmops.common.actions'),
      key: 'actions',
      fixed: 'right',
      width: 90,
      render: (_, product) =>
        canWrite ? (
          <Button type="link" size="small" onClick={() => openEditor(product)}>
            {t('litellmops.common.edit')}
          </Button>
        ) : null,
    },
  ];

  return (
    <Card
      title={t('litellmops.sales.products')}
      extra={
        <Space wrap>
          <Input.Search
            allowClear
            placeholder={t('litellmops.sales.product.channelFilter')}
            onSearch={(channel) =>
              setParams((current) => ({ ...current, page: 1, channel: channel || undefined }))
            }
          />
          <Input.Search
            allowClear
            placeholder={t('litellmops.sales.product.shopFilter')}
            onSearch={(shop) =>
              setParams((current) => ({ ...current, page: 1, shop: shop || undefined }))
            }
          />
          {canWrite ? (
            <Button type="primary" onClick={() => openEditor()}>
              {t('litellmops.sales.product.create')}
            </Button>
          ) : null}
        </Space>
      }
    >
      <OpsQueryState
        data={products.data}
        error={products.error}
        isError={products.isError}
        isPending={products.isPending}
        onRetry={() => void products.refetch()}
      >
        <Table<SalesProduct>
          rowKey="id"
          loading={products.isFetching}
          columns={columns}
          dataSource={products.data?.items ?? []}
          locale={{ emptyText: <Empty description={t('litellmops.sales.product.empty')} /> }}
          scroll={{ x: 1_150 }}
          pagination={{
            current: params.page,
            pageSize: params.page_size,
            total: products.data?.total,
            onChange: (page, pageSize) =>
              setParams((current) => ({ ...current, page, page_size: pageSize })),
          }}
        />
      </OpsQueryState>

      <Modal
        destroyOnHidden
        title={t(
          editing === 'new' ? 'litellmops.sales.product.create' : 'litellmops.sales.product.edit',
        )}
        open={Boolean(editing)}
        confirmLoading={save.isPending}
        onCancel={() => setEditing(undefined)}
        onOk={() => void form.validateFields().then((values) => save.mutate(values))}
      >
        <Alert type="warning" showIcon message={t('litellmops.sales.product.autoWarning')} />
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="channel"
            label={t('litellmops.sales.product.channel')}
            rules={[{ required: true }]}
          >
            <Input disabled={editing !== 'new'} autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="shop"
            label={t('litellmops.sales.product.shop')}
            rules={[{ required: true }]}
          >
            <Input disabled={editing !== 'new'} autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="external_item_id"
            label={t('litellmops.sales.product.externalItem')}
            rules={[{ required: true }]}
          >
            <Input disabled={editing !== 'new'} autoComplete="off" />
          </Form.Item>
          <Form.Item name="sku" label={t('litellmops.sales.product.sku')}>
            <Input disabled={editing !== 'new'} autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="title"
            label={t('litellmops.sales.product.title')}
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="price_cny"
            label={t('litellmops.sales.product.priceYuan')}
            rules={[{ required: true }]}
          >
            <InputNumber min={0.01} precision={2} stringMode={false} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="credit_usd"
            label={t('litellmops.sales.product.creditUsd')}
            rules={[{ required: true }]}
          >
            <InputNumber min={0.01} precision={2} stringMode={false} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="enabled" label={t('litellmops.common.enabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            name="auto_apply"
            label={t('litellmops.sales.product.autoApply')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name="raise_keys"
            label={t('litellmops.sales.product.raiseKeys')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}

function OrderDetail({ id, onClose, t }: { id?: string; onClose: () => void; t: Translator }) {
  const order = useSalesOrder(id);
  return (
    <Drawer
      destroyOnHidden
      title={t('litellmops.sales.order.detail')}
      width="min(760px, 100vw)"
      open={Boolean(id)}
      onClose={onClose}
    >
      <OpsQueryState
        data={order.data}
        error={order.error}
        isError={order.isError}
        isPending={order.isPending}
        onRetry={() => void order.refetch()}
      >
        {order.data ? (
          <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
            <Descriptions.Item label={t('litellmops.sales.order.externalId')} span={2}>
              <code>{order.data.external_order_id}</code>
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.common.status')}>
              <OrderStatusTag status={order.data.status} />
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.sales.order.payment')}>
              <Tag>{t(`litellmops.sales.payment.${order.data.payment_status}`)}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.sales.order.email')}>
              {order.data.user_email || '—'}
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.sales.order.product')}>
              {order.data.product_title || order.data.product_id || '—'}
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.sales.order.paid')}>
              {formatCnyFen(order.data.paid_cny_fen)}
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.sales.order.credit')}>
              {formatUsdMicro(order.data.credit_usd_micro)}
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.sales.order.before')}>
              {formatUsd(order.data.before_budget)}
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.sales.order.target')}>
              {formatUsd(order.data.target_after)}
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.sales.order.source')}>
              {t(`litellmops.sales.source.${order.data.source_trust}`)}
            </Descriptions.Item>
            <Descriptions.Item label={t('litellmops.common.updatedAt')}>
              {formatDateTime(order.data.updated_at)}
            </Descriptions.Item>
            {order.data.error_message || order.data.error_code || order.data.last_error_code ? (
              <Descriptions.Item label={t('litellmops.common.error')} span={2}>
                <Alert
                  type="error"
                  showIcon
                  message={order.data.error_code || order.data.last_error_code}
                  description={order.data.error_message}
                />
              </Descriptions.Item>
            ) : null}
          </Descriptions>
        ) : null}
      </OpsQueryState>
    </Drawer>
  );
}

function OrdersPanel({
  access,
  products,
  t,
}: {
  access: ReturnType<typeof buildOpsAccess>;
  products: SalesProduct[];
  t: Translator;
}) {
  const { message, modal } = App.useApp();
  const client = useQueryClient();
  const [params, setParams] = useState<SalesOrderParams>({ page: 1, page_size: 20 });
  const [createOpen, setCreateOpen] = useState(false);
  const [detailId, setDetailId] = useState<string>();
  const [matchOrder, setMatchOrder] = useState<SalesOrder>();
  const [refundOrder, setRefundOrder] = useState<SalesOrder>();
  const [createForm] = Form.useForm<ManualOrderForm>();
  const [matchForm] = Form.useForm<{ product_id: string; user_email: string }>();
  const [refundForm] = Form.useForm<{ reason: string }>();
  const orders = useSalesOrders(params);

  const refresh = async () => {
    await client.invalidateQueries({ queryKey: ['litellmops', 'sales-orders'] });
    await client.invalidateQueries({ queryKey: ['litellmops', 'sales-order'] });
    await client.invalidateQueries({ queryKey: ['litellmops', 'managed-users'] });
  };

  const create = useMutation({
    mutationFn: (values: ManualOrderForm) => {
      const data: SalesOrderInput = {
        channel: 'xianyu',
        shop: values.shop.trim(),
        external_order_id: values.external_order_id.trim(),
        adjustment_type: 'credit',
        product_id: values.product_id,
        user_email: values.user_email.trim(),
        paid_cny_fen: Math.round(values.paid_cny * 100),
        payment_status: 'paid',
        source_trust: 'manual',
        note: values.note?.trim(),
      };
      return operationsAPI.sales.createOrder(data);
    },
    onSuccess: async (order) => {
      await refresh();
      setCreateOpen(false);
      createForm.resetFields();
      setDetailId(order.id);
      message.success(t('litellmops.sales.order.created'));
    },
    onError: (error) => message.error(mutationErrorMessage(error)),
  });

  const transition = useMutation({
    mutationFn: async ({ action, order }: { action: SalesOrderAction; order: SalesOrder }) => {
      if (action === 'verify') return operationsAPI.sales.verify(order.id);
      if (action === 'approve') return operationsAPI.sales.approve(order.id);
      if (action === 'execute') return operationsAPI.sales.execute(order.id);
      if (action === 'reconcile') return operationsAPI.sales.reconcile(order.id);
      throw new Error('unsupported transition');
    },
    onSuccess: (order) => {
      setDetailId(order.id);
      message.success(t('litellmops.sales.order.transitioned'));
    },
    onError: (error) => message.error(mutationErrorMessage(error)),
    onSettled: refresh,
  });

  const match = useMutation({
    mutationFn: (values: { product_id: string; user_email: string }) =>
      operationsAPI.sales.match(matchOrder?.id as string, values),
    onSuccess: async (order) => {
      await refresh();
      setMatchOrder(undefined);
      matchForm.resetFields();
      setDetailId(order.id);
      message.success(t('litellmops.sales.order.transitioned'));
    },
    onError: (error) => message.error(mutationErrorMessage(error)),
  });

  const refund = useMutation({
    mutationFn: (values: { reason: string }) =>
      operationsAPI.sales.reviewRefund(refundOrder?.id as string, values),
    onSuccess: async (order) => {
      await refresh();
      setRefundOrder(undefined);
      refundForm.resetFields();
      setDetailId(order.id);
      message.success(t('litellmops.sales.order.refundReviewed'));
    },
    onError: (error) => message.error(mutationErrorMessage(error)),
  });

  const canRun = (action: SalesOrderAction) => {
    if (action === 'verify' || action === 'match') return access.canVerifySales;
    if (action === 'approve') return access.canApproveSales;
    if (action === 'execute') return access.canExecuteSales;
    if (action === 'reconcile') return access.canReconcileSales;
    return access.canReviewRefund;
  };

  const run = (order: SalesOrder, action: SalesOrderAction) => {
    if (action === 'match') {
      setMatchOrder(order);
      matchForm.setFieldsValue({
        product_id: order.product_id ?? undefined,
        user_email: order.user_email,
      });
      return;
    }
    if (action === 'refund-review') {
      setRefundOrder(order);
      refundForm.setFieldsValue({ reason: '' });
      return;
    }
    modal.confirm({
      title: t(`litellmops.sales.action.${action}`),
      content: t(
        action === 'execute'
          ? 'litellmops.sales.action.executeWarning'
          : 'litellmops.sales.action.auditWarning',
      ),
      okText: t(`litellmops.sales.action.${action}`),
      okButtonProps: { danger: action === 'execute' },
      onOk: () => transition.mutateAsync({ action, order }),
    });
  };

  const columns: TableColumnsType<SalesOrder> = [
    {
      title: t('litellmops.sales.order.externalId'),
      dataIndex: 'external_order_id',
      fixed: 'left',
      width: 190,
      render: (value, order) => (
        <Button type="link" size="small" onClick={() => setDetailId(order.id)}>
          <code>{value}</code>
        </Button>
      ),
    },
    { title: t('litellmops.sales.order.email'), dataIndex: 'user_email', width: 210 },
    {
      title: t('litellmops.sales.order.product'),
      dataIndex: 'product_title',
      width: 170,
      render: (value, order) => value || order.product_id || '—',
    },
    {
      title: t('litellmops.sales.order.paid'),
      dataIndex: 'paid_cny_fen',
      width: 110,
      render: formatCnyFen,
    },
    {
      title: t('litellmops.sales.order.credit'),
      dataIndex: 'credit_usd_micro',
      width: 110,
      render: formatUsdMicro,
    },
    {
      title: t('litellmops.sales.order.payment'),
      dataIndex: 'payment_status',
      width: 110,
      render: (value) => <Tag>{t(`litellmops.sales.payment.${value}`)}</Tag>,
    },
    {
      title: t('litellmops.common.status'),
      dataIndex: 'status',
      width: 160,
      render: (value) => <OrderStatusTag status={value} />,
    },
    {
      title: t('litellmops.sales.order.source'),
      dataIndex: 'source_trust',
      width: 110,
      render: (value) => (
        <Tag color={value === 'trusted' ? 'success' : 'default'}>
          {t(`litellmops.sales.source.${value}`)}
        </Tag>
      ),
    },
    {
      title: t('litellmops.common.updatedAt'),
      dataIndex: 'updated_at',
      width: 180,
      render: formatDateTime,
    },
    {
      title: t('litellmops.common.actions'),
      key: 'actions',
      fixed: 'right',
      width: 260,
      render: (_, order) => (
        <Space size={2} wrap>
          <Button size="small" type="link" onClick={() => setDetailId(order.id)}>
            {t('litellmops.common.detail')}
          </Button>
          {availableOrderActions(order)
            .filter(canRun)
            .map((action) => (
              <Button
                key={action}
                size="small"
                danger={action === 'execute' || action === 'refund-review'}
                loading={transition.isPending && transition.variables?.order.id === order.id}
                onClick={() => run(order, action)}
              >
                {t(`litellmops.sales.action.${action}`)}
              </Button>
            ))}
        </Space>
      ),
    },
  ];

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Alert type="warning" showIcon message={t('litellmops.sales.order.safety')} />
      <Alert type="info" showIcon message={t('litellmops.sales.order.connector')} />
      <Card
        title={t('litellmops.sales.orders')}
        extra={
          <Space wrap>
            <Input.Search
              allowClear
              placeholder={t('litellmops.sales.order.externalId')}
              onSearch={(external_order_id) =>
                setParams((current) => ({
                  ...current,
                  page: 1,
                  external_order_id: external_order_id || undefined,
                }))
              }
            />
            <Input.Search
              allowClear
              placeholder={t('litellmops.sales.order.email')}
              onSearch={(user_email) =>
                setParams((current) => ({
                  ...current,
                  page: 1,
                  user_email: user_email || undefined,
                }))
              }
            />
            <Select
              allowClear
              placeholder={t('litellmops.common.status')}
              style={{ minWidth: 170 }}
              options={[
                'received',
                'verified_paid',
                'mapped',
                'approved',
                'executing',
                'applied_unverified',
                'completed',
                'retryable_failed',
                'terminal_failed',
                'reconcile_required',
                'refund_review',
                'reversed',
              ].map((value) => ({ value, label: t(`litellmops.sales.status.${value}`) }))}
              onChange={(status) => setParams((current) => ({ ...current, page: 1, status }))}
            />
            {access.canWriteSales ? (
              <Button type="primary" onClick={() => setCreateOpen(true)}>
                {t('litellmops.sales.order.create')}
              </Button>
            ) : null}
          </Space>
        }
      >
        <OpsQueryState
          data={orders.data}
          error={orders.error}
          isError={orders.isError}
          isPending={orders.isPending}
          onRetry={() => void orders.refetch()}
        >
          <Table<SalesOrder>
            rowKey="id"
            loading={orders.isFetching}
            columns={columns}
            dataSource={orders.data?.items ?? []}
            locale={{ emptyText: <Empty description={t('litellmops.sales.order.empty')} /> }}
            scroll={{ x: 1_600 }}
            pagination={{
              current: params.page,
              pageSize: params.page_size,
              total: orders.data?.total,
              onChange: (page, pageSize) =>
                setParams((current) => ({ ...current, page, page_size: pageSize })),
            }}
          />
        </OpsQueryState>
      </Card>

      <Modal
        destroyOnHidden
        title={t('litellmops.sales.order.create')}
        open={createOpen}
        confirmLoading={create.isPending}
        onCancel={() => setCreateOpen(false)}
        onOk={() => void createForm.validateFields().then((values) => create.mutate(values))}
      >
        <Alert type="info" showIcon message={t('litellmops.sales.order.manualCheck')} />
        <Form form={createForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="shop"
            label={t('litellmops.sales.product.shop')}
            rules={[{ required: true }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="external_order_id"
            label={t('litellmops.sales.order.externalId')}
            rules={[{ required: true, min: 4 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item name="product_id" label={t('litellmops.sales.order.product')}>
            <Select
              allowClear
              showSearch
              optionFilterProp="label"
              options={products
                .filter((product) => product.enabled)
                .map((product) => ({
                  value: product.id,
                  label: `${product.title} · ${formatCnyFen(product.price_cny_fen)} → ${formatUsdMicro(product.credit_usd_micro)}`,
                }))}
            />
          </Form.Item>
          <Form.Item
            name="user_email"
            label={t('litellmops.sales.order.email')}
            rules={[{ required: true, type: 'email' }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="paid_cny"
            label={t('litellmops.sales.order.paidYuan')}
            rules={[{ required: true }]}
          >
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="note" label={t('litellmops.common.note')}>
            <Input.TextArea rows={2} maxLength={500} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        destroyOnHidden
        title={t('litellmops.sales.action.match')}
        open={Boolean(matchOrder)}
        confirmLoading={match.isPending}
        onCancel={() => setMatchOrder(undefined)}
        onOk={() => void matchForm.validateFields().then((values) => match.mutate(values))}
      >
        <Form form={matchForm} layout="vertical">
          <Form.Item
            name="product_id"
            label={t('litellmops.sales.order.product')}
            rules={[{ required: true }]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              options={products
                .filter((product) => product.enabled)
                .map((product) => ({ value: product.id, label: product.title }))}
            />
          </Form.Item>
          <Form.Item
            name="user_email"
            label={t('litellmops.sales.order.email')}
            rules={[{ required: true, type: 'email' }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        destroyOnHidden
        title={t('litellmops.sales.action.refund-review')}
        open={Boolean(refundOrder)}
        confirmLoading={refund.isPending}
        onCancel={() => setRefundOrder(undefined)}
        onOk={() => void refundForm.validateFields().then((values) => refund.mutate(values))}
      >
        <Alert type="error" showIcon message={t('litellmops.sales.order.refundWarning')} />
        <Form form={refundForm} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="reason"
            label={t('litellmops.common.reason')}
            rules={[{ required: true, min: 4 }]}
          >
            <Input.TextArea rows={3} maxLength={500} />
          </Form.Item>
        </Form>
      </Modal>

      <OrderDetail id={detailId} onClose={() => setDetailId(undefined)} t={t} />
    </Space>
  );
}

export default function SalesView() {
  const intl = useIntl();
  const t: Translator = (id, values) => intl.formatMessage({ id }, values);
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const access = useMemo(
    () => buildOpsAccess(initialState?.currentUser),
    [initialState?.currentUser],
  );
  const productQuery = useSalesProducts({ page: 1, page_size: 100 });

  if (!access.canReadSales) return <PageForbidden />;

  return (
    <PageContainer title={t('litellmops.sales.title')} content={t('litellmops.sales.description')}>
      <Tabs
        items={[
          {
            key: 'orders',
            label: t('litellmops.sales.orders'),
            children: (
              <OrdersPanel access={access} products={productQuery.data?.items ?? []} t={t} />
            ),
          },
          {
            key: 'products',
            label: t('litellmops.sales.products'),
            children: access.canReadProducts ? (
              <ProductPanel canWrite={access.canWriteProducts} t={t} />
            ) : (
              <PageForbidden />
            ),
          },
        ]}
      />
    </PageContainer>
  );
}
