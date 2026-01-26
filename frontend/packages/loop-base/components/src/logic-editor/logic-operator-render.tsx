// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Select } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { type OperatorRenderProps } from '../logic-expr';
import { findFieldByPath } from './utils';
import {
  useDataTypeList,
  type LogicOperation,
  type LogicFilterLeft,
  type RenderProps,
} from './logic-types';

export default function OperatorRender(
  props: OperatorRenderProps<
    LogicFilterLeft,
    string,
    string | number | undefined
  > &
    RenderProps,
) {
  const I18n = useI18n();
  const { expr, onExprChange, fields, disabled = false } = props;
  const field = findFieldByPath(fields, expr.left);
  const dataTypeList = useDataTypeList();
  const dataType = dataTypeList.find(item => item.type === field?.type);
  if (!field) {
    return null;
  }
  const { disabledOperations = [], customOperations } = field;
  let options = (field.operatorProps?.operations ??
    dataType?.operations ??
    []) as LogicOperation[];

  if (Array.isArray(customOperations)) {
    options = customOperations;
  } else if (disabledOperations.length > 0) {
    options = options.filter(item => !disabledOperations.includes(item.value));
  }

  return (
    <div className="w-24">
      <Select
        placeholder={I18n.t('operator')}
        value={expr.operator}
        style={{ width: '100%' }}
        disabled={disabled}
        optionList={options}
        onChange={val => {
          onExprChange?.({
            ...expr,
            operator: val as string | undefined,
          });
        }}
      />
    </div>
  );
}
