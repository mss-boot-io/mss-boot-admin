import { Access } from '@/components/MssBoot/Access';
import { getRuntimeLogs, exportRuntimeLogs } from '@/services/admin/log';
import { getLoginLogs } from '@/services/admin/loginLog';
import { getAuditLogs } from '@/services/admin/auditLog';
import { DownloadOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Tabs, Tag, message } from 'antd';
import React, { useRef } from 'react';
import { fieldIntl } from '@/util/fieldIntl';

const levelColors: Record<string, string> = {
  info: 'blue',
  warn: 'orange',
  error: 'red',
  debug: 'green',
};

const statusColors: Record<string, string> = {
  enabled: 'green',
  success: 'green',
  disabled: 'red',
  failed: 'red',
};

const RuntimeLogTab: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const columns: ProColumns<API.LogEntry>[] = [
    {
      title: fieldIntl(intl, 'timestamp'),
      dataIndex: 'timestamp',
      width: 180,
      hideInSearch: true,
    },
    {
      title: fieldIntl(intl, 'level'),
      dataIndex: 'level',
      width: 100,
      valueType: 'select',
      valueEnum: {
        info: { text: 'INFO' },
        warn: { text: 'WARN' },
        error: { text: 'ERROR' },
        debug: { text: 'DEBUG' },
      },
      render: (_, record) => {
        const color = levelColors[record.level ?? ''] || 'default';
        return <Tag color={color}>{record.level?.toUpperCase()}</Tag>;
      },
    },
    {
      title: fieldIntl(intl, 'message'),
      dataIndex: 'message',
      ellipsis: true,
      hideInSearch: true,
    },
    {
      title: fieldIntl(intl, 'keyword'),
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: {
        placeholder: intl.formatMessage({
          id: 'pages.log.keyword.placeholder',
          defaultMessage: '搜索关键词',
        }),
      },
    },
  ];

  return (
    <ProTable<API.LogEntry, API.LogSearchParams>
      headerTitle={intl.formatMessage({ id: 'pages.log.runtime', defaultMessage: '运行时日志' })}
      actionRef={actionRef}
      rowKey="timestamp"
      search={{ labelWidth: 'auto' }}
      toolBarRender={() => [
        <Button key="refresh" icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>
          <FormattedMessage id="pages.log.refresh" defaultMessage="刷新" />
        </Button>,
        <Access key="export" accessible={false}>
          <Button
            type="primary"
            icon={<DownloadOutlined />}
            onClick={async () => {
              try {
                const res = await exportRuntimeLogs();
                window.open(res);
                message.success(
                  intl.formatMessage({
                    id: 'pages.log.export.success',
                    defaultMessage: '导出成功',
                  }),
                );
              } catch (e) {
                message.error(
                  intl.formatMessage({ id: 'pages.log.export.failed', defaultMessage: '导出失败' }),
                );
              }
            }}
          >
            <FormattedMessage id="pages.log.export" defaultMessage="导出" />
          </Button>
        </Access>,
      ]}
      request={async (params) => {
        const res = await getRuntimeLogs({
          current: params.current,
          pageSize: params.pageSize,
          level: params.level,
          keyword: params.keyword,
        });
        return { data: res?.list || [], total: res?.total || 0, success: true };
      }}
      columns={columns}
      expandable={{
        expandedRowRender: (record) => (
          <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-all', fontSize: 12 }}>
            {record.raw}
          </pre>
        ),
      }}
    />
  );
};

const LoginLogTab: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const columns: ProColumns<API.LoginLog>[] = [
    {
      title: fieldIntl(intl, 'username'),
      dataIndex: 'username',
      width: 150,
    },
    {
      title: fieldIntl(intl, 'ip'),
      dataIndex: 'ip',
      width: 140,
      hideInSearch: true,
    },
    {
      title: fieldIntl(intl, 'location'),
      dataIndex: 'location',
      width: 150,
      hideInSearch: true,
    },
    {
      title: fieldIntl(intl, 'status'),
      dataIndex: 'status',
      width: 100,
      valueType: 'select',
      valueEnum: {
        enabled: {
          text: intl.formatMessage({ id: 'pages.status.success', defaultMessage: '成功' }),
        },
        disabled: {
          text: intl.formatMessage({ id: 'pages.status.failed', defaultMessage: '失败' }),
        },
      },
      render: (_, record) => {
        const color = statusColors[record.status ?? ''] || 'default';
        return (
          <Tag color={color}>
            {record.status === 'enabled'
              ? intl.formatMessage({ id: 'pages.status.success', defaultMessage: '成功' })
              : intl.formatMessage({ id: 'pages.status.failed', defaultMessage: '失败' })}
          </Tag>
        );
      },
    },
    {
      title: fieldIntl(intl, 'message'),
      dataIndex: 'message',
      ellipsis: true,
      hideInSearch: true,
    },
    {
      title: fieldIntl(intl, 'loginAt'),
      dataIndex: 'loginAt',
      width: 180,
      valueType: 'dateTime',
      hideInSearch: true,
    },
  ];

  return (
    <ProTable<API.LoginLog, API.LoginLogSearchParams>
      headerTitle={intl.formatMessage({ id: 'pages.log.login', defaultMessage: '登录日志' })}
      actionRef={actionRef}
      rowKey="id"
      search={{ labelWidth: 'auto' }}
      toolBarRender={() => [
        <Button key="refresh" icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>
          <FormattedMessage id="pages.log.refresh" defaultMessage="刷新" />
        </Button>,
      ]}
      request={async (params) => {
        const res = await getLoginLogs({
          current: params.current,
          pageSize: params.pageSize,
          username: params.username,
        });
        return { data: res?.data || [], total: res?.total || 0, success: true };
      }}
      columns={columns}
    />
  );
};

