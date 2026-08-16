import CheckCircleOutlined from '@ant-design/icons/CheckCircleOutlined';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { Tag, Typography } from 'antd';
import { useRuntimeMessage } from '@/shared/i18n/runtime';

const capabilityMessageIDs = [
  'migration.capability.release',
  'migration.capability.shell',
  'migration.capability.session',
  'migration.capability.authorization',
  'migration.capability.business',
  'migration.capability.generator',
];

export default function MigrationPage() {
  const formatMessage = useRuntimeMessage();
  return (
    <PageContainer title={formatMessage('migration.title')}>
      <ProCard variant="outlined">
        <Typography.Paragraph>{formatMessage('migration.description')}</Typography.Paragraph>
        <ul className="m-0 list-none divide-y divide-[var(--mss-color-split)] p-0">
          {capabilityMessageIDs.map((messageID) => (
            <li className="flex items-center justify-between gap-4 py-3" key={messageID}>
              <span>{formatMessage(messageID)}</span>
              <Tag icon={<CheckCircleOutlined />} color="success">
                {formatMessage('migration.status.ready')}
              </Tag>
            </li>
          ))}
        </ul>
      </ProCard>
    </PageContainer>
  );
}
