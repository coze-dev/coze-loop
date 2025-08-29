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
      title: '创建人',
      name: 'created_by',
      type: 'options',
      setterProps: {
        optionList: [
          { label: '张三', value: 1 },
          { label: '李四', value: 2 },
          { label: '王五', value: 3 },
        ],
      },
    },
    {
      title: '评测对象类型',
      name: 'eval_target_type',
      type: 'options',
      setterProps: {
        optionList: [
          { label: 'Prompt', value: 1 },
          { label: 'Coze 智能体', value: 2 },
        ],
      },
    },
    {
      title: '评测对象',
      name: 'eval_target',
      type: 'options',
      setterProps: {
        optionList: [
          { label: '百科达人', value: 1 },
          { label: '笑话大王', value: 2 },
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
