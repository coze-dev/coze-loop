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
    description: '作为输入投递给评测对象',
    additionalProperties: false,
  },
  {
    name: 'reference_output',
    content_type: ContentType.Text,
    type: DataType.String,
    default_display_format: FieldDisplayFormat.PlainText,
    description: '预期理想输出，可作为评估时的参考标准',
    additionalProperties: false,
  },
];
export const DEFALUT_COZE_WORKFLOW_COLUMNS = [
  {
    name: 'parameter',
    content_type: ContentType.Text,
    type: DataType.Object,
    default_display_format: FieldDisplayFormat.JSON,
    description:
      '工作流开始节点的输入参数及取值，你可以在指定工作流的编排页面查看参数列表。',
    additionalProperties: false,
  },
  {
    name: 'bot_id',
    content_type: ContentType.Text,
    type: DataType.String,
    default_display_format: FieldDisplayFormat.PlainText,
    description: '工作流需要关联的 Coze 智能体 ID。',
    additionalProperties: false,
  },
  {
    name: 'ext',
    content_type: ContentType.Text,
    type: DataType.Object,
    default_display_format: FieldDisplayFormat.JSON,
    description: '用于指定工作流需要的一些额外的字段。',
    additionalProperties: false,
  },
  {
    name: 'app_id',
    content_type: ContentType.Text,
    type: DataType.String,
    default_display_format: FieldDisplayFormat.PlainText,
    description: '该工作流关联的应用的 ID。',
    additionalProperties: false,
  },
  {
    name: 'reference_output',
    content_type: ContentType.Text,
    type: DataType.Object,
    default_display_format: FieldDisplayFormat.JSON,
    description: '预期理想输出，可作为评估时的参考标准',
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
    label: '默认',
    value: CreateTemplate.Default,
    displayText: '默认',
  },
  {
    label: (
      <div className="flex items-center gap-1">
        <span>Coze 工作流</span>
        <InfoTooltip content="一键将评测集的列调整为兼容工作流执行 API 的数据格式。" />
      </div>
    ),
    value: CreateTemplate.CozeWorkflow,
    displayText: 'Coze 工作流',
  },
];
