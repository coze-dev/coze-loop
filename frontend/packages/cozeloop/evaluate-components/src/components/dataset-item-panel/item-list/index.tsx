import { useMemo } from 'react';

import {
  ContentType,
  type EvaluationSet,
  type FieldSchema,
  type Turn,
} from '@cozeloop/api-schema/evaluation';
import { withField } from '@coze-arch/coze-design';

import {
  validateMultiPartData,
  validateTextFieldData,
} from '../../dataset-item/util';
import { DatasetFieldItemRender } from '../../dataset-item/dataset-field-render';

interface DatasetItemRenderListProps {
  datasetDetail?: EvaluationSet;
  turn?: Turn;
  fieldSchemas?: FieldSchema[];
  isEdit: boolean;
  fieldKey?: string;
  itemMaxHeightAuto?: boolean; // 列内容高度不受限制
}
const FormFieldItemRender = withField(DatasetFieldItemRender);

export const DatasetItemRenderList = ({
  fieldSchemas,
  isEdit,
  turn,
  fieldKey,
  itemMaxHeightAuto,
  datasetDetail,
}: DatasetItemRenderListProps) => {
  const fieldSchemaMap = useMemo(() => {
    const map = new Map<string, FieldSchema>();
    fieldSchemas?.forEach(item => {
      if (item.key) {
        map.set(item.key, item);
      }
    });
    return map;
  }, [fieldSchemas]);
  return (
    <div className="flex flex-col">
      {turn?.field_data_list?.map((fieldData, index) => {
        const fieldSchema = fieldSchemaMap.get(fieldData.key || '');
        return (
          <FormFieldItemRender
            noLabel
            datasetID={datasetDetail?.id}
            className={itemMaxHeightAuto ? '!max-h-none' : ''}
            field={`${fieldKey}.field_data_list[${index}]`}
            key={fieldData?.key}
            fieldSchema={fieldSchema}
            fieldData={fieldData}
            showColumnKey
            expand={true}
            showEmpty={true}
            displayFormat={true}
            isEdit={isEdit}
            multipartConfig={datasetDetail?.spec?.multi_modal_spec}
            rules={[
              {
                validator: (_, value, callback) => {
                  if (fieldSchema?.content_type === ContentType.MultiPart) {
                    const res = validateMultiPartData(
                      value?.content?.multi_part,
                      callback,
                      fieldSchema,
                    );
                    return res;
                  }
                  const res = validateTextFieldData(
                    value?.content?.text,
                    callback,
                    fieldSchema,
                  );
                  return res;
                },
              },
            ]}
          />
        );
      })}
    </div>
  );
};
