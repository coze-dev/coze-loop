// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode } from 'react';

import {
  type Role,
  type Message,
  type VariableDef,
} from '@cozeloop/api-schema/prompt';

import { type BasicPromptEditorProps } from '../basic-prompt-editor';

export type PromptMessage = Message & {
  id?: string;
  key?: string;
  optimize_key?: string;
};

type BasicEditorProps = Pick<
  BasicPromptEditorProps,
  | 'variables'
  | 'height'
  | 'minHeight'
  | 'maxHeight'
  | 'forbidJinjaHighlight'
  | 'forbidVariables'
  | 'linePlaceholder'
  | 'isGoTemplate'
  | 'customExtensions'
  | 'canSearch'
  | 'isJinja2Template'
>;

export interface PromptEditorProps extends BasicEditorProps {
  className?: string;
  message?: PromptMessage;
  dragBtnHidden?: boolean;
  messageTypeDisabled?: boolean;
  disabled?: boolean;
  isInDrag?: boolean;
  placeholder?: string;
  messageTypeList?: Array<{ label: string; value: Role }>;
  leftActionBtns?: ReactNode;
  rightActionBtns?: ReactNode;
  hideActionWrap?: boolean;
  isFullscreen?: boolean;
  placeholderRoleValue?: Role;
  onMessageChange?: (v: PromptMessage) => void;
  onMessageTypeChange?: (v: Role) => void;
  children?: ReactNode;
  modalVariableEnable?: boolean;
  modalVariableBtnHidden?: boolean;
  segmentEnable?: boolean;
  cozeLibrarys?: unknown[];
  onDelete?: (message?: PromptMessage) => void;
  onIsFullscreenChange?: (v: boolean) => void;
  snippetBtnHidden?: boolean;
  insertSnippetVariables?: (variables: VariableDef[]) => void;
}

export interface PromptDiffEditorProps extends PromptEditorProps {
  preMessage: PromptMessage;
}
