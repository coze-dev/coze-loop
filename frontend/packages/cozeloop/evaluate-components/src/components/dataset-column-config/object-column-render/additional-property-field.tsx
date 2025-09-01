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
      { label: '是', value: 'true' },
      { label: '否', value: 'false' },
    ]}
    onChange={newValue => {
      onChange?.(newValue === 'true');
    }}
  ></Select>
);
