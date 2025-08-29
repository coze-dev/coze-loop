import { experimentItemRunStatusInfoList } from '@cozeloop/evaluate-components';
import { type TurnRunState } from '@cozeloop/api-schema/evaluation';
import { Select, type SelectProps } from '@coze-arch/coze-design';

import ExperimentItemRunStatus from '../previews/experiment-item-run-status';

const statusOptions = experimentItemRunStatusInfoList.map(item => ({
  label: item.name,
  value: item.status,
}));

function RenderSelectedItem(optionNode: Record<string, unknown>) {
  const option = optionNode;
  const content = (
    <ExperimentItemRunStatus status={option.value as TurnRunState} />
  );
  return {
    isRenderInTag: false,
    content,
  };
}

/** 实验单个数据运行状态标签 */
export default function ExperimentItemRunStatusSelect({
  value,
  onChange,
  onBlur,
  ...rest
}: {
  value?: TurnRunState[];
  onChange?: (value: TurnRunState[]) => void;
  onBlur?: () => void;
} & SelectProps) {
  return (
    <Select
      prefix="状态"
      placeholder="请选择"
      multiple={true}
      showClear={true}
      maxTagCount={2}
      optionList={statusOptions}
      renderSelectedItem={RenderSelectedItem}
      {...rest}
      value={value}
      onChange={val => {
        onChange?.(val as TurnRunState[]);
      }}
      onBlur={() => onBlur?.()}
    />
  );
}
