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
import { API, getSystemName, showError, showSuccess } from '../../helpers';
import Turnstile from 'react-turnstile';

export default function InvitationRegisterForm({ status }) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [agreed, setAgreed] = useState(false);
  const [turnstile, setTurnstile] = useState('');
  const [email, setEmail] = useState('');
  const [sendingCode, setSendingCode] = useState(false);
  const [cooldown, setCooldown] = useState(false);
  const initialCode =
    new URLSearchParams(window.location.hash.slice(1)).get('invite') || '';
  const needsAgreement =
    status.user_agreement_enabled || status.privacy_policy_enabled;
  async function submit(values) {
    if (values.password !== values.password2)
      return showError(t('两次输入的密码不一致'));
    if (needsAgreement && !agreed)
      return showError(t('请先阅读并同意用户协议和隐私政策'));
    if (status.turnstile_check && !turnstile)
      return showError(t('请完成人机验证'));
    setBusy(true);
    try {
      const { data } = await API.post(
        `/api/user/register?turnstile=${encodeURIComponent(turnstile)}`,
        values,
      );
      if (!data.success) return showError(data.message);
      setSubmitted(true);
      // Remove the invitation from browser history after it has been consumed.
      window.history.replaceState(null, '', window.location.pathname);
    } catch (error) {
      showError(error.response?.data?.message || t('申请提交失败，请稍后重试'));
    } finally {
      setBusy(false);
    }
  }
  async function sendCode() {
    if (!email || cooldown) return;
    setSendingCode(true);
    try {
      const { data } = await API.get(
        `/api/verification?email=${encodeURIComponent(email.trim().toLowerCase())}&turnstile=${encodeURIComponent(turnstile)}`,
      );
      if (data.success) {
        showSuccess(t('验证码已发送'));
        setCooldown(true);
        setTimeout(() => setCooldown(false), 60000);
      } else showError(data.message);
    } catch {
      showError(t('验证码发送失败'));
    } finally {
      setSendingCode(false);
    }
  }
  return (
    <div className='max-w-lg mx-auto mt-24 px-4 pb-12'>
      <Card title={`${getSystemName()} · ${t('邀请注册')}`}>
        {submitted ? (
          <Banner
            type='success'
            title={t('申请已提交，等待管理员审批')}
            description={t(
              '批准前无法登录或调用 API。请等待管理员通知后再登录。',
            )}
          />
        ) : (
          <>
            <Banner
              type='info'
              className='mb-4'
              description={t(
                '仅受邀用户可以申请。请填写邀请绑定的邮箱，提交后由管理员审核。',
              )}
            />
            <Form onSubmit={submit} initValues={{ invite_code: initialCode }}>
              <Form.Input
                field='username'
                label={t('用户名')}
                rules={[
                  {
                    required: true,
                    pattern: /^[A-Za-z0-9_.-]{3,20}$/,
                    message: t('使用 3–20 位字母、数字、点、横线或下划线'),
                  },
                ]}
              />
              <Form.Input
                field='email'
                type='email'
                label={t('受邀邮箱')}
                onChange={setEmail}
                rules={[{ required: true, type: 'email' }]}
              />
              <Form.Input
                field='invite_code'
                label={t('邀请码')}
                rules={[{ required: true }]}
                autoComplete='off'
              />
              <Form.Input
                field='password'
                mode='password'
                label={t('密码')}
                rules={[{ required: true, min: 8, max: 20 }]}
                autoComplete='new-password'
              />
              <Form.Input
                field='password2'
                mode='password'
                label={t('确认密码')}
                rules={[{ required: true }]}
                autoComplete='new-password'
              />
              {status.email_verification && (
                <>
                  <Form.Input
                    field='verification_code'
                    label={t('邮箱验证码')}
                    rules={[{ required: true }]}
                  />
                  <Button
                    onClick={sendCode}
                    loading={sendingCode}
                    disabled={cooldown}
                  >
                    {cooldown ? t('请稍后重新发送') : t('获取验证码')}
                  </Button>
                </>
              )}
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
                {t('提交注册申请')}
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
