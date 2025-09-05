import { type ExptStatus, FieldType } from '@cozeloop/api-schema/evaluation';

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
