// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState, type PropsWithChildren } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import { Collapsible } from '@coze-arch/coze-design';

interface Props {
  title: string;
}
export function CollapsibleField({
  title,
  children,
}: PropsWithChildren<Props>) {
  const [isOpen, setIsOpen] = useState(true);
  return (
    <div>
      <div className="text-[14px] font-semibold coz-fg-plus px-6 py-3 border-0 border-t border-[var(--coz-stroke-primary)] border-solid flex items-center justify-between bg-[#F6F6FB]">
        {title}

        <div
          className="flex items-center coz-fg-secondary cursor-pointer"
          onClick={() => {
            setIsOpen(!isOpen);
          }}
        >
          <span className="mr-2 text-[13px]">
            {isOpen ? I18n.t('collapse') : I18n.t('expand')}
          </span>
          <IconCozArrowDown
            className={classNames('text-[16px] transition-transform', {
              'rotate-180': isOpen,
            })}
          />
        </div>
      </div>
      <Collapsible isOpen={isOpen}>{children}</Collapsible>
    </div>
  );
}
