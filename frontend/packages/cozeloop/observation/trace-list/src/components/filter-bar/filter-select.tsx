// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-params */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */

import { useMemo } from 'react';

import { keys } from 'lodash-es';
import { I18n } from '@cozeloop/i18n-adapter';
import { useSpace, useCurrentEnterpriseId } from '@cozeloop/biz-hooks-adapter';
import {
  type PlatformType,
  type SpanListType,
} from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';
import { Toast } from '@coze-arch/coze-design';

import { useTraceStore } from '@/stores/trace';
import { type FilterInvalidateEnum } from '@/consts/trace-attrs';
import { AUTO_EVAL_FEEDBACK } from '@/consts';
import { FilterSelectUI } from '@/components/filter-select-ui';

import {
  type CustomRightRenderMap,
  type LogicValue,
} from '../logic-expr/logic-expr';
import { MANUAL_FEEDBACK } from '../logic-expr/const';
import { validateViewName } from '../../utils/name-validate';
import type { View } from './custom-view';

interface FilterSelectProps {
  viewList: View[];
  activeViewKey: string | null | number;
  onApplyFilters: (
    newFilters: any,
    viewMethod: string | number,
    dataSource: string | number,
  ) => void;
  onSaveToCustomView: (viewId: string) => void;
  onSaveToCurrentView: (viewId: string) => void;
  platformEnumOptionList: { label: string; value: string | number }[];
  spanListTypeEnumOptionList: { label: string; value: string | number }[];
  customRightRenderMap?: CustomRightRenderMap;
  customLeftRenderMap?: CustomRightRenderMap;
}

export const FilterSelect = (props: FilterSelectProps) => {
  const {
    viewList,
    activeViewKey,
    onApplyFilters,
    onSaveToCustomView,
    onSaveToCurrentView,
    platformEnumOptionList,
    customRightRenderMap,
    customLeftRenderMap,
    spanListTypeEnumOptionList,
  } = props;
  const {
    filters,
    fieldMetas,
    setFieldMetas,
    setFilters,
    setSelectedPlatform,
    setSelectedSpanType,
    setApplyFilters,
    selectedSpanType,
    selectedPlatform,
    setFilterPopupVisible,
    filterPopupVisible,
    setLastUserRecord,
  } = useTraceStore();

  const { spaceID } = useSpace();
  const enterpriseId = useCurrentEnterpriseId();

  const handleApplyFilters = (
    newFilters: LogicValue,
    viewMethod: string | number,
    dataSource: string | number,
    metaInfo?: Record<string, any>,
  ) => {
    setFieldMetas(metaInfo);
    setFilters(newFilters);
    setApplyFilters(newFilters);
    setSelectedPlatform(dataSource);
    setSelectedSpanType(viewMethod);
    onApplyFilters(newFilters, viewMethod, dataSource);
    setLastUserRecord({
      filters: newFilters,
      selectedPlatform: dataSource,
      selectedSpanType: viewMethod,
    });
  };

  const handleSaveToCurrentView = async (params: {
    filters: LogicValue;
    viewMethod: string | number;
    dataSource: string | number;
  }) => {
    try {
      const view = viewList.find(v => v.id === activeViewKey);
      if (!view) {
        return;
      }
      await observabilityTrace.UpdateView({
        view_id: view.id,
        view_name: view.view_name,
        filters: JSON.stringify(params.filters),
        platform_type: String(params.dataSource) as PlatformType,
        span_list_type: String(params.viewMethod) as SpanListType,
        workspace_id: spaceID,
      });

      if (viewList.length > 5) {
        Toast.warning(I18n.t('new_view_cannot_be_displayed'));
      }
      setFilterPopupVisible(false);
      setApplyFilters(params.filters);
      setFilters(params.filters);
      onSaveToCurrentView(view.id.toString());
    } catch (e) {
      console.log(e);
    }
  };

  const handleSaveToCustomView = async (params: {
    filters: LogicValue;
    viewMethod: string | number;
    dataSource: string | number;
    name: string;
  }) => {
    try {
      const { id } = await observabilityTrace.CreateView({
        enterprise_id: enterpriseId,
        workspace_id: spaceID,
        view_name: params.name,
        span_list_type: String(params.viewMethod) as SpanListType,
        platform_type: String(params.dataSource) as PlatformType,
        filters: JSON.stringify(params.filters),
      });
      setFilterPopupVisible(false);
      setApplyFilters(params.filters);
      setFilters(params.filters);
      onSaveToCustomView(id.toString());
    } catch (e) {
      console.log(e);
    }
  };

  const invalidateExprs = useMemo(() => {
    const currentInvalidateExpr = filters?.filter_fields
      ?.filter(
        filedFilter =>
          !(keys(fieldMetas) ?? []).includes(
            filedFilter.field_name as FilterInvalidateEnum,
          ) &&
          filedFilter.logic_field_name_type !== AUTO_EVAL_FEEDBACK &&
          filedFilter.logic_field_name_type !== MANUAL_FEEDBACK,
      )
      .map(filedFilter => filedFilter.field_name);
    return new Set(currentInvalidateExpr);
  }, [filters?.filter_fields, fieldMetas]);

  const currentSelectedView = useMemo(() => {
    const view = viewList.find(v => v.id === activeViewKey);
    return view;
  }, [viewList, activeViewKey]);

  return (
    <FilterSelectUI
      spanTabOptionList={spanListTypeEnumOptionList}
      customRightRenderMap={customRightRenderMap}
      customLeftRenderMap={customLeftRenderMap}
      platformEnumOptionList={platformEnumOptionList}
      filters={(filters || {}) as unknown as LogicValue}
      fieldMetas={fieldMetas}
      viewMethod={selectedSpanType}
      dataSource={selectedPlatform}
      onApplyFilters={handleApplyFilters}
      visible={filterPopupVisible}
      selectedView={currentSelectedView}
      onVisibleChange={setFilterPopupVisible}
      onViewNameValidate={name =>
        validateViewName(
          name,
          viewList.map(v => v.view_name),
        )
      }
      onSaveToCurrentView={handleSaveToCurrentView}
      onSaveToCustomView={handleSaveToCustomView}
      invalidateExpr={invalidateExprs}
      allowSaveToCurrentView={
        !!currentSelectedView && !currentSelectedView.is_system
      }
    />
  );
};
