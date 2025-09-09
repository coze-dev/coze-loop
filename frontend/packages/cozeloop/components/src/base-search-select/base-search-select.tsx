// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { isNull, isUndefined } from 'lodash-es';
import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { IconCozRefresh } from '@coze-arch/coze-design/icons';
import {
  type RenderSelectedItemFn,
  Select,
  type OptionProps,
  Tooltip,
  Button,
} from '@coze-arch/coze-design';

import { getOptionsNotInList, transformValueToArray } from './utils';
import { type BaseSelectProps } from './types';

import styles from './index.module.less';

/**
 * 基础选择器组件
 * 解决两个主要问题：
 * 1. 初始选中值不在第一页数据中导致只显示ID
 * 2. 搜索后选中值不在结果中导致只显示ID
 */
// eslint-disable-next-line @coze-arch/max-line-per-function
const BaseSearchSelect = (props: BaseSelectProps) => {
  const {
    optionList: _optionList,
    loadOptionByIds,
    value,
    defaultValue,
    onSearch,
    renderSelectedItem,
    onChangeWithObject,
    onDropdownVisibleChange,
    showRefreshBtn,
    onClickRefresh,
  } = props;

  // 是否在搜索中
  const [searchWord, setSearchWord] = useState('');
  const [refreshFlag, setRefreshFlag] = useState([]);
  const [dropdownVisible, setDropdownVisible] = useState(false);
  const optionMapRef = useRef<Record<string, OptionProps>>({});

  const optionList = useMemo(() => _optionList || [], [_optionList]);

  const isOptionListNotExists = useMemo(() => !_optionList, [_optionList]);

  useEffect(() => {
    if (isOptionListNotExists) {
      return;
    }
    // 每次 optionList 变化, 将 optionList 中的 option 添加到 optionMapRef 中缓存
    (optionList || []).forEach(item => {
      optionMapRef.current[item?.value as string] = item;
    });
  }, [optionList]);

  // 初始化
  useRequest(
    async () => {
      const initialValue = value || defaultValue;
      // 1. 判断 list 有没有value, 有则不发请求
      if (
        isUndefined(initialValue) ||
        isNull(initialValue) ||
        initialValue === '' ||
        !loadOptionByIds
      ) {
        return;
      }

      // 2. 判断 optionList 中有没有 value 中不存在的 选项, 有则不发请求
      const optionsNotInList = getOptionsNotInList({
        value: initialValue,
        optionList,
        onChangeWithObject,
      });

      if (optionsNotInList.length === 0) {
        return;
      }

      try {
        const payload = transformValueToArray(value, onChangeWithObject);
        const fetchOptions = await loadOptionByIds(payload);
        if (fetchOptions) {
          setRefreshFlag([]);
          optionMapRef.current = {
            ...optionMapRef.current,
            ...fetchOptions.reduce((acc, item) => {
              acc[item?.value as string] = item;
              return acc;
            }, {}),
          };
        }
      } catch (error) {
        console.error('Failed to load selected option:', error);
      }
    },
    {
      refreshDeps: [value],
    },
  );

  /**
   * 最后展示的选项
   */
  const cacheOptions = useMemo(() => {
    // searchWord 表示处于搜索中, 不应该展示不在选项列表中的选项
    if (!value || searchWord) {
      return optionList;
    }

    const optionsNotInList = getOptionsNotInList({
      value,
      optionList,
      onChangeWithObject,
    });

    // value 所有选项都在选项列表中, 直接返回
    if (optionsNotInList.length === 0) {
      return optionList;
    }

    // value 不在选项列表, 则对value进行处理, 前面已经处理为arr了
    // 所以这里可以直接按arr 处理 返回缓存中的选项
    const optionsInCache = optionsNotInList.map(k => optionMapRef.current[k]);

    return [...optionsInCache, ...optionList];
  }, [optionList, value, searchWord, refreshFlag]);

  /**
   * 选中项渲染, 如果list中没有, 从缓存中拿当前的选种值
   */
  const RenderSelectedItem = useCallback(
    (optionNode: Record<string, unknown>, multipleProps?: any) => {
      const renderOpt =
        optionMapRef.current[optionNode?.value as string] || optionNode;

      if (renderSelectedItem) {
        return renderSelectedItem(renderOpt, multipleProps);
      }
      // 多选
      if (multipleProps) {
        return {
          isRenderInTag: true,
          content: renderOpt?.label || renderOpt?.value,
        };
      }
      return renderOpt?.label || renderOpt?.value;
    },
    [renderSelectedItem],
  );

  return (
    <Select
      suffix={
        showRefreshBtn && dropdownVisible ? (
          <Tooltip theme="dark" content="刷新">
            <div className="flex flex-row items-center">
              <Button
                className="!h-6 !w-6"
                icon={<IconCozRefresh />}
                size="small"
                color="secondary"
                onClick={() => onClickRefresh?.()}
              />
              <div className="h-3 w-0 border-0 border-l border-solid coz-stroke-primary ml-[2px]" />
            </div>
          </Tooltip>
        ) : null
      }
      {...props}
      onDropdownVisibleChange={visible => {
        setDropdownVisible(visible);
        onDropdownVisibleChange?.(visible);
      }}
      dropdownClassName={classNames(
        styles['select-dropdown-style'],
        props.dropdownClassName,
      )}
      optionList={isOptionListNotExists ? undefined : cacheOptions}
      renderSelectedItem={RenderSelectedItem as RenderSelectedItemFn}
      onSearch={(vs, e) => {
        setSearchWord(vs);
        if (onSearch) {
          onSearch?.(vs, e);
        }
      }}
    />
  );
};

export default BaseSearchSelect;
