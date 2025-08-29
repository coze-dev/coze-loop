import { type OperatorRenderProps } from '@cozeloop/components';
import { Select } from '@coze-arch/coze-design';

import { findFieldByPath } from './utils';
import {
  dataTypeList,
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
  const { expr, onExprChange, fields, disabled = false } = props;
  const field = findFieldByPath(fields, expr.left);
  const dataType = dataTypeList.find(item => item.type === field?.type);
  if (!field || !dataType) {
    return null;
  }
  const { disabledOperations = [], customOperations } = field;
  let options = (field.operatorProps?.operations ??
    dataType.operations ??
    []) as LogicOperation[];
  if (Array.isArray(customOperations)) {
    options = customOperations;
  } else if (disabledOperations.length > 0) {
    options = options.filter(item => !disabledOperations.includes(item.value));
  }

  console.log('OperatorRender', expr.operator, options);

  return (
    <div className="w-24">
      <Select
        placeholder="操作符"
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
