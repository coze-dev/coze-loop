import { useState } from 'react';

import {
  type FilterFields,
  type FilterField,
  QueryRelation,
} from '@cozeloop/api-schema/observation';

interface UseTraceSearchLogicParams {
  setSearchFilters?: (filters: FilterFields) => void;
  onClear?: () => void;
  selectedSpanId: string;
  searchFilters?: FilterFields;
}

export const useTraceSearchLogic = ({
  setSearchFilters,
  onClear,
  selectedSpanId,
  searchFilters,
}: UseTraceSearchLogicParams) => {
  const [inputFilterField, setInputFilterField] = useState<
    FilterField | undefined
  >();
  const [selectorFilterFields, setSelectorFilterFields] =
    useState<FilterFields>();

  // 合并inputFilterField和selectorFilterFields
  const mergeFilters = (
    inputFilter?: FilterField,
    selectorFilters?: FilterFields,
  ): FilterFields => {
    const isInputValid = inputFilter && inputFilter.values?.[0]?.trim() !== '';
    const isSelectorValid =
      selectorFilters && selectorFilters.filter_fields?.length > 0;
    if (isInputValid && !isSelectorValid) {
      return {
        query_and_or: QueryRelation.And,
        filter_fields: [inputFilter],
      };
    }
    if (!isInputValid && selectorFilters) {
      return selectorFilters;
    }
    if (isInputValid && isSelectorValid) {
      return {
        query_and_or: QueryRelation.And,
        filter_fields: [
          inputFilter,
          {
            sub_filter: selectorFilters,
          },
        ],
      };
    }
    return {
      query_and_or: QueryRelation.And,
      filter_fields: [],
    };
  };

  // 受控：提交搜索
  const handleSearch = (newFilterField: FilterField) => {
    setInputFilterField(newFilterField);
    const mergedFilters = mergeFilters(newFilterField, selectorFilterFields);
    setSearchFilters?.(mergedFilters);
  };

  // 处理selector过滤条件变化
  const handleSelectorChange = (newSelectorFilters: FilterFields) => {
    setSelectorFilterFields(newSelectorFilters);
    const mergedFilters = mergeFilters(inputFilterField, newSelectorFilters);
    setSearchFilters?.(mergedFilters);
  };

  return {
    inputFilterField,
    setInputFilterField,
    handleSearch,
    handleSelectorChange,
    selectorFilterFields,
  };
};
