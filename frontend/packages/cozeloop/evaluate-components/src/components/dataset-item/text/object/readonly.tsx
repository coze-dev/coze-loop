// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isObject } from 'lodash-es';
import { JsonViewer } from '@textea/json-viewer';
import { safeJsonParse } from '@cozeloop/toolkit';

import { PlainTextDatasetItemReadOnly } from '../string/plain-text/readonly';
import { jsonViewerConfig } from '../string/json/config';
import styles from '../string/index.module.less';
import { type DatasetItemProps } from '../../type';

export const ObjectDatasetItemReadOnly = (props: DatasetItemProps) => {
  const { fieldContent, displayFormat } = props;
  const jsonObject = safeJsonParse(fieldContent?.text || '');
  const isObjectData = isObject(jsonObject);
  const stringifyFieldContent = isObjectData
    ? { ...(fieldContent ?? {}), text: JSON.stringify(jsonObject) }
    : fieldContent;
  return isObjectData && displayFormat ? (
    <div className={styles['code-container-readonly']}>
      <JsonViewer {...jsonViewerConfig} value={jsonObject} />
    </div>
  ) : (
    <PlainTextDatasetItemReadOnly
      {...props}
      fieldContent={stringifyFieldContent}
    />
  );
};
