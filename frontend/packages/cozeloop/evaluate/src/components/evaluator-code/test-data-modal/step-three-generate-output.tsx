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

// start_aigc
import { useCallback, useMemo } from 'react';

import { ContentType } from '@cozeloop/api-schema/evaluation';
import { Button, useFormState } from '@coze-arch/coze-design';

import type { StepThreeGenerateOutputProps } from '../types';
import CommonTable from './common-table';

const StepThreeGenerateOutput: React.FC<
  StepThreeGenerateOutputProps
> = props => {
  const { onPrevStep, onImport, evaluationSetData, fieldSchemas } = props;
  const formState = useFormState();
  const { values: formValues } = formState;
  const { selectedItems } = formValues;
  const mockSetData = formValues?.mockSetData;

  const mergeData = useMemo(
    () =>
      evaluationSetData
        .filter(item => selectedItems?.has(item.item_id as string))
        .map(item => ({
          ...item,
          trunFieldData: {
            ...item.trunFieldData,
            fieldDataMap: {
              ...item.trunFieldData.fieldDataMap,
              actual_output:
                mockSetData?.[0]?.evaluate_target_output_fields?.actual_output,
            },
          },
        })),
    [evaluationSetData, mockSetData],
  );

  const mergeFieldSchemas = useMemo(
    () => [
      ...fieldSchemas,
      {
        key: 'actual_output',
        name: 'actual_output',
        default_display_format: 1,
        status: 1,
        isRequired: false,
        hidden: false,
        text_schema: '{"type": "string"}',
        description: '',
        content_type: ContentType.Text,
      },
    ],
    [fieldSchemas],
  );

  const handleImport = useCallback(() => {
    const payload = mockSetData || [];
    onImport(payload, mergeData);
  }, [mockSetData, onImport, mergeData]);

  return (
    <div className="flex flex-col">
      {/* 数据预览表格 */}
      <div>
        <div className="mb-2 text-sm font-medium text-gray-700">模拟数据</div>
        <CommonTable
          supportMultiSelect={false}
          data={mergeData}
          fieldSchemas={mergeFieldSchemas}
        />
      </div>

      {/* 操作按钮 */}
      <div className="flex pt-4 gap-2 ml-auto">
        <Button color="primary" onClick={onPrevStep}>
          上一步：关联评测对象
        </Button>

        <Button onClick={handleImport}>导入数据</Button>
      </div>
    </div>
  );
};

export default StepThreeGenerateOutput;
// end_aigc
