// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type CSSProperties, useEffect, useRef, useState } from 'react';

import classNames from 'classnames';
import { useLatest } from 'ahooks';
import { type Scenario, type Model } from '@cozeloop/api-schema/llm-manage';
import {
  Form,
  type FormApi,
  Popover,
  type PopoverProps,
  type SelectProps,
  Space,
  Typography,
} from '@coze-arch/coze-design';

import { ModelSelectWithObject } from '@/model-select';

import { convertModelToModelConfig, type ModelConfigWithName } from './utils';
import { ModelConfigFormCommunity } from './model-config-form-community';

export interface ModelConfigPopoverProps {
  models: Model[] | undefined;
  value?: ModelConfigWithName;
  disabled?: boolean;
  modelSelectProps?: SelectProps;
  popoverProps?: PopoverProps & {
    wrapClassName?: string;
    wrapStyle?: CSSProperties;
  };
  defaultActiveFirstModel?: boolean;
  /** 使用场景 */
  scenario?: Scenario;
  onChange?: (value?: ModelConfigWithName) => void;
  renderDisplayContent?: (
    model?: Model,
    isPopoverVisible?: boolean,
  ) => React.ReactNode;
  onModelChange?: (value?: Model) => void;
}

export function PopoverModelConfigEditor({
  value,
  disabled,
  modelSelectProps,
  popoverProps,
  onChange,
  renderDisplayContent,
  defaultActiveFirstModel = false,
  models = [],
  onModelChange,
}: ModelConfigPopoverProps) {
  const formApi = useRef<FormApi<ModelConfigWithName>>();
  const [initValues, setInitValues] = useState<ModelConfigWithName>();
  const { wrapClassName, wrapStyle, ...restPopoverProps } = popoverProps || {};
  const [selectModel, setSelectModel] = useState<Model | undefined>();
  const [isPopoverVisible, setPopoverVisible] = useState(false);
  const selectModelRef = useLatest(selectModel);
  const loadedRef = useRef(false);

  // 处理默认选中第一个的逻辑
  useEffect(() => {
    // 通过条件：设置了默认选中第一个模型，并且没有传入value，models不为空，没有加载过
    if (
      !defaultActiveFirstModel ||
      value ||
      loadedRef.current ||
      !models?.length
    ) {
      return;
    }
    const model = models[0];
    // 默认选中第一个模型
    if (model) {
      setSelectModel?.(model);
      const modelConfig = convertModelToModelConfig(model);
      setInitValues(modelConfig);
      formApi.current?.setValues?.(modelConfig, {
        isOverride: true,
      });
      onChange?.({ ...modelConfig });
      onModelChange?.(model);
    }
    loadedRef.current = true;
  }, [value, models]);

  // 处理初始加载时已传入值预览模型的逻辑
  useEffect(() => {
    // 通过条件：传入了value，并且selectModel为空，并且models不为空，并且没有加载过
    if (
      !value ||
      selectModelRef.current ||
      loadedRef.current ||
      !models?.length
    ) {
      return;
    }
    const model = models.find(
      item => `${item.model_id}` === `${value.model_id}`,
    );
    if (model) {
      setSelectModel?.(model);
      onModelChange?.(model);
    } else {
      const newModel = {
        model_id: value.model_id,
        name: value.model_id,
      };
      setSelectModel?.(newModel);
      onModelChange?.(newModel);
    }
    loadedRef.current = true;
  }, [value, models]);

  return (
    <Popover
      keepDOM={false}
      content={
        <div className="pt-1 px-4 pb-4" style={{ width: 496 }}>
          <Form<ModelConfigWithName>
            labelWidth={140}
            // 有值时value生效，否则 initValues 生效
            initValues={value || initValues}
            onValueChange={values => {
              onChange?.({ ...values });
            }}
            getFormApi={api => (formApi.current = api)}
            disabled={disabled}
          >
            <Space className="py-0 flex">
              <Typography.Title heading={6} style={{ minWidth: 120 }}>
                模型选择
              </Typography.Title>
              <ModelSelectWithObject
                {...modelSelectProps}
                className={classNames('grow', modelSelectProps?.className)}
                value={selectModel}
                disabled={disabled}
                onChange={newModel => {
                  setSelectModel(newModel);
                  const modelConfig = newModel
                    ? convertModelToModelConfig(newModel)
                    : {};
                  setInitValues(modelConfig);
                  formApi.current?.setValues?.(modelConfig, {
                    isOverride: true,
                  });
                  onModelChange?.(newModel);
                }}
                modelList={models}
              />
            </Space>
            <Typography.Title
              heading={6}
              style={{ marginTop: 12, marginBottom: 12 }}
            >
              参数配置
            </Typography.Title>
            <ModelConfigFormCommunity model={selectModel} />
          </Form>
        </div>
      }
      trigger="custom"
      visible={isPopoverVisible}
      onClickOutSide={() => setPopoverVisible(false)}
      {...restPopoverProps}
    >
      <div
        className={wrapClassName}
        style={wrapStyle}
        onClick={() => setPopoverVisible(true)}
      >
        {renderDisplayContent?.(selectModel, isPopoverVisible)}
      </div>
    </Popover>
  );
}
