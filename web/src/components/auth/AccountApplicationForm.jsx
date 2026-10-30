import React, { useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Checkbox,
  Form,
  Typography,
} from '@douyinfe/semi-ui';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import Turnstile from 'react-turnstile';
import { API, getSystemName, showError } from '../../helpers';

export default function AccountApplicationForm({ status }) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [feedback, setFeedback] = useState(null);
  const [agreed, setAgreed] = useState(false);
  const [turnstile, setTurnstile] = useState('');
  const needsAgreement =
    status.user_agreement_enabled || status.privacy_policy_enabled;
  async function submit(values) {
    if (needsAgreement && !agreed)
      return showError(t('请先阅读并同意用户协议和隐私政策'));
    if (status.turnstile_check && !turnstile)
      return showError(t('请完成人机验证'));
    setFeedback(null);
    setBusy(true);
    try {
      const { data } = await API.post(
        `/api/user/register?turnstile=${encodeURIComponent(turnstile)}`,
        { email: values.email.trim().toLowerCase() },
        { skipErrorHandler: true },
      );
      if (data.success) setSubmitted(true);
      else showError(data.message);
    } catch (error) {
      const duplicate = error.response?.data?.data?.status;
      if (
        error.response?.status === 409 &&
        ['registered', 'pending'].includes(duplicate)
      )
        setFeedback(duplicate);
      else
        showError(
          error.response?.data?.message || t('申请提交失败，请稍后重试'),
        );
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className='max-w-lg mx-auto mt-24 px-4 pb-12'>
      <Card title={`${getSystemName()} · ${t('申请账号')}`}>
        {submitted ? (
          <Banner
            type='success'
            title={t('申请已收到，等待管理员审核')}
            description={t(
              '审核通过后，账号和初始密码会发送到申请邮箱。登录后可在个人设置中修改密码。',
            )}
          />
        ) : (
          <>
            <Banner
              type='info'
              className='mb-4'
              description={t(
                '填写邮箱即可申请账号。管理员审核通过后，系统会通过邮件发放账号和初始密码。',
              )}
            />
            {feedback && (
              <Banner
                type='info'
                className='mb-4'
                title={
                  feedback === 'registered'
                    ? t('该邮箱已注册')
                    : t('该邮箱正在审核中')
                }
                description={
                  <>
                    {feedback === 'registered'
                      ? t('请直接登录，或联系管理员获取帮助。')
                      : t('你已提交申请，正在等待管理员审核，请勿重复提交。')}
                    {feedback === 'registered' && (
                      <>
                        {' '}
                        <Link to='/login'>{t('前往登录')}</Link>
                      </>
                    )}
                  </>
                }
              />
            )}
            <Form onSubmit={submit}>
              <Form.Input
                field='email'
                onChange={() => setFeedback(null)}
                type='email'
                label={t('申请邮箱')}
                autoComplete='email'
                rules={[{ required: true, type: 'email', max: 50 }]}
              />
              {needsAgreement && (
                <Checkbox
                  checked={agreed}
                  onChange={(e) => setAgreed(e.target.checked)}
                >
                  {t('我已阅读并同意')}{' '}
                  {status.user_agreement_enabled && (
                    <a
                      href='/user-agreement'
                      target='_blank'
                      rel='noopener noreferrer'
                    >
                      {t('用户协议')}
                    </a>
                  )}{' '}
                  {status.privacy_policy_enabled && (
                    <a
                      href='/privacy-policy'
                      target='_blank'
                      rel='noopener noreferrer'
                    >
                      {t('隐私政策')}
                    </a>
                  )}
                </Checkbox>
              )}
              {status.turnstile_check && (
                <Turnstile
                  sitekey={status.turnstile_site_key}
                  onVerify={setTurnstile}
                  onExpire={() => setTurnstile('')}
                />
              )}
              <Button
                htmlType='submit'
                theme='solid'
                block
                className='mt-4'
                loading={busy}
              >
                {t('提交账号申请')}
              </Button>
            </Form>
          </>
        )}
        <Typography.Paragraph className='mt-4'>
          <Link to='/login'>{t('返回登录')}</Link>
        </Typography.Paragraph>
      </Card>
    </div>
  );
}
