import { I18n } from '@cozeloop/i18n-adapter';

import { promptVariableDefToFieldSchema } from '@/utils/parse-prompt-variable';
import { type CreateExperimentValues } from '@/types/evaluate-target';
import { ReadonlyMappingItem } from '@/components/mapping-item-field/readonly-mapping-item';

import usePromptDetail from './plugin-eval-target-form/use-prompt-detail';

export function PromptFieldMappingPreview({
  createExperimentValues,
}: {
  /** 渲染数据 */
  createExperimentValues: CreateExperimentValues;
}) {
  const {
    evalTargetMapping,
    evalTarget = '',
    evalTargetVersion = '',
  } = createExperimentValues ?? {};
  const { promptDetail } = usePromptDetail({
    promptId: evalTarget as string,
    version: evalTargetVersion,
  });
  const variableDefs =
    promptDetail?.prompt_commit?.detail?.prompt_template?.variable_defs ?? [];
  const fieldSchemas = variableDefs.map(promptVariableDefToFieldSchema);
  return (
    <div className="flex flex-col gap-3">
      {fieldSchemas.map(fieldSchema => {
        const key = fieldSchema?.key ?? '';
        const optionSchema = evalTargetMapping?.[key];
        return (
          <ReadonlyMappingItem
            key={key}
            keyTitle={I18n.t('evaluation_object')}
            keySchema={fieldSchema}
            optionSchema={optionSchema}
          />
        );
      })}
    </div>
  );
}
