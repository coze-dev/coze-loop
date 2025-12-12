/* eslint-disable max-lines */
/* eslint-disable complexity */
/* eslint-disable max-lines-per-function */
/* eslint-disable @typescript-eslint/no-shadow */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/consistent-type-assertions */
/* eslint-disable @typescript-eslint/no-explicit-any */

import {
  useEffect,
  useMemo,
  useRef,
  useState,
  useImperativeHandle,
  forwardRef,
  type ReactNode,
} from 'react';

import { set, isEqual, cloneDeep, isEmpty } from 'lodash-es';
import {
  FieldType,
  PlatformType,
  QueryType,
  SpanListType,
  type FieldMeta,
} from '@cozeloop/api-schema/observation';
import { type DatePickerProps, type OptionProps } from '@coze-arch/coze-design';

import { type ValueOf } from '@/shared/types/utils';
import { type DatePickerOptions } from '@/shared/components/filter-bar/types';
import { type PreselectedDatePickerRef } from '@/shared/components/date-picker';
import {
  type CustomRightRenderMap,
  type LogicItem,
} from '@/shared/components/analytics-logic-expr/logic-expr';
import { useLocale } from '@/i18n';
import { useCustomView } from '@/features/trace-list/hooks/use-custom-view';
import { type PresetRange } from '@/features/trace-list/constants/time';
import {
  PlatformSelect,
  SpanTypeSelect,
  FilterSelect,
  PreselectedDatePicker,
  CustomView,
  type LogicValue,
  type TimeStamp,
} from '@/features/trace-list/components';

import { SelectorItemWrapper } from './selector-item-wrapper';

export const BUILD_IN_SELECTOR = {
  DateTimePicker: 'dateTimePicker',
  SpanListType: 'spanListType',
  PlatformType: 'platformType',
  FilterSelect: 'filterSelect',
  CustomView: 'customView',
};

export type LayoutMode = 'horizontal' | 'vertical';

export interface TraceSelectorState {
  platformType?: string;
  spanListType?: string;
  preset?: string;
  timeStamp?: TimeStamp;
  filters?: {
    query_and_or?: string;
    filter_fields: LogicItem[];
  };
  metaInfo?: any;
  [key: string]: any;
}

const getDiffKeys = (oldKeys: string[], newKeys: string[]) =>
  oldKeys.filter(key => !newKeys.includes(key));

export interface CustomSelector {
  render: (
    setFieldValue: (field: string, value: string | string[] | null) => void,
  ) => React.ReactNode;
  filterOptions?: {
    query_type: string;
    field_type?: string;
    defaultValue?: string;
  };
  field: string;
  fieldName: string;
}

export type TraceSelectorItems = (
  | ValueOf<typeof BUILD_IN_SELECTOR>
  | ((
      selectorState: TraceSelectorState,
      layoutMode: LayoutMode,
      disabled?: (name: string) => boolean,
    ) => CustomSelector | null)
)[];

export interface TraceSelectorProps {
  items: TraceSelectorItems;
  onChange?: (value: TraceSelectorState, source: string) => void;
  spanListTypeOptionList: OptionProps[];
  platformTypeOptionList: OptionProps[];
  customViewConfig?: {
    visibility?: boolean;
  };
  datePickerOptions: DatePickerOptions[];
  datePickerProps?: DatePickerProps;
  fieldMetas?: Record<string, FieldMeta>;
  getFieldMetas: (params: {
    platform_type: string | number;
    span_list_type: string | number;
  }) => Promise<Record<string, FieldMeta>>;
  customParams?: Record<string, any>;
  layoutMode?: LayoutMode;
  initValues: TraceSelectorState;
  customRightRenderMap?: CustomRightRenderMap;
  customLeftRenderMap?: CustomRightRenderMap;
  disabled?: boolean;
  disableFilterItemByFilterName?: (name: string) => boolean;
  ignoreKeys?: string[];
  disabledRowKeys?: string[];
  triggerFilterSelectRender?: ReactNode;
  platformTypeConfig?: {
    defaultValue?: string;
    optionList?: OptionProps[];
    visibility?: boolean;
  };
  spanListTypeConfig?: {
    defaultValue?: string;
    optionList?: OptionProps[];
    visibility?: boolean;
  };
}

