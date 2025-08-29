import { type DatasetItemProps } from '../../../type';
import { PlainTextDatasetItemReadOnly } from './readonly';
import { PlainTextDatasetItemEdit } from './edit';

export const PlainTextDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <PlainTextDatasetItemEdit {...props} />
  ) : (
    <PlainTextDatasetItemReadOnly {...props} />
  );
