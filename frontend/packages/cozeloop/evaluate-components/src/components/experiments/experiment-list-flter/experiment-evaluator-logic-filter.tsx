import { useMemo } from 'react';

import { GuardPoint, useGuard } from '@cozeloop/guard';
import { FieldType } from '@cozeloop/api-schema/evaluation';
import { type SelectProps } from '@coze-arch/coze-design';

import { useEvalTargetDefinition } from '@/stores/eval-target-store';
import { EvaluateSetSelect } from '@/components/selectors/evaluate-set-select';

import { EvaluatorSelect } from '../../selectors/evaluator-select';
import {
  EvalTargetCascadeSelect,
  type EvalTargetCascadeSelectValue,
} from '../../selectors/evaluate-target';
import LogicEditor, {
  type LogicFilter,
  type LogicField,
} from '../../logic-editor';
import { getLogicFieldName } from '../../../utils/evaluate-logic-condition';

function EvalTargetCascadeSelectSetter(props: SelectProps) {
  return (
    <EvalTargetCascadeSelect
      {...props}
      value={props.value as EvalTargetCascadeSelectValue}
      typeSelectProps={{
        className: '!w-24 shrink-0',
      }}
      evalTargetSelectProps={{
        className: 'w-full',
        multiple: true,
        maxTagCount: 1,
        onlyShowOptionName: true,
        filter: true,
        placeholder: '请选择评测对象',
      }}
    />
  );
}

export function ExperimentEvaluatorLogicFilter({
  value,
  disabledFields,
  onChange,
  onClose,
}: {
  value?: LogicFilter;
  disabledFields?: string[];
  onChange?: (newData?: LogicFilter) => void;
  onClose?: () => void;
}) {
  const { data: guardData } = useGuard({
    point: GuardPoint['eval.experiments.search_by_creator'],
  });

  const { getEvalTargetDefinitionList } = useEvalTargetDefinition();

  const evalTargetInfoList = getEvalTargetDefinitionList()
    ?.filter(item => !item.disableListFilter && item.targetInfo)
    .map(it => ({
      ...it.targetInfo,
      name: it.name,
      type: it.type,
    }));

  const filterFields = useMemo(() => {
    const newFilterFields: LogicField[] = [
      {
        title: '评测集',
        name: getLogicFieldName(FieldType.EvalSetID, 'eval_set'),
        type: 'options',
        setter: EvaluateSetSelect,
        setterProps: {
          className: 'w-full',
          multiple: true,
          maxTagCount: 1,
          onChangeWithObject: false,
        },
      },
      {
        title: '评测对象',
        name: getLogicFieldName(FieldType.SourceTarget, 'eval_target'),
        type: 'options',
        setter: EvalTargetCascadeSelectSetter,
      },
      {
        title: '评测对象类型',
        name: getLogicFieldName(FieldType.TargetType, 'eval_target_type'),
        type: 'options',
        setterProps: {
          optionList: evalTargetInfoList.map(({ name, type }) => ({
            label: name,
            value: type,
          })),
          multiple: true,
        },
      },
      {
        title: '评估器',
        name: getLogicFieldName(FieldType.EvaluatorID, 'evaluator'),
        type: 'options',
        setter: EvaluatorSelect,
        setterProps: {
          className: 'w-full',
          multiple: true,
          maxTagCount: 1,
        },
      },
      ...(!guardData.readonly
        ? [
            {
              title: '创建人',
              name: getLogicFieldName(FieldType.CreatorBy, 'create_by'),
              type: 'coze_user' as const,
            },
          ]
        : []),
    ];
    if (disabledFields?.length) {
      return newFilterFields.filter(
        field => !disabledFields.find(key => field.name.includes(key)),
      );
    }
    return newFilterFields;
  }, [disabledFields, guardData.readonly, evalTargetInfoList]);

  return (
    <LogicEditor
      fields={filterFields}
      value={value}
      onConfirm={newVal => onChange?.(newVal)}
      onClose={onClose}
    />
  );
}
