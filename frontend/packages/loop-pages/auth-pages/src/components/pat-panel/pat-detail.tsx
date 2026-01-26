// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { type PersonalAccessToken } from '@cozeloop/api-schema/foundation';
import { Typography } from '@coze-arch/coze-design';

import { getExpirationTime } from './utils';

import s from './pat-detail.module.less';

interface Props {
  token?: string;
  pat?: PersonalAccessToken;
}

export function PatDetail({ token, pat }: Props) {
  return (
    <div className={s.container}>
      <p className={s.warn}>{I18n.t('token_show_only_once')}</p>
      <div className={s.line}>
        <div className={s.title}>{I18n.t('name')}</div>
        <div className={s.content}>{pat?.name}</div>
      </div>
      <div className={s.line}>
        <div className={s.title}>{I18n.t('expiration_time')}</div>
        <div className={s.content}>{getExpirationTime(pat?.expire_at)}</div>
      </div>
      <div className={s.line}>
        <div className={s.title}>{I18n.t('token')}</div>
        <div className={s.content}>
          <Typography.Text
            className={s.token}
            copyable={true}
            ellipsis={{ rows: 1 }}
          >
            {token}
          </Typography.Text>
        </div>
      </div>
    </div>
  );
}
