// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { type SelectProps } from '@coze-arch/coze-design';

import { InfoIconTooltip } from '../common/info-icon-tooltip';

export function EvaluatorVersionSelect({
  evaluatorId,
  disabledVersionIds,
  variableRequired = true,
  showRefreshIcon = true,
  ...props
}: SelectProps & {
  evaluatorId?: string;
  disabledVersionIds?: string[];
  /** 是否要求评估器至少有一个变量 */
  variableRequired?: boolean;
  showRefreshIcon?: boolean;
}) {
  const { spaceID } = useSpace();

  const service = useRequest(
    async () => {
      if (!evaluatorId) {
        return [];
      }
      const res = await StoneEvaluationApi.ListEvaluatorVersions({
        workspace_id: spaceID,
        evaluator_id: evaluatorId,
        page_size: 200,
      });
      return res.evaluator_versions?.map(item => ({
        value: item.id,
        label: item.version,
        ...item,
      }));
    },
    {
      refreshDeps: [evaluatorId],
    },
  );

  const optionList = useMemo(
    () =>
      service.data?.map(item => {
        const { label } = item;
        // 当前版本没有变量，禁用该选项
        const hasVariable = Boolean(
          item?.evaluator_content?.input_schemas?.length,
        );
        // 如果当前版本已被选中
        const isSelected = Boolean(
          item.value &&
            props.value !== item.value &&
            disabledVersionIds?.includes(item.value),
        );
        return {
          ...item,
          label:
            variableRequired && !hasVariable ? (
              <div className="flex items-center coz-fg-secondary">
                {label}
                <InfoIconTooltip
                  className="ml-1"
                  tooltip={I18n.t('prompt_variable_to_ensure_effect')}
                />
              </div>
            ) : (
              <>{label}</>
            ),
          disabled: isSelected || (variableRequired && !hasVariable),
        };
      }),
    [service.data, disabledVersionIds, variableRequired],
  );

  return (
    <BaseSearchSelect
      remote
      {...props}
      loading={service.loading}
      showRefreshBtn={true}
      onClickRefresh={() => service.refresh()}
      optionList={optionList}
    />
  );
}
