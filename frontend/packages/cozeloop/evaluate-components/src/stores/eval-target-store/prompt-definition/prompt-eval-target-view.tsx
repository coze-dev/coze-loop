// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { OpenDetailButton } from '@cozeloop/components';
import { useResourcePageJump } from '@cozeloop/biz-hooks-adapter';
import { Tag } from '@coze-arch/coze-design';

import { type CreateExperimentValues } from '../../../types/evaluate-target';
import usePromptDetail from './plugin-eval-target-form/use-prompt-detail';

/**
 * prompt 评测对象 直接取用 prompt 详情即可
 */
export const PromptEvalTargetView = (props: {
  formValues: CreateExperimentValues;
}) => {
  const { formValues } = props;
  const { getPromptDetailURL } = useResourcePageJump();

  // prompt id
  const promptId = formValues.evalTarget || '';

  // prompt 版本
  const promptVersion = formValues.evalTargetVersion || '';

  const { promptDetail } = usePromptDetail({
    promptId: promptId as string,
    version: promptVersion as string,
  });

  const promptBasic = promptDetail?.prompt_basic;

  // prompt 名称
  const promptName = promptBasic?.display_name || '-';

  return (
    <>
      <div className="text-[16px] leading-[22px] font-medium coz-fg-primary mb-5">
        {I18n.t('evaluation_object')}
      </div>
      <div className="flex flex-row gap-5">
        <div className="flex-1 w-0">
          <div className="text-sm font-medium coz-fg-primary mb-2">
            {I18n.t('type')}
          </div>
          <div className="text-sm font-normal coz-fg-primary">Prompt</div>
        </div>
        <div className="flex-1 w-0 mb-4">
          <div className="text-sm font-medium coz-fg-primary mb-2">
            {I18n.t('name_and_version')}
          </div>
          <div className="flex flex-row items-center gap-1">
            <div className={'text-sm font-normal coz-fg-primary'}>
              {promptName}
            </div>
            <Tag color="primary" className="!h-5 !px-2 !py-[2px] rounded-[3px]">
              {promptVersion}
            </Tag>
            <OpenDetailButton
              url={getPromptDetailURL(promptId, promptVersion)}
            />
          </div>
        </div>
      </div>
    </>
  );
};
