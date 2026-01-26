// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import cn from 'classnames';
import { Divider } from '@coze-arch/coze-design';

import { CREATE_TEMPLATE_LIST, CreateTemplate } from './type';
interface CreateDatasetTemplateProps {
  onChange: (template: CreateTemplate) => void;
}

export const CreateDatasetTemplate = ({
  onChange,
}: CreateDatasetTemplateProps) => {
  const [template, setTemplate] = useState<CreateTemplate>(
    CreateTemplate.Default,
  );

  return (
    <div className="flex items-center gap-2">
      {CREATE_TEMPLATE_LIST.map((item, index) => (
        <>
          {index !== 0 && <Divider layout="vertical" className="!h-[12px]" />}
          <div
            className={cn(
              'cursor-pointer text-[14px] coz-fg-secondary',
              template === item.value && '!coz-fg-hglt',
            )}
            onClick={() => {
              setTemplate(item.value);
              onChange(item.value);
            }}
          >
            {item.label}
          </div>
        </>
      ))}
    </div>
  );
};
