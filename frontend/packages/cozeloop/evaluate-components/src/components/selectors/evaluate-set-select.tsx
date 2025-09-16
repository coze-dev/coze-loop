// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback } from 'react';

import { isArray } from 'lodash-es';
import { useDebounceFn, useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { useBaseURL, useSpace } from '@cozeloop/biz-hooks-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  type RenderSelectedItemFn,
  type SelectProps,
  Typography,
} from '@coze-arch/coze-design';

const EvaluationSetLabel = ({
  name,
  itemCount,
  showSetCount,
}: {
  name?: string;
  itemCount?: number | string;
  showSetCount?: boolean;
}) => (
  <div className="w-full flex pr-2 items-center">
    <div className="w-0 flex-1 flex overflow-hidden items-center">
      <Typography.Text className="w-fit" ellipsis={{ showTooltip: true }}>
        {name}
      </Typography.Text>
      {showSetCount ? (
        <Typography.Text size="small" className="!coz-fg-secondary ml-2 flex-1">
          ({itemCount}/5000)
        </Typography.Text>
      ) : null}
    </div>
  </div>
);

const genEvaluationSetOption = (
  item: EvaluationSet,
  showSetCount?: boolean,
) => ({
  value: item.id,
  label: (
    <EvaluationSetLabel
      name={item.name}
      itemCount={item.item_count}
      showSetCount={showSetCount}
    />
  ),
  ...item,
});

export function EvaluateSetSelect(
  props: SelectProps & {
    disableAddEvalSet?: boolean;
    // 是否展示数据集容量
    showSetCount?: boolean;
  },
) {
  const { spaceID } = useSpace();
  const { baseURL } = useBaseURL();
  const { multiple, showSetCount } = props;

  const service = useRequest(async (text?: string) => {
    const res = await StoneEvaluationApi.ListEvaluationSets({
      name: text || undefined,
      workspace_id: spaceID,
      page_size: 100,
    });
    return res.evaluation_sets?.map(item =>
      genEvaluationSetOption(item, showSetCount),
    );
  });

  const handleSearch = useDebounceFn(service.run, {
    wait: 500,
  });

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

  const fetchOptionsByIds = useCallback(
    async value => {
      if (!value) {
        return [];
      }
      const res = await StoneEvaluationApi.ListEvaluationSets({
        workspace_id: spaceID,
        evaluation_set_ids: isArray(value) ? value : [value],
      });
      return (
        res.evaluation_sets?.map(item =>
          genEvaluationSetOption(item, showSetCount),
        ) || []
      );
    },
    [spaceID],
  );

  return (
    <BaseSearchSelect
      placeholder={I18n.t('select_evaluation_set')}
      renderSelectedItem={renderSelectedItem as RenderSelectedItemFn}
      filter
      remote
      loading={service.loading}
      onSearch={handleSearch.run}
      showRefreshBtn={true}
      onClickRefresh={() => service.run()}
      emptyContent={I18n.t('no_data_yet')}
      outerBottomSlot={
        !props.disableAddEvalSet ? (
          <div
            onClick={() => {
              window.open(`${baseURL}/evaluation/datasets/create`);
            }}
            className="h-8 px-2 flex flex-row items-center cursor-pointer"
          >
            <IconCozPlus className="h-4 w-4 text-brand-9 mr-2" />
            <div className="text-sm font-medium text-brand-9">
              {I18n.t('create_evaluation_set')}
            </div>
          </div>
        ) : null
      }
      optionList={service.data}
      loadOptionByIds={fetchOptionsByIds}
      {...props}
    />
  );
}
