import { type DatasetItemProps } from '../../type';
import { ObjectDatasetItemReadOnly } from './readonly';
import { ObjectDatasetItemEdit } from './edit';

export const ObjectDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <ObjectDatasetItemEdit {...props} />
  ) : (
    <ObjectDatasetItemReadOnly {...props} />
  );
