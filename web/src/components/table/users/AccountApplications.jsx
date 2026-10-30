import React, { useEffect, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Form,
  Modal,
  Select,
  Space,
  Table,
  Tag,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';

export default function AccountApplications() {
  const { t } = useTranslation();
  const [data, setData] = useState(null);
  const [status, setStatus] = useState('pending');
  const [loading, setLoading] = useState(false);
  const [acting, setActing] = useState(false);
  const [approveRow, setApproveRow] = useState(null);
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
      if (!response.success) return showError(response.message);
      if (response.data?.credential_delivery_status === 'failed')
        showError(t('账号已创建，但邮件发送失败，请重试账号邮件'));
      else showSuccess(t('操作成功'));
      setApproveRow(null);
      await load();
    } catch (error) {
      showError(error.response?.data?.message || t('操作失败'));
    } finally {
      setActing(false);
    }
  }
  const mailStatus = (value) =>
    t(
      {
        pending: '待发送',
        not_applicable: '不适用',
        unconfigured: '未配置',
        failed: '发送失败',
        sent: '已发送',
        sending: '发送中',
      }[value] || value,
    );
  if (!data) return null;
  return (
    <Card title={t('账号申请审批')} className='mb-4'>
      {!data.enabled && (
        <Banner
          type='warning'
          className='mb-3'
          description={t('账号审批策略尚未启用')}
        />
      )}
      {!data.notification_ready && (
        <Banner
          type='warning'
          className='mb-3'
          description={t(
            '请先配置邮件服务，审核通过后需要向申请人发送账号和初始密码。',
          )}
        />
      )}
      <Banner
        type='info'
        className='mb-3'
        description={t(
          '用户填写邮箱后在此等待审核。批准会创建普通账号，并将账号和初始密码发送到申请邮箱。',
        )}
      />
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
          { title: t('申请邮箱'), dataIndex: 'email' },
          {
            title: t('账号'),
            dataIndex: 'username',
            render: (value) => value || '—',
          },
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
            title: t('管理员通知'),
            dataIndex: 'notification_status',
            render: mailStatus,
          },
          {
            title: t('账号邮件'),
            dataIndex: 'credential_delivery_status',
            render: mailStatus,
          },
          {
            title: t('操作'),
            render: (_, row) =>
              row.status === 'pending' ? (
                <Space>
                  <Button
                    disabled={acting || !data.notification_ready}
                    onClick={() => setApproveRow(row)}
                  >
                    {t('批准并发送账号')}
                  </Button>
                  <Button
                    disabled={acting}
                    type='danger'
                    onClick={() =>
                      Modal.confirm({
                        title: t('拒绝注册申请'),
                        content: row.email,
                        onOk: () =>
                          action(
                            `/api/registration/requests/${row.id}/review`,
                            { action: 'reject' },
                          ),
                      })
                    }
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
              ) : (
                row.status === 'approved' &&
                ['failed', 'sending'].includes(
                  row.credential_delivery_status,
                ) && (
                  <Button
                    disabled={acting || !data.notification_ready}
                    onClick={() =>
                      Modal.confirm({
                        title: t('重试账号邮件'),
                        content: t(
                          '仅在用户首次登录前重试。重试会生成新的初始密码，已登录用户请使用找回密码。',
                        ),
                        onOk: () =>
                          action(
                            `/api/registration/requests/${row.id}/credentials`,
                          ),
                      })
                    }
                  >
                    {t('重试账号邮件')}
                  </Button>
                )
              ),
          },
        ]}
      />
      <Modal
        title={t('批准并发送账号')}
        visible={Boolean(approveRow)}
        footer={null}
        onCancel={() => setApproveRow(null)}
      >
        <p className='mb-3'>{approveRow?.email}</p>
        <Form
          key={approveRow?.id}
          initValues={{ initial_quota: 100000000 }}
          onSubmit={(values) =>
            action(`/api/registration/requests/${approveRow.id}/review`, {
              action: 'approve',
              initial_quota: values.initial_quota,
            })
          }
        >
          <Form.InputNumber
            field='initial_quota'
            label={t('初始额度')}
            min={0}
            max={1000000000}
            rules={[{ required: true }]}
          />
          <Button htmlType='submit' theme='solid' loading={acting}>
            {t('确认批准并发送')}
          </Button>
        </Form>
      </Modal>
    </Card>
  );
}
