import { deleteOnlineSessionUser } from '@/services/admin/onlineSession';
import { useIntl } from '@umijs/max';
import { Form, Input, Modal, message } from 'antd';
import React, { useEffect, useState } from 'react';

type Props = {
  open: boolean;
  /** 预填的 userID（行操作场景），不传则用户手动输入 */
  presetUserID?: string;
  onClose: () => void;
  onSuccess?: () => void;
};

const RevokeUserModal: React.FC<Props> = ({ open, presetUserID, onClose, onSuccess }) => {
  const intl = useIntl();
  const [form] = Form.useForm<{ userID: string }>();
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      form.setFieldsValue({ userID: presetUserID ?? '' });
    }
  }, [open, presetUserID, form]);

  const handleOk = async () => {
    const { userID } = await form.validateFields();
    setSubmitting(true);
    try {
      await deleteOnlineSessionUser({ userID });
      message.success(intl.formatMessage({ id: 'pages.onlineSession.result.revokeUser.success' }));
      onSuccess?.();
      onClose();
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={intl.formatMessage({ id: 'pages.onlineSession.confirm.revokeUser.title' })}
      open={open}
      onCancel={onClose}
      onOk={handleOk}
      confirmLoading={submitting}
      destroyOnClose
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          name="userID"
          label={intl.formatMessage({ id: 'pages.onlineSession.confirm.revokeUser.userIDLabel' })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.onlineSession.confirm.revokeUser.userIDRequired',
              }),
            },
          ]}
        >
          <Input disabled={!!presetUserID} autoFocus={!presetUserID} />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default RevokeUserModal;
