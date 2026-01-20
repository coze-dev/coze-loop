// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState, type ReactNode } from 'react';

import cls from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { type ModifyUserProfileRequest } from '@cozeloop/api-schema/foundation';
import { userService, useUserStore } from '@cozeloop/account';
import { CozAvatar, Input } from '@coze-arch/coze-design';

import { UsernameInput } from './username-input';
import { EditWrap } from './edit-wrap';

import s from './index.module.less';

interface UserInfoFieldProps {
  label?: ReactNode;
  children?: ReactNode;
}

function UserInfoField({ label, children }: UserInfoFieldProps) {
  return (
    <div className={s.field}>
      <div className={s.label}>{label}</div>
      {children}
    </div>
  );
}

interface Props {
  className?: string;
}

export function UserInfoPanel({ className }: Props) {
  const userInfo = useUserStore(store => store.userInfo);
  const patch = useUserStore(state => state.patch);
  const [name, setName] = useState<string | undefined>();
  const [nickName, setNickName] = useState<string | undefined>();

  const { runAsync: modifyUserInfo, loading } = useRequest(
    async (req: Partial<ModifyUserProfileRequest>) => {
      try {
        const newUserInfo = await userService.modifyUserProfile(req);
        patch({ userInfo: newUserInfo });
        return true;
      } catch (e) {
        console.error(e);
        return false;
      }
    },
    { manual: true },
  );

  useEffect(() => {
    setName(userInfo?.name);
    setNickName(userInfo?.nick_name);
  }, [userInfo]);

  return (
    <div className={cls(s.container, className)}>
      <CozAvatar src={userInfo?.avatar_url} size="xl">
        {userInfo?.nick_name}
      </CozAvatar>
      <UserInfoField label={I18n.t('username')}>
        <EditWrap
          loading={loading}
          canSave={Boolean(name)}
          displayComponent={name}
          editableComponent={
            <UsernameInput
              value={name}
              className={s.input}
              autoFocus={true}
              onChange={setName}
            />
          }
          onSave={() => modifyUserInfo({ name })}
          onCancel={() => setName(userInfo?.name)}
        />
      </UserInfoField>
      <UserInfoField label={I18n.t('user_custom_name')}>
        <EditWrap
          loading={loading}
          canSave={Boolean(nickName)}
          displayComponent={nickName}
          editableComponent={
            <Input
              className={s.input}
              maxLength={20}
              autoFocus={true}
              value={nickName}
              onChange={setNickName}
            />
          }
          onSave={() => modifyUserInfo({ nick_name: nickName })}
          onCancel={() => setNickName(userInfo?.nick_name)}
        />
      </UserInfoField>
      <UserInfoField label={I18n.t('email')}>{userInfo?.email}</UserInfoField>
      <div className={s.uid}>UID: {userInfo?.user_id}</div>
    </div>
  );
}
