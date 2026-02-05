// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useNavigate } from 'react-router-dom';
import { useState, useEffect } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { $notification } from '@cozeloop/api-schema';
import { useLogin, useLoginStatus, useRegister } from '@cozeloop/account';
import { Toast } from '@coze-arch/coze-design';

import { LoginPanel } from '@/components';

export function LoginPage() {
  const navigate = useNavigate();
  const login = useLogin();
  const register = useRegister();
  const loginStatus = useLoginStatus();
  const [loading, setLoading] = useState(false);

  const onRegister = async (email: string, password: string) => {
    try {
      setLoading(true);
      await register(email, password);
    } finally {
      setLoading(false);
    }
  };

  const onLogin = async (email: string, password: string) => {
    try {
      setLoading(true);
      await login(email, password);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const onApiError = (msg: string) => {
      Toast.error({
        className: 'api-error-toast',
        content: (
          <span className="inline-block max-w-[100%] break-all whitespace-normal">
            {msg || I18n.t('register_or_login_failed')}
          </span>
        ),
      });
    };

    $notification.addListener('apiError', onApiError);

    return () => {
      $notification.removeListener('apiError', onApiError);
    };
  }, []);

  useEffect(() => {
    loginStatus === 'logined' && navigate('/');
  }, [loginStatus]);

  return (
    <LoginPanel onLogin={onLogin} onRegister={onRegister} loading={loading} />
  );
}
