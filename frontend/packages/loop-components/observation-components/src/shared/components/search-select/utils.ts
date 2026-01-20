// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
export const transformValueToArray = (
  value: any,
  onChangeWithObject = false,
) => {
  // 对象传入
  if (onChangeWithObject) {
    // 对象数组
    if (Array.isArray(value)) {
      return value.map(item => item?.value || '').filter(Boolean);
    }
    // 单个对象
    return value?.value ? [value?.value] : [];
  }
  // 普通数组 或 字符串
  return Array.isArray(value) ? value : [value];
};

// 过滤 value 中不在 optionlist 中的 option
export const filterValueNotInOptionList = (
  value: any,
  optionSet: Set<string | number | undefined>,
  onChangeWithObject = false,
): Int64[] => {
  if (onChangeWithObject) {
    // 对象数组
    if (Array.isArray(value)) {
      return value
        .filter(item => !optionSet.has(item?.value))
        .map(it => it?.value);
    }
    // 单个对象
    return optionSet.has(value?.value) ? [] : [value?.value];
  }

  if (Array.isArray(value)) {
    return value.filter(item => !optionSet.has(item));
  }

  if (typeof value === 'string' || typeof value === 'number') {
    return optionSet.has(value) ? [] : [value];
  }

  return [];
};

export const getOptionsNotInList = ({
  value,
  optionList,
  onChangeWithObject = false,
}: {
  value: any;
  optionList: any[];
  onChangeWithObject?: boolean;
}) => {
  // 集合
  const existingValues = new Set(optionList.map(item => item.value));

  // 找到 value 里面不在选项列表中的选项
  const optionsNotInList = filterValueNotInOptionList(
    value,
    existingValues,
    onChangeWithObject,
  );
  return optionsNotInList;
};

export const initialValueChecker = (
  value: any,
  optionList: any[],
  onChangeWithObject = false,
) => {
  if (typeof value === 'string') {
    return true;
  }

  return value;
};
