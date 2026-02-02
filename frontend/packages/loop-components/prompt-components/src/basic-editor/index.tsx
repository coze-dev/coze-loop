// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
} from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  EditorProvider,
  Renderer,
  Placeholder,
} from '@coze-editor/editor/react';
import preset, { type EditorAPI } from '@coze-editor/editor/preset-prompt';
import { EditorView, keymap } from '@codemirror/view';
import { Prec, type Extension } from '@codemirror/state';
import { indentWithTab } from '@codemirror/commands';

import Variable, { type VariableType } from './extensions/variable';
import Validation from './extensions/validation';
import { search } from './extensions/search';
import MarkdownHighlight from './extensions/markdown';
import LanguageSupport from './extensions/language-support';
import { insertFourSpaces } from './extensions/keymap';
import JinjaHighlight from './extensions/jinja';
import { goExtension } from './extensions/go-template';

export interface PromptBasicEditorProps {
  defaultValue?: string;
  height?: number;
  minHeight?: number;
  maxHeight?: number;
  fontSize?: number;
  /**
   * 变量
   * 只需要传入文本型变量，非文本型变量不能出现在快捷操作中
   */
  variables?: VariableType[];
  forbidVariables?: boolean;
  linePlaceholder?: string;
  forbidJinjaHighlight?: boolean;
  readOnly?: boolean;
  customExtensions?: Extension[];
  autoScrollToBottom?: boolean;
  isGoTemplate?: boolean;
  isJinja2Template?: boolean;
  onChange?: (value: string) => void;
  onBlur?: () => void;
  onFocus?: () => void;
  children?: React.ReactNode;
  canSearch?: boolean;
}

export interface PromptBasicEditorRef {
  setEditorValue: (value?: string) => void;
  insertText?: (text: string) => void;
  getEditor?: () => EditorAPI | null;
}

const extensions = [
  EditorView.theme({
    '.cm-gutters': {
      backgroundColor: 'transparent',
      borderRight: 'none',
    },
    '.cm-scroller': {
      paddingLeft: '10px',
      paddingRight: '6px !important',
    },
  }),
  Prec.high(keymap.of([{ key: 'Tab', run: insertFourSpaces }, indentWithTab])),
];

export const PromptBasicEditor = forwardRef<
  PromptBasicEditorRef,
  PromptBasicEditorProps
>(
  (
    {
      defaultValue,
      onChange,
      variables,
      height,
      minHeight,
      maxHeight,
      fontSize = 13,
      forbidJinjaHighlight,
      forbidVariables,
      readOnly,
      linePlaceholder = I18n.t('prompt_please_input_content_variable_format'),
      customExtensions = [],
      autoScrollToBottom,
      onBlur,
      isGoTemplate,
      onFocus,
      canSearch,
      children,
      isJinja2Template,
    }: PromptBasicEditorProps,
    ref,
  ) => {
    const editorRef = useRef<EditorAPI | null>(null);

    useImperativeHandle(ref, () => ({
      setEditorValue: (value?: string) => {
        const editor = editorRef.current;
        if (!editor) {
          return;
        }
        editor?.setValue?.(value || '');
      },
      insertText: (text: string) => {
        const editor = editorRef.current;
        if (!editor) {
          return;
        }
        const range = editor.getSelection();
        if (!range) {
          return;
        }
        editor.replaceText({
          ...range,
          text,
          cursorOffset: 0,
        });
      },
      getEditor: () => editorRef.current,
    }));

    const newExtensions = useMemo(() => {
      const xExtensions = [...extensions, ...customExtensions];
      const searchExt = canSearch ? [...search()] : [];
      if (isGoTemplate) {
        return [...xExtensions, goExtension, ...searchExt];
      }
      return [...xExtensions, ...searchExt];
    }, [customExtensions, extensions, isGoTemplate, canSearch]);

    useEffect(() => {
      if (isJinja2Template || isGoTemplate) {
        setTimeout(() => {
          editorRef.current?.updateWholeDecorations();
        }, 150);
      }
    }, [variables, isJinja2Template, isGoTemplate]);

    return (
      <EditorProvider>
        <Renderer
          plugins={preset}
          defaultValue={defaultValue}
          options={{
            editable: !readOnly,
            readOnly,
            height,
            minHeight: minHeight || height,
            maxHeight: maxHeight || height,
            fontSize,
          }}
          onChange={e => onChange?.(e.value)}
          onFocus={onFocus}
          onBlur={onBlur}
          extensions={newExtensions}
          didMount={editor => {
            editorRef.current = editor;
            if (autoScrollToBottom) {
              editor.$view.dispatch({
                effects: EditorView.scrollIntoView(
                  editor.$view.state.doc.length,
                ),
              });
            }
          }}
        />

        {/* 输入 { 唤起变量选择 */}
        {!forbidVariables && (
          <Variable variables={variables || []} isGoTemplate={isGoTemplate} />
        )}

        <LanguageSupport />
        {/* Jinja 语法高亮 */}
        {!forbidJinjaHighlight && !isGoTemplate && (
          <>
            <Validation
              variables={variables}
              isNormalTemplate={!isJinja2Template && !isGoTemplate}
            />

            <JinjaHighlight isJinja2Template={isJinja2Template} />
          </>
        )}

        {/* Markdown 语法高亮 */}
        <MarkdownHighlight />

        {/* 激活行为空时的占位提示 */}

        <Placeholder>{linePlaceholder}</Placeholder>
        {children}
      </EditorProvider>
    );
  },
);
