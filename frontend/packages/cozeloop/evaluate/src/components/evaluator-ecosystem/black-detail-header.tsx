// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { InfoJump, renderTags } from '@cozeloop/evaluate-components';
import { RouteBackAction } from '@cozeloop/base-with-adapter-components';
import { type Evaluator, EvaluatorType } from '@cozeloop/api-schema/evaluation';
import { IconCozPlayFill } from '@coze-arch/coze-design/icons';
import { Button, Typography } from '@coze-arch/coze-design';

import { BlackDebugModal } from './black-debug-modal';

interface HeaderProps {
  evaluator?: Evaluator;
  onClickDebugBtn?: (evaluator: Evaluator) => void;
}

export function BlackDetailHeader({ evaluator, onClickDebugBtn }: HeaderProps) {
  const { evaluator_info, name, tags } = evaluator || {};
  const { user_manual_url, vendor, vendor_url } = evaluator_info || {};

  const [debugVisible, setDebugVisible] = useState(false);

  const handleClickDebugBtn = () => {
    setDebugVisible(true);
    onClickDebugBtn?.(evaluator || {});
  };

  return (
    <>
      <div className="px-6 py-2 h-[64px] flex-shrink-0 flex flex-row items-center border-0 border-b border-solid coz-stroke-primary">
        <RouteBackAction defaultModuleRoute="evaluation/evaluators?active_tab=builtin" />
        <div className="ml-2 flex-1">
          <div className="text-[14px] leading-5 font-medium coz-fg-plus flex items-center gap-x-1">
            <Typography.Text className="!coz-fg-plus !font-medium !text-[14px] !leading-[20px]">
              {name}
            </Typography.Text>
          </div>
          <div className="h-6 flex flex-row items-center">
            <div className="text-xs font-normal !coz-fg-secondary overflow-hidden text-ellipsis whitespace-nowrap leading-4 flex items-center">
              <span className="shrink-0">
                {I18n.t('evaluate_case_info_desc')}
              </span>
              <div className="overflow-hidden text-ellipsis whitespace-nowrap">
                {renderTags(tags)}
              </div>
              <div className="flex items-center text-sm coz-fg-secondary">
                {vendor && vendor_url ? (
                  <>
                    <div className="mx-3 h-3 w-0 border-0 border-l border-solid coz-stroke-primary" />
                    <span className="text-[13px]">
                      {I18n.t('evaluator_provider')}：
                    </span>
                    <InfoJump text={vendor || ''} url={vendor_url || ''} />
                  </>
                ) : null}
                {user_manual_url ? (
                  <>
                    <div className="mx-3 h-3 w-0 border-0 border-l border-solid coz-stroke-primary" />
                    <InfoJump
                      text={I18n.t('help_documentation')}
                      url={user_manual_url || ''}
                    />
                  </>
                ) : null}
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
      <BlackDebugModal
        visible={debugVisible}
        evaluator={evaluator}
        onCancel={() => setDebugVisible(false)}
        evaluatorType={EvaluatorType.Prompt}
      />
    </>
  );
}
