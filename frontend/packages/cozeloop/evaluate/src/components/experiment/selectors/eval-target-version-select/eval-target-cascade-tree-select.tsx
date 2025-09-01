import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { EvalTargetType } from '@cozeloop/api-schema/evaluation';
import { Select } from '@coze-arch/coze-design';

import PromptEvalTargetTreeSelect from './prompt-eval-target-tree-select';
import CozeBotEvalTargetTreeSelect from './coze-bot-eval-target-tree-select';

interface EvalTargetSelectValue {
  evalTargetType: EvalTargetType;
  ids: string[];
}

export default function EvalTargetCascadeTreeSelect({
  value,
  onChange,
}: {
  value?: EvalTargetSelectValue | undefined;
  onChange?: (val: EvalTargetSelectValue) => void;
}) {
  const { spaceID } = useSpace();
  const evalTargetType = value?.evalTargetType ?? EvalTargetType.CozeLoopPrompt;
  let evalTargetSelect: React.ReactNode = null;
  if (evalTargetType === EvalTargetType.CozeLoopPrompt) {
    evalTargetSelect = (
      <PromptEvalTargetTreeSelect
        spaceID={spaceID}
        value={value?.ids}
        onChange={newKeys => {
          onChange?.({
            evalTargetType:
              value?.evalTargetType ?? EvalTargetType.CozeLoopPrompt,
            ids: newKeys,
          });
        }}
      />
    );
  } else if (evalTargetType === EvalTargetType.CozeBot) {
    evalTargetSelect = (
      <CozeBotEvalTargetTreeSelect
        spaceID={spaceID}
        value={value?.ids}
        onChange={newKeys => {
          onChange?.({
            evalTargetType:
              value?.evalTargetType ?? EvalTargetType.CozeLoopPrompt,
            ids: newKeys,
          });
        }}
      />
    );
  }
  return (
    <div className="flex items-center gap-1">
      <Select
        className="!w-24 shrink-0"
        placeholder="评测对象类型"
        value={evalTargetType}
        showArrow={false}
        onChange={val => {
          onChange?.({
            evalTargetType: val as EvalTargetType,
            ids: [],
          });
        }}
        optionList={[
          { label: 'Prompt', value: EvalTargetType.CozeLoopPrompt },
          { label: 'Coze 智能体', value: EvalTargetType.CozeBot },
        ]}
      />
      <div className="grow">{evalTargetSelect}</div>
    </div>
  );
}
