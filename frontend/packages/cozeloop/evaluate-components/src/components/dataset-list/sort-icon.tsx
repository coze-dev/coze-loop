// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type SVGAttributes } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { IconButtonContainer } from '@cozeloop/components';
import { Tooltip } from '@coze-arch/coze-design';

const activeColor = 'rgb(var(--coze-up-brand-9))';
const unActiveColor = 'var(--coz-fg-dim)';

export const IconSortArrow = ({
  sortOrder,
  ...rest
}: {
  sortOrder?: 'ascend' | 'descend' | boolean;
} & SVGAttributes<HTMLOrSVGElement>) => {
  let descendColor = 'currentColor';
  let ascendColor = 'currentColor';
  if (sortOrder === 'descend') {
    descendColor = activeColor;
    ascendColor = unActiveColor;
  } else if (sortOrder === 'ascend') {
    descendColor = unActiveColor;
    ascendColor = activeColor;
  }

  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
      {...rest}
    >
      <path
        d="M7.00012 1.99976C7.55241 1.99976 8.00012 2.44747 8.00012 2.99976V17.2677L9.6341 14.4376C9.91024 13.9593 10.5218 13.7954 11.0001 14.0715C11.4784 14.3477 11.6423 14.9593 11.3661 15.4376L7.8758 21.483C7.85628 21.5183 7.83468 21.5523 7.81118 21.5849C7.72727 21.7015 7.62174 21.7953 7.50337 21.8641C7.38229 21.9347 7.24502 21.9807 7.09848 21.995C7.06439 21.9984 7.03007 22 6.99566 21.9999C6.64882 22.0011 6.31102 21.8217 6.12517 21.4998L2.62517 15.4376C2.34903 14.9593 2.5129 14.3477 2.99119 14.0715C3.46949 13.7954 4.08108 13.9593 4.35722 14.4376L6.00012 17.2832V2.99976C6.00012 2.44747 6.44784 1.99976 7.00012 1.99976Z"
        fill={descendColor}
      />
      <path
        d="M17.001 21.9999C16.4487 21.9999 16.001 21.5522 16.001 20.9999L16.001 6.73198L14.367 9.5621C14.0909 10.0404 13.4793 10.2043 13.001 9.92813C12.5227 9.65199 12.3589 9.04039 12.635 8.5621L16.1253 2.51665C16.1449 2.48135 16.1665 2.44735 16.19 2.41482C16.2739 2.29823 16.3794 2.20436 16.4978 2.13559C16.6189 2.06494 16.7561 2.019 16.9027 2.0047C16.9368 2.0013 16.9711 1.99964 17.0055 1.99977C17.3523 1.99854 17.6901 2.17802 17.876 2.49993L21.376 8.5621C21.6521 9.0404 21.4882 9.65199 21.0099 9.92813C20.5317 10.2043 19.9201 10.0404 19.6439 9.5621L18.001 6.71652L18.001 20.9999C18.001 21.5522 17.5533 21.9999 17.001 21.9999Z"
        fill={ascendColor}
      />
    </svg>
  );
};

/** 表格排序专用图标 */
export default function LoopTableSortIcon({
  sortOrder,
}: {
  sortOrder?: 'ascend' | 'descend' | boolean;
}) {
  let tooltip = I18n.t('click_ascending');
  if (sortOrder === 'ascend') {
    tooltip = I18n.t('click_descending');
  } else if (sortOrder === 'descend') {
    tooltip = I18n.t('restore_default_sort');
  }
  return (
    <Tooltip theme="dark" content={tooltip}>
      <div className="flex items-center">
        <IconButtonContainer
          icon={
            <IconSortArrow
              sortOrder={sortOrder}
              className="text-[var(--coz-fg-secondary)]"
            />
          }
        />
      </div>
    </Tooltip>
  );
}
