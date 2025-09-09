// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
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
        placeholder: I18n.t('please_select_evaluate_target'),
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
        title: I18n.t('evaluation_set'),
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
        title: I18n.t('evaluation_object'),
        name: getLogicFieldName(FieldType.SourceTarget, 'eval_target'),
        type: 'options',
        setter: EvalTargetCascadeSelectSetter,
      },
      {
        title: I18n.t('evaluate_target_type'),
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
        title: I18n.t('evaluator'),
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
              title: I18n.t('creator'),
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
