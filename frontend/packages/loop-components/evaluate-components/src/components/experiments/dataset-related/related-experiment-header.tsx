// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ExptStatus, FieldType } from '@cozeloop/api-schema/evaluation';

import { ExperimentStatusSelect } from '../experiment-list-flter/experiment-status-select';
import { ExperimentNameSearch } from '../experiment-list-flter/experiment-name-search';
import { ExperimentEvaluatorLogicFilter } from '../experiment-list-flter/experiment-evaluator-logic-filter';
import { type LogicFilter } from '../../logic-editor';

export interface ChartConfigValues {
  evaluators?: Int64[];
  chartType?: 'line' | 'bar';
  chartVisible?: boolean;
}

export interface FilterValues {
  name?: string;
  status?: ExptStatus[];
  eval_set?: Int64[];
  eval_set_version?: Int64[];
}

export const filterFields: { key: keyof FilterValues; type: FieldType }[] = [
  {
    key: 'status',
    type: FieldType.ExptStatus,
  },
  {
    key: 'eval_set',
    type: FieldType.EvalSetID,
  },
];

export default function RelatedExperimentHeader({
  filter,
  onFilterChange,
  logicFilter,
  disabledFields,
  onLogicFilterChange,
  actions: moreActions,
}: {
  filter?: FilterValues;
  onFilterChange?: (name: keyof FilterValues, val: unknown) => void;
  logicFilter?: LogicFilter;
  onLogicFilterChange?: (val: LogicFilter | undefined) => void;
  actions?: React.ReactNode;
  disabledFields?: string[];
}) {
  const filters = (
    <>
      <ExperimentNameSearch
        value={filter?.name}
        onChange={val => onFilterChange?.('name', val)}
      />
      <ExperimentStatusSelect
        style={{ minWidth: 160 }}
        value={filter?.status}
        onChange={val => onFilterChange?.('status', val)}
      />
      <ExperimentEvaluatorLogicFilter
        disabledFields={disabledFields}
        value={logicFilter}
        onChange={onLogicFilterChange}
      />
    </>
  );
  const actions = <>{moreActions}</>;
  return (
    <div className="flex items-center gap-2">
      <div className="flex items-center gap-2">{filters}</div>
      <div className="w-0 ml-auto" />
      {actions}
    </div>
  );
}
