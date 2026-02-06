// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import {
  LogicEditor,
  type LogicField,
  type LogicFilter,
} from '@cozeloop/evaluate-components';
import {
  type ExptStatus,
  FieldType,
  type Evaluator,
} from '@cozeloop/api-schema/evaluation';

export interface Filter {
  name?: string;
  eval_set?: Int64[];
  status?: ExptStatus[];
}

export const filterFields: { key: keyof Filter; type: FieldType }[] = [
  {
    key: 'status',
    type: FieldType.ExptStatus,
  },
  {
    key: 'eval_set',
    type: FieldType.EvalSetID,
  },
];

export default function ExperimentLogicFilter({
  logicFilter,
  evaluators = [],
  onChange,
  onClose,
}: {
  logicFilter: LogicFilter | undefined;
  evaluators: Evaluator[] | undefined;
  onChange: (newData?: LogicFilter) => void;
  onClose?: () => void;
}) {
  const logicFields: LogicField[] = [
    {
      title: I18n.t('creator'),
      name: 'created_by',
      type: 'options',
      setterProps: {
        optionList: [
          { label: I18n.t('user_zhangsan'), value: 1 },
          { label: I18n.t('user_lisi'), value: 2 },
          { label: I18n.t('user_wangwu'), value: 3 },
        ],
      },
    },
    {
      title: I18n.t('evaluate_target_type'),
      name: 'eval_target_type',
      type: 'options',
      setterProps: {
        optionList: [
          { label: 'Prompt', value: 1 },
          { label: I18n.t('coze_agent'), value: 2 },
        ],
      },
    },
    {
      title: I18n.t('evaluation_object'),
      name: 'eval_target',
      type: 'options',
      setterProps: {
        optionList: [
          { label: I18n.t('encyclopedia_expert'), value: 1 },
          { label: I18n.t('joke_king'), value: 2 },
        ],
      },
    },
    ...evaluators.map(evaluator => {
      const field: LogicField = {
        title: evaluator.name ?? '',
        name: `${evaluator.evaluator_id ?? ''}`,
        type: 'number' as const,
        setterProps: {
          step: 0.1,
        },
      };
      return field;
    }),
  ];

  return (
    <LogicEditor
      fields={logicFields}
      value={logicFilter}
      onChange={onChange}
      onClose={onClose}
    />
  );
}
