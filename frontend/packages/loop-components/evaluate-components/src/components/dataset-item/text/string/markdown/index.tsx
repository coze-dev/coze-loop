// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cn from 'classnames';
import { CodeEditor } from '@cozeloop/components';
import { MdBoxLazy } from '@coze-arch/bot-md-box-adapter/lazy';

import styles from '../index.module.less';
import { codeOptionsConfig } from '../code/config';
import { useEditorLoading } from '../../../use-editor-loading';
import { type DatasetItemProps } from '../../../type';

export const MarkdownDatasetItem = (props: DatasetItemProps) => {
  const { isEdit, fieldContent, onChange, className } = props;
  const { LoadingNode, onEditorMount } = useEditorLoading();
  return isEdit ? (
    <div className={cn(styles['object-container'], className)}>
      {LoadingNode}
      <CodeEditor
        language={'markdown'}
        value={fieldContent?.text || ''}
        options={{
          readOnly: !isEdit,
          ...codeOptionsConfig,
        }}
        theme="vs-dark"
        onChange={value => {
          onChange?.({
            ...fieldContent,
            text: value,
          });
        }}
        onMount={onEditorMount}
      />
    </div>
  ) : (
    <div className={cn(styles['code-container-readonly'], className)}>
      <MdBoxLazy
        className={styles.markdown}
        markDown={fieldContent?.text || ''}
        style={{
          fontSize: 12,
        }}
        imageOptions={{
          responsiveNaturalSize: {
            maxWidth: 120,
            maxHeight: 120,
          },
        }}
      />
    </div>
  );
};
