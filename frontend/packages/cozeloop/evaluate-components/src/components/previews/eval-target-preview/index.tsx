import {
  type EvaluationSet,
  type EvalTarget,
} from '@cozeloop/api-schema/evaluation';

import { useEvalTargetDefinition } from '@/stores/eval-target-store';

/** 评测对象预览 */
export function EvalTargetPreview(props: {
  evalTarget: EvalTarget | undefined;
  spaceID: Int64;
  evalSet?: EvaluationSet;
  enableLinkJump?: boolean;
  size?: 'small' | 'medium';
  jumpBtnClassName?: string;
  showIcon?: boolean;
}) {
  const {
    evalTarget,
    spaceID,
    enableLinkJump,
    size,
    jumpBtnClassName,
    showIcon,
    evalSet,
  } = props;
  const { getEvalTargetDefinition } = useEvalTargetDefinition();
  const { eval_target_type } = evalTarget ?? {};
  const target = getEvalTargetDefinition(eval_target_type ?? '');
  const Preview = target?.preview;
  if (evalTarget && Preview) {
    return (
      <Preview
        evalTarget={evalTarget}
        spaceID={spaceID}
        evalSet={evalSet}
        enableLinkJump={enableLinkJump}
        size={size}
        jumpBtnClassName={jumpBtnClassName}
        showIcon={showIcon}
      />
    );
  }
  return <>-</>;
}
