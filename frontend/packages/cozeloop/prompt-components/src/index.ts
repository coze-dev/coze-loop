// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export {
  PromptBasicEditor,
  PromptBasicEditorProps,
  PromptBasicEditorRef,
} from './basic-editor';

export { PromptDiffEditor } from './basic-editor/diff';

export {
  PromptEditor,
  PromptEditorProps,
  PromptMessage,
} from './prompt-editor';

// 新增 Jinja2 相关导出
export { TemplatePreview } from './template-preview';

// 开源版模型选择器
export { PopoverModelConfigEditor } from './model-config-editor-community/popover-model-config-editor';
export { PopoverModelConfigEditorQuery } from './model-config-editor-community/popover-model-config-editor-query';
export { BasicModelConfigEditor } from './model-config-editor-community/basic-model-config-editor';
export { ModelSelectWithObject } from './model-config-editor-community/model-select';

export { DevLayout } from './dev-layout';

export { PromptCreate } from './prompt-create';

export { getPlaceholderErrorContent } from './utils/prompt';

export {
  BaseJsonEditor,
  BaseRawTextEditor,
  EditorProvider,
} from './code-editor';

export { Decoration, EditorView, WidgetType, keymap } from '@codemirror/view';
export { EditorSelection } from '@codemirror/state';
export { type Extension, Prec } from '@codemirror/state';
