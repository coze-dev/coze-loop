// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback } from 'react';

import { useDebounceFn, useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { useBaseURL, useSpace } from '@cozeloop/biz-hooks-adapter';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  type RenderSelectedItemFn,
  type SelectProps,
  Typography,
} from '@coze-arch/coze-design';

const genEvaluatorOption = (item: Evaluator) => ({
  value: item.evaluator_id,
  label: (
    <div className="w-full flex pr-2">
      <Typography.Text className="w-0 flex-1" ellipsis={{ showTooltip: true }}>
        {item.name}
      </Typography.Text>
    </div>
  ),
  ...item,
});

export function EvaluatorSelect(props: SelectProps) {
  const { spaceID } = useSpace();
  const { baseURL } = useBaseURL();
  const { multiple } = props;

  const service = useRequest(async (text?: string) => {
    const res = await StoneEvaluationApi.ListEvaluators({
      workspace_id: spaceID,
      search_name: text || undefined,
      page_size: 100,
    });
    return res.evaluators?.map(genEvaluatorOption);
  });

  const handleSearch = useDebounceFn(service.run, {
    wait: 500,
  });

  const fetchOptionsByIds = useCallback(
    async value => {
      const res = await StoneEvaluationApi.BatchGetEvaluators({
        workspace_id: spaceID,
        evaluator_ids: value,
      });
      return res.evaluators?.map(genEvaluatorOption) || [];
    },
    [spaceID],
  );

  const renderSelectedItem = useCallback(
    (optionNode?: Record<string, unknown>) => {
      // 多选
      if (multiple) {
        return {
          isRenderInTag: true,
          content: (
            <Typography.Text
              className="max-w-[100px]"
              ellipsis={{ showTooltip: true }}
            >
              <>{optionNode?.name || optionNode?.value}</>
            </Typography.Text>
          ),
        };
      }
      return (optionNode?.label || optionNode?.value) as React.ReactNode;
    },
    [multiple],
  );

  return (
    <BaseSearchSelect
      filter
      remote
      emptyContent={I18n.t('no_data_yet')}
      placeholder={I18n.t('please_select_evaluator')}
      loading={service.loading}
      renderSelectedItem={renderSelectedItem as RenderSelectedItemFn}
      {...props}
      onSearch={handleSearch.run}
      loadOptionByIds={fetchOptionsByIds}
      showRefreshBtn={true}
      onClickRefresh={() => service.run()}
      outerBottomSlot={
        <div
          onClick={() => {
            window.open(`${baseURL}/evaluation/evaluators/create`);
          }}
          className="h-8 px-2 flex flex-row items-center cursor-pointer"
        >
          <IconCozPlus className="h-4 w-4 text-brand-9 mr-2" />
          <div className="text-sm font-medium text-brand-9">
            {I18n.t('create_evaluator')}
          </div>
        </div>
      }
      optionList={service.data}
    />
  );
}
