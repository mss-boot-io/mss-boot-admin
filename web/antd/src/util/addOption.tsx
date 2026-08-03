import { ProColumns } from '@ant-design/pro-components';
import React from 'react';
import { Access } from '@/components/MssBoot/Access';
import { Button, message, Popconfirm } from 'antd';
import { FormattedMessage, Link } from '@umijs/max';
// @ts-ignore
import { IntlShape } from 'react-intl';
import { deleteVirtualModel } from '@/pages/Virtual/service/virtual';

export const addOption = (
  intl: IntlShape,
  pathname: string,
  key: string | undefined,
  actionRef: any,
  columns: ProColumns<{ [key: string]: any }>[],
): ProColumns<{ [key: string]: any }>[] => {
  columns.push({
    title: <FormattedMessage id="pages.title.option" defaultMessage="Option" />,
    dataIndex: 'option',
    valueType: 'option',
    hideInDescriptions: true,
    hideInForm: true,
    render: (_, record) => [
      <Access key={`/${key}/edit`}>
        <Link to={`${pathname}/${record.id}`}>
          <Button key="edit">
            <FormattedMessage id="pages.title.edit" defaultMessage="Edit" />
          </Button>
        </Link>
      </Access>,
      <Access key={`${key}/delete`}>
        <Popconfirm
          key="delete"
          title={intl.formatMessage({ id: 'pages.title.delete', defaultMessage: 'Delete' })}
          description={intl.formatMessage({
            id: 'pages.description.delete.confirm',
            defaultMessage: 'Are you sure to delete this record?',
          })}
          onConfirm={async () => {
            const res = await deleteVirtualModel({ id: record.id!, key });
            if (!res) {
              message.success(
                intl.formatMessage({
                  id: 'pages.message.delete.success',
                  defaultMessage: 'Delete successfully!',
                }),
              );
              actionRef.current?.reload();
            }
          }}
          okText={intl.formatMessage({ id: 'pages.title.ok', defaultMessage: 'OK' })}
          cancelText={intl.formatMessage({ id: 'pages.title.cancel', defaultMessage: 'Cancel' })}
        >
          <Button key="delete.button">
            <FormattedMessage id="pages.title.delete" defaultMessage="Delete" />
          </Button>
        </Popconfirm>
      </Access>,
    ],
  });
  return columns;
};
