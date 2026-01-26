// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { groupBy } from 'lodash-es';
import { useDeepCompareEffect } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Model } from '@cozeloop/api-schema/llm-manage';
import {
  Avatar,
  Select,
  Typography,
  type SelectProps,
} from '@coze-arch/coze-design';

import { type ModelItemProps, ModelOption } from './model-option';

export interface ModelSelectOption {
  label: React.ReactNode;
  value: string | number;
  model: Model;
}

function getOption(model: Model) {
  const option: ModelSelectOption = {
    label: model.name ?? '',
    value: model.model_id ?? '',
    model,
  };
  return option;
}

function ModelSelectItem({ item }: { item: ModelItemProps }) {
  return (
    <div className="flex items-center w-full gap-1">
      {item?.series?.icon ? (
        <div className={'overflow-hidden flex-shrink-0 mr-[8px]'}>
          <Avatar
            src={item?.series?.icon}
            shape="square"
            size="extra-extra-small"
          />
        </div>
      ) : null}
      <Typography.Text
        style={{
          maxWidth: '400px',
          fontSize: '13px',
        }}
        ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
      >
        {item?.name}
      </Typography.Text>
    </div>
  );
}

export function ModelSelectWithObject(
  props: Omit<SelectProps, 'value' | 'onChange'> & {
    optionClassName?: string;
    value?: ModelItemProps;
    onChange?: (model: ModelItemProps | undefined) => void;
    modelList?: ModelItemProps[];
    defaultSelectFirstModel?: boolean;
  },
) {
  const {
    value,
    onChange,
    modelList = [],
    optionClassName,
    defaultSelectFirstModel = false,
  } = props;

  const { modelGroups, modelOptions, hasSeries } = useMemo(() => {
    const hasSeriesFlag = modelList.some(model => model?.series?.name);

    if (!hasSeriesFlag) {
      return {
        modelOptions: modelList,
        hasSeries: hasSeriesFlag,
        modelGroups: [],
      };
    }

    const modelSeriesGroups = groupBy(modelList, model => model?.series?.name);

    const groupedModels = Object.values(modelSeriesGroups).filter(
      (group): group is ModelItemProps[] => !!group?.length,
    );

    return {
      modelGroups: groupedModels,
      hasSeries: hasSeriesFlag,
      modelOptions: modelList,
    };
  }, [modelList]);

  const val = useMemo(() => (value ? getOption(value) : undefined), [value]);

  useDeepCompareEffect(() => {
    if (!value && defaultSelectFirstModel && modelList?.length) {
      onChange?.(modelList?.[0]);
    }
  }, [modelList, defaultSelectFirstModel, value]);

  return (
    <Select
      key={hasSeries ? 'series' : 'normal'}
      placeholder={I18n.t('choose_model')}
      {...props}
      // 使value为option对象，不能去掉
      onChangeWithObject={true}
      value={val}
      onChange={newVal => {
        const option = newVal as ModelSelectOption;
        onChange?.(option.model);
      }}
      renderSelectedItem={item => (
        <ModelSelectItem item={(item as ModelSelectOption).model || value} />
      )}
      showTick={!hasSeries}
      filter={(input, option) => {
        if (input && option?.model) {
          const item = option.model as Model;
          return Boolean(
            item?.name?.toLowerCase()?.includes(input.toLowerCase()),
          );
        }
        return true;
      }}
    >
      {hasSeries
        ? modelGroups.map(group => (
            <Select.OptGroup
              key={group[0]?.series?.name}
              label={`${group[0]?.series?.name} | ${
                group[0]?.series?.vendor
                  ? `${I18n.t('prompt_provided_by_placeholder1', { placeholder1: group[0]?.series?.vendor })}`
                  : ''
              }`}
            >
              {group.map(item => (
                <Select.Option
                  key={item?.model_id}
                  value={item?.model_id}
                  model={item}
                  disabled={item.disabled}
                >
                  <ModelOption
                    model={item}
                    selected={value?.model_id === item.model_id}
                    disabled={item.disabled}
                    className={optionClassName}
                  />
                </Select.Option>
              ))}
            </Select.OptGroup>
          ))
        : modelOptions.map(model => (
            <Select.Option
              className={optionClassName}
              key={model.model_id}
              value={model.model_id}
              model={model}
            >
              {model.name}
            </Select.Option>
          ))}
    </Select>
  );
}
