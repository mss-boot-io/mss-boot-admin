import { getOnlineSession } from '@/services/admin/onlineSession';
import { useIntl } from '@umijs/max';
import { Descriptions, Drawer, Result, Spin, Tag } from 'antd';
import moment from 'moment';
import React, { useEffect, useState } from 'react';
import { getSessionStatus, STATUS_COLOR, STATUS_INTL_ID } from '../utils';

type Props = {
  id?: string;
  open: boolean;
  onClose: () => void;
};

const SessionDetailDrawer: React.FC<Props> = ({ id, open, onClose }) => {
  const intl = useIntl();
  const [data, setData] = useState<API.UserSession | undefined>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | undefined>();

  useEffect(() => {
    if (!open || !id) {
      setData(undefined);
      setError(undefined);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(undefined);
    getOnlineSession({ id }, { skipErrorHandler: true })
      .then((res) => {
        if (!cancelled) setData(res);
      })
      .catch((err) => {
        if (!cancelled) setError(err);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, open]);

  const fmtTime = (t?: string) => (t ? moment(t).format('YYYY-MM-DD HH:mm:ss') : '—');
  const status = data ? getSessionStatus(data) : undefined;

  return (
    <Drawer
      title={intl.formatMessage({ id: 'pages.onlineSession.drawer.title' })}
      open={open}
      onClose={onClose}
      width={520}
      destroyOnClose
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: 48 }}>
          <Spin />
        </div>
      ) : error ? (
        <Result
          status="error"
          title={
            error.message || intl.formatMessage({ id: 'pages.onlineSession.result.detail.error' })
          }
        />
      ) : data ? (
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.columns.username' })}
          >
            {data.username || '—'}
          </Descriptions.Item>
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.columns.userID' })}
          >
            {data.userID || '—'}
          </Descriptions.Item>
          <Descriptions.Item label={intl.formatMessage({ id: 'pages.onlineSession.columns.ip' })}>
            {data.ip || '—'}
          </Descriptions.Item>
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.columns.userAgent' })}
          >
            <span style={{ wordBreak: 'break-all' }}>{data.userAgent || '—'}</span>
          </Descriptions.Item>
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.columns.loginAt' })}
          >
            {fmtTime(data.loginAt)}
          </Descriptions.Item>
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.columns.lastSeenAt' })}
          >
            {fmtTime(data.lastSeenAt)}
          </Descriptions.Item>
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.columns.expiredAt' })}
          >
            {fmtTime(data.expiredAt)}
          </Descriptions.Item>
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.columns.status' })}
          >
            {status ? (
              <Tag color={STATUS_COLOR[status]}>
                {intl.formatMessage({ id: STATUS_INTL_ID[status] })}
              </Tag>
            ) : (
              '—'
            )}
          </Descriptions.Item>
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.drawer.field.revokedBy' })}
          >
            {data.revokedBy || '—'}
          </Descriptions.Item>
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.drawer.field.revokedAt' })}
          >
            {fmtTime(data.revokedAt)}
          </Descriptions.Item>
          <Descriptions.Item
            label={intl.formatMessage({ id: 'pages.onlineSession.drawer.field.revokeReason' })}
          >
            {data.revokeReason || '—'}
          </Descriptions.Item>
        </Descriptions>
      ) : null}
    </Drawer>
  );
};

export default SessionDetailDrawer;
