import CameraOutlined from '@ant-design/icons/CameraOutlined';
import SaveOutlined from '@ant-design/icons/SaveOutlined';
import UserOutlined from '@ant-design/icons/UserOutlined';
import { getRequestErrorMessage } from '@mss-admin-core/shared/api/client';
import { fetchCurrentUser } from '@mss-admin-core/shared/auth/session';
import type { CurrentUser, InitialState } from '@mss-admin-core/shared/auth/types';
import { PageError, PageLoading } from '@mss-admin-core/shared/design-system/PageState';
import { queryClient, queryKeys } from '@mss-admin-core/shared/query/client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import type { UploadProps } from 'antd';
import { Alert, App, Avatar, Button, Col, Form, Input, Row, Select, Space, Upload } from 'antd';
import { useEffect } from 'react';
import { accountAPI } from './api';
import { buildProfileUpdate, type ProfileUpdate } from './contracts';

type ProfileFormValues = ProfileUpdate & {
  email?: string;
  username?: string;
};

function AvatarField({ onChange, value }: { onChange?: (value: string) => void; value?: string }) {
  const intl = useIntl();
  const { message } = App.useApp();
  const upload: NonNullable<UploadProps['customRequest']> = async ({
    file,
    onError,
    onSuccess,
  }) => {
    if (!(file instanceof Blob)) {
      onError?.(new Error('Avatar file is invalid'));
      return;
    }
    try {
      const avatar = await accountAPI.uploadAvatar(file);
      onChange?.(avatar);
      onSuccess?.({ avatar });
    } catch (error) {
      onError?.(error instanceof Error ? error : new Error('Avatar upload failed'));
    }
  };

  return (
    <Upload
      accept="image/png,image/jpeg,image/webp"
      beforeUpload={(file) => {
        if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
          void message.error(intl.formatMessage({ id: 'account.profile.avatarType' }));
          return Upload.LIST_IGNORE;
        }
        if (file.size > 2 * 1024 * 1024) {
          void message.error(intl.formatMessage({ id: 'account.profile.avatarSize' }));
          return Upload.LIST_IGNORE;
        }
        return true;
      }}
      customRequest={upload}
      maxCount={1}
      showUploadList={false}
    >
      <Button type="text" className="h-auto p-2">
        <Space orientation="vertical" size="small">
          <Avatar size={88} src={value || undefined} icon={<UserOutlined />} />
          <span>
            <CameraOutlined /> {intl.formatMessage({ id: 'account.profile.avatarAction' })}
          </span>
        </Space>
      </Button>
    </Upload>
  );
}

