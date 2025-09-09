// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable import/order */
/* eslint-disable @coze-arch/tsx-no-leaked-render */
/* eslint-disable @coze-arch/max-line-per-function */
import React, { useState, useCallback, useMemo } from 'react';
import {
  Input,
  type InputProps,
  Popover,
  Tag,
  Tooltip,
} from '@coze-arch/coze-design';
import {
  IconCozCheckMark,
  IconCozInfoCircle,
} from '@coze-arch/coze-design/icons';

import { TypographyText } from '../../text-ellipsis';
import {
  type OptionGroup,
  type OptionSchema,
  schemaSourceTypeMap,
} from '../types';

import styles from './index.module.less';

import { I18n } from '@cozeloop/i18n-adapter';
export interface GroupSelectProps
  extends Omit<InputProps, 'value' | 'onChange'> {
  /** 选项组数据 */
  optionGroups?: OptionGroup[];
  /** 当前选中的值 */
  value?: string;
  /** 选中项改变时的回调 */
  onChange?: (value?: string) => void;
  /** 前缀内容 */
  prefix?: React.ReactNode;
  /** 后缀内容 */
  suffix?: React.ReactNode;
  /** 渲染选中项的函数 */
  renderSelectedItem?: (optionNode: {
    value?: string;
    [key: string]: unknown;
  }) => React.ReactNode;
  /** 验证状态 */
  validateStatus?: 'error' | 'warning';
  /** 是否禁用 */
  disabled?: boolean;
  /** 占位符 */
  placeholder?: string;
  /** 分隔符，用于生成 value */
  separator?: string;
}

const defaultSeparator = '--';

function getGroupKey(group: OptionGroup) {
  const childrenNames = group.children?.map(e => e.name)?.join(',') ?? '';
  return group.schemaSourceType + childrenNames;
}

export default function GroupSelect(props: GroupSelectProps) {
  const {
    value,
    onChange,
    optionGroups = [],
    prefix,
    suffix,
    renderSelectedItem,
    validateStatus,
    disabled = false,
    placeholder = I18n.t('please_select'),
    separator = defaultSeparator,
    className,
    ...restProps
  } = props;

  const [visible, setVisible] = useState(false);
  const [searchText, setSearchText] = useState('');

  // 解析当前选中的值
  const selectedOption = useMemo(() => {
    if (!value) {
      return undefined;
    }

    const [schemaSourceType, name] = value.split(separator);
    const selectGroup = schemaSourceType
      ? optionGroups?.find(g => g.schemaSourceType === schemaSourceType)
      : undefined;
    const selectOptionSchema = name
      ? selectGroup?.children.find(s => s.name === name)
      : undefined;

    return selectOptionSchema;
  }, [value, optionGroups, separator]);

  // 处理选项选择
  const handleSelect = useCallback(
    (option: OptionSchema) => {
      const newValue = `${option.schemaSourceType}${separator}${option.name}`;
      onChange?.(newValue);
      setVisible(false);
      setSearchText(''); // 选中后清空搜索
    },
    [onChange, separator],
  );

  // 过滤选项组
  const filteredOptionGroups = useMemo(() => {
    if (!searchText.trim()) {
      return optionGroups;
    }

    const searchLower = searchText.toLowerCase();

    return optionGroups
      .map(group => ({
        ...group,
        children: group.children.filter(
          option =>
            option.name?.toLowerCase().includes(searchLower) ||
            option.description?.toLowerCase().includes(searchLower) ||
            option.content_type?.toLowerCase().includes(searchLower),
        ),
      }))
      .filter(group => group.children.length > 0);
  }, [optionGroups, searchText]);

  // 渲染选项
  const renderOption = useCallback(
    (option: OptionSchema) => (
      <div
        key={`${option.schemaSourceType}${separator}${option.name}`}
        className={`option-item ${value === `${option.schemaSourceType}${separator}${option.name}` ? 'selected' : ''}`}
        onClick={() => handleSelect(option)}
      >
        {value === `${option.schemaSourceType}${separator}${option.name}` && (
          <IconCozCheckMark className="check-icon" />
        )}
        <div className="w-full flex flex-row items-center pl-2 gap-1 max-w-[330px]">
          <TypographyText>{option.name}</TypographyText>
          {option.description ? (
            <Tooltip theme="dark" content={option.description}>
              <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)] shrink-0" />
            </Tooltip>
          ) : null}
          {option.content_type && (
            <Tag className="mx-3 ml-auto shrink-0" size="mini" color="primary">
              {option.content_type}
            </Tag>
          )}
        </div>
      </div>
    ),
    [value, separator, handleSelect],
  );

  // 防止点击内容区域关闭弹窗
  const handleContentClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
  }, []);

  // 渲染显示值
  const displayValue = useMemo(() => {
    if (renderSelectedItem && selectedOption) {
      return renderSelectedItem({ value, ...selectedOption });
    }
    return selectedOption?.name || '';
  }, [renderSelectedItem, selectedOption, value]);

  // 处理输入框变化 - 用于搜索
  const handleInputChange = useCallback(
    (inputValue: string) => {
      setSearchText(inputValue);
      // 如果有输入内容且弹窗未显示，则显示弹窗
      if (inputValue && !visible) {
        setVisible(true);
      }
    },
    [visible],
  );

  // 处理输入框点击
  const handleInputClick = useCallback(() => {
    if (!disabled) {
      setVisible(true);
    }
  }, [disabled]);

  // 弹窗内容
  const content = (
    <div className={styles['content-container']} onClick={handleContentClick}>
      <div className={styles['left-panel']}>
        {filteredOptionGroups.map(group => (
          <div key={getGroupKey(group)} className={styles['group-section']}>
            <div className={styles['group-title']}>
              {schemaSourceTypeMap[group.schemaSourceType]}
            </div>
            <div className={styles['group-items']}>
              {group.children.map(option => renderOption(option))}
            </div>
          </div>
        ))}
        {filteredOptionGroups.length === 0 && searchText && (
          <div className="text-center py-4 text-gray-500">
            {I18n.t('no_matching_options_found')}
          </div>
        )}
      </div>
    </div>
  );

  // 根据是否在搜索状态决定显示的值
  const inputDisplayValue = visible
    ? searchText
    : typeof displayValue === 'string'
      ? displayValue
      : '';

  return (
    <Popover
      visible={visible}
      onVisibleChange={newVisible => {
        setVisible(newVisible);
        if (!newVisible) {
          setSearchText(''); // 关闭时清空搜索
        }
      }}
      onClickOutSide={() => {
        setVisible(false);
        setSearchText(''); // 关闭时清空搜索
      }}
      trigger="custom"
      position="bottomLeft"
      content={content}
      showArrow={false}
      style={{ padding: 0 }}
    >
      <div
        className={
          className
            ? `${styles['select-container']} ${className}`
            : styles['select-container']
        }
      >
        <Input
          value={inputDisplayValue}
          placeholder={placeholder}
          readOnly={false} // 允许输入进行搜索
          disabled={disabled}
          prefix={prefix}
          suffix={suffix}
          onChange={handleInputChange}
          onClick={handleInputClick}
          onFocus={() => setVisible(true)}
          showClear={visible && searchText.length > 0} // 只在搜索时显示清除按钮
          onClear={() => setSearchText('')}
          {...restProps}
        />
      </div>
    </Popover>
  );
}
