// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export { PromptDevLayout } from './components/prompt-dev-layout';

// start: 编辑器相关导出，外部组件使用得保证code-mirror和coze-editor的实例一致
export {
  BasicPromptEditor,
  type BasicPromptEditorProps,
  type BasicPromptEditorRef,
} from './components/basic-prompt-editor';

export {
  BasicPromptDiffEditor,
  type BasicPromptDiffEditorRef,
} from './components/basic-prompt-editor/diff';

export { type EditorAPI } from '@coze-editor/editor/preset-code';

export { useEditor, useInjector, useLatest } from '@coze-editor/editor/react';

export { astDecorator, type SelectionEnlargerSpec } from '@coze-editor/editor';

export { type EditorAPI as EditorAPIPrompt } from '@coze-editor/editor/preset-prompt';

export { type SyntaxNode } from '@lezer/common';

import regexpDecorator, {
  updateRegexpDecorations,
} from '@coze-editor/extension-regexp-decorator';

export { regexpDecorator, updateRegexpDecorations };

export { Decoration, EditorView, WidgetType, keymap } from '@codemirror/view';

export { syntaxTree } from '@codemirror/language';

export {
  EditorSelection,
  type Extension,
  Prec,
  StateEffect,
  StateField,
  type EditorState,
} from '@codemirror/state';

export { cunstomFacet } from './components/basic-prompt-editor/custom-facet';

export {
  insertFourSpaces,
  deleteMarkupBackward,
  insertNewlineContinueMarkup,
} from './components/basic-prompt-editor/extensions/keymap';

export { PromptEditor } from './components/prompt-editor';
export type {
  PromptMessage,
  PromptEditorProps,
} from './components/prompt-editor/type';
// end: 编辑器相关导出

export { PromptCreateModal } from './components/prompt-create-modal';

// start: 模型配置相关
export { PopoverModelConfigEditor } from './components/model-config-editor/popover-model-config-editor';
export { BasicModelConfigEditor } from './components/model-config-editor/basic-model-config-editor';
export { ModelSelectWithObject } from './components/model-select';
export { type ModelItemProps } from './components/model-select/model-option';
export { ModelConfigForm } from './components/model-config-editor/model-config-form';
export { getInputSliderConfig } from './components/model-config-editor/utils';
export {
  type ModelConfigWithName,
  getDefaultModelConfig,
  convertModelToModelConfig,
} from './components/model-config-editor/utils';
// end: 模型配置相关

export {
  MermaidDiagram,
  type MermaidDiagramRef,
} from './components/mermaid-diagram';

export { PromptVersionSelect } from './components/prompt-version-select';

export { PromptTable } from './components/prompt-table';

export { promptDisplayColumns } from './components/prompt-list/column';
export { PromptList, type PromptTabKey } from './components/prompt-list';
export { PromptDeleteModal } from './components/prompt-delete-modal';

export { PromptDevelop } from './components/prompt-develop';
export { type PromptDevelopProps } from './components/prompt-develop/type';
export { showSubmitSuccess } from './components/prompt-submit/show-submit-success';

export { SnippetUseageModal } from './components/snippet-useage-modal';

// start:hooks
export { useVersionList } from './hooks/use-version-list';
export { usePrompt } from './hooks/use-prompt';
// end:hooks

// start: utils
export {
  getMultimodalVariableText,
  getPlaceholderErrorContent,
  splitMultimodalContent,
  multimodalPartsToContent,
  convertMultimodalMessage,
  convertMultimodalMessageToSend,
  getMockVariables,
  getToolNameList,
  nextVersion,
  versionValidate,
  setPromptStorageInfo,
  getPromptStorageInfo,
  getInputVariablesFromPrompt,
  getPlaceholderVariableKeys,
  getMultiModalVariableKeys,
} from './utils/prompt';
// end: utils

// start: store
export { usePromptStore } from './store/use-prompt-store';
export { usePromptMockDataStore } from './store/use-mockdata-store';
export { useBasicStore } from './store/use-basic-store';
// end: store
