import { I18n } from '@cozeloop/i18n-adapter';
import { exprGroupItemRunStatusInfoList } from '@cozeloop/evaluate-components';
import { type ItemRunState } from '@cozeloop/api-schema/evaluation';
import { Select, type SelectProps } from '@coze-arch/coze-design';

import { ExperimentGroupItemRunStatus } from '../previews/experiment-group-item-run-status';

const statusOptions = exprGroupItemRunStatusInfoList.map(item => ({
  label: item.name,
  value: item.status,
}));

function RenderSelectedItem(optionNode: Record<string, unknown>) {
  const option = optionNode;
  const content = (
    <ExperimentGroupItemRunStatus status={option.value as ItemRunState} />
  );
  return {
    isRenderInTag: false,
    content,
  };
}

/** 实验对话组运行状态标签 */
export function ExprGroupItemRunStatusSelect({
  value,
  onChange,
  onBlur,
  ...rest
}: {
  value?: ItemRunState[];
  onChange?: (value: ItemRunState[]) => void;
  onBlur?: () => void;
} & SelectProps) {
  return (
    <Select
      prefix={I18n.t('status')}
      placeholder={I18n.t('please_select', { field: '' })}
      multiple={true}
      showClear={true}
      maxTagCount={2}
      optionList={statusOptions}
      renderSelectedItem={RenderSelectedItem}
      {...rest}
      value={value}
      onChange={val => {
        onChange?.(val as ItemRunState[]);
      }}
      onBlur={() => onBlur?.()}
    />
  );
}
