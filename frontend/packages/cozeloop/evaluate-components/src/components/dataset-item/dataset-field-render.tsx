import { ContentType, type Content } from '@cozeloop/api-schema/evaluation';
import { Typography } from '@coze-arch/coze-design';

import { LoopTag } from '../tag';
import { useItemFormat } from './use-item-format';
import {
  DataType,
  dataTypeMap,
  type DatasetFieldItemRenderProps,
} from './type';
import { DatasetItem } from './dataset-item-render';

export const DatasetFieldItemRender = ({
  fieldData,
  onChange,
  showColumnKey,
  ...props
}: DatasetFieldItemRenderProps) => {
  const { fieldSchema, isEdit } = props;
  const onContentChange = (content: Content) => {
    onChange?.({
      key: fieldData?.key,
      name: fieldData?.name,
      content: {
        content_type: fieldSchema?.content_type,
        ...fieldData?.content,
        ...content,
      },
    });
  };
  const { type, formatSelect, format } = useItemFormat(
    fieldSchema,
    props.datasetID,
  );
  const fieldContent = {
    ...fieldData?.content,
    format,
  };
  const hiddenFormatSelect =
    fieldSchema?.content_type === ContentType.MultiPart && isEdit;
  return (
    <div className="flex flex-col gap-2">
      {showColumnKey ? (
        <div className="flex items-center gap-2 h-[24px]">
          <Typography.Text className="text-[14px] !font-medium">
            {fieldSchema?.name}
            {fieldSchema?.isRequired ? (
              <span className="text-red ml-[2px]">*</span>
            ) : (
              ''
            )}
          </Typography.Text>
          <LoopTag
            color="primary"
            size="mini"
            className="!font-semibold !text-[12px]"
          >
            {dataTypeMap[type] || dataTypeMap[DataType.String]}
          </LoopTag>
          <div className="flex-1 flex justify-end  ">
            {hiddenFormatSelect ? null : formatSelect}
          </div>
        </div>
      ) : null}
      <DatasetItem
        {...props}
        showColumnKey={false}
        onChange={onContentChange}
        fieldContent={fieldContent}
      />
    </div>
  );
};
