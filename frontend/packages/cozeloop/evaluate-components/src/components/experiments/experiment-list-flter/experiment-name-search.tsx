import classNames from 'classnames';
import { IconCozMagnifier } from '@coze-arch/coze-design/icons';
import { Search, type SearchProps } from '@coze-arch/coze-design';

export function ExperimentNameSearch({
  value,
  onChange,
  ...rest
}: {
  value?: string;
  disabled?: boolean;
  onChange?: (value: string) => void;
} & SearchProps) {
  return (
    <div className="w-60">
      <Search
        placeholder="搜索名称"
        prefix={<IconCozMagnifier />}
        {...rest}
        className={classNames('!w-full', rest.className)}
        style={{ width: '100%', flexShrink: 0, ...(rest.style ?? {}) }}
        value={value}
        onChange={val => onChange?.(val)}
      />
    </div>
  );
}
