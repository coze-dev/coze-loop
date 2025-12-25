// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */

/* eslint-disable @coze-arch/max-line-per-function */
import {
  forwardRef,
  useImperativeHandle,
  useRef,
  useEffect,
  useCallback,
  useMemo,
} from 'react';

import {
  type FilterFields,
  type FilterField,
} from '@cozeloop/api-schema/observation';
import {
  IconCozArrowDown,
  IconCozArrowMiddle,
  IconCozArrowUp,
  IconCozFilter,
  IconCozLoose,
  IconCozSetting,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Tooltip,
  Switch,
  Dropdown,
  Checkbox,
} from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';
import {
  TraceSelector,
  type TraceSelectorRef,
  type TraceSelectorProps,
} from '@/features/trace-selector';
import { isRootNodesDepthLessThan2 } from '@/features/trace-detail/utils/span';

import { type SpanNode } from '../trace-tree/type';
import { useTraceSearchLogic } from './use-trace-search-logic';
import { SearchInput } from './search-input';

interface TraceSearchProps {
  /** 受控：提交搜索 */
  setSearchFilters?: (filters: FilterFields) => void;
  /** 受控：清空搜索 */
  onClear?: () => void;
  /** 当前选中的spanId，用于滚动定位 */
  selectedSpanId: string;
  /** 切换展开/收起所有节点 */
  onToggleAll?: () => void;
  /** 是否全部展开 */
  isAllExpanded?: boolean;
  /** 当前搜索过滤条件 */
  searchFilters?: FilterFields;
  /** 是否过滤非关键节点 */
  filterNonCritical?: boolean;
  /** 切换过滤非关键节点 */
  onFilterNonCriticalChange?: (checked: boolean) => void;
  /** 搜索匹配到的 spanId 列表 */
  matchedSpanIds?: string[];
  /** 选择节点回调 */
  onSelect?: (id: string) => void;
  treeSelectorConfig?: TraceSelectorProps;
  /** 是否精简过滤模式 */
  isSlimMode?: boolean;
  /** 切换精简过滤模式 */
  setIsSlimMode?: (isSlimMode: boolean) => void;
  /** 根节点数据 */
  rootNodes?: SpanNode[];
}

