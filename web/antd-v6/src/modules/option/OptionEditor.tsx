import ArrowLeftOutlined from '@ant-design/icons/ArrowLeftOutlined';
import DeleteOutlined from '@ant-design/icons/DeleteOutlined';
import PlusOutlined from '@ant-design/icons/PlusOutlined';
import SaveOutlined from '@ant-design/icons/SaveOutlined';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { history, useIntl } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Result,
  Row,
  Select,
  Space,
  Typography,
} from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import { queryKeys } from '@/shared/query/client';
import { optionAPI } from './api';
import {
  MAX_OPTION_CATEGORY,
  MAX_OPTION_DESCRIPTION,
  MAX_OPTION_DISPLAY_NAME,
  MAX_OPTION_ITEM_COLOR,
  MAX_OPTION_ITEM_ICON,
  MAX_OPTION_ITEM_KEY,
  MAX_OPTION_ITEM_LABEL,
  MAX_OPTION_ITEM_SORT,
  MAX_OPTION_ITEM_VALUE,
  MAX_OPTION_ITEMS,
  MAX_OPTION_NAME,
  MAX_OPTION_REMARK,
  type OptionDetail,
  type OptionFormValues,
} from './contract';
import { useOption } from './query';

interface OptionEditorProps {
  id?: string;
  mode: 'create' | 'edit';
}

function detailValues(option: OptionDetail): OptionFormValues {
  return {
    category: option.category,
    displayName: option.displayName,
    description: option.description,
    name: option.name,
    remark: option.remark,
    status: option.status,
    items: option.items.map(({ id, key, label, value, color, sort, icon }) => ({
      id,
      key,
      label,
      value,
      color,
      sort,
      ...(icon ? { icon } : {}),
    })),
  };
}

