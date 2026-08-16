import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import type { TableColumnsType } from 'antd';
import {
  Alert,
  App,
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
} from 'antd';
import { useMemo, useState } from 'react';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { queryKeys } from '@/shared/query/client';
import AdministrationTable, { AdministrationStatusTag } from './AdministrationTable';
import { administrationAPI } from './api';
import {
  type AdministrationListParams,
  administrationSelectOptions,
  type DataScope,
  flattenAdministrationTree,
  type PostSummary,
  type PostWriteValues,
} from './contract';
import { useAdministrationPage } from './query';

interface PostManagementProps {
  canCreate: boolean;
  canDelete: boolean;
  canEdit: boolean;
}

const initialParams: AdministrationListParams = {
  current: 1,
  pageSize: 20,
  status: 'all',
};

const dataScopes: DataScope[] = [
  'all',
  'currentDept',
  'currentAndChildrenDept',
  'customDept',
  'self',
  'selfAndChildren',
  'selfAndAllChildren',
];

export default function PostManagement({ canCreate, canDelete, canEdit }: PostManagementProps) {
  const intl = useIntl();
  const { message } = App.useApp();
  const client = useQueryClient();
  const [params, setParams] = useState(initialParams);
  const posts = useAdministrationPage('posts', params);
  const dependencyParams = useMemo<AdministrationListParams>(
    () => ({ current: 1, pageSize: 100, status: 'enabled' }),
    [],
  );
  const departments = useAdministrationPage('departments', dependencyParams);
  const [editing, setEditing] = useState<PostSummary | 'create'>();
  const [form] = Form.useForm<PostWriteValues>();
  const selectedScope = Form.useWatch('dataScope', form);

  const save = useMutation({
    mutationFn: (values: PostWriteValues) =>
      editing === 'create'
        ? administrationAPI.posts.create(values)
        : administrationAPI.posts.update((editing as PostSummary).id, values),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: queryKeys.administration('posts') });
      setEditing(undefined);
      form.resetFields();
      void message.success(intl.formatMessage({ id: 'administration.save.success' }));
    },
  });
  const remove = useMutation({
    mutationFn: (post: PostSummary) => administrationAPI.posts.remove(post.id),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: queryKeys.administration('posts') });
      void message.success(intl.formatMessage({ id: 'administration.delete.success' }));
    },
  });

  const disabledParents = useMemo(() => {
    if (!editing || editing === 'create') return new Set<string>();
    return new Set(flattenAdministrationTree([editing]).map((item) => item.id));
  }, [editing]);
  const parentOptions = useMemo(
    () => administrationSelectOptions(posts.data?.data ?? [], { disabled: disabledParents }),
    [posts.data?.data, disabledParents],
  );
  const departmentOptions = useMemo(
    () => administrationSelectOptions(departments.data?.data ?? []),
    [departments.data?.data],
  );

  const openEditor = (post: PostSummary | 'create') => {
    setEditing(post);
    save.reset();
    if (post === 'create') {
      form.setFieldsValue({ dataScope: 'self', deptIDS: [], status: 'enabled', sort: 0 });
      return;
    }
    form.setFieldsValue({
      parentID: post.parentID,
      name: post.name,
      code: post.code,
      dataScope: post.dataScope,
      deptIDS: post.deptIDS,
      status: post.status,
      sort: post.sort,
    });
  };

  const columns: TableColumnsType<PostSummary> = [
    {
      title: intl.formatMessage({ id: 'administration.field.name' }),
      dataIndex: 'name',
      width: 240,
    },
    {
      title: intl.formatMessage({ id: 'post.field.code' }),
      dataIndex: 'code',
      width: 160,
    },
    {
      title: intl.formatMessage({ id: 'post.field.dataScope' }),
      dataIndex: 'dataScope',
      responsive: ['md'],
      render: (_, post) => intl.formatMessage({ id: `post.dataScope.${post.dataScope}` }),
    },
    {
      title: intl.formatMessage({ id: 'administration.field.status' }),
      dataIndex: 'status',
      width: 120,
      render: (_, post) => <AdministrationStatusTag status={post.status} />,
    },
    {
      title: intl.formatMessage({ id: 'administration.field.sort' }),
      dataIndex: 'sort',
      width: 90,
      responsive: ['lg'],
    },
    {
      title: intl.formatMessage({ id: 'administration.field.actions' }),
      key: 'actions',
      width: 190,
      fixed: 'right',
      render: (_, post) => (
        <Space size="small">
          {canEdit ? (
            <Button size="small" type="link" onClick={() => openEditor(post)}>
              {intl.formatMessage({ id: 'actions.edit' })}
            </Button>
          ) : null}
          {canDelete ? (
            <Popconfirm
              description={intl.formatMessage({ id: 'post.delete.description' })}
              title={intl.formatMessage({ id: 'post.delete.confirm' })}
              onConfirm={() => remove.mutate(post)}
            >
              <Button danger size="small" type="link">
                {intl.formatMessage({ id: 'actions.delete' })}
              </Button>
            </Popconfirm>
          ) : null}
        </Space>
      ),
    },
  ];

  return (
    <>
      <AdministrationTable
        columns={columns}
        emptyText={intl.formatMessage({ id: 'post.empty' })}
        params={params}
        query={posts}
        setParams={setParams}
        toolbar={
          canCreate ? (
            <Button type="primary" onClick={() => openEditor('create')}>
              {intl.formatMessage({ id: 'post.create.action' })}
            </Button>
          ) : null
        }
      />
      <Modal
        destroyOnHidden
        forceRender
        confirmLoading={save.isPending}
        open={Boolean(editing)}
        title={intl.formatMessage({
          id: editing === 'create' ? 'post.create.title' : 'post.edit.title',
        })}
        width={680}
        onCancel={() => {
          setEditing(undefined);
          form.resetFields();
          save.reset();
        }}
        onOk={() => form.submit()}
      >
        {save.isError ? (
          <Alert
            className="mb-4"
            description={getRequestErrorMessage(save.error)}
            showIcon
            type={getRequestStatus(save.error) === 409 ? 'warning' : 'error'}
          />
        ) : null}
        <Form<PostWriteValues>
          form={form}
          layout="vertical"
          onFinish={(values) => save.mutate(values)}
        >
          <Form.Item
            name="parentID"
            label={intl.formatMessage({ id: 'administration.field.parent' })}
          >
            <Select allowClear showSearch optionFilterProp="label" options={parentOptions} />
          </Form.Item>
          <Form.Item
            name="name"
            label={intl.formatMessage({ id: 'administration.field.name' })}
            rules={[{ required: true }, { max: 255 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="code"
            label={intl.formatMessage({ id: 'post.field.code' })}
            rules={[{ required: true }, { max: 255 }]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            name="dataScope"
            label={intl.formatMessage({ id: 'post.field.dataScope' })}
            rules={[{ required: true }]}
          >
            <Select
              options={dataScopes.map((value) => ({
                value,
                label: intl.formatMessage({ id: `post.dataScope.${value}` }),
              }))}
            />
          </Form.Item>
          {selectedScope === 'customDept' ? (
            <Form.Item
              name="deptIDS"
              label={intl.formatMessage({ id: 'post.field.departments' })}
              rules={[{ required: true }]}
            >
              <Select
                mode="multiple"
                showSearch
                optionFilterProp="label"
                options={departmentOptions}
              />
            </Form.Item>
          ) : null}
          <Form.Item
            name="status"
            label={intl.formatMessage({ id: 'administration.field.status' })}
            rules={[{ required: true }]}
          >
            <Select
              options={(['enabled', 'disabled'] as const).map((value) => ({
                value,
                label: intl.formatMessage({ id: `administration.status.${value}` }),
              }))}
            />
          </Form.Item>
          <Form.Item name="sort" label={intl.formatMessage({ id: 'administration.field.sort' })}>
            <InputNumber className="w-full" min={-1_000_000} max={1_000_000} precision={0} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
