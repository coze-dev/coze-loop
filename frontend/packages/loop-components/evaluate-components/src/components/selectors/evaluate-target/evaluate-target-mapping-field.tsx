// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, type FC } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { IconCozEmpty } from '@coze-arch/coze-design/icons';
import {
  EmptyState,
  Loading,
  type SelectProps,
  withField,
  type CommonFieldProps,
} from '@coze-arch/coze-design';

import { type OptionGroup } from '../../../components/mapping-item-field/types';
import { MappingItemField } from '../../../components/mapping-item-field';
import {
  getSchemaTypeText,
  getTypeText,
} from '../../../components/column-item-map';

import emptyStyles from './empty-state.module.less';

export interface EvaluateTargetMappingProps {
  loading?: boolean;
  keySchemas?: (FieldSchema & { type?: string })[];
  prefixField: string;
  evaluationSetSchemas?: FieldSchema[];
  selectProps?: SelectProps;
  keyTitle?: string;
}

const EvaluateTargetMappingField: FC<
  CommonFieldProps & EvaluateTargetMappingProps
> = withField((props: EvaluateTargetMappingProps) => {
  const {
    loading,
    keySchemas,
    prefixField,
    evaluationSetSchemas,
    selectProps,
    keyTitle,
  } = props;

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

  if (!keySchemas) {
    return (
      <div className="h-[84px] w-full flex items-center justify-center">
        <EmptyState
          size="default"
          icon={<IconCozEmpty className="coz-fg-dim text-32px" />}
          title={I18n.t('no_data')}
          className={emptyStyles['empty-state']}
        />
      </div>
    );
  }
  return (
    <>
      <div className={loading ? 'hidden' : ''}>
        {keySchemas?.map(k => (
          <MappingItemField
            key={k.name}
            noLabel
            field={`${prefixField}.${k.name}`}
            fieldClassName="!pt-0"
            keyTitle={keyTitle ?? I18n.t('evaluation_object')}
            keySchema={k}
            optionGroups={optionGroups}
            selectProps={selectProps}
            rules={[
              {
                validator: (_rule, v, callback) => {
                  if (!v) {
                    callback(I18n.t('please_select'));
                    return false;
                  }
                  if (getTypeText(v) !== getSchemaTypeText(k)) {
                    callback(I18n.t('selected_fields_inconsistent'));
                    return false;
                  }
                  return true;
                },
              },
            ]}
          />
        ))}
      </div>
      {/* loading态不能直接返回咯爱的loading dom，不渲染MappingItemField会导致表单数据丢失 */}
      {loading ? (
        <div className="h-[84px] w-full flex items-center justify-center">
          <Loading
            className="!w-full"
            size="large"
            label={I18n.t('loading_field_mapping')}
            loading={true}
          />
        </div>
      ) : null}
    </>
  );
});
export default EvaluateTargetMappingField;
