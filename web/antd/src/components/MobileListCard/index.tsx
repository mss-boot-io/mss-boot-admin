import React from 'react';
import { Card, Space, Tag, Button, Typography, Popconfirm } from 'antd';
import { EditOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons';
import styles from './MobileListCard.less';

const { Text, Paragraph } = Typography;

export interface MobileListCardColumn {
  title: string;
  dataIndex: string;
  render?: (value: any, record: any) => React.ReactNode;
  copyable?: boolean;
  tagColors?: Record<string, string>;
}

export interface MobileListCardProps {
  columns: MobileListCardColumn[];
  dataSource: any[];
  loading?: boolean;
  onView?: (record: any) => void;
  onEdit?: (record: any) => void;
  onDelete?: (record: any) => void;
  rowKey?: string;
  actions?: (record: any) => React.ReactNode[];
}

const MobileListCard: React.FC<MobileListCardProps> = ({
  columns,
  dataSource,
  loading,
  onView,
  onEdit,
  onDelete,
  rowKey = 'id',
  actions,
}) => {
  const renderValue = (col: MobileListCardColumn, record: any) => {
    const value = record[col.dataIndex];
    if (col.render) {
      return col.render(value, record);
    }
    if (col.tagColors && value) {
      const color = col.tagColors[value] || 'default';
      const displayValue = typeof value === 'boolean' 
        ? (value ? '启用' : '禁用')
        : String(value);
      return <Tag color={color}>{displayValue}</Tag>;
    }
    if (typeof value === 'boolean') {
      return <Tag color={value ? 'green' : 'red'}>{value ? '启用' : '禁用'}</Tag>;
    }
    if (value === null || value === undefined) {
      return <Text type="secondary">-</Text>;
    }
    if (col.copyable && typeof value === 'string' && value.length > 20) {
      return (
        <Paragraph copyable style={{ marginBottom: 0 }}>
          {value}
        </Paragraph>
      );
    }
    return String(value);
  };

  const defaultActions = (record: any) => {
    const actionList: React.ReactNode[] = [];
    if (onView) {
      actionList.push(
        <Button
          key="view"
          type="text"
          size="small"
          icon={<EyeOutlined />}
          onClick={() => onView(record)}
        />
      );
    }
    if (onEdit) {
      actionList.push(
        <Button
          key="edit"
          type="text"
          size="small"
          icon={<EditOutlined />}
          onClick={() => onEdit(record)}
        />
      );
    }
    if (onDelete) {
      actionList.push(
        <Popconfirm
          key="delete-confirm"
          title="确定要删除吗？"
          onConfirm={() => onDelete(record)}
          okText="确定"
          cancelText="取消"
        >
          <Button
            type="text"
            size="small"
            danger
            icon={<DeleteOutlined />}
          />
        </Popconfirm>
      );
    }
    return actionList;
  };

  return (
    <div className={styles.mobileListCard}>
      {dataSource.map((record) => (
        <Card
          key={record[rowKey]}
          className={styles.card}
          loading={loading}
          size="small"
          actions={actions ? actions(record) : defaultActions(record)}
        >
          <Space direction="vertical" style={{ width: '100%' }} size="small">
            {columns.map((col) => (
              <div key={col.dataIndex} className={styles.fieldRow}>
                <Text type="secondary" className={styles.label}>
                  {col.title}:
                </Text>
                <div className={styles.value}>
                  {renderValue(col, record)}
                </div>
              </div>
            ))}
          </Space>
        </Card>
      ))}
      {dataSource.length === 0 && !loading && (
        <div className={styles.empty}>
          <Text type="secondary">暂无数据</Text>
        </div>
      )}
    </div>
  );
};

export default MobileListCard;