export default function OptionEditor({ id, mode }: OptionEditorProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [form] = Form.useForm<OptionFormValues>();
  const detail = useOption(mode === 'edit' ? id : undefined);
  const [base, setBase] = useState<OptionDetail>();
  const [conflict, setConflict] = useState(false);
  const initialized = useRef(false);

  const acceptDetail = useCallback(
    (option: OptionDetail) => {
      form.setFieldsValue(detailValues(option));
      setBase(option);
      setConflict(false);
      initialized.current = true;
    },
    [form],
  );

  useEffect(() => {
    if (mode === 'edit' && detail.data && !initialized.current) {
      acceptDetail(detail.data);
    }
  }, [acceptDetail, detail.data, mode]);

  const save = useMutation({
    mutationFn: async (values: OptionFormValues) => {
      if (mode === 'create') return optionAPI.create(values);
      if (!id || !base) throw new Error('Option revision is unavailable');
      return optionAPI.update(id, values, base);
    },
    onSuccess: async (option) => {
      client.setQueryData(queryKeys.option(option.id), option);
      await client.invalidateQueries({ queryKey: queryKeys.options });
      void message.success(
        intl.formatMessage({
          id: mode === 'create' ? 'option.create.success' : 'option.update.success',
        }),
      );
      history.push('/option');
    },
    onError: (error) => {
      if (getRequestStatus(error) === 412) setConflict(true);
    },
  });
  const loadStatus = getRequestStatus(detail.error);
  const saveStatus = getRequestStatus(save.error);

  if (loadStatus === 403 || saveStatus === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'option.forbidden.write' })} />;
  }
  if (mode === 'edit' && detail.isPending && !base) return <PageLoading rows={10} />;
  if (mode === 'edit' && loadStatus === 404) {
    return (
      <Result
        extra={
          <Button type="primary" onClick={() => history.push('/option')}>
            {intl.formatMessage({ id: 'option.backToList' })}
          </Button>
        }
        status="404"
        subTitle={intl.formatMessage({ id: 'option.notFound' })}
      />
    );
  }
  if (mode === 'edit' && detail.isError && !base) {
    return (
      <PageError
        message={getRequestErrorMessage(detail.error)}
        onRetry={() => void detail.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
        title={intl.formatMessage({ id: 'states.loadError' })}
      />
    );
  }

  const reloadLatest = async () => {
    const latest = await detail.refetch();
    if (latest.data) acceptDetail(latest.data);
  };
  const builtIn = Boolean(base?.builtIn);

  return (
    <Form<OptionFormValues>
      form={form}
      initialValues={{
        category: 'system',
        description: '',
        displayName: '',
        items: [],
        remark: '',
        status: 'enabled',
      }}
      layout="vertical"
      onFinish={(values) => save.mutate(values)}
    >
      <Space orientation="vertical" size="middle" className="w-full">
        {builtIn ? (
          <Alert
            showIcon
            description={intl.formatMessage({ id: 'option.builtIn.description' })}
            title={intl.formatMessage({ id: 'option.builtIn.title' })}
            type="info"
          />
        ) : null}
        {detail.isError && base ? (
          <Alert
            showIcon
            title={intl.formatMessage({ id: 'option.detail.refreshFailed' })}
            type="warning"
          />
        ) : null}
        {conflict ? (
          <Alert
            action={
              <Popconfirm
                description={intl.formatMessage({ id: 'option.conflict.reloadDescription' })}
                title={intl.formatMessage({ id: 'option.conflict.reloadConfirm' })}
                onConfirm={reloadLatest}
              >
                <Button size="small">{intl.formatMessage({ id: 'option.conflict.reload' })}</Button>
              </Popconfirm>
            }
            description={intl.formatMessage({ id: 'option.conflict.description' })}
            showIcon
            title={intl.formatMessage({ id: 'option.conflict.title' })}
            type="warning"
          />
        ) : null}
        {save.isError && saveStatus !== 412 ? (
          <Alert
            closable
            description={getRequestErrorMessage(save.error)}
            showIcon
            title={intl.formatMessage({ id: 'option.save.failed' })}
            type="error"
            onClose={() => save.reset()}
          />
        ) : null}
        <Card variant="outlined">
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item
                name="name"
                label={intl.formatMessage({ id: 'option.field.name' })}
                rules={[{ required: true }, { max: MAX_OPTION_NAME }]}
              >
                <Input autoComplete="off" disabled={builtIn} />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                name="category"
                label={intl.formatMessage({ id: 'option.field.category' })}
                rules={[{ required: true }, { max: MAX_OPTION_CATEGORY }]}
              >
                <Input autoComplete="off" disabled={builtIn} />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                name="displayName"
                label={intl.formatMessage({ id: 'option.field.displayName' })}
                rules={[{ max: MAX_OPTION_DISPLAY_NAME }]}
              >
                <Input autoComplete="off" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                name="status"
                label={intl.formatMessage({ id: 'option.field.status' })}
                rules={[{ required: true }]}
              >
                <Select
                  disabled={builtIn}
                  options={(['enabled', 'disabled'] as const).map((value) => ({
                    value,
                    label: intl.formatMessage({ id: `option.status.${value}` }),
                  }))}
                />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item
            name="description"
            label={intl.formatMessage({ id: 'option.field.description' })}
            rules={[{ max: MAX_OPTION_DESCRIPTION }]}
          >
            <Input.TextArea rows={4} showCount maxLength={MAX_OPTION_DESCRIPTION} />
          </Form.Item>
          <Form.Item
            name="remark"
            label={intl.formatMessage({ id: 'option.field.remark' })}
            rules={[{ max: MAX_OPTION_REMARK }]}
          >
            <Input.TextArea rows={2} showCount maxLength={MAX_OPTION_REMARK} />
          </Form.Item>
        </Card>
        <Form.List
          name="items"
          rules={[
            {
              validator: async (_, items) => {
                if (!Array.isArray(items)) return;
                if (items.length > MAX_OPTION_ITEMS) {
                  throw new Error(intl.formatMessage({ id: 'option.items.limit' }));
                }
                const keys = new Set<string>();
                const values = new Set<string>();
                for (const item of items) {
                  const key = typeof item?.key === 'string' ? item.key.trim() : '';
                  const value = typeof item?.value === 'string' ? item.value.trim() : '';
                  if (key && keys.has(key)) {
                    throw new Error(
                      intl.formatMessage({ id: 'option.items.duplicateKey' }, { key }),
                    );
                  }
                  if (value && values.has(value)) {
                    throw new Error(
                      intl.formatMessage({ id: 'option.items.duplicateValue' }, { value }),
                    );
                  }
                  if (key) keys.add(key);
                  if (value) values.add(value);
                }
              },
            },
          ]}
        >
          {(fields, { add, remove }, { errors }) => (
            <Card
              extra={
                <Button
                  disabled={fields.length >= MAX_OPTION_ITEMS}
                  icon={<PlusOutlined />}
                  onClick={() =>
                    add({
                      color: '',
                      icon: '',
                      key: '',
                      label: '',
                      sort: fields.length + 1,
                      value: '',
                    })
                  }
                >
                  {intl.formatMessage({ id: 'option.items.add' })}
                </Button>
              }
              title={intl.formatMessage({ id: 'option.items.title' }, { count: fields.length })}
              variant="outlined"
            >
              {fields.length === 0 ? (
                <Typography.Text type="secondary">
                  {intl.formatMessage({ id: 'option.items.empty' })}
                </Typography.Text>
              ) : null}
              <Space orientation="vertical" size="small" className="w-full">
                {fields.map((field, index) => (
                  <Card key={field.key} size="small" variant="outlined">
                    <Form.Item name={[field.name, 'id']} hidden>
                      <Input />
                    </Form.Item>
                    <Row align="top" gutter={12}>
                      <Col xs={24} md={6}>
                        <Form.Item
                          name={[field.name, 'key']}
                          label={intl.formatMessage({ id: 'option.item.key' })}
                          rules={[{ required: true }, { max: MAX_OPTION_ITEM_KEY }]}
                        >
                          <Input autoComplete="off" />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={6}>
                        <Form.Item
                          name={[field.name, 'label']}
                          label={intl.formatMessage({ id: 'option.item.label' })}
                          rules={[{ required: true }, { max: MAX_OPTION_ITEM_LABEL }]}
                        >
                          <Input autoComplete="off" />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={8}>
                        <Form.Item
                          name={[field.name, 'value']}
                          label={intl.formatMessage({ id: 'option.item.value' })}
                          rules={[{ required: true }, { max: MAX_OPTION_ITEM_VALUE }]}
                        >
                          <Input autoComplete="off" />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={4}>
                        <Form.Item
                          name={[field.name, 'sort']}
                          label={intl.formatMessage({ id: 'option.item.sort' })}
                          rules={[{ required: true }]}
                        >
                          <InputNumber
                            className="w-full"
                            controls={false}
                            max={MAX_OPTION_ITEM_SORT}
                            min={-MAX_OPTION_ITEM_SORT}
                            precision={0}
                          />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={8}>
                        <Form.Item
                          name={[field.name, 'color']}
                          label={intl.formatMessage({ id: 'option.item.color' })}
                          rules={[{ max: MAX_OPTION_ITEM_COLOR }]}
                        >
                          <Input autoComplete="off" />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={12}>
                        <Form.Item
                          name={[field.name, 'icon']}
                          label={intl.formatMessage({ id: 'option.item.icon' })}
                          rules={[{ max: MAX_OPTION_ITEM_ICON }]}
                        >
                          <Input autoComplete="off" />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={4}>
                        <Form.Item label={intl.formatMessage({ id: 'actions.delete' })}>
                          <Button
                            danger
                            aria-label={intl.formatMessage(
                              { id: 'option.item.remove' },
                              { index: index + 1 },
                            )}
                            icon={<DeleteOutlined />}
                            onClick={() => remove(field.name)}
                          />
                        </Form.Item>
                      </Col>
                    </Row>
                  </Card>
                ))}
              </Space>
              <Form.ErrorList errors={errors} />
            </Card>
          )}
        </Form.List>
        <Space wrap>
          <Button icon={<ArrowLeftOutlined />} onClick={() => history.push('/option')}>
            {intl.formatMessage({ id: 'option.backToList' })}
          </Button>
          <Button htmlType="submit" icon={<SaveOutlined />} loading={save.isPending} type="primary">
            {intl.formatMessage({ id: 'actions.save' })}
          </Button>
        </Space>
      </Space>
    </Form>
  );
}
