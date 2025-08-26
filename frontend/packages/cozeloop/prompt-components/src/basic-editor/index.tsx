// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { forwardRef, useImperativeHandle, useMemo, useRef } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type VariableDef } from '@cozeloop/api-schema/prompt';
import {
  EditorProvider,
  Renderer,
  Placeholder,
} from '@coze-editor/editor/react';
import preset from '@coze-editor/editor/preset-prompt';
import { EditorView } from '@codemirror/view';
import { type Extension } from '@codemirror/state';

import Variable from './extensions/variable';
import Validation from './extensions/validation';
import MarkdownHighlight from './extensions/markdown';
import LanguageSupport from './extensions/language-support';
import JinjaHighlight from './extensions/jinja';
import JinjaCompletion from './extensions/jinja-completion';
import JinjaValidation from './extensions/jinja-validation';
import { goExtension } from './extensions/go-template';
import { TemplateApi } from '@cozeloop/api-schema';

export interface PromptBasicEditorProps {
  defaultValue?: string;
  height?: number;
  minHeight?: number;
  maxHeight?: number;
  fontSize?: number;
  variables?: VariableDef[];
  forbidVariables?: boolean;
  linePlaceholder?: string;
  forbidJinjaHighlight?: boolean;
  readOnly?: boolean;
  customExtensions?: Extension[];
  autoScrollToBottom?: boolean;
  isGoTemplate?: boolean;
  enableJinja2Preview?: boolean;
  templateType?: 'normal' | 'jinja2';
  onTemplateValidation?: (isValid: boolean, error?: string) => void;
  onChange?: (value: string) => void;
  onBlur?: () => void;
  onFocus?: () => void;
  children?: React.ReactNode;
}

export interface PromptBasicEditorRef {
  setEditorValue: (value?: string) => void;
  insertText?: (text: string) => void;
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
      linePlaceholder = I18n.t('please_input_with_vars'),
      customExtensions,
      autoScrollToBottom,
      onBlur,
      isGoTemplate,
      enableJinja2Preview,
      templateType = 'normal',
      onTemplateValidation,
      onFocus,
      children,
    }: PromptBasicEditorProps,
    ref,
  ) => {
    const editorRef = useRef<any>(null);

    useImperativeHandle(ref, () => ({
      setEditorValue: (value?: string) => {
        const editor = editorRef.current;
        if (!editor) {
          return;
        }
        editor?.setValue?.(value);
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
    }));

    const newExtensions = useMemo(() => {
      const xExtensions = customExtensions || extensions;
      if (isGoTemplate) {
        return [...xExtensions, goExtension];
      }

      // 添加 Jinja2 支持
      return [
        ...xExtensions,
        JinjaCompletion(),
      ];
    }, [customExtensions, extensions, isGoTemplate]);

    // 添加模板验证逻辑
    useEffect(() => {
      if (templateType === 'jinja2' && defaultValue && onTemplateValidation) {
        const validateTemplate = async () => {
          try {
            const response = await TemplateApi.validateTemplate({
              template: defaultValue,
              template_type: templateType,
            });

            if (response.code === 200 && response.data) {
              onTemplateValidation(response.data.is_valid, response.data.error_message);
            } else {
              onTemplateValidation(false, response.msg || 'Validation failed');
            }
          } catch (error: any) {
            onTemplateValidation(false, error.message || 'Validation failed');
          }
        };

        // 延迟验证，避免频繁调用
        const timeoutId = setTimeout(validateTemplate, 1000);
        return () => clearTimeout(timeoutId);
      }
    }, [defaultValue, templateType, onTemplateValidation]);

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
        {!forbidVariables && <Variable variables={variables || []} />}

        <LanguageSupport />
        {/* Jinja 语法高亮 */}
        {!forbidJinjaHighlight && (
          <>
            <Validation />
            <JinjaHighlight />
            <JinjaValidation />
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
