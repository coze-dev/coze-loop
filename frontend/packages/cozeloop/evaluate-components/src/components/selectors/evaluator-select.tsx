// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback } from 'react';

import { useDebounceFn, useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { useOpenWindow, useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type EvaluatorType,
  type Evaluator,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozAi,
  IconCozCode,
  IconCozPlus,
} from '@coze-arch/coze-design/icons';
import {
  Menu,
  type RenderSelectedItemFn,
  type SelectProps,
  Typography,
} from '@coze-arch/coze-design';

import { getEvaluatorIcon } from '@/utils/evaluator';

const genEvaluatorOption = (item: Evaluator) => ({
  value: item.evaluator_id,
  label: (
    <div className="w-full flex pr-2 items-center gap-2">
      {getEvaluatorIcon(item.evaluator_type as EvaluatorType)}
      <Typography.Text className="w-0 flex-1" ellipsis={{ showTooltip: true }}>
        {item.name}
      </Typography.Text>
    </div>
  ),
  ...item,
});

export function EvaluatorSelect(
  props: SelectProps & { evaluatorTypes?: EvaluatorType[] | undefined },
) {
  const { spaceID } = useSpace();
  const { multiple, evaluatorTypes } = props;
  const { openBlank } = useOpenWindow();

  const service = useRequest(async (text?: string) => {
    const res = await StoneEvaluationApi.ListEvaluators({
      workspace_id: spaceID,
      search_name: text || undefined,
      evaluator_type: evaluatorTypes,
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
            <div className="w-full flex pr-2 items-center gap-2">
              {getEvaluatorIcon(optionNode?.evaluator_type as EvaluatorType)}
              <Typography.Text
                className="max-w-[100px]"
                ellipsis={{ showTooltip: true }}
              >
                <>{optionNode?.name || optionNode?.value || ''}</>
              </Typography.Text>
            </div>
          ),
        };
      }
      return (optionNode?.label || optionNode?.value || '') as React.ReactNode;
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
        <>
          <Menu
            position="bottomRight"
            render={
              <Menu.SubMenu className="w-[174px]" mode="menu">
                <Menu.Item
                  onClick={() => {
                    openBlank('/evaluation/evaluators/create/llm');
                  }}
                >
                  <div className="flex flex-row items-center">
                    <IconCozAi className="mr-1" />
                    <span>{I18n.t('evaluate_llm_evaluator')}</span>
                  </div>
                </Menu.Item>
                <Menu.Item
                  onClick={() => {
                    openBlank(
                      '/evaluation/evaluators/create/code?templateKey=custom&templateLang=Python',
                    );
                  }}
                >
                  <div className="flex flex-row items-center">
                    <IconCozCode className="mr-1" />
                    <span>{I18n.t('evaluate_code_evaluator')}</span>
                  </div>
                </Menu.Item>
              </Menu.SubMenu>
            }
          >
            <div className="h-8 px-2 flex flex-row items-center cursor-pointer">
              <IconCozPlus className="h-4 w-4 text-brand-9 mr-2" />
              <div className="text-sm font-medium text-brand-9">
                {I18n.t('create_evaluator')}
              </div>
            </div>
          </Menu>
        </>
      }
      optionList={service.data}
    />
  );
}
