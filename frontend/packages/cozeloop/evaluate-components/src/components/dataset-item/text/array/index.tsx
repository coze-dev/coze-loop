import { type DatasetItemProps } from '../../type';
import { ArrayDatasetItemReadOnly } from './readonly';
import { ArrayDatasetItemEdit } from './edit';

export const ArrayDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <ArrayDatasetItemEdit {...props} />
  ) : (
    <ArrayDatasetItemReadOnly {...props} />
  );
