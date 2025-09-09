// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import classNames from 'classnames';
import { useDebounceFn, useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { useResourcePageJump, useSpace } from '@cozeloop/biz-hooks-adapter';
import { EvalTargetType } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import { type SelectProps } from '@coze-arch/coze-design';

import { useGlobalEvalConfig } from '@/stores/eval-global-config';

import { getPromptEvalTargetOption } from './utils';

/**
 * 评测对象选择器, 公共, 开源逻辑
 */
const PromptEvalTargetSelect = ({
  showCreateBtn = false,
  onlyShowOptionName = false,
  ...props
}: SelectProps & { showCreateBtn?: boolean; onlyShowOptionName?: boolean }) => {
  const { spaceID } = useSpace();
  const [createPromptVisible, setCreatePromptVisible] = useState(false);
  const { PromptCreate } = useGlobalEvalConfig();
  const { getPromptDetailURL } = useResourcePageJump();

  const service = useRequest(async (text?: string) => {
    const res = await StoneEvaluationApi.ListSourceEvalTargets({
      target_type: EvalTargetType.CozeLoopPrompt,
      name: text || undefined,
      workspace_id: spaceID,
      page_size: 100,
    });
    return res.eval_targets?.map(item =>
      getPromptEvalTargetOption(item, onlyShowOptionName),
    );
  });

  const handleSearch = useDebounceFn(service.run, {
    wait: 500,
  });

  return (
    <>
      <BaseSearchSelect
        className={classNames(props.className)}
        emptyContent={I18n.t('no_data')}
        loading={service.loading}
        onSearch={handleSearch.run}
        showRefreshBtn={true}
        onClickRefresh={() => service.run()}
        outerBottomSlot={
          showCreateBtn ? (
            <div
              onClick={() => {
                setCreatePromptVisible(true);
              }}
              className="h-8 px-2 flex flex-row items-center cursor-pointer"
            >
              <IconCozPlus className="h-4 w-4 text-brand-9 mr-2" />
              <div className="text-sm font-medium text-brand-9">
                {I18n.t('create_new_prompt')}
              </div>
            </div>
          ) : null
        }
        optionList={service.data}
        {...props}
      />
      {showCreateBtn && PromptCreate ? (
        <PromptCreate
          visible={createPromptVisible}
          onCancel={() => setCreatePromptVisible(false)}
          onOk={res => {
            window.open(getPromptDetailURL(`${res.id}`));
            setCreatePromptVisible(false);
            service.run();
          }}
        />
      ) : null}
    </>
  );
};

export default PromptEvalTargetSelect;
