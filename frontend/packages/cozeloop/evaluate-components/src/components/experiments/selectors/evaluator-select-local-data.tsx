import cls from 'classnames';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';
import { Select, type SelectProps } from '@coze-arch/coze-design';

import { EvaluatorPreview } from '../../previews/evaluator-preview';

import styles from './select-local-data.module.less';

export function EvaluatorSelectLocalData({
  evaluators,
  showVersion = true,
  className,
  ...props
}: SelectProps & { evaluators?: Evaluator[]; showVersion?: boolean }) {
  return (
    <Select
      prefix="评估器"
      placeholder="请选择评估器"
      {...props}
      className={cls(styles['render-selected-item'], className)}
      // semi 导出类型就是 Record<string, any>
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      renderSelectedItem={(optionNode: Record<string, any>) => ({
        isRenderInTag: true,
        content: (
          <span
            className="ml-1 overflow-hidden text-xs"
            style={{ maxWidth: 120 }}
          >
            <EvaluatorPreview
              evaluator={optionNode?.evaluator}
              className="text-xs"
              tagProps={{ size: 'mini' }}
            />
          </span>
        ),
      })}
      optionList={evaluators?.map(evaluator => ({
        label: (
          <span
            className="ml-1 overflow-hidden text-xs"
            style={{ maxWidth: 120 }}
          >
            {showVersion ? (
              <EvaluatorPreview
                evaluator={evaluator}
                className="w-full overflow-hidden"
              />
            ) : (
              evaluator?.name
            )}
          </span>
        ),
        value: evaluator?.current_version?.id ?? '',
        evaluator,
      }))}
    />
  );
}
