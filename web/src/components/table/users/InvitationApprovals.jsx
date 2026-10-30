import React, { useEffect, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

export default function InvitationApprovals() {
  const { t } = useTranslation();
  const [data, setData] = useState(null);
  const [status, setStatus] = useState('pending');
  const [loading, setLoading] = useState(false);
  const [acting, setActing] = useState(false);
  const [invitation, setInvitation] = useState(null);
  async function load() {
    setLoading(true);
    try {
      const { data: response } = await API.get(
        `/api/registration/?status=${status}`,
      );
      if (response.success) setData(response.data);
      else showError(response.message);
    } catch {
      showError(t('读取注册申请失败'));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
  }, [status]);
  async function action(path, body) {
    if (acting) return;
    setActing(true);
    try {
      const { data: response } = await API.post(path, body);
      if (!response.success) {
        showError(response.message);
        return;
      }
      showSuccess(t('操作成功'));
      await load();
    } catch (error) {
      showError(error.response?.data?.message || t('操作失败'));
    } finally {
      setActing(false);
    }
  }
  function review(row, approve) {
    Modal.confirm({
      title: approve ? t('批准注册申请') : t('拒绝注册申请'),
      content: `${row.username} · ${row.email}`,
      onOk: () =>
        action(`/api/registration/requests/${row.id}/review`, {
          action: approve ? 'approve' : 'reject',
        }),
    });
  }
  async function create(values) {
    setActing(true);
    try {
      const { data: response } = await API.post(
        '/api/registration/invitations',
        values,
      );
      if (response.success) setInvitation(response.data);
      else showError(response.message);
    } catch {
      showError(t('创建邀请失败'));
    } finally {
      setActing(false);
      await load();
    }
  }
  if (!data) return null;
  return (
    <Card title={t('邀请与注册审批')} className='mb-4'>
      {!data.enabled && (
        <Banner
          type='warning'
          className='mb-3'
          description={t(
            '邀请注册策略尚未启用，新的注册仍按当前平台设置处理。',
          )}
        />
      )}
      {!data.notification_ready && (
        <Banner
          type='info'
          className='mb-3'
          description={t(
            '邮件通知尚未配置。申请会保留在此列表，可直接审批，配置邮件后也可重发通知。',
          )}
        />
      )}
      <Form
        onSubmit={create}
        layout='horizontal'
        initValues={{ valid_days: 7, initial_quota: 100000000 }}
      >
        <Form.Input
          field='email'
          label={t('受邀邮箱')}
          type='email'
          rules={[{ required: true, type: 'email' }]}
        />
        <Form.InputNumber
          field='valid_days'
          label={t('有效天数')}
          min={1}
          max={30}
        />
        <Form.InputNumber
          field='initial_quota'
          label={t('批准后额度')}
          min={0}
          max={1000000000}
        />
        <Button htmlType='submit' theme='solid' loading={acting}>
          {t('创建一次性邀请')}
        </Button>
      </Form>
      <Modal
        title={t('请私下保存并发送邀请')}
        visible={Boolean(invitation)}
        onCancel={() => setInvitation(null)}
        onOk={() => setInvitation(null)}
      >
        <Typography.Paragraph>
          {t('邀请码仅显示一次。链接绑定指定邮箱，使用后失效。')}
        </Typography.Paragraph>
        <Typography.Paragraph>
          {invitation?.invitation?.email}
        </Typography.Paragraph>
        <Input
          readOnly
          value={
            invitation
              ? `${window.location.origin}/register#invite=${encodeURIComponent(invitation.invite_code)}`
              : ''
          }
        />
      </Modal>
      <Space className='my-4'>
        <Select
          value={status}
          onChange={setStatus}
          optionList={[
            { value: 'pending', label: t('待审批') },
            { value: 'approved', label: t('已批准') },
            { value: 'rejected', label: t('已拒绝') },
          ]}
        />
        <Button onClick={load} loading={loading}>
          {t('刷新')}
        </Button>
      </Space>
      <Table
        rowKey='id'
        loading={loading}
        dataSource={data.requests}
        pagination={{ pageSize: 10 }}
        columns={[
          { title: t('用户名'), dataIndex: 'username' },
          { title: t('邮箱'), dataIndex: 'email' },
          {
            title: t('状态'),
            dataIndex: 'status',
            render: (value) => (
              <Tag>
                {t(
                  { pending: '待审批', approved: '已批准', rejected: '已拒绝' }[
                    value
                  ] || value,
                )}
              </Tag>
            ),
          },
          {
            title: t('邮件通知'),
            dataIndex: 'notification_status',
            render: (value) =>
              t(
                {
                  pending: '待发送',
                  unconfigured: '未配置',
                  failed: '发送失败',
                  sent: '已发送',
                }[value] || value,
              ),
          },
          {
            title: t('操作'),
            render: (_, row) =>
              row.status === 'pending' && (
                <Space>
                  <Button disabled={acting} onClick={() => review(row, true)}>
                    {t('批准')}
                  </Button>
                  <Button
                    disabled={acting}
                    type='danger'
                    onClick={() => review(row, false)}
                  >
                    {t('拒绝')}
                  </Button>
                  <Button
                    disabled={acting || !data.notification_ready}
                    onClick={() =>
                      action(`/api/registration/requests/${row.id}/notify`)
                    }
                  >
                    {t('重发通知')}
                  </Button>
                </Space>
              ),
          },
        ]}
      />
      <Typography.Title heading={5} className='mt-4'>
        {t('最近邀请')}
      </Typography.Title>
      <Table
        rowKey='id'
        dataSource={data.invitations}
        pagination={{ pageSize: 5 }}
        columns={[
          { title: t('邮箱'), dataIndex: 'email' },
          {
            title: t('有效期至'),
            dataIndex: 'expires_at',
            render: (v) => new Date(v * 1000).toLocaleString(),
          },
          {
            title: t('状态'),
            render: (_, row) =>
              t(
                row.consumed_at
                  ? '已使用'
                  : row.revoked_at
                    ? '已撤销'
                    : row.expires_at <= Date.now() / 1000
                      ? '已过期'
                      : '可使用',
              ),
          },
          {
            title: t('操作'),
            render: (_, row) =>
              !row.consumed_at &&
              !row.revoked_at && (
                <Button
                  disabled={acting}
                  onClick={() =>
                    Modal.confirm({
                      title: t('撤销邀请'),
                      content: row.email,
                      onOk: () =>
                        action(
                          `/api/registration/invitations/${row.id}/revoke`,
                        ),
                    })
                  }
                >
                  {t('撤销')}
                </Button>
              ),
          },
        ]}
      />
    </Card>
  );
}
