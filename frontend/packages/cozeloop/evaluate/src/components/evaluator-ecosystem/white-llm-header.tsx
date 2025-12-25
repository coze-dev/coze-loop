// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { renderTags } from '@cozeloop/evaluate-components';
import { RouteBackAction } from '@cozeloop/base-with-adapter-components';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';
import { IconCozPlayFill } from '@coze-arch/coze-design/icons';
import { Button, Typography } from '@coze-arch/coze-design';

import { DebugModal } from '@/pages/evaluator/evaluator-create/debug-modal';

interface HeaderProps {
  evaluator?: Evaluator;
  onClickDebugBtn?: (evaluator: Evaluator) => void;
}

export function WhiteDetailHeader({ evaluator, onClickDebugBtn }: HeaderProps) {
  const [debugValue, setDebugValue] = useState<Evaluator | undefined>(
    undefined,
  );

  const handleClickDebugBtn = useCallback(() => {
    setDebugValue(evaluator);
    onClickDebugBtn?.(evaluator as Evaluator);
  }, [onClickDebugBtn, evaluator]);

  return (
    <>
      <div className="px-6 py-2 h-[64px] flex-shrink-0 flex flex-row items-center border-0 border-b border-solid coz-stroke-primary">
        <RouteBackAction defaultModuleRoute="evaluation/evaluators?active_tab=builtin" />
        <div className="ml-2 flex-1">
          <div className="text-[14px] leading-5 font-medium coz-fg-plus flex items-center gap-x-1">
            <Typography.Text className="!coz-fg-plus !font-medium !text-[14px] !leading-[20px]">
              {evaluator?.name}
            </Typography.Text>
          </div>
          <div className="h-6 flex flex-row items-center">
            <div className="text-xs font-normal !coz-fg-secondary overflow-hidden text-ellipsis whitespace-nowrap leading-4 flex items-center">
              <span className="shrink-0">
                {I18n.t('evaluate_case_info_desc')}
              </span>
              <div className="overflow-hidden text-ellipsis whitespace-nowrap">
                {renderTags(evaluator?.tags)}
              </div>
            </div>
          </div>
        </div>

        <div className="flex-shrink-0 flex flex-row gap-2">
          <Button
            color="highlight"
            onClick={handleClickDebugBtn}
            icon={<IconCozPlayFill />}
          >
            {I18n.t('debug')}
          </Button>
        </div>
      </div>
      {debugValue ? (
        <DebugModal
          initValue={debugValue}
          disableConfig={true}
          onCancel={() => setDebugValue(undefined)}
          onSubmit={(newValue: Evaluator) => {
            // const saveData = cloneDeep(newValue);
            setDebugValue(undefined);
          }}
        />
      ) : null}
    </>
  );
}
