// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines */
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable max-lines-per-function */
import React, { useEffect, useMemo, useRef, useState } from 'react';

import { isEmpty, keys } from 'lodash-es';
import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type PlatformType,
  type SpanListType,
} from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';
import {
  IconCozFilter,
  IconCozInfoCircle,
  IconCozArrowDown,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Dropdown,
  Input,
  Select,
  Tooltip,
  Popover,
  Toast,
} from '@coze-arch/coze-design';

import { NumberDot } from '../number-dot';
import { checkFilterHasEmpty } from '../logic-expr/utils';
import {
  type LogicValue,
  type CustomRightRenderMap,
  AnalyticsLogicExpr,
} from '../logic-expr/logic-expr';
import { AUTO_EVAL_FEEDBACK, MANUAL_FEEDBACK } from '../logic-expr/const';
import type { View } from '../filter-bar/custom-view';

import styles from './index.module.less';

export interface FilterSelectUIProps {
  filters: LogicValue;
  onFiltersChange?: (params: {
    filters: LogicValue;
    viewMethod: string;
    dataSource: string;
  }) => void;
  fieldMetas: any;
  viewMethod: string | number;
  dataSource: string | number;
  onClearFilters?: () => void;
  onApplyFilters?: (
    filters: LogicValue,
    viewMethod: string | number,
    dataSource: string | number,
    metaInfo?: Record<string, any>,
  ) => void;
  onViewNameValidate?: (name: string) => { isValid: boolean; message: string };
  triggerRender?: React.ReactNode;
  invalidateExpr?: Set<string>;
  onSaveToCurrentView?: (params: {
    filters: LogicValue;
    viewMethod: string;
    dataSource: string;
  }) => void;
  onSaveToCustomView?: (params: {
    filters: LogicValue;
    viewMethod: string;
    dataSource: string;
    name: string;
  }) => void;
  customFooter?: (props: {
    onCancel?: () => void;
    onSave?: () => void;
    currentFilter: {
      filters: LogicValue;
      viewMethod: string;
      dataSource: string;
    };
  }) => React.JSX.Element;
  onVisibleChange?: (visible: boolean) => void;
  visible?: boolean;
  allowSaveToCurrentView?: boolean;
  selectedView?: View;
  platformEnumOptionList: { label: string; value: string | number }[];
  customRightRenderMap?: CustomRightRenderMap;
  customLeftRenderMap?: CustomRightRenderMap;
  spanTabOptionList: { label: string; value: string | number }[];
  ignoreKeys?: string[];
  readonly?: boolean;
  disabled?: boolean;
  hideSaveToViewButton?: boolean;
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
    viewMethod: initViewMethod,
    dataSource: initDataSource,
    onClearFilters,
    onApplyFilters,
    onViewNameValidate,
    triggerRender,
    onSaveToCurrentView,
    onSaveToCustomView,
    customFooter,
    onVisibleChange,
    allowSaveToCurrentView = false,
    visible: propsVisible,
    selectedView,
    invalidateExpr,
    platformEnumOptionList,
    spanTabOptionList,
    customRightRenderMap,
    customLeftRenderMap,
    ignoreKeys = [],
    readonly = false,
    disabled = false,
    hideSaveToViewButton = false,
  } = props;

  const [filterVisible, setFilterVisible] = useState(propsVisible || false);
  const [saveViewVisible, setSaveViewVisible] = useState(false);
  const [saveViewName, setSaveViewName] = useState<string>('');
  const [saveViewNameVisible, setSaveViewNameVisible] = useState(false);
  const { spaceID } = useSpace();

  const [localFilters, setLocalFilters] = useState<LogicValue>(
    filterFiltersWithIgnoreKeys(filters, ignoreKeys || []),
  );
  const [localViewMethod, setLocalViewMethod] = useState(initViewMethod);
  const [localDataSource, setLocalDataSource] = useState(initDataSource);
  const [saveViewNameMessage, setSaveViewNameMessage] = useState('');
  const [saveViewNameValidate, setSaveViewNameValidate] = useState(false);

  const filterWrapperRef = useRef<HTMLDivElement>(null);
  const sizeSelectRef = useRef<HTMLDivElement>(null);

  const disableApply = checkFilterHasEmpty(localFilters);
  const guard = useGuard({ point: GuardPoint['ob.trace.custom_view'] });

  const { data: fieldMetasInner } = useRequest(
    async () => {
      if (readonly || disabled) {
        return props.fieldMetas || {};
      }
      const result = await observabilityTrace.GetTracesMetaInfo(
        {
          platform_type: localDataSource as PlatformType,
          span_list_type: localViewMethod as SpanListType,
          workspace_id: spaceID,
        },
        {
          __disableErrorToast: true,
        },
      );
      return result?.field_metas ?? {};
    },
    {
      refreshDeps: [localDataSource, localViewMethod],
      onError(e) {
        Toast.error(
          I18n.t('observation_fetch_meta_error', {
            msg: e.message || '',
          }),
        );
      },
    },
  );

  const fieldMetas = useMemo(() => {
    if (readonly || disabled) {
      return props.fieldMetas;
    }

    return fieldMetasInner;
  }, [readonly, disabled, fieldMetasInner, props.fieldMetas]);

  const invalidateExprs = useMemo(() => {
    if (!fieldMetas) {
      return new Set() as Set<string>;
    }

    const currentInvalidateExpr = localFilters?.filter_fields
      ?.filter(
        filedFilter =>
          !(keys(fieldMetas) ?? []).includes(filedFilter.field_name) &&
          filedFilter.logic_field_name_type !== AUTO_EVAL_FEEDBACK &&
          filedFilter.logic_field_name_type !== MANUAL_FEEDBACK,
      )
      .map(filedFilter => filedFilter.field_name);

    return new Set(currentInvalidateExpr);
  }, [localFilters?.filter_fields, fieldMetas]);

  const shouldHideAndLine = readonly && isEmpty(filters.filter_fields);

  const handleApply = () => {
    onApplyFilters?.(
      localFilters,
      localViewMethod,
      localDataSource,
      fieldMetas,
    );
    setFilterVisible(false);
  };

  useEffect(() => {
    if (propsVisible === undefined) {
      return;
    }
    setFilterVisible(propsVisible);
  }, [propsVisible]);

  const FixedSelect = () => (
    <>
      <div className="box-border h-[32px] flex items-center gap-x-2 justify-between">
        <Select
          value={I18n.t('viewing_method')}
          disabled
          className="!outline-none !h-[32px] !w-[280px] box-border"
          showArrow={false}
        />
        <Select
          defaultValue={I18n.t('belong_to')}
          className="w-[80px] box-border !h-[32px]"
          disabled
        />
        <Select
          value={localViewMethod}
          optionList={spanTabOptionList}
          className="min-w-[124px] box-border flex-1 !h-[32px]"
          onChange={value => {
            setLocalViewMethod(value as string);
          }}
          disabled={readonly || disabled}
        />
      </div>
      <div className="box-border h-[32px] flex items-center gap-x-2 justify-between">
        <Select
          value={I18n.t('data_source')}
          disabled
          className="!outline-none !h-[32px] !w-[280px] box-border"
          showArrow={false}
        />
        <Select
          defaultValue={I18n.t('belong_to')}
          className="w-[80px] box-border !h-[32px]"
          disabled
        />
        <Select
          value={localDataSource}
          optionList={platformEnumOptionList}
          className="min-w-[124px] box-border flex-1 !h-[32px]"
          onChange={value => {
            setLocalDataSource(value as string);
          }}
          disabled={readonly || disabled}
        />
      </div>
    </>
  );

  const renderSaveView = () => (
    <div className="shadow-default coz-bg-max rounded-[6px] flex flex-col gap-y-2 min-w-[240px]">
      <div>{I18n.t('view_name')}</div>
      <div className="rounded-[6px]">
        <Input
          placeholder={I18n.t('please_input', { field: I18n.t('view_name') })}
          value={saveViewName}
          onChange={value => {
            const trimValue = value.trim();
            setSaveViewName(trimValue);
            const { isValid, message } = onViewNameValidate?.(trimValue) ?? {};
            if (isValid) {
              setSaveViewNameMessage('');
              setSaveViewNameValidate(true);
            } else {
              setSaveViewNameMessage(message ?? '');
              setSaveViewNameValidate(false);
            }
          }}
        />
      </div>
      {saveViewNameMessage ? (
        <div className="text-[#D0292F] text-[12px]">{saveViewNameMessage}</div>
      ) : null}
      <div className="flex items-center justify-end gap-x-1">
        <Button
          type="primary"
          color="primary"
          onClick={() => {
            setSaveViewNameVisible(false);
            setSaveViewVisible(false);
          }}
        >
          {I18n.t('cancel')}
        </Button>
        <Button
          disabled={!saveViewNameValidate}
          type="primary"
          color="brand"
          onClick={() => {
            if (!saveViewNameValidate) {
              return;
            }
            setSaveViewNameVisible(false);
            setSaveViewVisible(false);
            onSaveToCustomView?.({
              filters: localFilters,
              viewMethod: localViewMethod.toString(),
              dataSource: localDataSource.toString(),
              name: saveViewName,
            });
          }}
        >
          {I18n.t('save')}
        </Button>
      </div>
    </div>
  );

  return (
    <Dropdown
      visible={filterVisible}
      trigger="custom"
      keepDOM={false}
      onVisibleChange={visible => {
        if (!visible) {
          setLocalViewMethod(initViewMethod);
          setLocalDataSource(initDataSource);
          setSaveViewName('');
          setSaveViewNameMessage('');
          setLocalFilters({} as unknown as LogicValue);
        } else {
          setLocalFilters(filterFiltersWithIgnoreKeys(filters, ignoreKeys));
          setLocalViewMethod(initViewMethod);
          setLocalDataSource(initDataSource);
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
          className="min-w-[656px] max-w-[656x] w-[656px] min-h-[256px] py-3 box-border flex gap-y-3 flex-col"
          onClick={e => {
            e.stopPropagation();
            e.preventDefault();
          }}
        >
          <div className="flex w-full items-center justify-between px-4 box-border">
            <div className="flex items-center gap-x-1 text-[var(--coz-fg-primary)]">
              <div className="text-[14px] font-medium leading-[20px]">
                {I18n.t('filter')}
              </div>
              <Tooltip
                theme="dark"
                trigger="hover"
                content={I18n.t('viewing_method_data_source_linkage')}
              >
                <IconCozInfoCircle />
              </Tooltip>
            </div>
            {!readonly && !disabled && (
              <span
                className="text-[12px] leading-[16px] font-medium text-[var(--coz-fg-secondary)] flex items-center hover:text-[rgb(var(--coze-up-brand-9))] cursor-pointer"
                onClick={() => {
                  onClearFilters?.();
                  setLocalFilters({} as unknown as LogicValue);
                }}
              >
                {I18n.t('clear')}
              </span>
            )}
          </div>
          <div
            className={classNames(
              'pl-[54px] box-border relative pr-4',
              shouldHideAndLine && '!pl-4',
            )}
            ref={filterWrapperRef}
          >
            {!shouldHideAndLine && (
              <div
                className="absolute w-[32px] h-[28px] bg-white left-[17px] z-[101] flex items-center text-[var(--coz-fg-secondary)] text-[13px]"
                style={{
                  bottom:
                    'calc((100% - ((100% - 80px) / 2) - 16px) / 2 + (100% - 80px) / 2 - 14px)',
                }}
              >
                {I18n.t('observation_and')}
              </div>
            )}

            <div
              className={classNames(styles.fixedSelect, {
                [styles['hide-line']]: shouldHideAndLine,
              })}
            >
              <FixedSelect />
            </div>
            {!shouldHideAndLine && (
              <div
                ref={sizeSelectRef}
                className={classNames(styles.sizedSelect, {
                  [styles.empty]: isEmpty(localFilters),
                })}
              >
                <div
                  className={classNames(styles['logic-expr-wrapper'], {
                    [styles['logic-expr-wrapper-empty']]: isEmpty(localFilters),
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
                        setLocalFilters(value ?? {});
                      }}
                      ignoreKeys={ignoreKeys}
                      disabled={readonly || disabled}
                    />
                  ) : null}
                </div>
              </div>
            )}
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
                    viewMethod: localViewMethod.toString(),
                    dataSource: localDataSource.toString(),
                  },
                })
              ) : (
                <>
                  <>
                    {!hideSaveToViewButton && (
                      <div>
                        {selectedView ? (
                          <Dropdown
                            trigger="custom"
                            visible={saveViewVisible}
                            preventScroll
                            position="bottomRight"
                            onClickOutSide={() => {
                              if (saveViewNameVisible) {
                                return;
                              }
                              setSaveViewVisible(false);
                            }}
                            onVisibleChange={visible => {
                              setSaveViewVisible(visible);
                            }}
                            render={
                              <Dropdown.Menu className="!min-w-[140px] !max-w-[140px] !w-[140px] !box-border">
                                <Dropdown.Item
                                  disabled={!allowSaveToCurrentView}
                                  type="primary"
                                  className={styles['dropdown-item']}
                                  onClick={() => {
                                    setSaveViewVisible(false);
                                    onSaveToCurrentView?.({
                                      filters: localFilters,
                                      viewMethod: localViewMethod.toString(),
                                      dataSource: localDataSource.toString(),
                                    });
                                  }}
                                >
                                  {I18n.t('save_to_current_view')}
                                </Dropdown.Item>

                                <Popover
                                  visible={saveViewNameVisible}
                                  showArrow
                                  zIndex={9999}
                                  trigger="click"
                                  position="right"
                                  onVisibleChange={visible => {
                                    setSaveViewNameVisible(visible);
                                    if (!visible) {
                                      setSaveViewVisible(false);
                                    }
                                  }}
                                  content={renderSaveView()}
                                >
                                  <Dropdown.Item
                                    type="primary"
                                    className="!py-0 !px-2 !box-border"
                                    onClick={() => {
                                      setSaveViewNameVisible(true);
                                    }}
                                  >
                                    {I18n.t('save_as_view')}
                                  </Dropdown.Item>
                                </Popover>
                              </Dropdown.Menu>
                            }
                          >
                            <Button
                              type="primary"
                              color="primary"
                              disabled={guard.data.readonly || disableApply}
                              className={`${allowSaveToCurrentView ? '' : '!text-[var(--coz-fg-dim)] !bg-[rgba(var(--coze-bg-5), var(--coze-bg-5-alpha))'}`}
                              onClick={event => {
                                event.preventDefault();
                                event.stopPropagation();
                                setSaveViewVisible(true);
                              }}
                            >
                              <div className="flex items-center gap-x-2">
                                <span>{I18n.t('save_view')}</span>
                                <IconCozArrowDown />
                              </div>
                            </Button>
                          </Dropdown>
                        ) : (
                          <Popover
                            visible={saveViewNameVisible}
                            showArrow
                            trigger="custom"
                            position="bottom"
                            onVisibleChange={visible => {
                              setSaveViewNameVisible(visible);
                              if (!visible) {
                                setSaveViewVisible(false);
                              }
                            }}
                            content={renderSaveView()}
                          >
                            <Button
                              type="primary"
                              color="primary"
                              disabled={guard.data.readonly || disableApply}
                              onClick={() => {
                                setSaveViewNameVisible(true);
                              }}
                            >
                              {I18n.t('save_view')}
                            </Button>
                          </Popover>
                        )}
                      </div>
                    )}
                  </>

                  <Button
                    type="primary"
                    color="brand"
                    onClick={handleApply}
                    disabled={disableApply}
                  >
                    {I18n.t('apply')}
                  </Button>
                </>
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
                <div className="text-sm">{I18n.t('filter')}</div>
                <NumberDot
                  count={
                    (filters.filter_fields?.length ?? 0) +
                    2 -
                    (invalidateExpr?.size ?? 0)
                  }
                  color={(invalidateExpr?.size ?? 0 > 0) ? 'error' : 'brand'}
                />
              </div>
            </Button>
          </div>
        )}
      </div>
    </Dropdown>
  );
};
