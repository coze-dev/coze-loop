import { useState } from 'react';

import cn from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { CodeEditor, handleCopy } from '@cozeloop/components';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { Button, SemiSelect } from '@coze-arch/coze-design';

import styles from '../index.module.less';
import { useEditorLoading } from '../../../use-editor-loading';
import { type DatasetItemProps } from '../../../type';
import { codeOptionsConfig, languageList } from './config';

export const CodeDatasetItem = ({
  fieldContent,
  onChange,
  isEdit,
  className,
}: DatasetItemProps) => {
  const [language, setLanguage] = useState('java');
  const { LoadingNode, setLoading, loading } = useEditorLoading();
  return (
    <div
      className={cn(styles['code-container'], className)}
      style={loading ? { backgroundColor: 'white' } : {}}
    >
      {LoadingNode}
      <div className="flex items-center justify-between px-3  ">
        <SemiSelect
          zIndex={16000}
          size="small"
          className={styles.language}
          optionList={languageList.map(item => ({
            label: item,
            value: item,
          }))}
          value={language}
          onChange={value => {
            setLanguage(value as string);
          }}
        />
        <Button
          icon={<IconCozCopy />}
          onClick={() => {
            handleCopy(fieldContent?.text || '');
          }}
          className={styles.copy}
          color="primary"
          size="small"
        >
          {I18n.t('evaluate_copy')}
        </Button>
      </div>
      <div className="flex-1 rounded-[6px] py-3 bg-[#1e1e1e] overflow-hidden">
        <CodeEditor
          language={language}
          value={fieldContent?.text || ''}
          options={{
            readOnly: !isEdit,
            ...codeOptionsConfig,
          }}
          onMount={() => {
            setLoading(false);
          }}
          theme="vs-dark"
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
