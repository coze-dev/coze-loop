/* eslint-disable @typescript-eslint/naming-convention */
import { type ComponentType } from 'react';

// 这里对workspace内的依赖只能依赖 @cozeloop 命名空间下的包
import { PromptCreate } from '@cozeloop/prompt-components';
import { type prompt } from '@cozeloop/api-schema/prompt';
import { type Scenario, type Model } from '@cozeloop/api-schema/llm-manage';
import {
  type Experiment,
  type ModelConfig,
} from '@cozeloop/api-schema/evaluation';
import { type PopoverProps } from '@coze-arch/coze-design';

export interface ModelConfigEditorProps {
  value?: ModelConfig;
  onChange?: (value?: ModelConfig) => void;
  onModelChange?: (value?: Model) => void;
  /** 刷新模型数据 */
  refreshModelKey?: number;
  disabled?: boolean;
  popoverProps?: PopoverProps;
  /** 使用场景 */
  scenario?: Scenario;
  [k: string]: unknown;
}

export interface FetchPromptDetailParams {
  promptID: string;
  version: string;
  spaceID: string;
}
interface PromptCreateProps {
  visible: boolean;
  onCancel: () => void;
  onOk: (prompt: { id?: string | number }) => void;
}

export interface ExptExportDropdownButtonProps {
  experiment?: Experiment;
  iconPosition?: 'left' | 'right';
  [key: string]: unknown; // 允许传递其他 Button 相关属性
  onExportModalOpen?: () => void;
  onExportCSV?: () => void;
  onViewDownloadFiles?: () => void;
  source?: string;
}

interface EvaluateConfig {
  traceEvalTargetPlatformType: string | number;
  traceEvaluatorPlatformType: string | number;
  /** 在线评测Trace平台类型 */
  traceOnlineEvalPlatformType: string | number;
  // 不给默认值, 后面这部分不需要了, 返回值暂时写为 any 不影响
  customGetEvalTargetDetail?: (
    params: FetchPromptDetailParams,
  ) => Promise<prompt.Prompt>;
  PromptCreate: ComponentType<PromptCreateProps>;
  showCreateEvaluateSetTemplateSelect: boolean;
  /** 实验导出按钮组件 */
  /** 实验导出下拉按钮组件 */
  ExptExportDropdownButton?: ComponentType<ExptExportDropdownButtonProps>;
  TableExportActionButton?: ComponentType<{
    onClick?: () => void;
    disabled?: boolean;
  }>;
}

/** 全局配置 */
const config: EvaluateConfig = {
  traceEvalTargetPlatformType: '',
  traceEvaluatorPlatformType: '',
  traceOnlineEvalPlatformType: '',
  showCreateEvaluateSetTemplateSelect: false,
  PromptCreate,
  ExptExportDropdownButton: undefined,
  TableExportActionButton: undefined,
};

/** 评测全局配置 */
const globalEvalConfig = {
  /** Trace评测对象平台类型 */
  get traceEvalTargetPlatformType() {
    return config.traceEvalTargetPlatformType;
  },
  /** Trace评估器平台类型 */
  get traceEvaluatorPlatformType() {
    return config.traceEvaluatorPlatformType;
  },
  /** Trace在线评测平台类型 */
  get traceOnlineEvalPlatformType() {
    return config.traceOnlineEvalPlatformType;
  },
  /** 设置Trace平台类型 */
  setTracePlatformType({
    traceEvalTargetPlatformType,
    traceEvaluatorPlatformType,
    traceOnlineEvalPlatformType,
  }: {
    traceEvalTargetPlatformType: string | number;
    traceEvaluatorPlatformType: string | number;
    traceOnlineEvalPlatformType?: string | number;
  }) {
    config.traceEvalTargetPlatformType = traceEvalTargetPlatformType;
    config.traceEvaluatorPlatformType = traceEvaluatorPlatformType;
    config.traceOnlineEvalPlatformType = traceOnlineEvalPlatformType ?? '';
  },

  /**
   * 自定义 评测对象详情 数据获取
   */
  get customGetEvalTargetDetail() {
    return config.customGetEvalTargetDetail;
  },
  /** 设置自定义 评测对象详情 数据获取 */
  setCustomGetEvalTargetDetail(
    fetch: (params: FetchPromptDetailParams) => Promise<prompt.Prompt>,
  ) {
    config.customGetEvalTargetDetail = fetch;
  },
  /** 创建Prompt组件 */
  get PromptCreate() {
    return config.PromptCreate;
  },
  /** 设置模型配置编辑器 */
  setPromptCreate(editor: ComponentType<PromptCreateProps>) {
    config.PromptCreate = editor;
  },
  /** 是否支持创建评测集进行模版切换 */
  get showCreateEvaluateSetTemplateSelect() {
    return config.showCreateEvaluateSetTemplateSelect;
  },
  /** 设置是否支持创建评测集进行模版切换 */
  setShowCreateEvaluateSetTemplateSelect(show: boolean) {
    config.showCreateEvaluateSetTemplateSelect = show;
  },

  /** 实验导出下拉按钮组件 */
  get ExptExportDropdownButton() {
    return config.ExptExportDropdownButton;
  },
  /** 设置实验导出下拉按钮组件 */
  setExptExportDropdownButton(
    component: ComponentType<ExptExportDropdownButtonProps>,
  ) {
    config.ExptExportDropdownButton = component;
  },
  /** 实验列表导出按钮组件 */
  get TableExportActionButton() {
    return config.TableExportActionButton;
  },
  /** 设置实验列表导出按钮组件 */
  setTableExportActionButton(
    component: ComponentType<{
      onClick?: () => void;
      disabled?: boolean;
    }>,
  ) {
    config.TableExportActionButton = component;
  },
};

export function useGlobalEvalConfig() {
  return globalEvalConfig;
}
