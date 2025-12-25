// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { InfoTooltip } from '@cozeloop/components';
import { FieldDisplayFormat } from '@cozeloop/api-schema/data';

import {
  ContentType,
  type ConvertFieldSchema,
  DataType,
} from '../dataset-item/type';

export interface IDatasetCreateForm {
  name?: string;
  columns?: ConvertFieldSchema[];
  description?: string;
}

export const DEFAULT_COLUMNS = [
  {
    name: 'input',
    content_type: ContentType.Text,
    type: DataType.String,
    default_display_format: FieldDisplayFormat.PlainText,
    description: I18n.t('evaluation_set_input_tips'),
    additionalProperties: false,
  },
  {
    name: 'reference_output',
    content_type: ContentType.Text,
    type: DataType.String,
    default_display_format: FieldDisplayFormat.PlainText,
    description: I18n.t('expected_ideal_output'),
    additionalProperties: false,
  },
];

export const DEFALUT_COZE_WORKFLOW_COLUMNS = [
  {
    name: 'parameter',
    content_type: ContentType.Text,
    type: DataType.Object,
    default_display_format: FieldDisplayFormat.JSON,
    description: I18n.t('evaluation_set_workflow_params_tips'),
    additionalProperties: false,
  },
  {
    name: 'bot_id',
    content_type: ContentType.Text,
    type: DataType.String,
    default_display_format: FieldDisplayFormat.PlainText,
    description: I18n.t('workflow_need_associated_coze_agent_id'),
    additionalProperties: false,
  },
  {
    name: 'ext',
    content_type: ContentType.Text,
    type: DataType.Object,
    default_display_format: FieldDisplayFormat.JSON,
    description: I18n.t('workflow_additional_fields'),
    additionalProperties: false,
  },
  {
    name: 'app_id',
    content_type: ContentType.Text,
    type: DataType.String,
    default_display_format: FieldDisplayFormat.PlainText,
    description: I18n.t('workflow_associated_coze_agent_id'),
    additionalProperties: false,
  },
  {
    name: 'reference_output',
    content_type: ContentType.Text,
    type: DataType.Object,
    default_display_format: FieldDisplayFormat.JSON,
    description: I18n.t('expected_ideal_output'),
    additionalProperties: false,
  },
];

export const DEFAULT_COLUMN_SCHEMA: ConvertFieldSchema = {
  name: '',
  content_type: ContentType.Text,
  type: DataType.String,
  default_display_format: FieldDisplayFormat.PlainText,
  additionalProperties: false,
};

export const DEFAULT_DATASET_CREATE_FORM: IDatasetCreateForm = {
  name: '',
  columns: DEFAULT_COLUMNS,
  description: '',
};
export const enum CreateTemplate {
  Default = 'default',
  CozeWorkflow = 'coze_workflow',
}

export const COLUMNS_MAP = {
  [CreateTemplate.Default]: DEFAULT_COLUMNS,
  [CreateTemplate.CozeWorkflow]: DEFALUT_COZE_WORKFLOW_COLUMNS,
};

export const CREATE_TEMPLATE_LIST = [
  {
    label: I18n.t('default'),
    value: CreateTemplate.Default,
    displayText: I18n.t('default'),
  },
  {
    label: (
      <div className="flex items-center gap-1">
        <span>{I18n.t('coze_workflow')}</span>
        <InfoTooltip content={I18n.t('adjust_the_columns_with_workflow_api')} />
      </div>
    ),

    value: CreateTemplate.CozeWorkflow,
    displayText: I18n.t('coze_workflow'),
  },
];