export interface TraceSearchRef {
  /** 清空搜索 */
  clear: () => void;
}
export const TraceSearch = forwardRef<TraceSearchRef, TraceSearchProps>(
  (
    {
      setSearchFilters,
      onClear,
      isSlimMode = false,
      setIsSlimMode,
      selectedSpanId,
      onToggleAll,
      isAllExpanded = false,
      searchFilters,
      filterNonCritical = false,
      onFilterNonCriticalChange,
      matchedSpanIds = [],
      onSelect,
      treeSelectorConfig,
      rootNodes,
    },
    ref,
  ) => {
    const { t } = useLocale();
    const treeSelectorRef = useRef<TraceSelectorRef>(null);
    const {
      inputFilterField,
      handleSearch,
      setInputFilterField,
      handleSelectorChange,
    } = useTraceSearchLogic({
      setSearchFilters,
      onClear,
      selectedSpanId,
      searchFilters,
    });

    // 当前选中的匹配节点索引
    const currentMatchIndex = matchedSpanIds?.findIndex(
      id => id === selectedSpanId,
    );
    const hasMatches = matchedSpanIds?.length > 0;

    // 切换到下一个匹配节点
    const handleNextMatch = () => {
      if (!hasMatches || !onSelect) {
        return;
      }
      const nextIndex = (currentMatchIndex + 1) % matchedSpanIds.length;
      onSelect(matchedSpanIds[nextIndex]);
    };
    const handleClear = () => {
      setInputFilterField(undefined);
      treeSelectorRef?.current?.setFieldValues(
        [
          {
            key: 'filters',
            value: {
              filter_fields: [],
            },
          },
        ],
        'filterSelect',
      );
    };
    useImperativeHandle(ref, () => ({
      clear: handleClear,
    }));
    // 切换到上一个匹配节点
    const handlePrevMatch = () => {
      if (!hasMatches || !onSelect) {
        return;
      }
      const prevIndex =
        (currentMatchIndex - 1 + matchedSpanIds.length) % matchedSpanIds.length;
      onSelect(matchedSpanIds[prevIndex]);
    };
    // 键盘事件处理函数
    const handleKeyDown = useCallback(
      (event: KeyboardEvent) => {
        // 只在搜索状态下且有匹配结果时处理键盘事件
        if (!hasMatches || !onSelect) {
          return;
        }
        if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
          event.preventDefault();
          if (event.key === 'ArrowUp') {
            handlePrevMatch();
          } else if (event.key === 'ArrowDown') {
            handleNextMatch();
          }
        }
      },
      [hasMatches, onSelect, handlePrevMatch, handleNextMatch],
    );

    // 添加键盘事件监听
    useEffect(() => {
      // 只在搜索状态下且有匹配结果时添加监听
      if (!hasMatches) {
        return;
      }
      document.addEventListener('keydown', handleKeyDown);
      return () => {
        document.removeEventListener('keydown', handleKeyDown);
      };
    }, [handleKeyDown, hasMatches]);
    const filterCount =
      treeSelectorRef?.current?.getState()?.filters?.filter_fields?.length || 0;
    const isNodesDepthLessThan2 = useMemo(
      () => isRootNodesDepthLessThan2(rootNodes),
      [rootNodes],
    );
    return (
      <div className="">
        <div className="flex px-1 py-1 items-center gap-1 border-0 border-b border-solid !border-[var(--semi-color-border-2)]">
          <SearchInput
            value={inputFilterField}
            onChange={(newFilterField: FilterField) => {
              handleSearch(newFilterField);
            }}
            placeholder={t('input_placeholder')}
            className="flex-1"
          />
          {treeSelectorConfig ? (
            <TraceSelector
              ref={treeSelectorRef}
              {...treeSelectorConfig}
              onChange={newValue => {
                const selectorFilters = (newValue?.filters ||
                  []) as FilterFields;
                handleSelectorChange(selectorFilters);
              }}
              triggerFilterSelectRender={
                <Tooltip content={t('filter')}>
                  <Button
                    color="secondary"
                    icon={<IconCozFilter />}
                    className="coz-fg-secondary relative"
                  >
                    {filterCount > 0 ? (
                      <div className="absolute flex items-center justify-center top-[2px] right-[5px] w-[16px] h-[16px] bg-[rgba(var(--coze-up-brand-4))] text-[rgba(var(--coze-up-brand-9))] rounded-full !text-[10px] ">
                        {filterCount}
                      </div>
                    ) : null}
                  </Button>
                </Tooltip>
              }
            />
          ) : null}
          <Tooltip
            content={isAllExpanded ? t('collapse_all') : t('expand_all')}
            theme="dark"
          >
            <Button
              color="secondary"
              disabled={isSlimMode || isNodesDepthLessThan2}
              icon={
                isAllExpanded ? (
                  <IconCozArrowMiddle className="w-[16px] h-[16px]" />
                ) : (
                  <IconCozLoose className="w-[16px] h-[16px]" />
                )
              }
              onClick={onToggleAll}
              className="coz-fg-secondary"
            />
          </Tooltip>

          <Dropdown
            trigger="click"
            position="bottomRight"
            showArrow={false}
            content={
              <div
                className="p-2"
                onClick={() => {
                  if (!hasMatches) {
                    onFilterNonCriticalChange?.(!filterNonCritical);
                  }
                }}
              >
                <div
                  className={`flex items-center gap-4  ${hasMatches ? 'cursor-not-allowed' : 'cursor-pointer'}`}
                >
                  <Tooltip
                    content="开启后，调用树将仅展示模型与工具节点"
                    position="left"
                    theme="dark"
                    className="!w-[280px] !max-w-[280px]"
                  >
                    <span
                      className={`${hasMatches ? 'coz-fg-secondary' : 'coz-fg-primary'} text-[14px]`}
                    >
                      {t('show_critical_nodes_only')}
                    </span>
                  </Tooltip>
                  <Switch
                    size="small"
                    checked={filterNonCritical}
                    disabled={hasMatches}
                  />
                </div>
              </div>
            }
          >
            <div>
              <Tooltip content={t('more_settings')}>
                <Button
                  color="secondary"
                  icon={<IconCozSetting className="w-[16px] h-[16px]" />}
                  className="coz-fg-secondary"
                />
              </Tooltip>
            </div>
          </Dropdown>
        </div>
        {hasMatches ? (
          <div className="flex px-3 py-2 w-full  justify-between items-center border-0 border-b border-solid !border-[var(--semi-color-border-2)]">
            <div className="flex  flex-1 gap-1 items-center">
              <span className="!text-[12px]">
                <span className="!min-w-[10px] inline-block">
                  {currentMatchIndex === -1 ? '-' : currentMatchIndex + 1}
                </span>{' '}
                / {matchedSpanIds.length}
              </span>
              <Tooltip content={t('prev_item')}>
                <Button
                  size="mini"
                  color="secondary"
                  className="ml-2"
                  onClick={handlePrevMatch}
                  disabled={matchedSpanIds.length <= 1}
                  icon={<IconCozArrowUp className="w-[16px] h-[16px]" />}
                ></Button>
              </Tooltip>
              <Tooltip content={t('next_item')}>
                <Button
                  size="mini"
                  color="secondary"
                  className="ml-1"
                  onClick={handleNextMatch}
                  disabled={matchedSpanIds.length <= 1}
                  icon={<IconCozArrowDown className="w-[16px] h-[16px]" />}
                ></Button>
              </Tooltip>
            </div>
            <Tooltip content={t('filter_show_results_only')}>
              <Checkbox
                checked={isSlimMode}
                onChange={e => {
                  setIsSlimMode?.(!!e?.target?.checked);
                }}
                className="ml-2 !text-[12px]"
              >
                {t('filter_item')}
              </Checkbox>
            </Tooltip>
          </div>
        ) : null}
      </div>
    );
  },
);
