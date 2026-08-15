import { ArrowLeftOutlined, DeleteOutlined, PlusOutlined, SaveOutlined } from '@ant-design/icons';
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
import { languageAPI } from './api';
import {
  type LanguageDetail,
  type LanguageFormValues,
  MAX_LANGUAGE_DEFINE_GROUP,
  MAX_LANGUAGE_DEFINE_KEY,
  MAX_LANGUAGE_DEFINE_VALUE,
  MAX_LANGUAGE_DEFINITIONS,
  normalizeLanguageName,
} from './contract';
import { useLanguage } from './query';

interface LanguageEditorProps {
  id?: string;
  mode: 'create' | 'edit';
}

function detailValues(detail: LanguageDetail): LanguageFormValues {
  return {
    name: detail.name,
    remark: detail.remark,
    status: detail.status,
    defines: detail.defines.map((definition) => ({ ...definition })),
  };
}

export default function LanguageEditor({ id, mode }: LanguageEditorProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [form] = Form.useForm<LanguageFormValues>();
  const detail = useLanguage(mode === 'edit' ? id : undefined);
  const [baseRevision, setBaseRevision] = useState<string>();
  const [conflict, setConflict] = useState(false);
  const initialized = useRef(false);

  const acceptDetail = useCallback(
    (language: LanguageDetail) => {
      form.setFieldsValue(detailValues(language));
      setBaseRevision(language.updatedAt);
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
    mutationFn: async (values: LanguageFormValues) => {
      if (mode === 'create') return languageAPI.create(values);
      if (!id || !baseRevision) throw new Error('Language revision is unavailable');
      return languageAPI.update(id, values, baseRevision);
    },
    onSuccess: async (language) => {
      client.setQueryData(queryKeys.language(language.id), language);
      await client.invalidateQueries({ queryKey: queryKeys.languages });
      void message.success(
        intl.formatMessage({
          id: mode === 'create' ? 'language.create.success' : 'language.update.success',
        }),
      );
      void message.info(intl.formatMessage({ id: 'language.runtime.reloadNotice' }));
      history.push('/language');
    },
    onError: (error) => {
      if (getRequestStatus(error) === 409) setConflict(true);
    },
  });
  const loadStatus = getRequestStatus(detail.error);
  const saveStatus = getRequestStatus(save.error);

  if (loadStatus === 403 || saveStatus === 403) {
    return <PageForbidden message={intl.formatMessage({ id: 'language.forbidden.write' })} />;
  }
  if (mode === 'edit' && detail.isPending && !baseRevision) return <PageLoading rows={9} />;
  if (mode === 'edit' && loadStatus === 404) {
    return (
      <Result
        extra={
          <Button type="primary" onClick={() => history.push('/language')}>
            {intl.formatMessage({ id: 'language.backToList' })}
          </Button>
        }
        status="404"
        subTitle={intl.formatMessage({ id: 'language.notFound' })}
      />
    );
  }
  if (mode === 'edit' && detail.isError && !baseRevision) {
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

  return (
    <Form<LanguageFormValues>
      form={form}
      initialValues={{ defines: [], remark: '', status: 'enabled' }}
      layout="vertical"
      onFinish={(values) => save.mutate(values)}
    >
      <Space orientation="vertical" size="middle" className="w-full">
        <Alert
          showIcon
          description={intl.formatMessage({ id: 'language.localeScope.description' })}
          title={intl.formatMessage({ id: 'language.localeScope.title' })}
          type="info"
        />
        {detail.isError && baseRevision ? (
          <Alert
            showIcon
            title={intl.formatMessage({ id: 'language.detail.refreshFailed' })}
            type="warning"
          />
        ) : null}
        {conflict ? (
          <Alert
            action={
              <Popconfirm
                description={intl.formatMessage({ id: 'language.conflict.reloadDescription' })}
                title={intl.formatMessage({ id: 'language.conflict.reloadConfirm' })}
                onConfirm={reloadLatest}
              >
                <Button size="small">
                  {intl.formatMessage({ id: 'language.conflict.reload' })}
                </Button>
              </Popconfirm>
            }
            description={intl.formatMessage({ id: 'language.conflict.description' })}
            showIcon
            title={intl.formatMessage({ id: 'language.conflict.title' })}
            type="warning"
          />
        ) : null}
        {save.isError && saveStatus !== 409 ? (
          <Alert
            closable
            description={getRequestErrorMessage(save.error)}
            showIcon
            title={intl.formatMessage({ id: 'language.save.failed' })}
            type="error"
            onClose={() => save.reset()}
          />
        ) : null}
        <Card variant="outlined">
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item
                name="name"
                label={intl.formatMessage({ id: 'language.field.name' })}
                rules={[
                  { required: true, message: intl.formatMessage({ id: 'language.name.required' }) },
                  { max: 255 },
                  {
                    validator: async (_, value) => {
                      if (typeof value !== 'string' || !value.trim()) return;
                      try {
                        normalizeLanguageName(value);
                      } catch {
                        throw new Error(intl.formatMessage({ id: 'language.name.invalid' }));
                      }
                    },
                  },
                ]}
              >
                <Input autoComplete="off" placeholder="zh-CN" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                name="status"
                label={intl.formatMessage({ id: 'language.field.status' })}
                rules={[{ required: true }]}
              >
                <Select
                  options={(['enabled', 'disabled'] as const).map((value) => ({
                    value,
                    label: intl.formatMessage({ id: `language.status.${value}` }),
                  }))}
                />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item
            name="remark"
            label={intl.formatMessage({ id: 'language.field.remark' })}
            rules={[{ max: 255 }]}
          >
            <Input.TextArea rows={3} showCount maxLength={255} />
          </Form.Item>
        </Card>
        <Form.List
          name="defines"
          rules={[
            {
              validator: async (_, definitions) => {
                if (!Array.isArray(definitions)) return;
                if (definitions.length > MAX_LANGUAGE_DEFINITIONS) {
                  throw new Error(intl.formatMessage({ id: 'language.definitions.limit' }));
                }
                const keys = new Set<string>();
                for (const definition of definitions) {
                  const group =
                    typeof definition?.group === 'string' ? definition.group.trim() : '';
                  const key = typeof definition?.key === 'string' ? definition.key.trim() : '';
                  if (!group || !key) continue;
                  const compound = `${group}\u0000${key}`;
                  if (keys.has(compound)) {
                    throw new Error(
                      intl.formatMessage(
                        { id: 'language.definitions.duplicate' },
                        { key: `${group}.${key}` },
                      ),
                    );
                  }
                  keys.add(compound);
                }
              },
            },
          ]}
        >
          {(fields, { add, remove }, { errors }) => (
            <Card
              extra={
                <Button
                  disabled={fields.length >= MAX_LANGUAGE_DEFINITIONS}
                  icon={<PlusOutlined />}
                  onClick={() => add({ group: '', key: '', value: '' })}
                >
                  {intl.formatMessage({ id: 'language.definitions.add' })}
                </Button>
              }
              title={intl.formatMessage(
                { id: 'language.definitions.title' },
                { count: fields.length },
              )}
              variant="outlined"
            >
              {fields.length === 0 ? (
                <Typography.Text type="secondary">
                  {intl.formatMessage({ id: 'language.definitions.empty' })}
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
                          name={[field.name, 'group']}
                          label={intl.formatMessage({ id: 'language.definition.group' })}
                          rules={[{ required: true }, { max: MAX_LANGUAGE_DEFINE_GROUP }]}
                        >
                          <Input autoComplete="off" />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={6}>
                        <Form.Item
                          name={[field.name, 'key']}
                          label={intl.formatMessage({ id: 'language.definition.key' })}
                          rules={[{ required: true }, { max: MAX_LANGUAGE_DEFINE_KEY }]}
                        >
                          <Input autoComplete="off" />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={10}>
                        <Form.Item
                          name={[field.name, 'value']}
                          label={intl.formatMessage({ id: 'language.definition.value' })}
                          rules={[{ max: MAX_LANGUAGE_DEFINE_VALUE }]}
                        >
                          <Input.TextArea rows={2} />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={2}>
                        <Form.Item
                          label={index === 0 ? intl.formatMessage({ id: 'actions.delete' }) : ' '}
                        >
                          <Button
                            danger
                            aria-label={intl.formatMessage({ id: 'language.definition.remove' })}
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
          <Button icon={<ArrowLeftOutlined />} onClick={() => history.push('/language')}>
            {intl.formatMessage({ id: 'language.backToList' })}
          </Button>
          <Button htmlType="submit" icon={<SaveOutlined />} loading={save.isPending} type="primary">
            {intl.formatMessage({ id: 'actions.save' })}
          </Button>
        </Space>
      </Space>
    </Form>
  );
}
