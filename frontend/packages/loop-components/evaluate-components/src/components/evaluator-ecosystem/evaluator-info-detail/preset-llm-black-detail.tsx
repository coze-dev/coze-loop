// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef, useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';
import { IconCozArrowRight } from '@coze-arch/coze-design/icons';

import { getSchemaDefaultValueObj } from '../utils';
import { type BlackSchemaEditorGroupValue } from '../types';
import { BlackSchemaEditorGroup } from '../black-schema-editor-group';
import { EvaluatorInfoContent } from './evaluator-info-content';

const PresetLLMBlackDetail = ({
  evaluator,
  disabled = true,
  enableCollapse = false,
  defaultOpen = true,
}: {
  evaluator: Evaluator;
  disabled?: boolean;
  enableCollapse?: boolean;
  defaultOpen?: boolean;
}) => {
  const [editorValue, setEditorValue] = useState<BlackSchemaEditorGroupValue>({
    inputValue: '',
    outputValue: '',
  });

  const [open, setOpen] = useState(defaultOpen || false);

  const initFlag = useRef(false);

  useEffect(() => {
    if (!evaluator) {
      return;
    }

    if (!initFlag.current) {
      const evaluatorContent = evaluator.current_version?.evaluator_content;
      const inputVs = getSchemaDefaultValueObj(evaluatorContent?.input_schemas);
      const outputVs = getSchemaDefaultValueObj(
        evaluatorContent?.output_schemas,
      );
      setEditorValue({
        inputValue: JSON.stringify(inputVs || '', null, 2),
        outputValue: JSON.stringify(outputVs || '', null, 2),
      });
      initFlag.current = true;
    }
  }, []);

  return (
    <div className="w-full flex-1 overflow-y-auto styled-scrollbar">
      {enableCollapse ? (
        <div
          className="h-5 my-1 flex flex-row items-center cursor-pointer text-sm coz-fg-primary font-semibold"
          onClick={() => setOpen(!open)}
        >
          {I18n.t('evaluator_detail')}
          <IconCozArrowRight
            className={classNames(
              'h-4 w-4 ml-2 coz-fg-plus transition-transform',
              open ? 'rotate-90' : '',
            )}
          />
        </div>
      ) : null}

      <div className={open ? 'block' : 'hidden'}>
        <div className="text-sm font-medium coz-fg-primary mt-4">
          {I18n.t('application_scene')}
        </div>
        <EvaluatorInfoContent evaluator={evaluator} />
        <div className="text-sm font-medium coz-fg-primary mt-3 mb-2">
          {I18n.t('evaluate_config')}
        </div>
        <BlackSchemaEditorGroup value={editorValue} disabled={disabled} />
      </div>
    </div>
  );
};

export { PresetLLMBlackDetail };
