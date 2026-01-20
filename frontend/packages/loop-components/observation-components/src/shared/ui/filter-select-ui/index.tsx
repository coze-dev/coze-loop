// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */

/* eslint-disable @coze-arch/max-line-per-function */

import React, { useEffect, useMemo, useRef, useState } from 'react';

import { isEmpty, keys } from 'lodash-es';
import cls from 'classnames';
import { useRequest } from 'ahooks';
import { type FieldMeta } from '@cozeloop/api-schema/observation';
import { IconCozFilter } from '@coze-arch/coze-design/icons';
import { Button, Dropdown, Spin, Toast } from '@coze-arch/coze-design';

import type { View } from '@/shared/components/filter-bar/custom-view';
import {
  checkFilterHasEmpty,
  getExprTypeByFileName,
} from '@/shared/components/analytics-logic-expr/utils';
import {
  type LogicValue,
  AnalyticsLogicExpr,
  type CustomRightRenderMap,
} from '@/shared/components/analytics-logic-expr/logic-expr';
import {
  API_FEEDBACK,
  AUTO_EVAL_FEEDBACK,
  MANUAL_FEEDBACK,
  METADATA,
} from '@/shared/components/analytics-logic-expr/const';
import { useLocale } from '@/i18n';

import { NumberDot } from '../number-dot';

import styles from './index.module.less';

export interface FilterSelectUIProps {
  mode?: 'simple' | 'popup';
  filters: LogicValue;
  customLeftRenderMap?: CustomRightRenderMap;
  customRightRenderMap?: CustomRightRenderMap;
  onFiltersChange?: (params: {
    filters: LogicValue;
    spanListType: string;
    platformType: string;
  }) => void;
  fieldMetas?: Record<string, FieldMeta>;
  getFieldMetas?: (params: {
    platform_type: string;
    span_list_type: string;
  }) => Promise<Record<string, FieldMeta>> | Record<string, FieldMeta>;
  spanListType: string;
  platformType: string;
  onClearFilters?: () => void;

  onApplyFilters?: (
    filters: LogicValue,
    spanListType: string,
    platformType: string,
    metaInfo?: Record<string, FieldMeta>,
  ) => void;
  onViewNameValidate?: (name: string) => { isValid: boolean; message: string };
  triggerRender?: React.ReactNode;
  invalidateExpr?: Set<string>;
  customFooter?: (props: {
    onCancel?: () => void;
    onSave?: () => void;
    currentFilter: {
      filters: LogicValue;
      spanListType: string;
      platformType: string;
    };
  }) => React.JSX.Element;
  onVisibleChange?: (visible: boolean) => void;
  visible?: boolean;
  selectedView?: View;
  ignoreKeys?: string[];
  disabledRowKeys?: string[];
  readonly?: boolean;
  disabled?: boolean;
  workspaceId?: string;
}

const filterFiltersWithIgnoreKeys: (
  filters: LogicValue,
  ignoreKeys?: string[],
) => LogicValue = (filters: LogicValue, ignoreKeys?: string[]) => {
  if (!ignoreKeys || isEmpty(ignoreKeys)) {
    return filters;
  }

  const { query_and_or, filter_fields, sub_filter } = filters;

  return {
    query_and_or,
    filter_fields: filter_fields?.filter(
      fieldFilter =>
        !ignoreKeys.includes(
          fieldFilter.logic_field_name_type ?? fieldFilter.field_name,
        ),
    ),
    sub_filter: sub_filter?.map(spanFilter =>
      filterFiltersWithIgnoreKeys(spanFilter, ignoreKeys),
    ),
  };
};

