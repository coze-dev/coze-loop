// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { CodeEditor } from '@cozeloop/components';

import { useEditorLoading } from '@/components/dataset-item/use-editor-loading';
import styles from '@/components/dataset-item/text/string/index.module.less';

import { codeOptionsConfig } from '../../dataset-item/text/string/code/config';

interface ObjectCodeEditorProps {
  value: string;
  onChange?: (value: string) => void;
  disabled: boolean;
}

export const ObjectCodeEditor = ({
  value,
  onChange,
  disabled,
}: ObjectCodeEditorProps) => {
  const { LoadingNode, onEditorMount } = useEditorLoading();
  return (
    <div className={styles['code-editor']} style={{ height: 200 }}>
      {LoadingNode}
      <CodeEditor
        language={'json'}
        value={value}
        options={{
          readOnly: disabled,
          ...codeOptionsConfig,
        }}
        onMount={onEditorMount}
        theme="vs-dark"
        onChange={newValue => {
          onChange?.(newValue || '');
        }}
      />
    </div>
  );
};
