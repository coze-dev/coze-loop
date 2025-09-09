// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, type FC } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { getTypeText } from '@cozeloop/evaluate-components';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { IconCozEmpty } from '@coze-arch/coze-design/icons';
import {
  EmptyState,
  Loading,
  withField,
  type CommonFieldProps,
} from '@coze-arch/coze-design';

import {
  type OptionSchema,
  type OptionGroup,
} from '@/components/mapping-item-field/types';
import { MappingItemField } from '@/components/mapping-item-field';

import emptyStyles from './empty-state.module.less';

export interface EvaluateTargetMappingProps {
  loading?: boolean;
  keySchemas?: FieldSchema[];
  evaluationSetSchemas?: FieldSchema[];
  prefixField: string;
  value?: Record<string, OptionSchema>;
  onChange?: (v?: Record<string, OptionSchema>) => void;
}

export const EvaluateTargetMappingField: FC<
  CommonFieldProps & EvaluateTargetMappingProps
> = withField(function ({
  loading,
  keySchemas,
  evaluationSetSchemas,
  prefixField,
  // value,
  // onChange,
  // ...props
}: EvaluateTargetMappingProps) {
  const optionGroups = useMemo(
    () =>
      evaluationSetSchemas
        ? [
            {
              schemaSourceType: 'set',
              children: evaluationSetSchemas?.map(s => ({
                ...s,
                schemaSourceType: 'set',
              })),
            } satisfies OptionGroup,
          ]
        : [],
    [evaluationSetSchemas],
  );
  if (loading) {
    return (
      <div className="h-[84px] w-full flex items-center justify-center">
        <Loading
          className="!w-full"
          size="large"
          label={I18n.t('loading_field_mapping')}
          loading={true}
        ></Loading>
      </div>
    );
  }

  if (!keySchemas) {
    return (
      <div className="h-[84px] w-full flex items-center justify-center">
        <EmptyState
          size="default"
          icon={<IconCozEmpty className="coz-fg-dim text-32px" />}
          title={I18n.t('no_data')}
          className={emptyStyles['empty-state']}
          // description="请选择评测对象和版本号后再查看"
        />
      </div>
    );
  }
  return (
    <div>
      {keySchemas?.map(k => (
        <MappingItemField
          key={k.name}
          noLabel
          field={`${prefixField}.${k.name}`}
          fieldClassName="!pt-0"
          keyTitle={I18n.t('evaluation_object')}
          keySchema={k}
          optionGroups={optionGroups}
          rules={[
            {
              validator: (_rule, v) => {
                if (!v) {
                  return new Error(I18n.t('please_select', { field: '' }));
                }
                if (getTypeText(v) !== getTypeText(k)) {
                  return new Error(I18n.t('selected_fields_inconsistent'));
                }
                return true;
              },
            },
          ]}
        />
      ))}
    </div>
  );
});
