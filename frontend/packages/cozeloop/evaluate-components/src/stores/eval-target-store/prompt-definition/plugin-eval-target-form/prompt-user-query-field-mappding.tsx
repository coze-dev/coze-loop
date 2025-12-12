// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  ContentType,
  PromptUserQueryFieldKey,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';

import { type OptionGroup } from '@/components/mapping-item-field/types';
import { MappingItemField } from '@/components/mapping-item-field';
import { getSchemaTypeText, getTypeText } from '@/components/column-item-map';

import { userQueryKeySchema } from '../utils';

export function PromptUserQueryFieldMapping({
  evaluationSetSchemas,
  required = true,
}: {
  evaluationSetSchemas: FieldSchema[] | undefined;
  required?: boolean;
}) {
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
  return (
    <MappingItemField
      noLabel
      field={`evalTargetMapping.${PromptUserQueryFieldKey}`}
      fieldClassName="!pt-0"
      keyTitle={I18n.t('user_input')}
      keySchema={userQueryKeySchema}
      selectProps={{
        prefix: I18n.t('evaluation_set'),
      }}
      isRequired={required}
      optionGroups={optionGroups}
      rules={[
        {
          validator: (_rule, v, callback) => {
            if (!v && required) {
              callback(I18n.t('please_select'));
              return false;
            }
            if (
              getTypeText(v) &&
              getTypeText(v) !==
                getSchemaTypeText({ content_type: ContentType.Text })
            ) {
              callback(I18n.t('selected_fields_inconsistent'));
              return false;
            }
            return true;
          },
        },
      ]}
    />
  );
}
