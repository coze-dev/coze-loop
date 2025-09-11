import { CodeEditor } from '@cozeloop/components';

import styles from '../string/index.module.less';
import { codeOptionsConfig } from '../string/code/config';
import { useEditorObjectHelper } from '../../use-object-helper';
import { type DatasetItemProps } from '../../type';
export const ObjectDatasetItemEdit = (props: DatasetItemProps) => {
  const { fieldContent, onChange } = props;
  const { LoadingNode, onMount, HelperNode } = useEditorObjectHelper(props);

  return (
    <div className="relative">
      {HelperNode}
      <div className={styles['object-container']}>
        {LoadingNode}
        <CodeEditor
          language={'json'}
          value={fieldContent?.text || ''}
          options={{
            ...codeOptionsConfig,
          }}
          theme="vs-dark"
          onMount={onMount}
          onChange={value => {
            onChange?.({
              ...fieldContent,
              text: value,
            });
          }}
        />
      </div>
    </div>
  );
};
