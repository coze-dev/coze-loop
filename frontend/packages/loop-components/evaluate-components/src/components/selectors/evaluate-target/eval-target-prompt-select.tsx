// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback, useState } from 'react';

import classNames from 'classnames';
import { useDebounceFn, useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect, type BaseSelectProps } from '@cozeloop/components';
import {
  useResourcePageJump,
  useOpenWindow,
  useSpace,
} from '@cozeloop/biz-hooks-adapter';
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
}: SelectProps &
  BaseSelectProps & {
    showCreateBtn?: boolean;
    onlyShowOptionName?: boolean;
  }) => {
  const { spaceID } = useSpace();
  const [createPromptVisible, setCreatePromptVisible] = useState(false);
  const { PromptCreate } = useGlobalEvalConfig();
  const { getPromptDetailURL } = useResourcePageJump();
  const { openBlank } = useOpenWindow();
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

  const fetchTargetOptionsByIds = useCallback(
    async (ids: string[] | number[]) => {
      const res = await StoneEvaluationApi.BatchGetEvalTargetsBySource({
        workspace_id: spaceID || '',
        source_target_ids: ids as string[],
        eval_target_type: EvalTargetType.CozeLoopPrompt,
        need_source_info: true,
      });
      return (res?.eval_targets || []).map(item =>
        getPromptEvalTargetOption(item, onlyShowOptionName),
      );
    },
    [spaceID],
  );
  return (
    <>
      <BaseSearchSelect
        className={classNames(props.className)}
        emptyContent={I18n.t('no_data')}
        loading={service.loading}
        onSearch={handleSearch.run}
        showRefreshBtn={true}
        loadOptionByIds={fetchTargetOptionsByIds}
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
                {I18n.t('new_prompt')}
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
            openBlank(getPromptDetailURL(`${res.id}`));
            setCreatePromptVisible(false);
            service.run();
          }}
        />
      ) : null}
    </>
  );
};

export default PromptEvalTargetSelect;
