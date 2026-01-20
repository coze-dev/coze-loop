// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import React, { useCallback, useState } from 'react';

import { useDebounceFn, useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { BaseSearchSelect } from '@cozeloop/components';
import { useOpenWindow, useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type Evaluator,
  type EvaluatorType,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozAi,
  IconCozCode,
  IconCozInfoCircle,
  IconCozLongArrowTopRight,
  IconCozPlus,
} from '@coze-arch/coze-design/icons';
import {
  Menu,
  Search,
  SegmentTab,
  Tooltip,
  Typography,
  type RenderSelectedItemFn,
  type SelectProps,
} from '@coze-arch/coze-design';

import { getEvaluatorJumpUrlV2 } from '../evaluator/utils';
import { getEvaluatorIcon } from '../../utils/evaluator';

const genEvaluatorOption = (
  item: Evaluator,
  openBlank: (url: string) => void,
) => ({
  value: item.evaluator_id,
  label: (
    <div className="w-full flex pr-2 items-center gap-2">
      {getEvaluatorIcon(item.evaluator_type as EvaluatorType, item?.tags)}
      <Typography.Text className="w-0 flex-1" ellipsis={{ showTooltip: true }}>
        {item.name}
      </Typography.Text>
      <Tooltip
        content={
          <div>
            <div>
              {item.description || I18n.t('evaluate_no_evaluator_desc')}
            </div>
            <div
              className="flex items-center gap-1 cursor-pointer"
              onClick={event => {
                event.stopPropagation();
                const url = getEvaluatorJumpUrlV2(item);
                openBlank(url || '');
              }}
            >
              {I18n.t('view_detail')}
              <IconCozLongArrowTopRight />
            </div>
          </div>
        }
        position="right"
        mouseLeaveDelay={200}
      >
        <IconCozInfoCircle className="coz-fg-primary" />
      </Tooltip>
    </div>
  ),
  ...item,
});

export function EvaluatorAggregationSelect(props: SelectProps) {
  const { spaceID } = useSpace();
  const { openBlank } = useOpenWindow();
  const { multiple } = props;
  const [activeTab, setActiveTab] = useState<'custom' | 'preset'>('custom');

  const service = useRequest(
    async (text?: string) => {
      const res = await StoneEvaluationApi.ListEvaluators({
        workspace_id: spaceID,
        search_name: text || undefined,
        builtin: activeTab === 'preset',
        filter_option: {
          search_keyword: text || undefined,
        },
        page_size: 100,
      });

      const result =
        res.evaluators?.map(item => genEvaluatorOption(item, openBlank)) || [];

      return result;
    },
    {
      refreshDeps: [activeTab],
    },
  );

  const handleSearch = useDebounceFn(service.run, {
    wait: 500,
  });

  const fetchOptionsByIds = useCallback(
    async (value: (string | number)[]) => {
      if (!value || value.length === 0) {
        return [];
      }
      const res = await StoneEvaluationApi.BatchGetEvaluators({
        workspace_id: spaceID,
        evaluator_ids: value as string[],
      });
      return (
        res.evaluators?.map(item => genEvaluatorOption(item, openBlank)) || []
      );
    },
    [spaceID],
  );

  const renderSelectedItem = useCallback(
    (optionNode?: Record<string, unknown>) => {
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
  ) as RenderSelectedItemFn;

  const outerBottomSlotContent = (
    <div
      className={`${activeTab === 'preset' ? 'opacity-50 cursor-not-allowed' : ''} h-8 px-2 flex flex-row items-center`}
    >
      <IconCozPlus className="h-4 w-4 text-brand-9 mr-2" />
      <div className="text-sm font-medium text-brand-9">
        {I18n.t('new_evaluator')}
      </div>
    </div>
  );

  const onDropdownVisibleChange = (visible: boolean) => {
    // 重新打开下拉列表时，清空搜索框
    if (visible) {
      handleSearch.run('');
    }
  };

  return (
    <BaseSearchSelect
      {...props}
      filter
      remote
      disabledCacheOptions={true}
      placeholder={I18n.t('please_select_evaluator')}
      onSearch={handleSearch.run}
      loadOptionByIds={fetchOptionsByIds}
      showRefreshBtn
      onClickRefresh={() => service.run()}
      optionList={service.data}
      loading={service.loading}
      onDropdownVisibleChange={onDropdownVisibleChange}
      renderSelectedItem={renderSelectedItem}
      outerTopSlot={
        <div className="px-2 py-1">
          <SegmentTab
            options={[
              { label: I18n.t('self_built_evaluator'), value: 'custom' },
              { label: I18n.t('preset_evaluator'), value: 'preset' },
            ]}
            value={activeTab}
            onChange={e => setActiveTab(e.target.value as 'custom' | 'preset')}
          />
          <div className="my-2 w-full">
            <Search
              width="100%"
              placeholder={I18n.t('evaluate_search_evaluator')}
              onSearch={handleSearch.run}
            />
          </div>
        </div>
      }
      outerBottomSlot={
        activeTab === 'preset' ? (
          <Tooltip theme="dark" content={I18n.t('evaluate_preset_no_new')}>
            {outerBottomSlotContent}
          </Tooltip>
        ) : (
          <div className="h-8 items-center cursor-pointer">
            <Menu
              style={{ minWidth: '347px' }}
              position="top"
              mouseLeaveDelay={500}
              render={
                <Menu.SubMenu className="w-[347px]" mode="menu">
                  <Menu.Item
                    className="min-w-[340px]"
                    onClick={() =>
                      openBlank('/evaluation/evaluators/create/llm')
                    }
                  >
                    <div className="flex flex-row items-center">
                      <IconCozAi className="mr-1" />
                      <span>{I18n.t('evaluate_llm_evaluator')}</span>
                    </div>
                  </Menu.Item>
                  <Menu.Item
                    className="min-w-[340px]"
                    onClick={() =>
                      openBlank(
                        '/evaluation/evaluators/create/code?templateKey=custom&templateLang=Python',
                      )
                    }
                  >
                    <div className="flex flex-row items-center">
                      <IconCozCode className="mr-1" />
                      <span>{I18n.t('evaluate_code_evaluator')}</span>
                    </div>
                  </Menu.Item>
                </Menu.SubMenu>
              }
            >
              {outerBottomSlotContent}
            </Menu>
          </div>
        )
      }
    />
  );
}
