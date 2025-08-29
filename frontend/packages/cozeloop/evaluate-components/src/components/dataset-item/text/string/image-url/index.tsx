import { Image, Input } from '@coze-arch/coze-design';

import { type DatasetItemProps } from '../../../type';

export const ImageUrlDatasetItem = (props: DatasetItemProps) => {
  const { isEdit, fieldContent, onChange, className } = props;
  return isEdit ? (
    <Input
      value={fieldContent?.text}
      onChange={val => onChange?.({ text: val })}
    />
  ) : (
    <Image className={className} src={fieldContent?.text} />
  );
};
