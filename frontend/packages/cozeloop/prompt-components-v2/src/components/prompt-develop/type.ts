// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/naming-convention */
import { type Prompt, type VariableVal } from '@cozeloop/api-schema/prompt';
import { type ParamSchema, type Model } from '@cozeloop/api-schema/llm-manage';
import {
  type UserInfoDetail,
  type BusinessType,
} from '@cozeloop/api-schema/foundation';
import { type customRequestArgs } from '@coze-arch/coze-design';

import { type DebugMessage } from '@/store/use-mockdata-store';

import { type PromptMessage } from '../prompt-editor/type';
import { type ModelConfigWithName } from '../model-config-editor/utils';

interface TabConfig {
  key: string; // 唯一标识符，如 'observation' 或 'evaluation'
  title: string; // 页签显示的标题
  children: React.ReactNode; // 渲染页面的函数或直接传入 React 组件
}

interface UploadFileParams {
  file: File;
  fileType?: 'image' | 'object';
  onProgress?: customRequestArgs['onProgress'];
  onSuccess?: customRequestArgs['onSuccess'];
  onError?: customRequestArgs['onError'];
  spaceID: string;
  businessType?: BusinessType;
}
type I18nFunction = (
  key: string,
  options?: Record<string, unknown>,
  fallbackText?: string,
) => string;

export interface ButtonConfigProps {
  disabled?:
    | boolean
    | ((props: {
        prompt?: Prompt;
        [key: string]: unknown | undefined;
      }) => boolean);
  hidden?:
    | boolean
    | ((props: {
        prompt?: Prompt;
        [key: string]: unknown | undefined;
      }) => boolean);
  onClick?: (props: {
    prompt?: Prompt;
    [key: string]: unknown | undefined;
  }) => void;
  onSuccess?: (props: {
    prompt?: Prompt;
    [key: string]: unknown | undefined;
  }) => void;
}

// 基础属性
export interface BasePromptDevelopProps {
  bizID?: 'Fornax' | 'CozeLoop';
  spaceID: string;
  readonly?: boolean;
  mcpEnable?: boolean;
  activeTab?: string;
  extraTabs?: TabConfig[];
  tabsChange?: (tabKey: string) => void;
  wrapperClassName?: string; // 自定义页签容器的 className
  isPlayground?: boolean;
  isOneModeCompare?: boolean;
  // header 额外按钮
  renderHeaderButtons?: (
    currentButtons: React.ReactNode[],
    prompt?: Prompt,
  ) => React.ReactNode;
  // header 下拉菜单按钮
  renderExtraHeaderDropdown?: (prompt?: Prompt) => React.ReactNode;
  buttonConfig?: {
    deleteButton?: ButtonConfigProps;
    backButton?: ButtonConfigProps;
    editButton?: ButtonConfigProps;
    copyButton?: ButtonConfigProps;
    createButton?: ButtonConfigProps;
    submitButton?: ButtonConfigProps;
    viewCodeButton?: ButtonConfigProps;
    traceHistoryButton?: ButtonConfigProps;
    traceLogButton?: ButtonConfigProps;
    compareButton?: ButtonConfigProps;
    snippetJumpButton?: ButtonConfigProps;
    promptJumpButton?: ButtonConfigProps;
  };
  sendEvent?: (name: string, params: Record<string, unknown>) => void;
  uploadFile?: (params: UploadFileParams) => Promise<string>;
  renerTipBanner?: (prompt?: Prompt) => React.ReactNode;
  modelInfo?: {
    list?: Model[];
    loading?: boolean;
    refresh?: () => void;
    customModelFormItemsRender?: (props: {
      model: Model;
      paramSchemas: ParamSchema[];
    }) => React.ReactNode;
    getDefaultModelConfig?: (model: Model) => Record<string, unknown>;
    convertModelToModelConfig?: (model: Model) => ModelConfigWithName;
    renderExtraConfigDiff?: (props: {
      basModelConfig: ModelConfigWithName;
      currentModelConfig: ModelConfigWithName;
      addDiffItem: (
        key: string,
        baseValue?: string | number | undefined,
        currentValue?: string | number | undefined,
      ) => void;
    }) => void;
  };
  userInfo?: UserInfoDetail;
  canDiffEdit?: boolean;
  // 编辑器类型选择
  renderTemplateType?: (props: {
    prompt?: Prompt;
    streaming?: boolean;
  }) => React.ReactNode;
  // 编辑器左侧按钮
  renderEditorLeftActions?: (props: {
    message?: PromptMessage;
    prompt?: Prompt;
    messageList?: PromptMessage[];
  }) => React.ReactNode;
  // 编辑器右侧按钮
  renderEditorRightActions?: (props: {
    message?: PromptMessage;
    prompt?: Prompt;
    messageList?: PromptMessage[];
  }) => React.ReactNode;
  debugAreaConfig?: {
    hideRoleChange?: boolean; // 消息角色是否可以修改
    canAddMessage?: boolean;
    canEditMessageType?: boolean; // TXT MD 改变
    canEditMultiModalMessage?: boolean;
    emptyLogoUrl?: string;
    assistantLogoUrl?: string;
    systemLogoUrl?: string;
    renderMessageItem?: (props: {
      message?: DebugMessage;
      streaming?: boolean;
    }) => React.ReactNode;
    renderExtraToolActions?: (props: {
      message?: DebugMessage;
      lastMessage?: DebugMessage;
      prompt?: Prompt;
      variables?: VariableVal[];
      originalSystemPrompt?: PromptMessage;
    }) => React.ReactNode;
    canSignleRound?: boolean;
  };
  multiModalConfig?: {
    imageSupported?: boolean;
    videoSupported?: boolean;
    intranetUrlValidator?: (url: string) => boolean;
  };
  I18n?: { t: I18nFunction };
  onSubmitSuccess?: (props: {
    prompt?: Prompt;
    version?: string;
    totalReferenceCount?: number;
  }) => void;
  onPublishSuccess?: (props: { prompt?: Prompt }) => void;
  submitConfig?: {
    hideVersionLabel?: boolean;
  };
  hideSnippet?: boolean;
  wikiConfig?: {};
}

// Playground 模式属性
export interface PlaygroundModeProps extends BasePromptDevelopProps {
  isPlayground?: true;
}

// 开发模式属性（非 Playground）
export interface PromptDevelopProps extends BasePromptDevelopProps {
  promptID?: string;
  queryVersion?: string;
  onPromptLoaded?: (promptInfo?: Prompt) => void;
}

export interface PromptLayoutProps {
  getPromptLoading?: boolean;
  wrapperClassName?: string; // 自定义页签容器的 className
}
