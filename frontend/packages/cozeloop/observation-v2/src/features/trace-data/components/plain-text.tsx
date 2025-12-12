import { isObject } from 'lodash-es';
import cls from 'classnames';
import { JsonViewer, type JsonViewerProps } from '@textea/json-viewer';

import { getJsonViewConfig } from '@/features/trace-data/constants/json-view';

import styles from './index.module.less';

export const PlantText = ({ content }: { content: string }) => (
  <span className={cls(styles['view-string'], {})}>{content || '-'}</span>
);

export const renderPlainText = (
  content: string | object,
  config?: Partial<JsonViewerProps>,
) =>
  isObject(content) ? (
    <JsonViewer
      {...getJsonViewConfig({ enabledValuesTypes: ['previousResponseId'] })}
      {...(config ?? {})}
      value={content}
    />
  ) : (
    <PlantText content={content as string} />
  );

export const renderJsonContent = (
  content: string | object,
  config?: Partial<JsonViewerProps>,
) =>
  isObject(content) ? (
    <JsonViewer
      {...getJsonViewConfig({ enabledValuesTypes: ['previousResponseId'] })}
      {...(config ?? {})}
      value={content}
    />
  ) : (
    <PlantText content={content as string} />
  );
