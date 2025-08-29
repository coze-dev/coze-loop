import { type DatasetItemProps } from '../../type';
import { IntegerDatasetItemReadOnly } from './readonly';
import { IntegerDatasetItemEdit } from './edit';

export const IntegerDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <IntegerDatasetItemEdit {...props} />
  ) : (
    <IntegerDatasetItemReadOnly {...props} />
  );
