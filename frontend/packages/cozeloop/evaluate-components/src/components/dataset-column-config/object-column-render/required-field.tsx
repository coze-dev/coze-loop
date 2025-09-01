import { Select } from '@coze-arch/coze-design';

interface RequiredFieldProps {
  value: boolean;
  onChange?: (value: boolean) => void;
  disabled?: boolean;
  className?: string;
}

export const RequiredField = ({
  value,
  onChange,
  disabled,
  className,
}: RequiredFieldProps) => (
  <Select
    disabled={disabled}
    className={className}
    value={value === true ? 'true' : 'false'}
    optionList={[
      { label: '是', value: 'true' },
      { label: '否', value: 'false' },
    ]}
    onChange={newValue => {
      onChange?.(newValue === 'true');
    }}
  ></Select>
);
