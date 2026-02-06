// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { Tooltip } from '@coze-arch/coze-design';

export function OpenDetailText({
  className,
  text,
  url,
  customOpen,
}: {
  url: string;
  className?: string;
  text?: string;
  customOpen?: () => void;
}) {
  return (
    <Tooltip theme="dark" content={I18n.t('view_detail')}>
      <div
        className={classNames(
          'flex-shrink-0 text-sm text-brand-9 font-normal cursor-pointer !p-[2px] ',
          className,
        )}
        onClick={e => {
          e.stopPropagation();
          if (customOpen) {
            customOpen();
          } else {
            window.open(url, '_blank');
          }
        }}
      >
        {text || I18n.t('view_detail')}
      </div>
    </Tooltip>
  );
}
