// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, useState } from 'react';

import { isEqual } from 'lodash-es';
import { useDebounceFn } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  COMMON_OUTPUT_FIELD_NAME,
  DEFAULT_TEXT_STRING_SCHEMA,
  useEvalTargetDefinition,
} from '@cozeloop/evaluate-components';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { IconCozPlusFill } from '@coze-arch/coze-design/icons';
import { useFormState, ArrayField, Button } from '@coze-arch/coze-design';

import {
  type EvaluatorPro,
  type CreateExperimentValues,
} from '@/types/experiment/experiment-create';

import { EvaluatorFieldItem } from '../../evaluator-field-item';

export interface EvaluatorFormProps {
  initValue: CreateExperimentValues['evaluatorProList'];
  evaluationSetVersionDetail: CreateExperimentValues['evaluationSetVersionDetail'];
  evalTargetVersionDetail: CreateExperimentValues['evalTargetVersionDetail'];
}

// 评测对象的输出字段定义
const evaluateTargetSchemas: FieldSchema[] = [
  {
    name: COMMON_OUTPUT_FIELD_NAME,
    description: I18n.t('actual_output'),
    ...DEFAULT_TEXT_STRING_SCHEMA,
  },
];

// 获取已选择的评估器版本ID列表
const getSelectedVersionIds = (evaluatorProList: EvaluatorPro[]) => {
  const list: string[] = [];
  evaluatorProList?.forEach(ep => {
    const versionId = ep?.evaluatorVersion?.id;
    if (versionId) {
      list.push(String(versionId));
    }
  });
  return list;
};

export const EvaluatorForm = (props: EvaluatorFormProps) => {
  const { initValue, evaluationSetVersionDetail, evalTargetVersionDetail } =
    props;

  const formState = useFormState();
  const formValues = formState.values as CreateExperimentValues;
  const evaluatorProList = formValues?.evaluatorProList || [];

  const [selectedVersionIds, setSelectedVersionIds] = useState(() =>
    getSelectedVersionIds(evaluatorProList),
  );

  const { getEvalTargetDefinition } = useEvalTargetDefinition();

  // 计算并更新已选择的评估器版本ID列表
  const calcSelectedVersionIds = useDebounceFn(
    (ls: EvaluatorPro[]) => {
      const newList = getSelectedVersionIds(ls);

      setSelectedVersionIds(pre => {
        if (isEqual(pre, newList)) {
          return pre;
        }
        return newList;
      });
    },
    {
      wait: 200,
    },
  );

  const targetDefinition = getEvalTargetDefinition(
    formValues?.evalTargetType as string,
  );

  const memoEvaluateTargetSchemas = useMemo(() => {
    // 没有选择评测对象, 就是使用了评测集作为评测对象, 直接返回空数组
    if (!formValues?.evalTargetType) {
      return [];
    }

    // 是否自定义评估器字段转换
    if (targetDefinition?.transformEvaluatorEvalTargetSchemas) {
      return targetDefinition.transformEvaluatorEvalTargetSchemas(
        evalTargetVersionDetail,
      );
    }
    // 若无自定义转换, 直接返回默认
    return evaluateTargetSchemas;
  }, [evalTargetVersionDetail, formValues?.evalTargetType]);

  // TODO: FIXME: @武文琦 这里的evaluatorProList副作用更新不及时 更新选中的版本ID
  calcSelectedVersionIds.run(evaluatorProList);
  // useEffect(() => {
  //   // 更新选中的版本ID
  //   calcSelectedVersionIds.run(evaluatorProList);
  // }, [evaluatorProList]);

  return (
    <div className="flex flex-col gap-5 mt-3">
      <ArrayField field="evaluatorProList" initValue={initValue || [{}]}>
        {({ addWithInitValue, arrayFields }) => (
          <>
            {arrayFields?.map((arrayField, index) => (
              <EvaluatorFieldItem
                key={arrayField.key + arrayField.field}
                arrayField={arrayField}
                index={index}
                evaluationSetSchemas={
                  evaluationSetVersionDetail?.evaluation_set_schema
                    ?.field_schemas
                }
                evaluateTargetSchemas={memoEvaluateTargetSchemas}
                selectedVersionIds={selectedVersionIds}
                getEvaluatorMappingFieldRules={
                  targetDefinition?.evaluator?.getEvaluatorMappingFieldRules
                }
              />
            ))}
            <Button
              block
              icon={<IconCozPlusFill />}
              color="primary"
              onClick={() => {
                addWithInitValue({});
              }}
              disabled={arrayFields.length >= 10}
            >
              {I18n.t('add_evaluator')}
            </Button>
          </>
        )}
      </ArrayField>
    </div>
  );
};
