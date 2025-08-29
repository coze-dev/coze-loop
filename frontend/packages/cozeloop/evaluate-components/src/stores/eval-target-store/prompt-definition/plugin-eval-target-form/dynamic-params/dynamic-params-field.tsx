import { type ReactNode, useState } from 'react';

import classNames from 'classnames';
import {
  IconCozArrowRight,
  IconCozInfoCircle,
} from '@coze-arch/coze-design/icons';
import { Tooltip } from '@coze-arch/coze-design';

export const DynamicParamsField = ({
  children,
  open: defaultOpen,
}: {
  children: ReactNode;
  open?: boolean;
}) => {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div>
      <div
        className="h-5 flex flex-row items-center cursor-pointer text-sm coz-fg-primary font-semibold"
        onClick={() => setOpen(pre => !pre)}
      >
        参数注入
        <Tooltip
          theme="dark"
          content="请求评测对象时，可注入填写的参数，来拿到评测对象的输出结果。如请求环境的泳道、测试账号的 UID 等。"
        >
          <IconCozInfoCircle className="ml-1 w-4 h-4 coz-fg-secondary" />
        </Tooltip>
        <IconCozArrowRight
          className={classNames(
            'h-4 w-4 ml-2 coz-fg-plus transition-transform',
            open ? 'rotate-90' : '',
          )}
        />
      </div>
      <div className={open ? '' : 'hidden'}>{children}</div>
    </div>
  );
};
