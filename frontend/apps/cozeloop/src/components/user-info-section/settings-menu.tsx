// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { useOpenWindow } from '@cozeloop/biz-hooks-adapter';
import { PERSONAL_ENTERPRISE_ID } from '@cozeloop/account';
import { IconCozExit, IconCozSetting } from '@coze-arch/coze-design/icons';
import { Divider } from '@coze-arch/coze-design';

import { UserInfo } from './user-info';

interface Props {
  onAction?: (action: 'logout' | 'setting') => void;
}

export function SettingsMenu({ onAction }: Props) {
  const enterpriseID = PERSONAL_ENTERPRISE_ID;

  const menus = [
    {
      icon: <IconCozSetting />,
      text: I18n.t('account_settings'),
      onClick: () => {
        onAction?.('setting');
      },
      disabled: false,
    },
    {
      icon: <IconCozExit />,
      text: I18n.t('logout'),
      onClick: () => {
        onAction?.('logout');
      },
    },
  ];

  const { openSelf } = useOpenWindow();

  return (
    <div className="w-[270px] py-3">
      <div className="coz-fg-secondary text-xs leading-[18px] px-5 py-[3px] mb-2">
        个人
      </div>
      <div className="mx-4">
        <UserInfo
          onClick={() => {
            openSelf('', {
              enterpriseID: PERSONAL_ENTERPRISE_ID,
            });
          }}
          active={enterpriseID === PERSONAL_ENTERPRISE_ID}
        />
      </div>
      <Divider margin={12} />
      {menus.map((menu, index) => (
        <div
          key={index}
          className={classNames(
            'h-9 flex items-center rounded-[6px] px-2 mx-3 text-[16px]',
            menu.disabled
              ? 'cursor-not-allowed text-[--semi-color-text-3]'
              : 'cursor-pointer hover:coz-mg-primary coz-fg-primary',
          )}
          onClick={menu.disabled ? undefined : menu.onClick}
        >
          {menu.icon}
          <span className="text-sm ml-2">{menu.text}</span>
        </div>
      ))}
    </div>
  );
}
