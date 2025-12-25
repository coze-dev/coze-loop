// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { EvalTargetType } from '@cozeloop/api-schema/evaluation';
import { Select, type SelectProps } from '@coze-arch/coze-design';

import { useEvalTargetDefinition } from '../../../stores/eval-target-store';

export interface EvalTargetCascadeSelectValue {
  type: EvalTargetType;
  evalTargetId: Int64[] | Int64;
}

export function EvalTargetCascadeSelect({
  value,
  onChange,
  typeSelectProps,
  evalTargetSelectProps,
}: {
  value?: EvalTargetCascadeSelectValue | undefined;
  onChange?: (val: EvalTargetCascadeSelectValue) => void;
  typeSelectProps?: SelectProps;
  evalTargetSelectProps?: SelectProps & {
    /** 选项仅显示名称即可，简化显示渲染 */ onlyShowOptionName?: boolean;
  };
}) {
  const { getEvalTargetDefinitionList, getEvalTargetDefinition } =
    useEvalTargetDefinition();
  const evalTargetType = value?.type ?? EvalTargetType.CozeLoopPrompt;
  let evalTargetSelect: React.ReactNode = null;
  const EvalTargetSelect = getEvalTargetDefinition(evalTargetType)?.selector;

  const evalTargetOptions = getEvalTargetDefinitionList()
    .filter(info => info.selector)
    .map(info => ({
      label: info.name,
      value: info.type,
    }));

  if (EvalTargetSelect) {
    evalTargetSelect = (
      <EvalTargetSelect
        {...evalTargetSelectProps}
        value={value?.evalTargetId}
        onChange={newKeys => {
          onChange?.({
            type: value?.type ?? EvalTargetType.CozeLoopPrompt,
            evalTargetId: newKeys as Int64,
          });
        }}
      />
    );
  }

  return (
    <div className="flex items-center gap-2 overflow-hidden">
      <Select
        placeholder={I18n.t('evaluate_target_type')}
        showArrow={false}
        {...typeSelectProps}
        optionList={evalTargetOptions}
        value={evalTargetType}
        onChange={val => {
          onChange?.({
            type: val as EvalTargetType,
            evalTargetId: [],
          });
        }}
      />

      <div className="grow overflow-hidden">{evalTargetSelect}</div>
    </div>
  );
}
