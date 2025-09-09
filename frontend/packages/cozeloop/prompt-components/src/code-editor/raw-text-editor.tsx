// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { forwardRef, useCallback, useEffect, useRef } from 'react';

import { EditorProvider } from '@coze-editor/editor/react';
import { type EditorAPI } from '@coze-editor/editor/preset-universal';

import { TextEditor } from './text-editor';

interface RawTextEditorProps {
  value: string;
  onChange?: (value?: string) => void;
  className?: string;
  readonly?: boolean;
  dataTestID?: string;
  placeholder?: string | HTMLElement;
  minHeight?: string | number;
}

export const BaseRawTextEditor = forwardRef<HTMLDivElement, RawTextEditorProps>(
  (props, ref) => {
    const { value, onChange, placeholder, className, minHeight, readonly } =
      props;

    const apiRef = useRef<EditorAPI | null>(null);

    const handleChange = useCallback(
      (e: { value: string }) => {
        if (typeof onChange === 'function') {
          onChange(e.value);
        }
      },
      [onChange],
    );

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

    return (
      <EditorProvider>
        <div ref={ref} className={className}>
          <TextEditor
            defaultValue={value ?? ''}
            onChange={handleChange}
            options={{
              placeholder,
              lineWrapping: true,
              minHeight,
              fontSize: 13,
              editable: !readonly,
              lineHeight: 20,
            }}
            didMount={api => (apiRef.current = api)}
          />
        </div>
      </EditorProvider>
    );
  },
);
