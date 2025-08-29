import { Cascader, type CascaderProps } from '@coze-arch/coze-design';

import {
  MUTIPART_DATA_TYPE_LIST_WITH_ARRAY,
  type DataType,
} from '../dataset-item/type';

interface FieldTypeProps {
  value: DataType;
  onChange?: (value?: DataType) => void;
  disabled?: boolean;
  className?: string;
  treeData?: CascaderProps['treeData'];
  zIndex?: number;
}

export const DataTypeSelect = ({
  value,
  onChange,
  disabled,
  className,
  zIndex,
  treeData = MUTIPART_DATA_TYPE_LIST_WITH_ARRAY,
}: FieldTypeProps) => (
  <Cascader
    value={value?.includes('array') ? ['array', value] : [value]}
    disabled={disabled}
    className={className}
    zIndex={zIndex}
    displayRender={selected => {
      const selectedValue = selected as string[];
      return selectedValue?.length > 1
        ? `Array<${selectedValue?.pop()}>`
        : selectedValue?.pop();
    }}
    treeData={treeData}
    onChange={newValue => {
      onChange?.((newValue as DataType[])?.pop());
    }}
  ></Cascader>
);
