// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef, type ReactNode } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { Button, Space } from '@coze-arch/coze-design';

import {
  BasicPromptDiffEditor,
  type BasicPromptDiffEditorRef,
} from '../basic-prompt-editor/diff';
import { DiffEditorLayout } from './diff-editor-layout';

const DEFAULT_EDITOR_HEIGHT = 400;

interface DiffEditorContainerProps {
  preValue?: string;
  currentValue?: string;
  preVersion?: ReactNode;
  currentVersion?: ReactNode;
  diffEditorHeight?: string | number;
  children?: ReactNode;
  editorAble?: boolean;
  className?: string;
  onChange?: (value: string) => void;
  showDiffNav?: boolean;
}

export function DiffEditorContainer({
  preValue,
  currentValue,
  preVersion,
  currentVersion,
  diffEditorHeight = DEFAULT_EDITOR_HEIGHT,
  children,
  editorAble,
  className,
  onChange,
  showDiffNav,
}: DiffEditorContainerProps) {
  const diffEditorRef = useRef<BasicPromptDiffEditorRef>(null);

  const prev = () => {
    diffEditorRef.current?.goToPreviousChunk?.();
  };

  const next = () => {
    diffEditorRef.current?.goToNextChunk?.();
  };

  return (
    <DiffEditorLayout
      className={className}
      preVersion={preVersion}
      currentVersion={currentVersion}
      diffEditorHeight={diffEditorHeight}
      currentHeaderExtraActions={
        showDiffNav ? (
          <Space>
            <Button size="mini" color="primary" onClick={prev}>
              {I18n.t('prompt_previous_diff')}
            </Button>
            <Button size="mini" color="primary" onClick={next}>
              {I18n.t('prompt_next_diff')}
            </Button>
          </Space>
        ) : null
      }
    >
      <BasicPromptDiffEditor
        oldValue={preValue || ''}
        newValue={currentValue || ''}
        editorAble={editorAble}
        onChange={onChange}
        ref={diffEditorRef}
      >
        {children}
      </BasicPromptDiffEditor>
    </DiffEditorLayout>
  );
}