export default function ProfilePanel() {
  const intl = useIntl();
  const { message } = App.useApp();
  const { initialState, setInitialState } = useModel('@@initialState') as {
    initialState?: InitialState;
    setInitialState: (
      state: InitialState | ((previous?: InitialState) => InitialState | undefined),
    ) => Promise<void>;
  };
  const [form] = Form.useForm<ProfileFormValues>();
  const currentUser = useQuery({
    queryKey: queryKeys.currentUser,
    queryFn: async () => (await fetchCurrentUser()) ?? null,
    initialData: initialState?.currentUser,
  });
  const update = useMutation({
    mutationFn: async (values: ProfileFormValues) => {
      await accountAPI.updateProfile(buildProfileUpdate(values as Partial<CurrentUser>));
      await queryClient.invalidateQueries({ queryKey: queryKeys.currentUser });
      const refreshed = await queryClient.fetchQuery({
        queryKey: queryKeys.currentUser,
        queryFn: async () => (await fetchCurrentUser()) ?? null,
        staleTime: 0,
      });
      if (!refreshed) throw new Error('Current user disappeared after profile update');
      return refreshed;
    },
    onSuccess: async (user) => {
      await setInitialState((previous) =>
        previous ? { ...previous, currentUser: user } : previous,
      );
      void message.success(intl.formatMessage({ id: 'account.profile.saved' }));
    },
  });

  useEffect(() => {
    if (!currentUser.data) return;
    form.setFieldsValue({
      ...currentUser.data,
      tags: currentUser.data.tags ? [...currentUser.data.tags] : [],
    });
  }, [currentUser.data, form]);

  if (currentUser.isPending && !currentUser.data) return <PageLoading rows={8} />;
  if (currentUser.isError || !currentUser.data) {
    return (
      <PageError
        message={intl.formatMessage({ id: 'account.profile.loadFailed' })}
        onRetry={() => void currentUser.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }

  return (
    <Form<ProfileFormValues>
      form={form}
      layout="vertical"
      requiredMark="optional"
      onFinish={(values) => update.mutate(values)}
    >
      {update.isError ? (
        <Alert
          className="mb-5"
          showIcon
          type="error"
          title={intl.formatMessage({ id: 'account.profile.saveFailed' })}
          description={getRequestErrorMessage(update.error)}
        />
      ) : null}
      <Row gutter={[24, 0]}>
        <Col xs={24} lg={7}>
          <Form.Item name="avatar" label={intl.formatMessage({ id: 'account.profile.avatar' })}>
            <AvatarField />
          </Form.Item>
        </Col>
        <Col xs={24} lg={17}>
          <Row gutter={16}>
            <Col xs={24} md={12}>
              <Form.Item
                name="username"
                label={intl.formatMessage({ id: 'account.profile.username' })}
              >
                <Input disabled autoComplete="username" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                name="email"
                label={intl.formatMessage({ id: 'account.profile.email' })}
                extra={intl.formatMessage({ id: 'account.profile.emailReadOnly' })}
              >
                <Input disabled autoComplete="email" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                name="name"
                label={intl.formatMessage({ id: 'account.profile.name' })}
                rules={[{ required: true }, { max: 100 }]}
              >
                <Input autoComplete="name" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                name="phone"
                label={intl.formatMessage({ id: 'account.profile.phone' })}
                rules={[{ max: 20 }]}
              >
                <Input autoComplete="tel" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                name="title"
                label={intl.formatMessage({ id: 'account.profile.title' })}
                rules={[{ max: 100 }]}
              >
                <Input />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item
                name="group"
                label={intl.formatMessage({ id: 'account.profile.group' })}
                rules={[{ max: 255 }]}
              >
                <Input />
              </Form.Item>
            </Col>
            <Col xs={24} md={8}>
              <Form.Item
                name="country"
                label={intl.formatMessage({ id: 'account.profile.country' })}
                rules={[{ max: 20 }]}
              >
                <Input autoComplete="country-name" />
              </Form.Item>
            </Col>
            <Col xs={24} md={8}>
              <Form.Item
                name="province"
                label={intl.formatMessage({ id: 'account.profile.province' })}
                rules={[{ max: 20 }]}
              >
                <Input autoComplete="address-level1" />
              </Form.Item>
            </Col>
            <Col xs={24} md={8}>
              <Form.Item
                name="city"
                label={intl.formatMessage({ id: 'account.profile.city' })}
                rules={[{ max: 20 }]}
              >
                <Input autoComplete="address-level2" />
              </Form.Item>
            </Col>
          </Row>
        </Col>
        <Col span={24}>
          <Form.Item
            name="address"
            label={intl.formatMessage({ id: 'account.profile.address' })}
            rules={[{ max: 255 }]}
          >
            <Input autoComplete="street-address" />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item
            name="signature"
            label={intl.formatMessage({ id: 'account.profile.signature' })}
            rules={[{ max: 255 }]}
          >
            <Input />
          </Form.Item>
        </Col>
        <Col xs={24} md={12}>
          <Form.Item name="tags" label={intl.formatMessage({ id: 'account.profile.tags' })}>
            <Select mode="tags" tokenSeparators={[',']} />
          </Form.Item>
        </Col>
        <Col span={24}>
          <Form.Item name="profile" label={intl.formatMessage({ id: 'account.profile.profile' })}>
            <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} />
          </Form.Item>
        </Col>
      </Row>
      <Button htmlType="submit" type="primary" icon={<SaveOutlined />} loading={update.isPending}>
        {intl.formatMessage({ id: 'actions.save' })}
      </Button>
    </Form>
  );
}
