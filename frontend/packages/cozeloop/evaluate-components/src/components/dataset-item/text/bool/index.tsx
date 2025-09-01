import { type DatasetItemProps } from '../../type';
import { BoolDatasetItemReadOnly } from './readonly';
import { BoolDatasetItemEdit } from './edit';

export const BoolDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <BoolDatasetItemEdit {...props} />
  ) : (
    <BoolDatasetItemReadOnly {...props} />
  );
