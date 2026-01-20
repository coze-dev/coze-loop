// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback } from 'react';

import { useDebounceFn, useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { useRouteInfo, useSpace } from '@cozeloop/biz-hooks-adapter';
import { type Experiment, type Filters } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  type RenderSelectedItemFn,
  type SelectProps,
  Typography,
} from '@coze-arch/coze-design';

export function ExperimentsSelect(
  props: SelectProps & {
    disableAddExperiment?: boolean;
    filters?: Filters;
    createExperimentUrl?: string;
  },
) {
  const { spaceID } = useSpace();
  const { getBaseURL } = useRouteInfo();
  const { multiple } = props;

  const service = useRequest(
    async (text?: string) => {
      const res = await StoneEvaluationApi.ListExperiments({
        filter_option: { fuzzy_name: text, filters: props?.filters },
        workspace_id: spaceID,
        page_size: 100,
        page_number: 1,
      });
      return res.experiments?.map(item => ({
        value: item.id,
        label: item.name,
        detail: item,
      }));
    },
    {
      refreshDeps: [spaceID, props?.filters],
    },
  );

  const handleSearch = useDebounceFn(service.run, {
    wait: 500,
  });

  const renderSelectedItem = useCallback(
    (optionNode?: Record<string, unknown>) => {
      const detail = optionNode?.detail as Experiment;
      // 多选
      if (multiple) {
        return {
          isRenderInTag: true,
          content: (
            <Typography.Text
              className="max-w-[100px]"
              ellipsis={{ showTooltip: true }}
            >
              <>{detail?.name || optionNode?.value}</>
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
      placeholder={I18n.t('evaluate_please_select_evaluation_experiment')}
      renderSelectedItem={renderSelectedItem as RenderSelectedItemFn}
      filter
      remote
      loading={service.loading}
      onSearch={handleSearch.run}
      showRefreshBtn={true}
      onClickRefresh={() => service.run()}
      outerBottomSlot={
        !props.disableAddExperiment ? (
          <div
            onClick={() => {
              window.open(
                `${getBaseURL()}${props.createExperimentUrl || '/evaluation/experiments/create'}`,
              );
            }}
            className="h-8 px-2 flex flex-row items-center cursor-pointer"
          >
            <IconCozPlus className="h-4 w-4 text-brand-9 mr-2" />
            <div className="text-sm font-medium text-brand-9">
              {I18n.t('new_experiment')}
            </div>
          </div>
        ) : null
      }
      optionList={service.data}
      {...props}
    />
  );
}
