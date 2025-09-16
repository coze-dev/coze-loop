// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isObject } from 'lodash-es';
import cn from 'classnames';
import { JsonViewer } from '@textea/json-viewer';
import { safeJsonParse } from '@cozeloop/toolkit';
import { CodeEditor } from '@cozeloop/components';

import { PlainTextDatasetItemReadOnly } from '../plain-text/readonly';
import styles from '../index.module.less';
import { codeOptionsConfig } from '../code/config';
import { useEditorLoading } from '../../../use-editor-loading';
import { type DatasetItemProps } from '../../../type';
import { jsonViewerConfig } from './config';
export const JSONDatasetItem = (props: DatasetItemProps) => {
  const { fieldContent, onChange, isEdit, className } = props;
  const { LoadingNode, onEditorMount } = useEditorLoading();
  const jsonObject = safeJsonParse(fieldContent?.text || '');
  return isEdit ? (
    <div className={cn(styles['object-container'], className)}>
      {LoadingNode}
      <CodeEditor
        language={'json'}
        value={fieldContent?.text || ''}
        options={{
          readOnly: !isEdit,
          ...codeOptionsConfig,
        }}
        theme="vs-dark"
        onMount={onEditorMount}
        onChange={value => {
          onChange?.({
            ...fieldContent,
            text: value,
          });
        }}
      />
    </div>
  ) : isObject(jsonObject) ? (
    <div className={cn(styles['code-container-readonly'], className)}>
      <JsonViewer {...jsonViewerConfig} value={jsonObject} />
    </div>
  ) : (
    <PlainTextDatasetItemReadOnly {...props} />
  );
};
