import { Input } from '@coze-arch/coze-design';

import { type DatasetItemProps } from '../../type';

export const IntegerDatasetItemEdit = ({
  fieldContent,
  onChange,
}: DatasetItemProps) => (
  <>
    <Input
      placeholder={I18n.t('cozeloop_open_evaluate_enter_integer')}
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
