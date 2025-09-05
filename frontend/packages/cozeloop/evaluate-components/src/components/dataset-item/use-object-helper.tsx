import { useEffect, useRef } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { Divider, Popconfirm, Typography } from '@coze-arch/coze-design';

import { InfoIconTooltip } from '../common';
import { generateDefaultBySchema } from './util';
import { useEditorLoading } from './use-editor-loading';
import { type DatasetItemProps } from './type';

export const useEditorObjectHelper = (props: DatasetItemProps) => {
  const { fieldContent, onChange, fieldSchema } = props;
  const editorRef = useRef(null);
  const { LoadingNode, onEditorMount } = useEditorLoading();
  useEffect(() => {
    if (
      fieldContent?.text === undefined &&
      fieldSchema &&
      fieldSchema?.isRequired
    ) {
      const defaultValue = generateDefaultBySchema(fieldSchema);
      onChange?.({
        ...fieldContent,
        text: defaultValue,
      });
    }
  }, []);
  const onMount = editor => {
    editorRef.current = editor;
    // 监听monaco的粘贴命令（Command/Action方式，不是原生DOM事件！）
    editor.trigger('anyString', 'editor.action.formatDocument');
    editor.onDidPaste(() => {
      // 粘贴后自动格式化
      editor.trigger('paste', 'editor.action.formatDocument');
    });
    onEditorMount();
  };

  const parseJSONValue = () => {
    try {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (editorRef?.current as any)?.trigger(
        'anyString',
        'editor.action.formatDocument',
      );
    } catch (error) {
      console.error(error);
    }
  };
  const generateJSONObject = () => {
    if (fieldSchema) {
      const defaultValue = generateDefaultBySchema(fieldSchema, false);
      onChange?.({
        ...fieldContent,
        text: defaultValue,
      });
    }
  };

  const HelperNode = (
    <div className="flex gap-2 items-center absolute right-0 -top-[30px]">
      <Typography.Text link onClick={parseJSONValue}>
        {I18n.t('format_json')}
      </Typography.Text>
      <Divider layout="vertical" className="w-[1px] h-[14px]" />
      <Popconfirm
        title={I18n.t(
          'cozeloop_open_evaluate_autocomplete_overwrites_original',
        )}
        content={I18n.t(
          'cozeloop_open_evaluate_autocomplete_confirm_overwrite_all_fields',
        )}
        okText={I18n.t('confirm')}
        onConfirm={generateJSONObject}
        okButtonColor="yellow"
        cancelText={I18n.t('cancel')}
      >
        <div className="flex items-center gap-1">
          <Typography.Text link>{I18n.t('field_completion')}</Typography.Text>
          <InfoIconTooltip
            tooltip={I18n.t(
              'cozeloop_open_evaluate_click_autocomplete_all_fields',
            )}
          ></InfoIconTooltip>
        </div>
      </Popconfirm>
    </div>
  );
  return {
    LoadingNode,
    HelperNode,
    onMount,
  };
};
