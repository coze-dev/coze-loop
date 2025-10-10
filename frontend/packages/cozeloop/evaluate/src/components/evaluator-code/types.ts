// start_aigc
import { type EvaluationSetItemTableData } from '@cozeloop/evaluate-components';
import {
  type FieldSchema,
  type EvaluatorOutputData,
  type FieldData,
} from '@cozeloop/api-schema/evaluation';
import { type Form } from '@coze-arch/coze-design';

import { type CodeEvaluatorLanguageFE } from '@/constants';

/**
 * 测试数据项类型
 */
export interface TestData {
  [key: string]: unknown;
}

export enum TestDataSource {
  Dataset = 'dataset',
  Custom = 'custom',
}

export interface TestDataConfig {
  source?: TestDataSource;
  setData?: TestData[];
  customData?: Record<string, unknown>;
  originSelectedData?: EvaluationSetItemTableData[];
}

export interface CodeEvaluatorValue {
  funcExecutor?: BaseFuncExecutorValue;
  testData?: TestDataConfig;
  runResults?: EvaluatorOutputData[];
}

export interface CodeEvaluatorConfigProps {
  value?: CodeEvaluatorValue;
  fieldPath?: string;
  onChange?: (value: CodeEvaluatorValue) => void;
  disabled?: boolean;
  debugLoading?: boolean;
  resultsClassName?: string;
  editorHeight?: string;
}

export interface BaseFuncExecutorValue {
  language?: CodeEvaluatorLanguageFE;
  code?: string;
}

export interface BaseFuncExecutorProps {
  value?: BaseFuncExecutorValue;
  onChange?: (value: BaseFuncExecutorValue) => void;
  disabled?: boolean;
  editorHeight?: string;
}

/**
 * 自定义数据编辑器组件接口
 */
export interface BaseDataSetConfigProps {
  /**
   * 是否禁用
   */
  disabled?: boolean;
  /**
   * 字段值
   */
  value?: TestDataConfig;
  onChange?: (value: TestDataConfig) => void;
}

export interface TrialOperationResultsProps {
  results?: EvaluatorOutputData[];
  loading?: boolean;
  className?: string;
}

export interface EditorGroupProps {
  // value?: CodeEvaluatorValue;
  fieldPath?: string;
  disabled?: boolean;
  editorHeight?: string;
}

type OnImportType = (
  data: TestDataItem[],
  originSelectedData?: EvaluationSetItemTableData[],
) => void;

// 新增：测试数据项接口
export interface TestDataItem {
  evaluate_dataset_fields?: Record<string, FieldData>;
  evaluate_target_output_fields?: Record<string, FieldData>;
  [key: string]: unknown;
}

// 新增：测试数据模态框相关接口
export interface TestDataModalProps {
  visible: boolean;
  setSelectedItems?: (items: EvaluationSetItemTableData[]) => void;
  onClose: () => void;
  onImport: OnImportType;
  prevCount?: number;
}

export interface ModalState {
  /* 表单数据 */
  evaluationSetId?: string;
  evaluationSetVersion?: string;
  evaluateTarget?: string;

  /* 渲染数据 */
  currentStep: 0 | 1 | 2;
  selectedItems?: Set<string>;
  mockSetData?: TestDataItem[];
}

// 新增：通用表格组件接口
export interface CommonTableProps {
  data: EvaluationSetItemTableData[];
  // data: TestDataItem[] | EvaluationSetItemTableData[];
  selectedItems?: Set<string>;
  onSelectionChange?: (selectedItems: Set<string>) => void;
  showActualOutput?: boolean;
  loading?: boolean;
  fieldSchemas?: FieldSchema[];
  supportMultiSelect?: boolean;
  // 分页相关参数
  pageSize?: number;
  defaultPageSize?: number;
  showSizeChanger?: boolean;
  pageSizeOptions?: number[];
  prevCount?: number;
}

// 新增：可折叠编辑器数组接口
export interface CollapsibleEditorArrayProps {
  data: TestDataItem[];
  onChange: (data: TestDataItem[]) => void;
}
// end_aigc

// 步骤组件属性接口
export interface StepOneEvaluateSetProps {
  fieldSchemas: FieldSchema[];
  setFieldSchemas: (data: FieldSchema[]) => void;
  formRef: React.RefObject<Form<ModalState>>;
  onNextStep: () => void;
  evaluationSetData: EvaluationSetItemTableData[];
  setEvaluationSetData: (data: EvaluationSetItemTableData[]) => void;
  onImport: OnImportType;
  prevCount?: number;
}

export interface StepTwoEvaluateTargetProps {
  fieldSchemas: FieldSchema[];
  setFieldSchemas: (data: FieldSchema[]) => void;
  formRef: React.RefObject<Form<ModalState>>;
  onPrevStep: () => void;
  onNextStep: () => void;
  evaluationSetData: EvaluationSetItemTableData[];
}

export interface StepThreeGenerateOutputProps {
  fieldSchemas: FieldSchema[];
  formRef: React.RefObject<Form<ModalState>>;
  onPrevStep: () => void;
  onImport: OnImportType;
  evaluationSetData: EvaluationSetItemTableData[];
  setEvaluationSetData: (data: EvaluationSetItemTableData[]) => void;
}

export interface IFormValues {
  name?: string;
  description?: string;
  config: CodeEvaluatorValue;
}
