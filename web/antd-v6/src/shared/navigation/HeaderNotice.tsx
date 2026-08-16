import BellOutlined from '@ant-design/icons/BellOutlined';
import { useQuery } from '@tanstack/react-query';
import { Link, useIntl } from '@umijs/max';
import { Badge, Tooltip } from 'antd';
import { operationsAPI } from '@/modules/operations/api';
import { hasPermission } from '@/shared/auth/access';
import type { CurrentUser } from '@/shared/auth/types';
import { queryKeys } from '@/shared/query/client';

export default function HeaderNotice({ user }: { user?: CurrentUser }) {
  const intl = useIntl();
  const enabled = hasPermission(user, '/notice');
  const unread = useQuery({
    queryKey: queryKeys.noticeUnread,
    queryFn: operationsAPI.notices.unread,
    enabled,
    refetchInterval: 60_000,
    staleTime: 30_000,
  });

  if (!enabled) return null;

  const label = intl.formatMessage({ id: 'notice.header.label' });
  return (
    <Tooltip title={label}>
      <Link
        aria-label={label}
        className="inline-flex h-8 w-8 items-center justify-center rounded-full text-base text-current hover:bg-black/5"
        to="/notice"
      >
        <Badge count={unread.data?.length ?? 0} overflowCount={99} size="small">
          <BellOutlined />
        </Badge>
      </Link>
    </Tooltip>
  );
}
