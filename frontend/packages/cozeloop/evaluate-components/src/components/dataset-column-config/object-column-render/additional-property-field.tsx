import { I18n } from '@cozeloop/i18n-adapter';
import { Select } from '@coze-arch/coze-design';

interface RequiredFieldProps {
  value: boolean;
  onChange?: (value: boolean) => void;
  disabled?: boolean;
  className?: string;
  hiddenValue?: boolean;
}

export const AdditionalPropertyField = ({
  value,
  onChange,
  disabled,
  className,
  hiddenValue,
}: RequiredFieldProps) => (
  <Select
    className={className}
    disabled={disabled}
    value={hiddenValue ? undefined : value === false ? 'false' : 'true'}
    optionList={[
      { label: I18n.t('yes'), value: 'true' },
      { label: I18n.t('no'), value: 'false' },
    ]}
    onChange={newValue => {
      onChange?.(newValue === 'true');
    }}
  ></Select>
);
