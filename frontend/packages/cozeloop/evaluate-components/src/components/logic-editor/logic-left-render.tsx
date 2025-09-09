import { I18n } from '@cozeloop/i18n-adapter';
import { type LeftRenderProps } from '@cozeloop/components';
import { Cascader, Select } from '@coze-arch/coze-design';

import { findFieldByPath } from './utils';
import {
  dataTypeList,
  type LogicOperation,
  type RenderProps,
  type LogicField,
  type LogicFilterLeft,
} from './logic-types';

function fieldsToOptions(fields: LogicField[]) {
  return fields.map(field => ({
    label: field.title,
    value: field.name,
    children: field.children?.length
      ? fieldsToOptions(field.children)
      : undefined,
  }));
}

export default function LeftRender(
  props: LeftRenderProps<LogicFilterLeft, string, string | number | undefined> &
    RenderProps,
) {
  const { expr, onExprChange, fields, disabled, enableCascadeMode } = props;
  if (enableCascadeMode) {
    return (
      <div className="w-56">
        <Cascader
          placeholder={I18n.t('please_select', { field: '' })}
          value={expr.left}
          className="w-full"
          disabled={disabled}
          treeData={fieldsToOptions(fields)}
          onChange={cascadeVal => {
            if (!Array.isArray(cascadeVal)) {
              return;
            }
            const fieldPaths = cascadeVal as string[];
            const field = findFieldByPath(fields, fieldPaths);
            const { disabledOperations = [], customOperations } = field ?? {};
            const dataType = dataTypeList.find(
              item => item.type === field?.type,
            );
            let operations = dataType?.operations ?? [];
            if (Array.isArray(customOperations)) {
              operations = customOperations;
            } else if (disabledOperations.length > 0) {
              operations = operations.filter(
                item => !disabledOperations.includes(item.value),
              );
            }
            onExprChange?.({
              left: [...fieldPaths],
              operator: operations?.find(e => e.value === expr.operator)
                ? expr.operator
                : operations?.[0]?.value,
              right: undefined,
            });
          }}
        />
      </div>
    );
  }
  return (
    <div className="w-40">
      <Select
        placeholder={I18n.t('please_select', { field: '' })}
        value={expr.left}
        className="w-full"
        disabled={disabled}
        filter={true}
        optionList={fieldsToOptions(fields)}
        onChange={val => {
          const field = fields.find(item => item.name === val);
          const { disabledOperations = [] } = field ?? {};
          const dataType = dataTypeList.find(item => item.type === field?.type);
          let operations = (field?.operatorProps?.operations ??
            dataType?.operations ??
            []) as LogicOperation[];

          if (disabledOperations.length > 0) {
            operations = operations.filter(
              item => !disabledOperations.includes(item.value),
            );
          }
          onExprChange?.({
            left: val as string | undefined,
            operator: operations?.find(e => e.value === expr.operator)
              ? expr.operator
              : operations?.[0]?.value,
            right: undefined,
          });
        }}
      />
    </div>
  );
}
