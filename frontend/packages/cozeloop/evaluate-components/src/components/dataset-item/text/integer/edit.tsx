import { Input } from '@coze-arch/coze-design';

import { type DatasetItemProps } from '../../type';

export const IntegerDatasetItemEdit = ({
  fieldContent,
  onChange,
}: DatasetItemProps) => (
  <>
    <Input
      placeholder="请输入integer"
      className="rounded-[6px]"
      value={fieldContent?.text}
      onChange={value => {
        onChange?.({
          ...fieldContent,
          text: value,
        });
      }}
    />
  </>
);
