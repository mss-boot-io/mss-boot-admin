import { Access } from '@/components/MssBoot/Access';
import { getNotices, getNoticesId, putNoticeReadId } from '@/services/admin/notice';
import { idRender } from '@/util/columnOptions';
import { useResponsive } from '@/hooks/useResponsive';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProDescriptions,
  ProDescriptionsItemProps,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl, useSearchParams } from '@umijs/max';
import { Button, Drawer, message } from 'antd';
import React, { useCallback, useRef, useState } from 'react';
import { fieldIntl } from '@/util/fieldIntl';
import MobileNoticeList from './Mobile/NoticeList';

type NoticeListParams = API.getNoticesParams & { type?: API.NoticeType };

const noticeTypes: readonly API.NoticeType[] = ['notification', 'message', 'event', 'mail'];
const isNoticeType = (value: string | null): value is API.NoticeType =>
  value !== null && noticeTypes.some((type) => type === value);

const Index: React.FC = () => {
  const [showDetail, setShowDetail] = useState<boolean>(false);

  const actionRef = useRef<ActionType>();
  const [currentRow, setCurrentRow] = useState<API.Notice>();
  const markingReadIDRef = useRef<string>();
  const [markingReadID, setMarkingReadID] = useState<string>();
  const [searchParams] = useSearchParams();
  const { isMobile } = useResponsive();

  const intl = useIntl();
  const typeParam = searchParams.get('type');
  const selectedType = isNoticeType(typeParam) ? typeParam : undefined;
  const requestMobileNotices = useCallback((params: { current: number; pageSize: number }) => {
    const noticeParams: NoticeListParams = { ...params, type: selectedType };
    return getNotices(noticeParams);
  }, [selectedType]);

  const requestMarkAsRead = async (record: API.Notice) => {
    if (!record.id) {
      return;
    }
    await putNoticeReadId({ id: record.id });
  };

  const markAsRead = async (record: API.Notice) => {
    if (!record.id || markingReadIDRef.current) {
      return;
    }

    markingReadIDRef.current = record.id;
    setMarkingReadID(record.id);
    try {
      await requestMarkAsRead(record);
      message.success(
        intl.formatMessage({
          id: 'pages.title.notice.read',
          defaultMessage: 'Mark as read',
        }),
      );
    } finally {
      markingReadIDRef.current = undefined;
      setMarkingReadID(undefined);
    }
  };

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
        mail: {
          text: fieldIntl(intl, 'options.email'),
          color: 'cyan',
          status: 'mail',
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
          null
        ) : (
          <Access key="/notice/read" permission="/notice/read">
            <Button
              loading={markingReadID === record.id}
              disabled={Boolean(markingReadID && markingReadID !== record.id)}
              onClick={async () => {
                await markAsRead(record);
                actionRef.current?.reload();
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
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.notice.list.title',
        defaultMessage: 'Notice List',
      })}
    >
      {isMobile ? (
        <MobileNoticeList
          request={requestMobileNotices}
          onView={(record) => {
            setCurrentRow(record);
            setShowDetail(true);
          }}
          onMarkRead={requestMarkAsRead}
        />
      ) : (
        <ProTable<API.Notice, NoticeListParams>
          headerTitle={intl.formatMessage({
            id: 'pages.notice.list.title',
            defaultMessage: 'Notice List',
          })}
          actionRef={actionRef}
          rowKey="id"
          search={{
            labelWidth: 120,
          }}
          params={{ type: selectedType }}
          request={getNotices}
          columns={columns}
        />
      )}

      <Drawer
        width={isMobile ? '100%' : 600}
        open={showDetail}
        onClose={() => {
          setCurrentRow(undefined);
          setShowDetail(false);
        }}
        closable
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
