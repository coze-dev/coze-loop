// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { forwardRef, useImperativeHandle, useRef, type ReactNode } from 'react';

import {
  EditorProvider,
  goToNextChunk,
  goToPreviousChunk,
  MergeViewRenderer,
} from '@coze-editor/editor/react';
import preset from '@coze-editor/editor/preset-prompt';
import { EditorView } from '@codemirror/view';
import { type Extension } from '@codemirror/state';

import MarkdownHighlight from './extensions/markdown';
import LanguageSupport from './extensions/language-support';
import JinjaHighlight from './extensions/jinja';
import { cunstomFacet } from './custom-facet';

const extensions: Extension[] = [
  // diff theme
  EditorView.theme({
    '&.cm-merge-b .cm-changedLine': {
      background: 'transparent !important',
    },
    '&.cm-merge-b .cm-line': {
      paddingLeft: '12px !important',
    },
    '&.cm-merge-b .cm-changedText': {
      background: 'rgba(34,184,24,0.3)',
    },
    '&.cm-merge-a .cm-changedText, .cm-deletedChunk .cm-deletedText': {
      background: 'rgba(238,68,51,0.3)',
    },
  }),
];

export interface PromptDiffEditorRef {
  goToPreviousChunk: () => void;
  goToNextChunk?: () => void;
}
interface PromptDiffEditorProps {
  oldValue?: string;
  newValue?: string;
  autoScrollToBottom?: boolean;
  children?: ReactNode;
  editorAble?: boolean;
  onChange?: (value: string) => void;
}

export const PromptDiffEditor = forwardRef<
  PromptDiffEditorRef,
  PromptDiffEditorProps
>(
  (
    {
      oldValue,
      newValue,
      autoScrollToBottom,
      children,
      editorAble,
      onChange,
    }: PromptDiffEditorProps,
    ref,
  ) => {
    const editorRef = useRef<any>(null);

    useImperativeHandle(ref, () => ({
      goToPreviousChunk: () => {
        const view = editorRef.current?.$view;
        if (!view) {
          return;
        }
        goToPreviousChunk(view);
      },
      goToNextChunk: () => {
        const view = editorRef.current?.$view;
        if (!view) {
          return;
        }
        goToNextChunk(view);
      },
    }));

    return (
      <EditorProvider>
        <MergeViewRenderer
          plugins={preset}
          domProps={{
            style: {
              flex: 1,
              fontSize: 12,
              width: '100%',
            },
          }}
          mergeConfig={{
            gutter: true,
            collapseUnchanged: {
              margin: 3,
              minSize: 4,
            },
            diffConfig: {
              scanLimit: 3000,
            },
          }}
          a={{
            defaultValue: oldValue,
            extensions: [
              ...extensions,
              cunstomFacet.of({
                id: 'a',
                oldValue,
                newValue,
              }),
            ],
            options: {
              editable: false,
              readOnly: true,
            },
          }}
          b={{
            defaultValue: newValue,
            extensions: [
              ...extensions,
              cunstomFacet.of({
                id: 'b',
                oldValue,
                newValue,
              }),
            ],
            options: {
              editable: editorAble,
              readOnly: !editorAble,
            },
          }}
          didMount={editor => {
            if (autoScrollToBottom) {
              editor.b.$view.dispatch({
                effects: EditorView.scrollIntoView(
                  editor.b.$view.state.doc.length,
                ),
              });
            }
            if (onChange) {
              editor.b.$on('change', e => {
                onChange(e.value);
              });
            }
            editorRef.current = editor.a;
          }}
        />
        <LanguageSupport />
        <JinjaHighlight />
        <MarkdownHighlight />
        {children}
      </EditorProvider>
    );
  },
);
