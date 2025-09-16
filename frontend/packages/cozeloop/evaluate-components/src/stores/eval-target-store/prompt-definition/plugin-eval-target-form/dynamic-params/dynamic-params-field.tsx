// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode, useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
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
        {I18n.t('evaluate_parameter_injection')}
        <Tooltip
          theme="dark"
          content={I18n.t(
            'cozeloop_open_evaluate_inject_parameters_for_evaluation_request',
          )}
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
