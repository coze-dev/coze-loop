// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  EvaluatorTagKey,
  EvaluatorTagType,
} from '@cozeloop/api-schema/evaluation';
import {
  RadioGroup,
  Spin,
  Typography,
  type TagProps,
} from '@coze-arch/coze-design';

import { type EvaluatorTypeTagText, type TemplateFilter } from './types';
import { listEvaluatorTags } from './api';

import styles from './evaluator-template-filter.module.less';

// 选项数据结构
interface FilterOption {
  value: string;
  label: string;
}

function toOption(name: string | number) {
  return {
    value: name as string,
    label: name as string,
  };
}

function FilterTag({ children, ...rest }: TagProps) {
  const { color = 'primary', className } = rest;
  const whiteClassName = 'bg-white coz-fg-primary ';
  const brandClassname =
    'bg-[rgba(var(--coze-brand-1),var(--coze-brand-1-alpha))] coz-fg-hglt';
  const colorClassName = color === 'brand' ? brandClassname : whiteClassName;
  return (
    <div
      {...rest}
      className={classNames(
        'content-center rounded-[8px] px-4 text-[14px] font-medium h-9 border border-solid border-[var(--coz-stroke-primary)] cursor-pointer',
        colorClassName,
        className,
      )}
    >
      {children}
    </div>
  );
}

const FilterGroup = ({
  title,
  options,
  selectedValues,
  onSelect,
  isSingleSelect = false,
  className,
  titleClassName,
}: {
  title: string;
  options: FilterOption[] | undefined;
  selectedValues: string | string[];
  onSelect: (value: string) => void;
  isSingleSelect?: boolean;
  className?: string;
  titleClassName?: string;
}) => (
  <div className={classNames('py-2', className)}>
    <div
      className={classNames(
        'text-sm font-medium coz-fg-secondary mb-[10px]',
        titleClassName,
      )}
    >
      {title}
    </div>
    <div className="flex flex-wrap gap-[10px]">
      {options?.map(option => {
        const isSelected = isSingleSelect
          ? selectedValues === option.value
          : Array.isArray(selectedValues) &&
            selectedValues.includes(option.value);
        return (
          <FilterTag
            key={option.value}
            color={isSelected ? 'brand' : 'primary'}
            onClick={() => onSelect(option.value)}
          >
            {option.label}
          </FilterTag>
        );
      })}
    </div>
  </div>
);

function EvaluatorTypeSelect({
  filterEvaluatorTypes,
  disabledEvaluatorTypes,
  isTypeSingleSelect,
  className,
  value,
  onSelect,
}: {
  filterEvaluatorTypes: string[];
  disabledEvaluatorTypes?: EvaluatorTypeTagText[];
  value?: string[];
  isTypeSingleSelect?: boolean;
  className?: string;
  onSelect?: (value: string) => void;
}) {
  const options = filterEvaluatorTypes.map(tagName => ({
    value: tagName,
    label: tagName,
    disabled: disabledEvaluatorTypes?.includes(tagName as EvaluatorTypeTagText),
  }));
  return (
    <div className={className}>
      <div
        className={
          'text-sm font-medium coz-fg-secondary mb-[10px] !coz-fg-primary'
        }
      >
        {I18n.t('type')}
      </div>
      <div className="flex flex-wrap gap-[10px]">
        {!isTypeSingleSelect
          ? options.map(option => {
              const isSelected =
                Array.isArray(value) && value.includes(option.value);
              return (
                <FilterTag
                  key={option.value}
                  color={isSelected ? 'brand' : 'primary'}
                  onClick={() => onSelect?.(option.value)}
                >
                  {option.label}
                </FilterTag>
              );
            })
          : null}
        {isTypeSingleSelect ? (
          <RadioGroup
            options={options}
            type="button"
            className={styles['w-full-radio-group']}
            value={value?.[0]}
            onChange={e => onSelect?.(e.target.value)}
          />
        ) : null}
      </div>
    </div>
  );
}

export interface EvaluatorTemplateFilterProps {
  filter?: TemplateFilter;
  showClearAll?: boolean;
  showScenariosClear?: boolean;
  isTypeSingleSelect?: boolean;
  disabledEvaluatorTypes?: EvaluatorTypeTagText[];
  onFilterChange?: (filter: TemplateFilter | undefined) => void;
  className?: string;
  tagType?: EvaluatorTagType;
}

