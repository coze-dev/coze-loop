/* eslint-disable complexity */
/* eslint-disable max-lines-per-function */
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
  Highlight,
} from '@coze-arch/coze-design';
import {
  IconCozArrowRight,
  IconCozCheckMarkFill,
  IconCozInfoCircle,
} from '@coze-arch/coze-design/icons';

import {
  getInputTypeText,
  type GetInputTypeTextParams,
  TypographyText,
} from '@cozeloop/evaluate-components';

import {
  type ExpandedProperty,
  type OptionGroup,
  type OptionSchema,
  schemaSourceTypeMap,
} from './types';

import styles from './group-select.module.less';
import { I18n } from '@cozeloop/i18n-adapter';

export interface GroupSelectProps
  extends Omit<InputProps, 'value' | 'onChange'> {
  /** 选项组数据 */
  optionGroups?: OptionGroup[];
  /** 当前选中的值 */
  value?: string;
  /** 选中项改变时的回调 */
  onChange?: (value?: string, item?: OptionSchema | ExpandedProperty) => void;
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

interface RightPanelProps {
  rightPanelData: ExpandedProperty[];
  value?: string;
  hoveredKey: string;
  separator: string;
  handleRightItemSelect: (item: ExpandedProperty) => void;
}

const defaultSeparator = '--';

function getGroupKey(group: OptionGroup) {
  const childrenNames = group.children?.map(e => e.name)?.join(',') ?? '';
  return group.schemaSourceType + childrenNames;
}

const RightPanel = (props: RightPanelProps) => {
  const { rightPanelData, value, separator, handleRightItemSelect } = props;
  return (
    <div className={styles['right-panel']}>
      {rightPanelData.map(item => {
        const isChecked =
          value === `${item?.schemaSourceType}${separator}${item.key}`;
        return (
          <div
            key={item.key}
            className={`${styles['option-item']} ${
              isChecked ? styles['option-item-selected'] : ''
            }`}
            onClick={() => handleRightItemSelect(item)}
          >
            <div className={styles['item-check-wrapper']}>
              <IconCozCheckMarkFill
                style={{
                  width: 16,
                  height: 16,
                  color: 'var(--coz-fg-hglt)',
                }}
                className={`check-icon ${isChecked ? '' : 'hidden'}`}
              />
            </div>
            <div
              className="w-full flex flex-row items-center pl-2 gap-1 max-w-[330px]"
              style={{ width: 'calc(100% - 16px)' }}
            >
              <TypographyText>{item.label}</TypographyText>
              {item.description ? (
                <Tooltip theme="dark" content={item.description}>
                  <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)] shrink-0" />
                </Tooltip>
              ) : null}
              <Tag
                className="ml-auto shrink-0 text-[10px] font-semibold text-[var(--coz-fg-secondary)]"
                size="mini"
                color="primary"
              >
                {getInputTypeText(item as unknown as GetInputTypeTextParams)}
              </Tag>
            </div>
          </div>
        );
      })}
    </div>
  );
};

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
    placeholder = I18n.t('please_select', { field: '' }),
    separator = defaultSeparator,
    className,
    ...restProps
  } = props;

  const [visible, setVisible] = useState(false);
  const [searchText, setSearchText] = useState('');
  const [hoveredKey, setHoveredKey] = useState<string | null>(null);

  // 解析当前选中的值
  const selectedOption = useMemo(() => {
    if (!value) {
      return undefined;
    }

    const [schemaSourceType, name] = value.split(separator);
    const selectGroup = schemaSourceType
      ? optionGroups?.find(g => g.schemaSourceType === schemaSourceType)
      : undefined;

    // 一级字段, 逻辑不变
    if (!value.includes('.')) {
      return name
        ? selectGroup?.children.find(s => s.name === name)
        : undefined;
    } else {
      // 对象中嵌套字段搜索, 新增
      const expanded: ExpandedProperty[] = [];
      selectGroup?.children.forEach(child => {
        if (child?.expandedProperties?.length) {
          child?.expandedProperties.forEach(property => {
            expanded.push(property);
          });
        }
      });
      return expanded.find(e => e.key === name);
    }
    // return selectOptionSchema;
  }, [value, optionGroups, separator]);

  // 处理选项选择
  const handleSelect = useCallback(
    (option: OptionSchema) => {
      const newValue = `${option.schemaSourceType}${separator}${option.name}`;
      onChange?.(newValue, option);
      setVisible(false);
      setSearchText(''); // 选中后清空搜索
    },
    [onChange, separator],
  );

  // 处理右侧面板选项选择
  const handleRightItemSelect = useCallback(
    (item: ExpandedProperty) => {
      const { schemaSourceType, key } = item;
      const newValue = `${schemaSourceType}${separator}${key}`;
      onChange?.(newValue, item);
      setVisible(false);
      setSearchText('');
      setHoveredKey(null);
    },
    [hoveredKey, onChange, separator],
  );

  // 获取右侧面板数据
  const getRightPanelData = useCallback(
    (optionKey: string) => {
      // 根据optionKey找到对应的选项
      for (const group of optionGroups) {
        const option = group.children.find(
          child =>
            `${child.schemaSourceType}${separator}${child.name}` === optionKey,
        );
        if (option && option.expandedProperties) {
          return option.expandedProperties;
        }
      }
      return [];
    },
    [optionGroups, separator],
  );

  // 过滤选项组
  const filteredOptionGroups = useMemo(() => {
    if (!searchText.trim()) {
      return optionGroups;
    }

    const searchLower = searchText.toLowerCase();

    const flattenOptions: OptionSchema[] = [];

    optionGroups.forEach(group => {
      group.children.forEach(option => {
        flattenOptions.push(option);
        option?.expandedProperties?.forEach(property => {
          flattenOptions.push({
            ...property,
            name: property.name || '',
            schemaSourceType: group.schemaSourceType,
          });
        });
      });
    });

    const result = optionGroups
      .map(group => ({
        ...group,
        children: flattenOptions
          .filter(option => option.schemaSourceType === group.schemaSourceType)
          .filter(
            option =>
              option.name?.toLowerCase().includes(searchLower) ||
              option.description?.toLowerCase().includes(searchLower) ||
              option.fieldType?.toLowerCase().includes(searchLower),
          ),
      }))
      .filter(group => group.children.length > 0);
    return result;
  }, [optionGroups, searchText]);

  // 渲染选项
  const renderOption = useCallback(
    (option: OptionSchema & { text_schema?: { type?: string } }) => {
      const optionKey = `${option.schemaSourceType}${separator}${option.name}`;
      const isChecked = value?.includes(optionKey) || false;
      const isShowArrowIcon =
        (option.fieldType as string)?.toLowerCase() === 'object' &&
        option?.expandedProperties?.length &&
        option?.expandedProperties?.length > 0;

      return (
        <div
          key={optionKey}
          className={`${styles['option-item']} ${
            value === optionKey ? styles['option-item-selected'] : ''
          }`}
          onClick={() => {
            handleSelect(option);
          }}
          onMouseEnter={() => {
            setHoveredKey(optionKey);
          }}
        >
          <div className={styles['item-check-wrapper']}>
            <IconCozCheckMarkFill
              style={{
                width: 16,
                height: 16,
                color: 'var(--coz-fg-hglt)',
              }}
              className={`check-icon ${isChecked ? '' : 'hidden'}`}
            />
          </div>
          <div className="w-full flex flex-row items-center pl-2 max-w-[330px]">
            {!searchText ? (
              <TypographyText>{option.name}</TypographyText>
            ) : (
              <Highlight
                highlightClassName="text-[var(--coz-fg-primary)] !text-[#5A4DED] bg-transparent"
                sourceString={option.name}
                searchWords={[searchText]}
              />
            )}
            {option.description ? (
              <Tooltip theme="dark" content={option.description}>
                <IconCozInfoCircle className="ml-1 text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)] shrink-0" />
              </Tooltip>
            ) : null}
            {option.fieldType && (
              <Tag
                className="shrink-0 ml-auto text-[10px] font-semibold text-[var(--coz-fg-secondary)]"
                size="mini"
                color="primary"
              >
                {getInputTypeText(option as unknown as GetInputTypeTextParams)}
              </Tag>
            )}
            {isShowArrowIcon ? (
              <IconCozArrowRight className="w-4 h-4 text-[var(--coz-fg-secondary)]" />
            ) : null}
          </div>
        </div>
      );
    },
    [value, separator, handleSelect, searchText],
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

  // 获取右侧面板数据
  const rightPanelData = hoveredKey ? getRightPanelData(hoveredKey) : [];

  // 弹窗内容
  const content = (
    <div className={styles['content-container']} onClick={handleContentClick}>
      <div
        className={styles['left-panel']}
        style={{ width: searchText ? '480px' : '' }}
      >
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
            {I18n.t('no_matching_option')}
          </div>
        )}
      </div>

      {/* 右侧面板 */}
      {hoveredKey && !searchText && rightPanelData.length > 0 && (
        <RightPanel
          rightPanelData={rightPanelData}
          value={value}
          hoveredKey={hoveredKey}
          separator={separator}
          handleRightItemSelect={handleRightItemSelect}
        />
      )}
    </div>
  );

  // 根据是否在搜索状态决定显示的值
  const inputDisplayValue = visible
    ? searchText
    : typeof displayValue === 'string'
      ? displayValue
      : '';

  const handleVisibleChange = (newVisible: boolean) => {
    setVisible(newVisible);
    if (!newVisible) {
      setSearchText(''); // 关闭时清空搜索
      setHoveredKey(null); // 关闭时清空hover状态
    }
  };
  const handleClickOutside = () => {
    setVisible(false);
    setSearchText(''); // 关闭时清空搜索
    setHoveredKey(null); // 关闭时清空hover状态
  };

  return (
    <Popover
      visible={visible}
      onVisibleChange={handleVisibleChange}
      onClickOutSide={handleClickOutside}
      trigger="custom"
      position="bottomLeft"
      content={content}
      showArrow={false}
      style={{ padding: 0, borderRadius: 6 }}
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
