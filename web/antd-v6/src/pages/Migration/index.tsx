import { CheckCircleOutlined, ClockCircleOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { List, Tag, Typography } from 'antd';

type CapabilityState = 'ready' | 'active' | 'planned';

const capabilities: Array<{ name: string; state: CapabilityState }> = [
  { name: '独立依赖、构建与发布契约', state: 'ready' },
  { name: 'React 19 / antd 6 / Pro v3 应用壳', state: 'ready' },
  { name: 'Cookie、CSRF 与 WebSocket ticket 后端', state: 'active' },
  { name: '身份、RBAC、动态授权菜单与主题', state: 'active' },
  { name: '组织、配置、任务、通知与观测模块', state: 'planned' },
  { name: 'Supplier v6 生成器 golden', state: 'planned' },
];

export default function MigrationPage() {
  return (
    <PageContainer title="Ant Design 6 迁移状态">
      <ProCard variant="outlined">
        <Typography.Paragraph>
          这是实现期诊断页面，只陈述已交付门禁，不用占位页冒充业务功能。未完成模块不会进入授权菜单。
        </Typography.Paragraph>
        <List
          dataSource={capabilities}
          renderItem={(item) => (
            <List.Item
              extra={
                item.state === 'ready' ? (
                  <Tag icon={<CheckCircleOutlined />} color="success">
                    已建立
                  </Tag>
                ) : (
                  <Tag icon={<ClockCircleOutlined />} color="processing">
                    {item.state === 'active' ? '实施中' : '待迁移'}
                  </Tag>
                )
              }
            >
              {item.name}
            </List.Item>
          )}
        />
      </ProCard>
    </PageContainer>
  );
}
