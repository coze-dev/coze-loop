/* eslint-disable complexity */
import classNames from 'classnames';
import { FieldDisplayFormat } from '@cozeloop/api-schema/data';
import { Typography } from '@coze-arch/coze-design';

import { LoopTag } from '../tag';
import { getColumnType } from './util';
import {
  ContentType,
  DataType,
  dataTypeMap,
  displayFormatType,
  type DatasetItemProps,
} from './type';
import { TextDatasetItem } from './text';
import { MultipartDatasetItem } from './multipart';
import { ImageDatasetItem } from './image';
import { EmptyDatasetItem } from './empty';
import { AudioDatasetItem } from './audio';

const ItemContenRenderMap = {
  [ContentType.Text]: TextDatasetItem,
  [ContentType.Image]: ImageDatasetItem,
  [ContentType.Audio]: AudioDatasetItem,
  [ContentType.MultiPart]: MultipartDatasetItem,
};

export const DatasetItem = (props: DatasetItemProps) => {
  const {
    fieldSchema,
    fieldContent,
    showColumnKey,
    className,
    isEdit,
    showEmpty,
  } = props;

  const Component =
    ItemContenRenderMap[fieldSchema?.content_type || ContentType.Text] ||
    TextDatasetItem;
  const isEmpty =
    fieldContent?.multi_part === undefined && fieldContent?.text === undefined;
  return (
    <div className={classNames('flex flex-col gap-2', className)}>
      {showColumnKey ? (
        <div className="flex items-center gap-1">
          <Typography.Text className="text-[14px] !font-medium">
            {fieldSchema?.name}
          </Typography.Text>
          <LoopTag color="primary">
            {dataTypeMap[getColumnType(fieldSchema)] ||
              dataTypeMap[DataType.String]}
          </LoopTag>
          <LoopTag color="primary">
            {
              displayFormatType[
                fieldContent?.format ||
                  fieldSchema?.default_display_format ||
                  FieldDisplayFormat.PlainText
              ]
            }
          </LoopTag>
        </div>
      ) : null}
      {showEmpty && isEmpty && !isEdit ? (
        <EmptyDatasetItem />
      ) : (
        <Component {...props} />
      )}
    </div>
  );
};
