import { Access } from '@/components/MssBoot/Access';
import { getNotices, getNoticesId, putNoticeReadId } from '@/services/admin/notice';
import { idRender } from '@/util/columnOptions';
import { indexTitle } from '@/util/indexTitle';
import { useResponsive } from '@/hooks/useResponsive';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProDescriptions,
  ProDescriptionsItemProps,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, Link, useIntl, useParams, useSearchParams, history } from '@umijs/max';
import { Button, Drawer, message } from 'antd';
import React, { useRef, useState } from 'react';
import { fieldIntl } from '@/util/fieldIntl';
import MobileNoticeList from './Mobile/NoticeList';

const Index: React.FC = () => {
  const [showDetail, setShowDetail] = useState<boolean>(false);

  const actionRef = useRef<ActionType>();
  const [currentRow, setCurrentRow] = useState<API.Notice>();
  const { id } = useParams();
  const [searchParams] = useSearchParams();
  const { isMobile } = useResponsive();

  const intl = useIntl();

  const columns: ProColumns<API.Notice>[] = [
    {
      title: fieldIntl(intl, 'id'),
      dataIndex: 'id',
      hideInForm: true,
      render: (dom, entity) => {
        return idRender(dom, entity, setCurrentRow, setShowDetail);
      },
    },
    {
      title: fieldIntl(intl, 'title'),
      dataIndex: 'title',
      ellipsis: true,
      copyable: true,
    },
    {
      title: fieldIntl(intl, 'type'),
      dataIndex: 'type',
      width: '5%',
      valueEnum: {
        notification: {
          text: fieldIntl(intl, 'options.notification'),
          color: 'red',
          status: 'notification',
        },
        message: {
          text: fieldIntl(intl, 'options.message'),
          color: 'blue',
          status: 'message',
        },
        event: {
          text: fieldIntl(intl, 'options.event'),
          color: 'gold',
          status: 'event',
        },
      },
    },
    {
      title: fieldIntl(intl, 'status'),
      dataIndex: 'status',
      width: '5%',
      valueEnum: {
        urgent: {
          text: fieldIntl(intl, 'options.urgent'),
          color: 'red',
          status: 'urgent',
        },
        doing: {
          text: fieldIntl(intl, 'options.doing'),
          color: 'green',
          status: 'doing',
        },
        processing: {
          text: fieldIntl(intl, 'options.processing'),
          color: 'blue',
          status: 'processing',
        },
        todo: {
          text: fieldIntl(intl, 'options.todo'),
          color: 'gold',
          status: 'todo',
        },
      },
    },
    {
      title: fieldIntl(intl, 'datetime'),
      dataIndex: 'datetime',
      valueType: 'dateTime',
    },
    {
      title: fieldIntl(intl, 'description'),
      dataIndex: 'description',
      ellipsis: true,
    },
    {
      title: fieldIntl(intl, 'sendTime'),
      dataIndex: 'createdAt',
      valueType: 'dateTime',
      hideInSearch: true,
    },
    {
      title: <FormattedMessage id="pages.title.option" />,
      dataIndex: 'option',
      valueType: 'option',
      hideInDescriptions: true,
      width: '6%',
      hideInForm: true,
      render: (_, record) => [
        record.read ? (
          ''
        ) : (
          <Access key="/notice/read">
            <Button
              onClick={async () => {
                await putNoticeReadId({ id: record.id! });
                message
                  .success(
                    intl.formatMessage({
                      id: 'pages.title.notice.read',
                      defaultMessage: 'Mark as read',
                    }),
                  )
                  .then(() => actionRef.current?.reload());
              }}
            >
              <FormattedMessage id="pages.title.notice.read" defaultMessage="Mark as read" />
            </Button>
          </Access>
        ),
      ],
    },
  ];

  return (
    <PageContainer title={indexTitle(id)}>
      {isMobile && !id ? (
        <MobileNoticeList
          request={getNotices}
          onEdit={(record) => history.push(`/notice/${record.id}`)}
          onCreate={() => history.push('/notice/create')}
          onDelete={async () => {
            message.success(intl.formatMessage({ id: 'pages.message.delete.success' }));
          }}
        />
      ) : (
        <ProTable<API.Notice, API.getNoticesParams>
        headerTitle={intl.formatMessage({
          id: 'pages.notice.list.title',
          defaultMessage: 'Notice List',
        })}
        actionRef={actionRef}
        rowKey="id"
        search={{
          labelWidth: 120,
        }}
        toolBarRender={() => [
          <Access key="/notice/create">
            <Link to="/notice/create" key="create">
              <Button type="primary" key="create">
                <PlusOutlined /> <FormattedMessage id="pages.table.new" defaultMessage="New" />
              </Button>
            </Link>
          </Access>,
        ]}
        // @ts-ignore
        params={{ type: searchParams.get('type') }}
        request={getNotices}
        columns={columns}
      />
      )}

      <Drawer
        width={600}
        open={showDetail}
        onClose={() => {
          setCurrentRow(undefined);
          setShowDetail(false);
        }}
        closable={false}
      >
        {currentRow?.title && (
          <ProDescriptions<API.Notice>
            column={2}
            title={currentRow?.title}
            request={async (params) => {
              // @ts-ignore
              const res = await getNoticesId(params);
              res.title = currentRow?.title;
              return {
                data: res,
              };
            }}
            params={{
              id: currentRow?.id,
            }}
            columns={columns as ProDescriptionsItemProps<API.Notice>[]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default Index;