// eslint-disable-next-line complexity
export function EvaluatorTemplateFilter({
  filter,
  disabledEvaluatorTypes,
  showClearAll = true,
  showScenariosClear = false,
  isTypeSingleSelect = false,
  onFilterChange,
  className,
  // 默认值与服务端保持一致, 预置评估器
  tagType = EvaluatorTagType.Evaluator,
}: EvaluatorTemplateFilterProps) {
  // const { spaceID } = useSpace();
  // 处理类型选择（单选）
  const handleTypeSingleSelect = (value: string) => {
    onFilterChange?.({
      ...filter,
      [EvaluatorTagKey.Category]: [value],
    });
  };

  // 处理多选选择
  const handleMultiSelect = (
    field: keyof TemplateFilter,
    value: string | number,
  ) => {
    const selectedValues = (filter?.[field] || []) as string[];
    const isSelected = selectedValues.includes(value as string);
    const newSelectedValues = isSelected
      ? selectedValues.filter(v => v !== value)
      : [...selectedValues, value];
    onFilterChange?.(
      filter
        ? { ...filter, [field]: newSelectedValues }
        : { [field]: newSelectedValues },
    );
  };

  const service = useRequest(async () => {
    const res = await listEvaluatorTags({
      tag_type: tagType,
    });
    return res?.tags;
  });

  const filterTagsData = service.data || {};

  // 清空所有选择
  const handleClearAll = () => {
    onFilterChange?.(undefined);
  };

  const handleScenarioClear = () => {
    onFilterChange?.({
      ...filter,
      [EvaluatorTagKey.BusinessScenario]: [],
      [EvaluatorTagKey.Objective]: [],
      [EvaluatorTagKey.TargetType]: [],
    });
  };

  return (
    <div
      className={classNames('p-4 rounded-lg border border-gray-200', className)}
    >
      {/* 标题和清空按钮 */}
      {showClearAll ? (
        <div className="flex justify-between items-center mb-4">
          <div className="text-[16px] coz-fg-primary font-medium">
            {I18n.t('evaluator_filter')}
          </div>
          <Typography.Text
            link={true}
            className="!text-sm !font-regular"
            onClick={handleClearAll}
          >
            {I18n.t('clear')}
          </Typography.Text>
        </div>
      ) : null}

      <EvaluatorTypeSelect
        className="pb-2 pt-0 mb-1"
        filterEvaluatorTypes={filterTagsData[EvaluatorTagKey.Category] ?? []}
        disabledEvaluatorTypes={disabledEvaluatorTypes}
        value={filter?.[EvaluatorTagKey.Category]}
        isTypeSingleSelect={isTypeSingleSelect}
        onSelect={val => {
          if (isTypeSingleSelect) {
            handleTypeSingleSelect(val);
          } else {
            handleMultiSelect(EvaluatorTagKey.Category, val);
          }
        }}
      />

      <div className="py-2 flex items-center">
        <div className="text-sm font-medium coz-fg-primary flex-1">
          {I18n.t('scene')}
        </div>
        {showScenariosClear ? (
          <Typography.Text
            link={true}
            className="!text-sm"
            onClick={handleScenarioClear}
          >
            {I18n.t('clear')}
          </Typography.Text>
        ) : null}
      </div>
      <Spin spinning={service.loading}>
        <FilterGroup
          title={I18n.t('evaluator_evaluation_object')}
          options={filterTagsData[EvaluatorTagKey.TargetType]?.map(toOption)}
          selectedValues={filter?.[EvaluatorTagKey.TargetType] || []}
          onSelect={value =>
            handleMultiSelect(EvaluatorTagKey.TargetType, value)
          }
        />
        <FilterGroup
          title={I18n.t('evaluator_objectives')}
          options={filterTagsData[EvaluatorTagKey.Objective]?.map(toOption)}
          selectedValues={filter?.[EvaluatorTagKey.Objective] || []}
          onSelect={value =>
            handleMultiSelect(EvaluatorTagKey.Objective, value)
          }
        />
        <FilterGroup
          title={I18n.t('business_scenario')}
          options={filterTagsData[EvaluatorTagKey.BusinessScenario]?.map(
            toOption,
          )}
          selectedValues={filter?.[EvaluatorTagKey.BusinessScenario] || []}
          onSelect={value =>
            handleMultiSelect(EvaluatorTagKey.BusinessScenario, value)
          }
        />
      </Spin>
    </div>
  );
}