const cleanChangeFilter = (state: TraceSelectorState) => {
  const newFilter: TraceSelectorState = {
    platformType: state.platformType,
    spanListType: state.spanListType,
    preset: state.preset,
    timeStamp: state.timeStamp,
    filters: {
      query_and_or: state.filters?.query_and_or ?? 'and',
      filter_fields:
        state.filters?.filter_fields?.map(item => {
          const newItem = {};
          Object.keys(item).forEach(key => {
            if (item[key] !== undefined) {
              newItem[key] = item[key];
            }
          });
          return newItem as LogicItem;
        }) ?? [],
    },
  };
  return newFilter;
};

export interface TraceSelectorRef {
  setFieldValues: (
    newStateArray: { key: keyof TraceSelectorState; value: any }[],
    source: string,
  ) => void;

  getState: () => TraceSelectorState;
  closeSelect: () => void;
  closeDatePicker: () => void;
}

export const TraceSelector = forwardRef<TraceSelectorRef, TraceSelectorProps>(
  (props, ref) => {
    const {
      items,
      onChange,
      spanListTypeOptionList,
      platformTypeOptionList,
      customViewConfig,
      datePickerOptions,
      datePickerProps,
      fieldMetas,
      getFieldMetas,
      customParams,
      layoutMode = 'horizontal',
      initValues,
      customRightRenderMap,
      customLeftRenderMap,
      disabled = false,
      triggerFilterSelectRender,
      ignoreKeys = [],
      disabledRowKeys = [],
      platformTypeConfig = {
        visibility: true,
        optionList: platformTypeOptionList,
        defaultValue: initValues.platformType,
      },
      spanListTypeConfig = {
        visibility: true,
        optionList: spanListTypeOptionList,
        defaultValue: initValues.spanListType,
      },
      disableFilterItemByFilterName = () => disabled ?? false,
    } = props;

    const { t } = useLocale();
    const datePickerRef = useRef<PreselectedDatePickerRef>(null);

    const [traceSelectorState, setTraceSelectorState] =
      useState<TraceSelectorState>(() => {
        const dynamicFilters: LogicItem[] = items
          .filter(
            (
              item,
            ): item is (
              selectorState: TraceSelectorState,
              layoutMode: LayoutMode,
              disabled?: (name: string) => boolean,
            ) => CustomSelector | null => typeof item !== 'string',
          )
          .map(item =>
            item(initValues, layoutMode, disableFilterItemByFilterName),
          )
          .filter(
            (filterItem): filterItem is CustomSelector => filterItem !== null,
          )
          .filter(
            filterItem =>
              filterItem.filterOptions && filterItem.filterOptions.defaultValue,
          )
          .map(filterItem => {
            const { filterOptions, field } = filterItem;
            const { query_type, field_type, defaultValue } =
              filterOptions as NonNullable<typeof filterOptions>;

            return {
              query_type,
              field_type,
              values: [defaultValue as string],
              field_name: field,
            } as LogicItem;
          });

        const existingFilterFields = initValues.filters?.filter_fields || [];
        const mergedFilterFields = [...existingFilterFields];

        dynamicFilters.forEach(dynamicFilter => {
          const existingIndex = mergedFilterFields.findIndex(
            item => item.field_name === dynamicFilter.field_name,
          );

          if (existingIndex === -1) {
            mergedFilterFields.push(dynamicFilter);
          }
        });

        const mergedInitValues = {
          ...initValues,
          filters: {
            ...initValues.filters,
            filter_fields: mergedFilterFields,
          },
        };

        return mergedInitValues;
      });

    useImperativeHandle(ref, () => ({
      setFieldValues: (
        newStateArray: { key: keyof TraceSelectorState; value: any }[],
        source: string,
      ) => {
        patchUpdateTraceSelectorState(newStateArray, source);
      },
      getState: () => traceSelectorState,
      closeSelect: () => {
        datePickerRef.current?.closeSelect();
      },
      closeDatePicker: () => {
        datePickerRef.current?.closeDatePicker();
      },
    }));

    const { setActiveViewKey, viewList, activeViewKey, updateViewList } =
      useCustomView({
        visibility: customViewConfig?.visibility,
        customParams,
      });
    const [filterPopupVisible, setFilterPopupVisible] = useState(false);
    const [lastUserRecord, setLastUserRecord] = useState<{
      filters: LogicValue;
      selectedPlatform: string;
      selectedSpanType: string;
    }>({
      filters: { filter_fields: [] },
      selectedPlatform: PlatformType.Cozeloop,
      selectedSpanType: SpanListType.RootSpan,
    });

    const latestTraceCustomKeys = useMemo<string[] | null>(
      () =>
        items
          .map(item => {
            if (typeof item === 'function') {
              const res = item(
                {
                  platformType: traceSelectorState.platformType,
                },
                layoutMode,
              );
              return res?.field ?? '';
            }

            return null;
          })
          .filter(Boolean) as string[],
      [items, traceSelectorState.platformType, layoutMode],
    );

    const latestTraceCustomKeysRef = useRef<string[] | null>(
      latestTraceCustomKeys,
    );

    useEffect(() => {
      const needDeleteKeys = getDiffKeys(
        latestTraceCustomKeysRef.current || [],
        latestTraceCustomKeys || [],
      );
      const onChangeState = {
        ...traceSelectorState,
        filters: {
          ...traceSelectorState.filters,
          filter_fields: traceSelectorState.filters?.filter_fields?.filter(
            item => !(needDeleteKeys ?? []).includes(item.field_name),
          ),
        },
      } as TraceSelectorState;
      latestTraceCustomKeysRef.current = latestTraceCustomKeys ?? [];

      if (isEqual(traceSelectorState, onChangeState)) {
        return;
      }

      const cleanFilter = cleanChangeFilter(onChangeState);

      setTraceSelectorState(cleanFilter);
      onChange?.(cleanFilter, '');
    }, [latestTraceCustomKeys, traceSelectorState]);

    const handleDataChange = (
      onChangeState: TraceSelectorState,
      source: string,
    ) => {
      const newState: TraceSelectorState = {
        ...onChangeState,
        filters: {
          query_and_or: onChangeState.filters?.query_and_or,
          filter_fields:
            onChangeState.filters?.filter_fields?.filter(
              item =>
                !isEmpty(item.values) ||
                item.query_type === QueryType.NotExist ||
                item.query_type === QueryType.Exist,
            ) ?? [],
        },
      };

      onChange?.(newState, source);
    };

    const updateTraceSelectorState = (
      key: keyof TraceSelectorState,
      value: any,
      source: string,
    ) => {
      setTraceSelectorState((prevState: TraceSelectorState) => {
        const onChangeState: TraceSelectorState = cleanChangeFilter({
          ...prevState,
          [key]: value,
        });
        handleDataChange(onChangeState, source);
        return onChangeState;
      });
    };

    const patchUpdateTraceSelectorState = (
      newStateArray: { key: keyof TraceSelectorState; value: any }[],
      source: string,
    ) => {
      setTraceSelectorState((prevState: TraceSelectorState) => {
        const newStateUpdates = newStateArray.reduce((acc, { key, value }) => {
          acc[key] = value;
          return acc;
        }, {} as TraceSelectorState);

        const onChangeState: TraceSelectorState = cleanChangeFilter({
          ...prevState,
          ...newStateUpdates,
        });

        handleDataChange(onChangeState, source);

        return cloneDeep(onChangeState);
      });
    };

    const finalIgnoreKeys = useMemo(
      () =>
        items
          .map(item => {
            if (typeof item !== 'string') {
              return item(traceSelectorState, layoutMode)?.field;
            }
            return undefined;
          })
          .filter(Boolean)
          .concat(ignoreKeys) as string[],
      [items, traceSelectorState, layoutMode, ignoreKeys],
    );

    const FILTER_SELECTOR_MAP = {
      [BUILD_IN_SELECTOR.DateTimePicker]: (
        <SelectorItemWrapper
          layoutMode={layoutMode}
          title={t('filter_date_picker')}
        >
          <PreselectedDatePicker
            ref={datePickerRef}
            disabled={disableFilterItemByFilterName('dateTimePicker')}
            preset={traceSelectorState.preset as PresetRange}
            timeStamp={traceSelectorState.timeStamp as unknown as TimeStamp}
            datePickerOptions={datePickerOptions}
            datePickerProps={datePickerProps}
            onPresetChange={(preset, timeStamp) => {
              patchUpdateTraceSelectorState(
                [
                  { key: 'preset', value: preset },
                  { key: 'timeStamp', value: timeStamp },
                ],
                'dateTimePicker',
              );
              setActiveViewKey(null);
            }}
            oneTimeStampChange={timeStamp => {
              updateTraceSelectorState(
                'timeStamp',
                timeStamp,
                'dateTimePicker',
              );
              setActiveViewKey(null);
            }}
          />
        </SelectorItemWrapper>
      ),
      [BUILD_IN_SELECTOR.CustomView]: customViewConfig?.visibility ? (
        <SelectorItemWrapper
          layoutMode={layoutMode}
          title={t('filter_custom_view')}
        >
          <CustomView
            disabled={disableFilterItemByFilterName('customView')}
            viewList={viewList}
            onSelectView={view => {
              patchUpdateTraceSelectorState(
                [
                  {
                    key: 'platformType',
                    value: view?.platform_type ?? PlatformType.Cozeloop,
                  },
                  {
                    key: 'spanListType',
                    value: view?.spanList_type ?? SpanListType.RootSpan,
                  },
                ],
                'customView',
              );
              if (!view) {
                setActiveViewKey(null);
                return;
              }
              setActiveViewKey(view.id.toString());
            }}
            onDeleteView={updateViewList}
            onUpdateView={updateViewList}
            onCreateView={updateViewList}
            updateViewList={updateViewList}
            activeViewKey={activeViewKey}
            selectedPlatform={traceSelectorState.platformType as PlatformType}
            selectedSpanType={traceSelectorState.spanListType as SpanListType}
            onSelectedPlatformChange={platform => {
              updateTraceSelectorState(
                'platformType',
                platform,
                'platformType',
              );
            }}
            onSelectedSpanTypeChange={spanType => {
              updateTraceSelectorState(
                'spanListType',
                spanType,
                'spanListType',
              );
            }}
            customParams={customParams}
            filters={traceSelectorState.filters ?? { filter_fields: [] }}
            onFiltersChange={filters => {
              updateTraceSelectorState('filters', filters, 'filterSelect');
            }}
            lastUserRecord={lastUserRecord}
          />
        </SelectorItemWrapper>
      ) : null,
      [BUILD_IN_SELECTOR.SpanListType]: spanListTypeConfig.visibility ? (
        <SelectorItemWrapper
          layoutMode={layoutMode}
          title={t('filter_span_type')}
        >
          <SpanTypeSelect
            value={traceSelectorState.spanListType as SpanListType}
            optionList={spanListTypeOptionList}
            className={`${layoutMode === 'vertical' ? 'w-full' : ''}`}
            onChange={e => {
              updateTraceSelectorState('spanListType', e, 'spanListType');
              setActiveViewKey(null);
            }}
            layoutMode={layoutMode}
            disabled={disableFilterItemByFilterName('spanListType')}
          />
        </SelectorItemWrapper>
      ) : null,
      [BUILD_IN_SELECTOR.PlatformType]: platformTypeConfig.visibility ? (
        <SelectorItemWrapper
          layoutMode={layoutMode}
          title={t('filter_platform_type')}
        >
          <PlatformSelect
            disabled={disableFilterItemByFilterName('platformType')}
            value={traceSelectorState.platformType as PlatformType}
            optionList={platformTypeOptionList}
            onChange={e => {
              updateTraceSelectorState('platformType', e, 'platformType');
              setActiveViewKey(null);
            }}
            className={`${layoutMode === 'vertical' ? 'w-full' : ''}`}
          />
        </SelectorItemWrapper>
      ) : null,
      [BUILD_IN_SELECTOR.FilterSelect]: (
        <SelectorItemWrapper
          layoutMode={layoutMode}
          title={t('filter_filter_select')}
        >
          <FilterSelect
            disabled={disableFilterItemByFilterName('filterSelect')}
            viewList={viewList}
            triggerRender={triggerFilterSelectRender}
            filters={traceSelectorState.filters || { filter_fields: [] }}
            activeViewKey={activeViewKey}
            onApplyFilters={(...args) => {
              const [newFilters, spanType, platformType, metaInfo] = args;

              // Find the fields in the original traceSelectorState.filters based on latestTraceCustomKeys
              const filteredFilters =
                traceSelectorState.filters?.filter_fields?.filter(item =>
                  latestTraceCustomKeys?.includes(item.field_name),
                ) || [];

              const updatedFilters = {
                query_and_or: newFilters?.query_and_or ?? 'and',
                filter_fields: [
                  ...filteredFilters,
                  ...(newFilters?.filter_fields ?? []),
                ],
              };

              patchUpdateTraceSelectorState(
                [
                  {
                    key: 'filters',
                    value: updatedFilters,
                  },
                  {
                    key: 'spanListType',
                    value: spanType,
                  },
                  {
                    key: 'platformType',
                    value: platformType,
                  },
                  {
                    key: 'metaInfo',
                    value: metaInfo,
                  },
                ],
                'filterSelect',
              );
              setActiveViewKey(null);
              setLastUserRecord({
                filters: newFilters,
                selectedPlatform: platformType,
                selectedSpanType: spanType,
              });
            }}
            selectedPlatform={traceSelectorState.platformType as PlatformType}
            selectedSpanType={traceSelectorState.spanListType as SpanListType}
            visible={filterPopupVisible}
            onVisibleChange={setFilterPopupVisible}
            fieldMetas={fieldMetas}
            getFieldMetas={getFieldMetas}
            ignoreKeys={finalIgnoreKeys}
            disabledRowKeys={disabledRowKeys}
            mode={layoutMode === 'horizontal' ? 'popup' : 'simple'}
            customLeftRenderMap={customLeftRenderMap}
            customRightRenderMap={customRightRenderMap}
          />
        </SelectorItemWrapper>
      ),
    };

    const containerClasses =
      layoutMode === 'vertical'
        ? 'flex flex-col gap-y-3'
        : 'flex items-center gap-x-2';

    return (
      <div className={layoutMode === 'vertical' ? 'space-y-4' : ''}>
        <div className={containerClasses}>
          {items.map((item, index) => {
            if (typeof item === 'string') {
              const Components = FILTER_SELECTOR_MAP[item];

              if (!Components) {
                return null;
              }

              return <div>{Components}</div>;
            } else {
              const renderItem = item(
                traceSelectorState,
                layoutMode,
                disableFilterItemByFilterName,
              );
              if (renderItem === null) {
                return null;
              }
              const { render, filterOptions, field, fieldName } = renderItem;
              return (
                <SelectorItemWrapper layoutMode={layoutMode} title={fieldName}>
                  <div key={field || index}>
                    {render((key, value) => {
                      setTraceSelectorState((prevState: TraceSelectorState) => {
                        const newState: TraceSelectorState = {
                          ...prevState,
                        };
                        if (filterOptions) {
                          if (!newState.filters?.filter_fields) {
                            set(newState, 'filters.filter_fields', []);
                          }

                          const existingFieldIndex =
                            newState.filters?.filter_fields?.findIndex(
                              item => item.field_name === field,
                            ) ?? -1;

                          if (value === null) {
                            if (existingFieldIndex !== -1) {
                              newState.filters?.filter_fields?.splice(
                                existingFieldIndex,
                                1,
                              );
                            }
                          } else {
                            if (existingFieldIndex !== -1) {
                              set(
                                newState,
                                `filters.filter_fields[${existingFieldIndex}].values`,
                                typeof value === 'string' ? [value] : value,
                              );
                              set(
                                newState,
                                `filters.filter_fields[${existingFieldIndex}].query_type`,
                                filterOptions.query_type,
                              );
                            } else {
                              newState.filters?.filter_fields.push({
                                field_name: field,
                                logic_field_name_type: field,
                                query_type: filterOptions.query_type,
                                values:
                                  typeof value === 'string' ? [value] : value,
                                field_type:
                                  filterOptions.field_type ?? FieldType.String,
                              });
                            }
                          }
                        } else {
                          set(newState, key, value);
                        }
                        const cleanFilter = cleanChangeFilter(newState);
                        setActiveViewKey(null);
                        // 触发onChange回调
                        handleDataChange(cleanFilter, field);

                        return cleanFilter;
                      });
                    })}
                  </div>
                </SelectorItemWrapper>
              );
            }
          })}
        </div>
      </div>
    );
  },
);
