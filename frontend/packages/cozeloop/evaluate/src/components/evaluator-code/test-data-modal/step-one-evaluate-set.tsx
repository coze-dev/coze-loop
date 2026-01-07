// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @coze-arch/max-line-per-function */
{
  /* start_aigc */
}
import { useCallback, useState } from 'react';

import { useRequest } from 'ahooks';
import {
  convertEvaluationSetItemListToTableData,
  EvaluateSetSelect,
  EvaluateSetVersionSelect,
} from '@cozeloop/evaluate-components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozLoading } from '@coze-arch/coze-design/icons';
import {
  Button,
  Tooltip,
  withField,
  useFormState,
} from '@coze-arch/coze-design';

import type { StepOneEvaluateSetProps } from '../types';
import CommonTable from './common-table';

const FormEvaluateSetSelect = withField(EvaluateSetSelect);

const StepOneEvaluateSet: React.FC<StepOneEvaluateSetProps> = ({
  formRef,
  onImport,
  onNextStep,
  evaluationSetData,
  setEvaluationSetData,
  fieldSchemas,
  setFieldSchemas,
  prevCount,
}) => {
  const { spaceID } = useSpace();

  const [isEmpty, setIsEmpty] = useState(true);

  const formState = useFormState();

  const { values: formValues } = formState;

  // 获取真实的评测集数据
  const { loading: dataLoading, run: loadEvaluationSetData } = useRequest(
    async (evaluationSetId: string, versionId: string) => {
      try {
        const fieldVersionData =
          await StoneEvaluationApi.GetEvaluationSetVersion({
            evaluation_set_id: evaluationSetId,
            workspace_id: spaceID,
            version_id: versionId,
          });

        const response = await StoneEvaluationApi.ListEvaluationSetItems({
          evaluation_set_id: evaluationSetId,
          workspace_id: spaceID,
          version_id: versionId,
          page_number: 1,
          page_size: 100, // 获取前100条数据
        });

        const schemaData =
          fieldVersionData?.version?.evaluation_set_schema?.field_schemas ?? [];

        const tableData = convertEvaluationSetItemListToTableData(
          response.items ?? [],
          schemaData,
        );

        setEvaluationSetData(tableData);
        setFieldSchemas(schemaData);
        return tableData;
      } catch (error) {
        console.error('获取评测集数据失败:', error);
        // 如果API调用失败，返回空数组
        setEvaluationSetData([]);
        return [];
      }
    },
    {
      manual: true,
    },
  );

  const handleEvaluationSetChange = () => {
    // 清空版本选择
    formRef.current?.formApi?.setValue('evaluationSetVersion', undefined);
    formRef.current?.formApi?.setValue('selectedItems', undefined);
    setEvaluationSetData([]);
    setFieldSchemas([]);
  };

  const handleEvaluationSetVersionChange = async (value: unknown) => {
    formRef.current?.formApi?.setValue('selectedItems', undefined);
    await loadEvaluationSetData(formValues?.evaluationSetId, value as string);
  };

  const handleSelectionChange = useCallback(
    (selectedItems: Set<string>) => {
      if (selectedItems.size > 0) {
        setIsEmpty(false);
      } else {
        setIsEmpty(true);
      }
      formRef.current?.formApi?.setValue('selectedItems', selectedItems);
    },
    [formRef],
  );

  const onDirectImport = () => {
    const selectedItems = formValues?.selectedItems || new Set();
    const selectedData = evaluationSetData.filter(item =>
      selectedItems.has(item.item_id as string),
    );

    const transformData = selectedData.map(item => ({
      evaluate_dataset_fields: item?.trunFieldData?.fieldDataMap || {},
    }));

    onImport(transformData, selectedData);
  };

  const renderDataContent = () => {
    if (dataLoading) {
      return (
        <div className="flex flex-row items-center justify-center py-8">
          <IconCozLoading className="w-4 h-4 animate-spin coz-fg-secondary" />
          <div className="ml-[6px] text-sm coz-fg-secondary">
            正在加载数据...
          </div>
        </div>
      );
    }

    return (
      <CommonTable
        supportMultiSelect={true}
        data={evaluationSetData}
        fieldSchemas={fieldSchemas}
        onSelectionChange={handleSelectionChange}
        loading={dataLoading}
        prevCount={prevCount}
      />
    );
  };

  return (
    <>
      <div className="space-y-4">
        {/* 评测集和版本选择 */}
        <div className="grid grid-cols-2 gap-4">
          <FormEvaluateSetSelect
            className="w-full"
            field="evaluationSetId"
            label="评测集"
            remote={true}
            filter={true}
            placeholder={'请选择评测集'}
            onChange={handleEvaluationSetChange}
            onChangeWithObject={false}
          />

          <EvaluateSetVersionSelect
            evaluationSetId={formValues?.evaluationSetId}
            onChange={handleEvaluationSetVersionChange}
            className="w-full"
            remote={true}
            filter={true}
            field="evaluationSetVersion"
            label={{
              text: '版本',
              className: 'justify-between pr-0',
            }}
            placeholder={'请选择版本号'}
          />
        </div>

        {/* 描述信息 */}
        {formValues?.evaluationSetDetail ? (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              描述
            </label>
            <div className="text-sm coz-fg-primary font-normal">
              {formValues?.evaluationSetDetail?.description || '-'}
            </div>
          </div>
        ) : null}

        {/* 数据表格 */}
        {renderDataContent()}
      </div>

      {/* 操作按钮 */}
      <div className="flex pt-4 gap-2 justify-end">
        <Tooltip
          theme="dark"
          content="如果你的评测集中已经包含评测对象输出，可直接导入你选中的数据"
        >
          <Button onClick={onDirectImport} color="primary" disabled={isEmpty}>
            直接导入
          </Button>
        </Tooltip>

        <Button onClick={onNextStep} disabled={isEmpty}>
          下一步：评测对象
        </Button>
      </div>
    </>
  );
};

export default StepOneEvaluateSet;
{
  /* end_aigc */
}
