import { I18n } from '@cozeloop/i18n-adapter';
import { type ExptStatus } from '@cozeloop/api-schema/evaluation';
import { Select, type SelectProps } from '@coze-arch/coze-design';

import { ExperimentRunStatus } from '../previews/experiment-run-status';
import { experimentRunStatusInfoList } from '../../../constants/experiment-status';

type ValueType = (string | number)[];

interface OptionItem {
  label: string;
  value: string | number;
}

const statusOptions = experimentRunStatusInfoList
  .filter(e => !e.hideInFilter)
  .map(item => ({
    label: item.name,
    value: item.status,
  }));

function RenderSelectedItem(optionNode: Record<string, unknown>) {
  const option = optionNode as unknown as OptionItem;
  const content = <ExperimentRunStatus status={option.value as ExptStatus} />;
  return {
    isRenderInTag: false,
    content,
  };
}

export function ExperimentStatusSelect({
  value,
  onChange,
  onBlur,
  ...rest
}: {
  value?: ValueType;
  disabled?: boolean;
  className?: string;
  onChange?: (value: ValueType) => void;
  onBlur?: () => void;
} & SelectProps) {
  return (
    <Select
      prefix={I18n.t('status')}
      placeholder={I18n.t('please_select', { field: '' })}
      showClear={true}
      maxTagCount={2}
      optionList={statusOptions}
      {...rest}
      renderSelectedItem={RenderSelectedItem}
      multiple={true}
      style={{ minWidth: 170, ...(rest.style ?? {}) }}
      value={value}
      onChange={val => onChange?.(val as ValueType)}
      onBlur={() => onBlur?.()}
    />
  );
}
