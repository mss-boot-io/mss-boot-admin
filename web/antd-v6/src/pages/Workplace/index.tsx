import { ApiOutlined, DeploymentUnitOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { useModel } from '@umijs/max';
import { Alert, Col, Row, Statistic, Tag, Typography } from 'antd';

export default function WorkplacePage() {
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;

  return (
    <PageContainer
      title={`欢迎，${currentUser?.name ?? currentUser?.username ?? 'MSS User'}`}
      content="Ant Design 6 独立应用基础已启用；业务能力按契约进行垂直迁移。"
      extra={<Tag color="blue">antd {__ANTD_VERSION__}</Tag>}
    >
      <Alert
        className="mb-6"
        showIcon
        type="info"
        message="独立发布边界"
        description="本应用拥有独立 lockfile、构建产物、镜像、tag 和回滚历史，不会覆盖 web/antd。"
      />
      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <ProCard variant="outlined">
            <Statistic title="UI Runtime" value="6.6.0" prefix={<DeploymentUnitOutlined />} />
          </ProCard>
        </Col>
        <Col xs={24} md={8}>
          <ProCard variant="outlined">
            <Statistic title="API Prefix" value="/admin/api" prefix={<ApiOutlined />} />
          </ProCard>
        </Col>
        <Col xs={24} md={8}>
          <ProCard variant="outlined">
            <Statistic
              title="Session Target"
              value="HttpOnly"
              prefix={<SafetyCertificateOutlined />}
            />
          </ProCard>
        </Col>
      </Row>
      <ProCard className="mt-4" variant="outlined" title="工程原则">
        <Typography.Paragraph>
          服务器状态由 React Query 管理；主题通过 CSS Variables 和语义 Token
          定制；菜单只能映射到编译期注册路由；权限最终由后端执行。
        </Typography.Paragraph>
      </ProCard>
    </PageContainer>
  );
}
