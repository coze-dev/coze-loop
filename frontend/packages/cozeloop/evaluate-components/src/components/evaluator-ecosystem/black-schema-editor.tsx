// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { CodeEditor } from '@cozeloop/components';
import { Typography } from '@coze-arch/coze-design';

import styles from './black-schema-editor.module.less';

interface IProps {
  value: string;
  onChange: (value: string) => void;
  title: string;
  height?: string;
  disabled?: boolean;
}

export const BlackSchemaEditor: React.FC<IProps> = ({
  value,
  onChange,
  title,
  height = '100%',
  disabled = false,
}) => (
  <div className={styles['black-schema-editor']}>
    <div className={styles.header}>
      <Typography.Text className={styles.title}>{title}</Typography.Text>
    </div>
    <div className={styles['editor-container']}>
      <CodeEditor
        height={height}
        value={value}
        onChange={v => onChange(v || '')}
        language="json"
        options={{
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          wordWrap: 'on',
          fontSize: 12,
          lineNumbers: 'on',
          folding: true,
          automaticLayout: true,
          readOnly: disabled,
        }}
      />
    </div>
  </div>
);
