import { type ComponentType } from 'react';

import { type prompt } from '@cozeloop/api-schema/prompt';
import {
  type EvaluationSetVersion,
  type EvaluationSet,
  type SubmitExperimentRequest,
  type EvalTarget,
  type EvalTargetVersion,
  type Evaluator,
  type EvaluatorVersion,
  type EvalTargetType,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { type RuleItem, type SelectProps } from '@coze-arch/coze-design';

import { type CozeTagColor } from '.';

export type SchemaSourceType = 'set' | 'target';

export type OptionSchema = FieldSchema & {
  schemaSourceType: SchemaSourceType;
};

export interface EvaluatorPro {
  evaluator?: Evaluator;
  evaluatorVersion?: EvaluatorVersion;
  evaluatorVersionDetail?: EvaluatorVersion;
  // key: 评估器字段名，value: 评测目标字段名
  evaluatorMapping?: Record<string, OptionSchema>;
}

export interface CreateExperimentValues extends SubmitExperimentRequest {
  // 评测对象版本详情
  evalTargetVersionDetail?: EvalTargetVersion & {
    inputs?: unknown;
    outputs?: unknown;
    end_type?: number;
  };

  evaluationSet?: string;
  // 评测集详情
  evaluationSetDetail?: EvaluationSet;
  // 评测集版本
  evaluationSetVersion?: string;
  // 评测集版本详情
  evaluationSetVersionDetail?: EvaluationSetVersion;

  evalTargetType?: EvalTargetType | string | number;
  evalTarget?: string;
  evalTargetVersion?: string;
  // evalTargetVersion?: EvalTargetVersion;
  promptDetail?: prompt.Prompt;
  // key: 评测集字段名，value: 评测目标字段名
  evalTargetMapping?: Record<string, OptionSchema>;

  evaluatorProList?: EvaluatorPro[];
}

export type BaseInfoValues = Pick<CreateExperimentValues, 'name' | 'desc'>;

export type EvaluateSetValues = Pick<
  CreateExperimentValues,
  | 'eval_set_id'
  | 'eval_set_version_id'
  | 'evaluationSet'
  | 'evaluationSetVersion'
  | 'evaluationSetVersionDetail'
>;

export type EvaluatorValues = Pick<
  CreateExperimentValues,
  'evaluator_version_ids' | 'evaluator_field_mapping' | 'evaluatorProList'
>;

export type CommonFormRef = {
  validate?: () => Promise<
    BaseInfoValues | EvaluateSetValues | EvaluateTargetValues | EvaluatorValues
  >;
  getFormApi?: () => { getValues: () => CreateExperimentValues };
} | null;

export type EvaluateTargetValues = Pick<
  CreateExperimentValues,
  | 'create_eval_target_param'
  | 'target_field_mapping'
  | 'evalTargetType'
  | 'evalTarget'
  | 'evalTargetVersion'
  | 'promptDetail'
  | 'evalTargetMapping'
>;

export interface PluginEvalTargetFormProps {
  /**表单数据 */
  formValues: CreateExperimentValues;
  /** 表单数据变化 */
  onChange: (key: keyof CreateExperimentValues, value: unknown) => void;
  /** 创建实验数据, 用于渲染 */
  createExperimentValues: CreateExperimentValues;
  /** 设置创建实验数据 */
  setCreateExperimentValues?: React.Dispatch<
    React.SetStateAction<CreateExperimentValues>
  >;
}

/**
 * 实验创建步骤枚举
 */
export enum ExtCreateStep {
  /** 基础信息 */
  BASE_INFO = 0,
  /** 评测集 */
  EVAL_SET = 1,
  /** 评测对象 */
  EVAL_TARGET = 2,
  /** 评估器 */
  EVALUATOR = 3,
  /** 实验创建 */
  CREATE_EXPERIMENT = 4,
}

/**
 * 评测对象信息
 */
export interface EvalTargetInfo {
  name?: string;
  type?: EvalTargetType;
  color?: string;
  tagColor?: CozeTagColor;
  icon?: React.ReactNode;
}

/** 步骤校验字段, 用于校验表单数据 */
export type ExtraValidFields = {
  [key in ExtCreateStep]?:
    | string[]
    | ((values: CreateExperimentValues) => string[]);
};

export interface EvalTargetDefinition {
  type: EvalTargetType;
  name: string;
  /** 评测对象描述 */
  description?: string;
  disableListFilter?: boolean;
  /** 评测对象来源 */
  evalTargetSource?: string;
  /** 评测对象信息 */
  targetInfo?: EvalTargetInfo;
  /** 评测对象下拉框选择器 */
  selector?: ComponentType<
    SelectProps & {
      /** 选项仅显示名称，不显示头像、描述等复杂信息 */
      onlyShowOptionName?: boolean;
    }
  >;
  /** 评测对象预览器 */
  preview?: ComponentType<{
    /** 评测对象 */
    evalTarget: EvalTarget;
    /** spaceID */
    spaceID: Int64;
    /** 评测集 */
    evalSet?: EvaluationSet;
    /** 是否显示打开详情链接 */
    enableLinkJump?: boolean;
    /** 尺寸 */
    size?: 'small' | 'medium';
    /** 跳转按钮类名 */
    jumpBtnClassName?: string;
    /** 是否显示图标, 目前 prompt 类型评测对象预览器在实验列表需要显示图标 */
    showIcon?: boolean;
  }>;
  /** View Submit 预览器中字段映射部分 */
  viewSubmitFieldMappingPreview?: ComponentType<{
    /** 渲染数据 */
    createExperimentValues: CreateExperimentValues;
  }>;
  /**
   * 评测对象表单插槽内容
   */
  evalTargetFormSlotContent?: (
    props: PluginEvalTargetFormProps,
  ) => React.JSX.Element | React.ReactNode;
  getEvalTargetVersionOption?: (item: EvalTargetVersion) => {
    value?: string;
    label?: React.ReactNode;
  };
  /**
   * 评测对象预览
   */
  evalTargetView?: (props: {
    /** 渲染数据 */
    values: CreateExperimentValues;
    /** 表单数据 */
    formValues: CreateExperimentValues;
  }) => React.JSX.Element;
  /**
   * 对应表单步骤, 需要额外校验的字段
   */
  extraValidFields?: ExtraValidFields;
  /**
   * 转换创建实验数据
   */
  transformCreateValues?: (
    values: CreateExperimentValues,
  ) => CreateExperimentValues;
  /**
   * 获取评测对象的输出字段定义
   */
  transformEvaluatorEvalTargetSchemas?: (
    evalTargetVersionDetail: CreateExperimentValues['evalTargetVersionDetail'],
  ) => FieldSchema[];
  evaluator?: {
    getEvaluatorMappingFieldRules?: (k: FieldSchema) => RuleItem[];
  };
  /**
   * 自定义复制实验初始化数据转换, 可能需要拉取额外的数据
   */
  transformCopyExperimentValues?: (
    values: CreateExperimentValues,
  ) => Promise<CreateExperimentValues>;
  /**
   * 获取初始化数据
   */
  getInitData?: (spaceID: string) => Promise<CreateExperimentValues>;
}

export interface IKeySchema {
  name: string;
  type: string;
  required?: boolean;
  schema?: IKeySchema[];
  input?: unknown;
}