export const FilterSelectUI = (props: FilterSelectUIProps) => {
  const {
    filters,
    spanListType,
    platformType,
    onClearFilters,
    onApplyFilters,
    triggerRender,
    customFooter,
    onVisibleChange,
    visible: propsVisible,
    invalidateExpr,
    ignoreKeys = [],
    disabledRowKeys = [],
    readonly = false,
    disabled = false,
    fieldMetas: propFieldMetas,
    getFieldMetas,
    mode = 'popup',
    customLeftRenderMap,
    customRightRenderMap,
  } = props;

  const [filterVisible, setFilterVisible] = useState(propsVisible || false);
  const [saveViewVisible] = useState(false);
  const [, setSaveViewName] = useState<string>('');
  const [saveViewNameVisible] = useState(false);
  const { t } = useLocale();

  const [localFilters, setLocalFilters] = useState<LogicValue>(
    filterFiltersWithIgnoreKeys(filters, ignoreKeys || []),
  );

  const [, setSaveViewNameMessage] = useState('');

  const { data: fetchedFieldMetas, loading } = useRequest(
    async () => {
      if (!getFieldMetas) {
        return propFieldMetas || {};
      }

      if (!platformType || !spanListType) {
        return {};
      }

      try {
        const result = await getFieldMetas({
          platform_type: platformType,
          span_list_type: spanListType,
        });
        return result || {};
      } catch (e) {
        Toast.error(
          t('analytics_fetch_meta_error', {
            msg: (e as Error)?.message || '',
          }),
        );
        return {};
      }
    },
    {
      refreshDeps: [platformType, spanListType],
      onError(e) {
        Toast.error(
          t('analytics_fetch_meta_error', {
            msg: e.message || '',
          }),
        );
      },
    },
  );

  const fieldMetas = useMemo(() => {
    if ((readonly || disabled) && propFieldMetas) {
      return propFieldMetas;
    }
    return fetchedFieldMetas;
  }, [readonly, disabled, fetchedFieldMetas, propFieldMetas]);

  const filterWrapperRef = useRef<HTMLDivElement>(null);
  const sizeSelectRef = useRef<HTMLDivElement>(null);

  const disableApply = checkFilterHasEmpty(localFilters);

  const invalidateExprs = useMemo(() => {
    if (!fieldMetas || loading) {
      return new Set() as Set<string>;
    }

    const currentInvalidateExpr = localFilters?.filter_fields
      ?.filter(filedFilter => {
        const filterType = getExprTypeByFileName(
          filedFilter.field_name,
          fieldMetas,
          filedFilter.logic_field_name_type,
        );
        return (
          !(keys(fieldMetas) ?? []).includes(filedFilter.field_name) &&
          filterType !== AUTO_EVAL_FEEDBACK &&
          filterType !== MANUAL_FEEDBACK &&
          filterType !== API_FEEDBACK &&
          filterType !== METADATA
        );
      })
      .map(filedFilter => filedFilter.field_name);

    return new Set(currentInvalidateExpr);
  }, [localFilters?.filter_fields, fieldMetas, loading]);

  const handleApply = () => {
    onApplyFilters?.(localFilters, spanListType, platformType, fieldMetas);
    setFilterVisible(false);
  };

  useEffect(() => {
    if (propsVisible === undefined) {
      return;
    }
    setFilterVisible(propsVisible);
  }, [propsVisible]);

  const filterCount =
    (filters.filter_fields?.filter(
      item => !(ignoreKeys ?? []).includes(item.field_name),
    ).length ?? 0) - (invalidateExpr?.size ?? 0);

  if (loading) {
    return <Spin />;
  }

  if (mode === 'simple') {
    return (
      <>
        {fieldMetas ? (
          <AnalyticsLogicExpr
            customRightRenderMap={customRightRenderMap}
            customLeftRenderMap={customLeftRenderMap}
            invalidateExpr={invalidateExprs}
            allowLogicOperators={['and', 'or']}
            tagFilterRecord={fieldMetas}
            value={localFilters}
            disableDuplicateSelect={true}
            defaultImmutableKeys={undefined}
            onChange={value => {
              setLocalFilters(value as LogicValue);
              onApplyFilters?.(
                value as LogicValue,
                spanListType,
                platformType,
                fieldMetas,
              );
            }}
            ignoreKeys={ignoreKeys}
            disabledRowKeys={disabledRowKeys}
            disabled={readonly || disabled}
          />
        ) : null}
      </>
    );
  }
  return (
    <Dropdown
      visible={filterVisible}
      trigger="custom"
      keepDOM={false}
      onVisibleChange={visible => {
        if (!visible) {
          setSaveViewName('');
          setSaveViewNameMessage('');
          setLocalFilters({} as unknown as LogicValue);
        } else {
          setLocalFilters(filterFiltersWithIgnoreKeys(filters, ignoreKeys));
        }
        onVisibleChange?.(visible);
      }}
      position="bottomRight"
      onClickOutSide={() => {
        if (saveViewVisible || saveViewNameVisible) {
          return;
        }
        setFilterVisible(false);
      }}
      zIndex={1000}
      render={
        <div
          className="min-w-[746px] max-w-[746px] w-[746px] py-3 box-border flex gap-y-3 flex-col"
          onClick={e => {
            e.stopPropagation();
            e.preventDefault();
          }}
        >
          <div className="flex w-full items-center justify-between px-4 box-border">
            <div className="flex items-center gap-x-1 text-[var(--coz-fg-primary)]">
              <div className="text-[14px] font-medium leading-[20px]">
                {t('filter')}
              </div>
            </div>
            {!readonly &&
              !disabled &&
              localFilters?.filter_fields !== undefined &&
              !isEmpty(localFilters.filter_fields) && (
                <span
                  className="text-[12px] leading-[16px] font-medium text-[var(--coz-fg-secondary)] flex items-center hover:text-[rgb(var(--coze-up-brand-9))] cursor-pointer"
                  onClick={() => {
                    onClearFilters?.();
                    setLocalFilters({} as unknown as LogicValue);
                  }}
                >
                  {t('clear_filter')}
                </span>
              )}
          </div>
          <div
            className={cls('box-border relative px-4')}
            ref={filterWrapperRef}
          >
            <div
              ref={sizeSelectRef}
              className={cls(styles.sizedSelect, {
                [styles.empty]:
                  localFilters?.filter_fields === undefined ||
                  isEmpty(localFilters.filter_fields),
              })}
            >
              <div
                className={cls(styles['logic-expr-wrapper'], {
                  [styles['logic-expr-wrapper-empty']]:
                    localFilters?.filter_fields === undefined ||
                    isEmpty(localFilters.filter_fields),
                })}
              >
                {fieldMetas ? (
                  <AnalyticsLogicExpr
                    customRightRenderMap={customRightRenderMap}
                    customLeftRenderMap={customLeftRenderMap}
                    invalidateExpr={invalidateExprs}
                    allowLogicOperators={['and', 'or']}
                    tagFilterRecord={fieldMetas}
                    value={localFilters}
                    disableDuplicateSelect={true}
                    defaultImmutableKeys={undefined}
                    onChange={value => {
                      setLocalFilters(value as LogicValue);
                    }}
                    ignoreKeys={ignoreKeys}
                    disabledRowKeys={disabledRowKeys}
                    disabled={readonly || disabled}
                  />
                ) : null}
              </div>
            </div>
          </div>
          {
            <div className="border-0 border-t border-solid border-[var(--coz-stroke-primary)] flex items-center justify-end gap-x-2 pt-3 px-4">
              {customFooter ? (
                customFooter({
                  onCancel: () => {
                    setFilterVisible(false);
                  },
                  onSave: () => {
                    setFilterVisible(false);
                  },
                  currentFilter: {
                    filters: localFilters,
                    spanListType,
                    platformType,
                  },
                })
              ) : (
                <Button
                  type="primary"
                  color="brand"
                  onClick={handleApply}
                  disabled={disableApply}
                >
                  {t('apply_button')}
                </Button>
              )}
            </div>
          }
        </div>
      }
    >
      <div
        onClick={() => {
          setFilterVisible(true);
        }}
      >
        {triggerRender && React.isValidElement(triggerRender) ? (
          triggerRender
        ) : (
          <div className="rounded-[6px] border border-solid border-[var(--coz-stroke-plus)] flex items-center justify-center box-border !h-[32px]">
            <Button
              className="flex items-center gap-x-1 !px-[8px] !py-[8px] !box-border !text-sm !h-[30px]"
              color="secondary"
              type="primary"
              size="small"
            >
              <div className="flex items-center gap-x-1">
                <IconCozFilter />
                <div className="text-sm">{t('filter')}</div>
                {(filterCount > 0 || invalidateExpr?.size !== 0) && (
                  <NumberDot
                    count={filterCount}
                    color={invalidateExpr?.size !== 0 ? 'error' : 'brand'}
                  />
                )}
              </div>
            </Button>
          </div>
        )}
      </div>
    </Dropdown>
  );
};
