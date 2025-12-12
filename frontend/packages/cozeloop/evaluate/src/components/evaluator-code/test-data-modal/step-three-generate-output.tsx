// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback, useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
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
        <div className="mb-2 text-sm font-medium text-gray-700">
          {I18n.t('evaluate_simulated_data')}
        </div>
        <CommonTable
          supportMultiSelect={false}
          data={mergeData}
          fieldSchemas={mergeFieldSchemas}
        />
      </div>

      {/* 操作按钮 */}
      <div className="flex pt-4 gap-2 ml-auto">
        <Button color="primary" onClick={onPrevStep}>
          {I18n.t('evaluate_previous_step_associate_targets')}
        </Button>

        <Button onClick={handleImport}>{I18n.t('import_data')}</Button>
      </div>
    </div>
  );
};

export default StepThreeGenerateOutput;
