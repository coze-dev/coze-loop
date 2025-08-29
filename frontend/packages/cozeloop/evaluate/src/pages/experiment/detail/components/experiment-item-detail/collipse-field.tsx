import { useState, type PropsWithChildren } from 'react';

import classNames from 'classnames';
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
          <span className="mr-2 text-[13px]">{isOpen ? '收起' : '展开'}</span>
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
