// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable security/detect-non-literal-regexp */
import React, {
  useCallback,
  useEffect,
  useRef,
  useImperativeHandle,
} from 'react';

import { EditorProvider } from '@coze-editor/editor/react';
import {
  type EditorAPI,
  transformerCreator,
} from '@coze-editor/editor/preset-code';
import { json } from '@coze-editor/editor/language-json';
import { EditorView } from '@codemirror/view';

import { CodeEditor } from './code-editor';

interface BaseJsonEditorProps {
  value: string;
  onChange?: (value: string) => void;
  className?: string;
  dataTestID?: string;
  placeholder?: string | HTMLElement;
  isDarkTheme?: boolean;
  readonly?: boolean;
  minHeight?: string | number;
  maxHeight?: string | number;
  editerHeight?: string | number;
  padding?: string | number;
  borderRadius?: string | number;
  onFocus?: () => void;
  onBlur?: () => void;
}

interface Match {
  match: string;
  range: [number, number];
}

const extensions = [
  EditorView.theme({
    '.cm-activeLineGutter': {
      backgroundColor: 'transparent !important',
    },
    '.cm-activeLine': {
      backgroundColor: 'transparent !important',
    },
  }),
];

function findAllMatches(inputString: string, regex: RegExp): Match[] {
  const globalRegex = new RegExp(
    regex,
    regex.flags.includes('g') ? regex.flags : `${regex.flags}g`,
  );
  let match;
  const matches: Match[] = [];

  while (true) {
    match = globalRegex.exec(inputString);
    if (!match) {
      break;
    }

    if (match.index === globalRegex.lastIndex) {
      globalRegex.lastIndex++;
    }
    matches.push({
      match: match[0],
      range: [match.index, match.index + match[0].length],
    });
  }

  return matches;
}

const transformer = transformerCreator(text => {
  const originalSource = text.toString();
  const matches = findAllMatches(originalSource, /\{\{([^\}]*)\}\}/g);

  if (matches.length > 0) {
    matches.forEach(({ range }) => {
      text.replaceRange(range[0], range[1], 'null');
    });
  }

  return text;
});

export const BaseJsonEditor = React.forwardRef(
  (props: BaseJsonEditorProps, ref) => {
    const {
      value,
      onChange,
      placeholder,
      className,
      isDarkTheme,
      readonly,
      minHeight = '100px',
      maxHeight,
      editerHeight,
      padding,
      borderRadius,
      onFocus,
      onBlur,
    } = props;

    const apiRef = useRef<EditorAPI | null>(null);

    const handleChange = useCallback(
      (e: { value: string }) => {
        if (typeof onChange === 'function') {
          onChange(e.value);
        }
      },
      [onChange],
    );

    useEffect(() => {
      apiRef.current?.updateASTDecorations();
    }, [isDarkTheme]);

    // 值受控;
    useEffect(() => {
      const editor = apiRef.current;

      if (!editor) {
        return;
      }

      if (typeof value === 'string' && value !== editor.getValue()) {
        editor.setValue(value);
      }
    }, [value]);

    const formatJson = async () => {
      const view = apiRef.current?.$view;
      if (!view) {
        return;
      }
      view.dispatch(
        await json.languageService.format(view.state, {
          tabSize: 2,
        }),
      );
    };

    useImperativeHandle(ref, () => ({
      formatJson,
    }));

    return (
      <EditorProvider>
        <div className={className}>
          <CodeEditor
            defaultValue={value ?? ''}
            onChange={handleChange}
            options={{
              placeholder,
              lineWrapping: true,
              theme: isDarkTheme ? 'coze-dark' : 'coze-light',
              languageId: 'json',
              editable: !readonly,
              transformer,
              minHeight,
              maxHeight,
              editerHeight,
              borderRadius,
              padding,
              fontSize: 13,
              lineHeight: 20,
            }}
            didMount={api => (apiRef.current = api)}
            extensions={extensions}
            onFocus={onFocus}
            onBlur={onBlur}
          />
        </div>
      </EditorProvider>
    );
  },
);
