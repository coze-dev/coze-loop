import { isObject } from 'lodash-es';
import { JsonViewer } from '@textea/json-viewer';
import { safeJsonParse } from '@cozeloop/toolkit';

import { PlainTextDatasetItemReadOnly } from '../string/plain-text/readonly';
import { jsonViewerConfig } from '../string/json/config';
import styles from '../string/index.module.less';
import { type DatasetItemProps } from '../../type';
export const ArrayDatasetItemReadOnly = (props: DatasetItemProps) => {
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
