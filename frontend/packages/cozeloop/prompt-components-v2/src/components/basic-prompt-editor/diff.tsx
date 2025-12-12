/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-explicit-any */
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  type ReactNode,
} from 'react';

import { isUndefined } from 'lodash-es';
import {
  EditorProvider,
  goToNextChunk,
  goToPreviousChunk,
  MergeViewRenderer,
} from '@coze-editor/editor/react';
import preset from '@coze-editor/editor/preset-prompt';
import { IconCozEmpty } from '@coze-arch/coze-design/icons';
import { EditorView } from '@codemirror/view';
import { type Extension } from '@codemirror/state';

import Variable, { type VariableType } from './extensions/variable';
import Validation from './extensions/validation';
import { search } from './extensions/search';
import MarkdownHighlight from './extensions/markdown';
import LanguageSupport from './extensions/language-support';
import JinjaHighlight from './extensions/jinja';
import { goExtension } from './extensions/go-template';
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
    '&.cm-merge-a': {
      background: '#FCFCFF',
    },
  }),
];

export interface BasicPromptDiffEditorRef {
  goToPreviousChunk: () => void;
  goToNextChunk?: () => void;
  insertText?: (text: string) => void;
}
interface BasicPromptDiffEditorProps {
  canSearch?: boolean;
  oldValue?: string;
  newValue?: string;
  autoScrollToBottom?: boolean;
  children?: ReactNode;
  editorAble?: boolean;
  isGoTemplate?: boolean;
  isJinja2Template?: boolean;
  customExtensions?: Extension[];
  variables?: VariableType[];
  forbidVariables?: boolean;
  forbidJinjaHighlight?: boolean;
  onChange?: (value: string) => void;
  onBlur?: () => void;
  onFocus?: () => void;
}

export const BasicPromptDiffEditor = forwardRef<
  BasicPromptDiffEditorRef,
  BasicPromptDiffEditorProps
>(
  (
    {
      oldValue,
      newValue,
      autoScrollToBottom,
      children,
      editorAble,
      customExtensions = [],
      isGoTemplate,
      isJinja2Template,
      onChange,
      canSearch,
      variables,
      forbidVariables,
      forbidJinjaHighlight,
      onBlur,
      onFocus,
    }: BasicPromptDiffEditorProps,
    ref,
  ) => {
    const editorRef = useRef<any>(null);
    const editorBRef = useRef<any>(null);

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
      insertText: (text: string) => {
        editorBRef.current?.replaceText({
          from: editorBRef.current?.$view.state.selection.main.from,
          to: editorBRef.current?.$view.state.selection.main.to,
          text,
        });
      },
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
          editorBRef.current?.updateWholeDecorations();
        }, 200);
      }
    }, [variables, isJinja2Template, isGoTemplate]);

    return (
      <EditorProvider>
        <div className="relative w-full">
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
                ...newExtensions,
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
                ...newExtensions,
                cunstomFacet.of({
                  id: 'b',
                  oldValue,
                  newValue,
                }),
              ],
              options: {
                editable: editorAble,
                readOnly: !editorAble,
              } as any,
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
                editor.b.$on('focus', () => {
                  onFocus?.();
                });
                editor.b.$on('blur', () => {
                  onBlur?.();
                });
              }
              editorRef.current = editor.a;
              editorBRef.current = editor.b;
            }}
          />
          <LanguageSupport />
          {!forbidVariables && (
            <Variable variables={variables || []} isGoTemplate={isGoTemplate} />
          )}
          {!forbidJinjaHighlight && !isGoTemplate && (
            <>
              <Validation
                variables={variables}
                isNormalTemplate={!isJinja2Template && !isGoTemplate}
              />
              <JinjaHighlight isJinja2Template={isJinja2Template} />
            </>
          )}
          <MarkdownHighlight />
          {children}

          {isUndefined(oldValue) ? (
            <div className="absolute left-0 top-0 w-[50%] h-full z-2 bg-[#fcfcff] flex items-center justify-center">
              <IconCozEmpty className="coz-fg-dim" fontSize={20} />
            </div>
          ) : null}

          {isUndefined(newValue) ? (
            <div
              className="absolute right-0 top-0 h-full z-2 bg-white flex items-center justify-center"
              style={{ width: 'calc(50% - 8px)' }}
            >
              <IconCozEmpty className="coz-fg-dim" fontSize={20} />
            </div>
          ) : null}
        </div>
      </EditorProvider>
    );
  },
);
