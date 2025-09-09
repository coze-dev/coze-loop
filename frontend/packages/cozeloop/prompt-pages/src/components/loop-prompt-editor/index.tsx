// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react';

import { useShallow } from 'zustand/react/shallow';
import {
  type PromptBasicEditorRef,
  PromptEditor,
  type PromptEditorProps,
} from '@cozeloop/prompt-components';
import { EditorTools } from '@cozeloop/biz-components-adapter';
import { Role } from '@cozeloop/api-schema/prompt';

import { usePromptStore } from '@/store/use-prompt-store';

interface LoopPromptEditorProps extends PromptEditorProps<string> {
  onDelete?: (id?: Int64) => void;
  optimizeBtnHidden?: boolean;
  showDragBtn?: boolean;
  children?: React.ReactNode;
}

export const LoopPromptEditor = forwardRef<
  PromptBasicEditorRef,
  LoopPromptEditorProps
>(({ showDragBtn, messageTypeList, ...restProps }, ref) => {
  const { promptInfo } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
    })),
  );
  const editorRef = useRef<PromptBasicEditorRef>(null);

  useImperativeHandle(ref, () => ({
    setEditorValue: (value?: string) => {
      editorRef.current?.setEditorValue?.(value);
    },
    insertText: (text: string) => {
      editorRef.current?.insertText?.(text);
    },
    getEditor: () => editorRef.current?.getEditor?.() || null,
  }));

  useEffect(() => {
    if (!restProps.optimizeBtnHidden) {
      if (!window.optimizeEditorMap) {
        window.optimizeEditorMap = {};
      }
      window.optimizeEditorMap[
        restProps?.message?.key || restProps?.message?.id || ''
      ] = editorRef.current;
    }
  }, [restProps.optimizeBtnHidden]);

  return (
    <>
      <PromptEditor
        ref={editorRef}
        rightActionBtns={
          <EditorTools<Role>
            onDelete={restProps.onDelete}
            disabled={restProps.disabled}
            message={restProps.message as any}
            promptInfo={promptInfo}
            onMessageChange={restProps.onMessageChange}
            optimizeBtnHidden={restProps.optimizeBtnHidden}
          />
        }
        dragBtnHidden={!showDragBtn}
        messageTypeList={
          messageTypeList ?? [
            { label: 'System', value: Role.System },
            { label: 'Assistant', value: Role.Assistant },
            { label: 'User', value: Role.User },
            { label: 'Placeholder', value: Role.Placeholder },
          ]
        }
        {...restProps}
      >
        {restProps.children}
      </PromptEditor>
    </>
  );
});
