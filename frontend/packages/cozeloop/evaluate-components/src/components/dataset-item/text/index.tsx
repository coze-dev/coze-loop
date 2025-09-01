import { useMemo } from 'react';

import { safeJsonParse } from '@cozeloop/toolkit';

import { CopyIcon } from '@/components/common/copy-icon';

import { getDataType } from '../util';
import { DataType, type DatasetItemProps } from '../type';
import { StringDatasetItem } from './string';
import { ObjectDatasetItem } from './object';
import { IntegerDatasetItem } from './integer';
import { FloatDatasetItem } from './float';
import { BoolDatasetItem } from './bool';
import { ArrayDatasetItem } from './array';

const TextColumnComponentMap = {
  [DataType.String]: StringDatasetItem,
  [DataType.Integer]: IntegerDatasetItem,
  [DataType.Boolean]: BoolDatasetItem,
  [DataType.Float]: FloatDatasetItem,
  [DataType.Object]: ObjectDatasetItem,
  [DataType.ArrayBoolean]: ArrayDatasetItem,
  [DataType.ArrayString]: ArrayDatasetItem,
  [DataType.ArrayInteger]: ArrayDatasetItem,
  [DataType.ArrayFloat]: ArrayDatasetItem,
  [DataType.ArrayObject]: ArrayDatasetItem,
};

export const TextDatasetItem = (props: DatasetItemProps) => {
  const type = getDataType(props.fieldSchema);
  const Component = TextColumnComponentMap[type] ?? StringDatasetItem;
  const copyText = useMemo(() => {
    const text = props.fieldContent?.text;
    const schemaType = type as DataType;
    // 去除Object和Array的空格和换行，注意这里序列化时是否有对象字段顺序问题
    if (
      text &&
      (schemaType === DataType.Object || schemaType?.startsWith('array'))
    ) {
      const minifyText = JSON.stringify(safeJsonParse(text || ''));
      return minifyText;
    }
    return text;
  }, [props.fieldContent, type]);
  return (
    <div className="relative group pr-5">
      <Component {...props} />
      {/* 复制功能 */}
      {copyText ? (
        <CopyIcon
          text={copyText}
          className="z-100 absolute top-0 right-0 invisible group-hover:visible"
          onClick={e => e.stopPropagation()}
        />
      ) : null}
    </div>
  );
};
