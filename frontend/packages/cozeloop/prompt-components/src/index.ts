export {
  PromptBasicEditor,
  type PromptBasicEditorProps,
  type PromptBasicEditorRef,
} from './basic-editor';

export {
  PromptDiffEditor,
  type PromptDiffEditorRef,
} from './basic-editor/diff';

export {
  BaseJsonEditor,
  BaseRawTextEditor,
  EditorProvider,
} from './code-editor';

export { SchemaEditor } from './schema-editor';

// 开源版模型选择器
export { PopoverModelConfigEditor } from './model-config-editor-community/popover-model-config-editor';
export { PopoverModelConfigEditorQuery } from './model-config-editor-community/popover-model-config-editor-query';
export { BasicModelConfigEditor } from './model-config-editor-community/basic-model-config-editor';
export { ModelSelectWithObject } from './model-select';

export { DevLayout } from './dev-layout';

export { getPlaceholderErrorContent } from './utils/prompt';

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

export { cunstomFacet } from './basic-editor/custom-facet';

export {
  insertFourSpaces,
  deleteMarkupBackward,
  insertNewlineContinueMarkup,
} from './basic-editor/extensions/keymap';

export { MermaidDiagram, type MermaidDiagramRef } from './mermaid-diagram';

export {
  PromptEditor,
  type PromptEditorProps,
  type PromptMessage,
} from './prompt-editor';
export { PromptCreate } from './prompt-create';

export {
  multimodalPartsToContent,
  splitMultimodalContent,
  getMultimodalVariableText,
} from './utils/prompt';
