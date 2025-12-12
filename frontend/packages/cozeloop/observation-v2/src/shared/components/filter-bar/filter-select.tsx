import { type ReactNode, useMemo } from 'react';

import { keys } from 'lodash-es';
import { type FieldMeta } from '@cozeloop/api-schema/observation';

import { FilterSelectUI } from '@/shared/ui/filter-select-ui';
import {
  type CustomRightRenderMap,
  type LogicValue,
} from '@/shared/components/analytics-logic-expr/logic-expr';
import {
  API_FEEDBACK,
  MANUAL_FEEDBACK,
  METADATA,
} from '@/shared/components/analytics-logic-expr/const';
import { validateViewName } from '@/features/trace-list/utils/name-validate';
import { AUTO_EVAL_FEEDBACK } from '@/features/trace-list/constants';

import { getExprTypeByFileName } from '../analytics-logic-expr/utils';
import type { View } from './custom-view';

interface FilterSelectProps {
  viewList: View[];
  activeViewKey: string | null | number;

  onApplyFilters: (
    newFilters: LogicValue,
    spanListType: string,
    platformType: string,
    metaInfo?: Record<string, FieldMeta>,
  ) => void;
  customRightRenderMap?: CustomRightRenderMap;
  customLeftRenderMap?: CustomRightRenderMap;
  filters: LogicValue;
  selectedSpanType: string;
  selectedPlatform: string;
  visible: boolean;
  onVisibleChange: (visible: boolean) => void;
  fieldMetas?: Record<string, FieldMeta>;
  getFieldMetas: (params: {
    platform_type: string;
    span_list_type: string;
  }) => Promise<Record<string, FieldMeta>>;
  ignoreKeys?: string[];
  disabledRowKeys?: string[];
  mode?: 'simple' | 'popup';
  disabled?: boolean;
  triggerRender?: ReactNode;
}

export const FilterSelect = (props: FilterSelectProps) => {
  const {
    viewList,
    activeViewKey,
    onApplyFilters,
    filters,
    selectedSpanType,
    selectedPlatform,
    visible,
    onVisibleChange,
    fieldMetas,
    getFieldMetas,
    ignoreKeys,
    disabledRowKeys,
    mode,
    customLeftRenderMap,
    customRightRenderMap,
    disabled,
    triggerRender,
  } = props;

  const invalidateExprs = useMemo(() => {
    const currentInvalidateExpr = filters?.filter_fields
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
      .map(filedFilter => filedFilter.field_name)
      .filter(field => !ignoreKeys?.includes(field));
    return new Set(currentInvalidateExpr);
  }, [filters?.filter_fields, fieldMetas, ignoreKeys]);

  const currentSelectedView = useMemo(() => {
    const view = viewList.find(v => v.id === activeViewKey);
    return view;
  }, [viewList, activeViewKey]);

  return (
    <FilterSelectUI
      disabled={disabled}
      filters={(filters || {}) as unknown as LogicValue}
      fieldMetas={fieldMetas}
      getFieldMetas={getFieldMetas}
      spanListType={selectedSpanType}
      platformType={selectedPlatform}
      onApplyFilters={onApplyFilters}
      visible={visible}
      selectedView={currentSelectedView}
      onVisibleChange={onVisibleChange}
      onViewNameValidate={name =>
        validateViewName(
          name,
          viewList.map(v => v.view_name),
        )
      }
      invalidateExpr={invalidateExprs}
      ignoreKeys={ignoreKeys}
      disabledRowKeys={disabledRowKeys}
      mode={mode}
      customLeftRenderMap={customLeftRenderMap}
      customRightRenderMap={customRightRenderMap}
      triggerRender={triggerRender}
    />
  );
};