const AuditLogTab: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const columns: ProColumns<API.AuditLog>[] = [
    {
      title: fieldIntl(intl, 'username'),
      dataIndex: 'username',
      width: 120,
    },
    {
      title: fieldIntl(intl, 'type'),
      dataIndex: 'type',
      width: 100,
      valueType: 'select',
      valueEnum: {
        login: { text: intl.formatMessage({ id: 'pages.log.type.login', defaultMessage: '登录' }) },
        logout: {
          text: intl.formatMessage({ id: 'pages.log.type.logout', defaultMessage: '登出' }),
        },
        create: {
          text: intl.formatMessage({ id: 'pages.log.type.create', defaultMessage: '创建' }),
        },
        update: {
          text: intl.formatMessage({ id: 'pages.log.type.update', defaultMessage: '更新' }),
        },
        delete: {
          text: intl.formatMessage({ id: 'pages.log.type.delete', defaultMessage: '删除' }),
        },
        export: {
          text: intl.formatMessage({ id: 'pages.log.type.export', defaultMessage: '导出' }),
        },
        import: {
          text: intl.formatMessage({ id: 'pages.log.type.import', defaultMessage: '导入' }),
        },
        config: {
          text: intl.formatMessage({ id: 'pages.log.type.config', defaultMessage: '配置' }),
        },
        security: {
          text: intl.formatMessage({ id: 'pages.log.type.security', defaultMessage: '安全' }),
        },
      },
    },
    {
      title: fieldIntl(intl, 'action'),
      dataIndex: 'action',
      width: 150,
      hideInSearch: true,
    },
    {
      title: fieldIntl(intl, 'resource'),
      dataIndex: 'resource',
      width: 150,
      ellipsis: true,
      hideInSearch: true,
    },
    {
      title: fieldIntl(intl, 'ip'),
      dataIndex: 'ip',
      width: 140,
      hideInSearch: true,
    },
    {
      title: fieldIntl(intl, 'status'),
      dataIndex: 'status',
      width: 80,
      valueType: 'select',
      valueEnum: {
        enabled: {
          text: intl.formatMessage({ id: 'pages.status.success', defaultMessage: '成功' }),
        },
        disabled: {
          text: intl.formatMessage({ id: 'pages.status.failed', defaultMessage: '失败' }),
        },
      },
      render: (_, record) => {
        const color = statusColors[record.status ?? ''] || 'default';
        return (
          <Tag color={color}>
            {record.status === 'enabled'
              ? intl.formatMessage({ id: 'pages.status.success', defaultMessage: '成功' })
              : intl.formatMessage({ id: 'pages.status.failed', defaultMessage: '失败' })}
          </Tag>
        );
      },
    },
    {
      title: fieldIntl(intl, 'message'),
      dataIndex: 'message',
      ellipsis: true,
      hideInSearch: true,
    },
    {
      title: fieldIntl(intl, 'duration'),
      dataIndex: 'duration',
      width: 100,
      hideInSearch: true,
      render: (_, record) => (record.duration ? `${record.duration}ms` : '-'),
    },
    {
      title: fieldIntl(intl, 'createdAt'),
      dataIndex: 'createdAt',
      width: 180,
      valueType: 'dateTime',
      hideInSearch: true,
    },
  ];

  return (
    <ProTable<API.AuditLog, API.AuditLogSearchParams>
      headerTitle={intl.formatMessage({ id: 'pages.log.audit', defaultMessage: '审计日志' })}
      actionRef={actionRef}
      rowKey="id"
      search={{ labelWidth: 'auto' }}
      toolBarRender={() => [
        <Button key="refresh" icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>
          <FormattedMessage id="pages.log.refresh" defaultMessage="刷新" />
        </Button>,
      ]}
      request={async (params) => {
        const res = await getAuditLogs({
          current: params.current,
          pageSize: params.pageSize,
          username: params.username,
          type: params.type,
        });
        return { data: res?.data || [], total: res?.total || 0, success: true };
      }}
      columns={columns}
    />
  );
};

const Log: React.FC = () => {
  const intl = useIntl();

  const items = [
    {
      key: 'login',
      label: intl.formatMessage({ id: 'pages.log.login', defaultMessage: '登录日志' }),
      children: <LoginLogTab />,
    },
    {
      key: 'audit',
      label: intl.formatMessage({ id: 'pages.log.audit', defaultMessage: '审计日志' }),
      children: <AuditLogTab />,
    },
    {
      key: 'runtime',
      label: intl.formatMessage({ id: 'pages.log.runtime', defaultMessage: '运行时日志' }),
      children: <RuntimeLogTab />,
    },
  ];

  return (
    <PageContainer>
      <Tabs items={items} />
    </PageContainer>
  );
};

export default Log;
