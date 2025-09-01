import { type DatasetItemProps } from '../../type';
import { FloatDatasetItemReadOnly } from './readonly';
import { FloatDatasetItemEdit } from './edit';

export const FloatDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <FloatDatasetItemEdit {...props} />
  ) : (
    <FloatDatasetItemReadOnly {...props} />
  );
