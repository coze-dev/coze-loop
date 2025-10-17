/*
 * Copyright 2025
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/* eslint-disable @coze-arch/max-line-per-function */
import { useCallback, useMemo, useRef } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type EvaluationSetItemTableData } from '@cozeloop/evaluate-components';
import { CodeEditor } from '@cozeloop/components';
import {
  IconCozPlus,
  IconCozQuestionMarkCircle,
} from '@coze-arch/coze-design/icons';
import {
  Select,
  Button,
  withField,
  type CommonFieldProps,
  Divider,
  Tooltip,
} from '@coze-arch/coze-design';

import { EVALUATOR_CODE_DOCUMENT_LINK } from '@/utils/evaluator';
import { defaultTestData, MAX_SELECT_COUNT } from '@/constants/code-evaluator';

import {
  type BaseDataSetConfigProps,
  type TestData,
  TestDataSource,
} from '../types';
import EvalSetTestData from './eval-set-test-data';

const JSON_INDENT = 2;

const dataSourceOptions = [
  {
    label: I18n.t('evaluate_based_on_evaluation_set'),
    value: TestDataSource.Dataset,
  },
  { label: I18n.t('evaluate_custom'), value: TestDataSource.Custom },
];

const toolTipContent = (
  <div>
    turn代表单轮问答的评测场景，其中：
    <br />
    evaluate_dataset_fields：评测集字段
    <br />
    evaluate_target_output_fields：评测对象字段
    <br />
    ext：补充字段
    <br />
    详细内容请参考
    <a href={EVALUATOR_CODE_DOCUMENT_LINK} target="_blank">
      文档
    </a>
    。
  </div>
);

/**
 * 自定义数据内容渲染
 */
const CustomDataContent: React.FC<{
  jsonString: string;
  handleJsonChange: (value: string | undefined) => void;
  disabled?: boolean;
}> = ({ jsonString, handleJsonChange, disabled }) => (
  <div className="flex flex-col h-full">
    {/* 数据编辑器 */}
    <div className="flex-1 overflow-hidden">
      <CodeEditor
        language="json"
        value={jsonString}
        onChange={handleJsonChange}
        options={{
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          wordWrap: 'on',
          fontSize: 14,
          lineNumbers: 'on',
          folding: true,
          automaticLayout: true,
          tabSize: 2,
          insertSpaces: true,
          formatOnPaste: true,
          formatOnType: true,
          bracketPairColorization: { enabled: true },
          suggest: {
            showKeywords: true,
            showSnippets: true,
          },
          readOnly: disabled,
        }}
        theme="vs-light"
        height="500px"
      />
    </div>
  </div>
);

/**
 * 自定义数据编辑器基础组件
 */
const BaseDataSetConfig: React.FC<
  BaseDataSetConfigProps & CommonFieldProps
> = props => {
  const { disabled, value, onChange } = props;
  const { source, setData, customData } = value || {};

  // 创建一个ref来存储EvalSetTestData组件所暴露的打开弹窗的方法
  const openModalRef = useRef<(() => void) | undefined>();

  // 调用子组件的打开弹窗方法
  const handleOpenTestDataModal = useCallback(() => {
    if (openModalRef.current) {
      openModalRef.current();
    }
  }, []);

  // 处理数据源变更
  const handleDataSourceChange = useCallback(
    (newSource: TestDataSource) => {
      // 如果切换到自定义且没有数据，设置默认数据
      if (newSource === TestDataSource.Custom) {
        onChange?.({
          ...value,
          source: newSource,
          customData: defaultTestData[0] || {},
        });
      } else if (newSource === TestDataSource.Dataset) {
        onChange?.({ ...value, source: newSource });
      }
    },
    [onChange, value],
  );

  // 将自定义数据转换为 JSON 字符串用于编辑器显示
  const customDataString = useMemo(() => {
    try {
      const custom = customData || {};
      return JSON.stringify(custom, null, JSON_INDENT);
    } catch (error) {
      console.error(I18n.t('evaluate_json_serialize_error'), error);
      return '{}';
    }
  }, [customData]);

  // 处理 JSON 编辑器的值变化
  const handleJsonChange = useCallback(
    (newJsonValue: string | undefined) => {
      if (!newJsonValue) {
        onChange?.({ ...value, customData: {} });
        return;
      }
      try {
        const parsedData = JSON.parse(newJsonValue);
        onChange?.({ ...value, customData: parsedData });
      } catch (error) {
        // JSON 解析错误时不更新数据，保持编辑器中的内容
        console.error(I18n.t('evaluate_json_parse_error'), error);
      }
    },
    [onChange, value],
  );

  const handleSetDataChange = (
    data: TestData[],
    originSelectedData?: EvaluationSetItemTableData[],
  ) => {
    const payload = {
      ...value,
      setData: data,
    };
    if (originSelectedData) {
      payload.originSelectedData = originSelectedData;
    }
    onChange?.(payload);
  };

  return (
    <>
      <div className="flex flex-col h-full">
        {/* Header */}
        <div
          className="flex items-center h-[44px] py-2 px-3"
          style={{
            borderBottom: '1px solid rgba(82, 100, 154, 0.13)',
          }}
        >
          <div className="flex items-center space-x-2">
            <h3 className="text-sm font-medium text-gray-900">
              {I18n.t('evaluate_test_data_turn')}
            </h3>
            <Tooltip
              content={toolTipContent}
              theme="dark"
              style={{ maxWidth: '400px' }}
            >
              <IconCozQuestionMarkCircle className="cursor-pointer" />
            </Tooltip>
            <Divider layout="vertical" className="!mr-3 !ml-3 h-3" />
          </div>
          <div className="flex items-center gap-x-3">
            <Select
              value={source}
              onChange={selectedValue =>
                handleDataSourceChange(selectedValue as TestDataSource)
              }
              className="max-w-[156px] w-[156px] h-[24px] min-h-[24px]"
              size="small"
              disabled={disabled}
            >
              {dataSourceOptions.map(option => (
                <Select.Option key={option.value} value={option.value}>
                  {option.label}
                </Select.Option>
              ))}
            </Select>
            {source === TestDataSource.Dataset && (
              <Tooltip
                content={I18n.t('evaluate_add_data_from_evaluation_set')}
                theme="dark"
              >
                <Button
                  size="mini"
                  color="primary"
                  icon={<IconCozPlus />}
                  onClick={handleOpenTestDataModal}
                  disabled={
                    disabled || (setData?.length || 0) >= MAX_SELECT_COUNT
                  }
                />
              </Tooltip>
            )}
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-hidden">
          <div
            className={`h-full flex justify-center ${source !== TestDataSource.Dataset ? 'hidden' : ''}`}
          >
            <EvalSetTestData
              testData={setData}
              disabled={disabled}
              importedTestData={setData || []}
              setImportedTestData={handleSetDataChange}
              onOpenModalRef={openModalRef}
            />
          </div>
          <div
            className={`${source !== TestDataSource.Custom ? 'hidden' : ''}`}
          >
            <CustomDataContent
              jsonString={customDataString ?? ''}
              handleJsonChange={handleJsonChange}
              disabled={disabled}
            />
          </div>
        </div>
      </div>
    </>
  );
};

// 使用withField包装组件
const DataSetConfig: React.ComponentType<
  BaseDataSetConfigProps & CommonFieldProps
> = withField(BaseDataSetConfig);

export default DataSetConfig;
