// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode } from 'react';

import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Button, Tooltip } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';

export const FieldWrapper = ({
  title,
  children,
  onCopy,
}: {
  title: string;
  children: ReactNode;
  onCopy?: (text?: string) => void;
}) => {
  const { t } = useLocale();
  return (
    <div className="border border-solid border-[var(--coz-stroke-primary)] rounded-[8px] overflow-hidden">
      <div className="border-0 border-b border-solid border-[var(--coz-stroke-primary)] bg-[var(--coz-bg-primary)] py-2 px-3">
        <span className="coz-fg-secondary text-[12px]">{title}</span>
        {onCopy ? (
          <Tooltip content={t('copy')} theme="dark">
            <Button
              size="mini"
              type="secondary"
              color="secondary"
              icon={<IconCozCopy className="coz-fg-secondary" />}
              onClick={() => onCopy?.(title)}
            />
          </Tooltip>
        ) : null}
      </div>
      <div className="px-3 py-2 max-h-[500px] overflow-auto styled-scrollbar text-[12px]">
        {children}
      </div>
    </div>
  );
};
