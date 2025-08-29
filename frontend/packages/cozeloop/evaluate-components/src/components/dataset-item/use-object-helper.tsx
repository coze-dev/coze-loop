import { useEffect, useRef } from 'react';

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
        格式化JSON
      </Typography.Text>
      <Divider layout="vertical" className="w-[1px] h-[14px]" />
      <Popconfirm
        title="自动补全字段将覆盖原有内容"
        content="自动补全数据结构内所有字段将覆盖原有内容。确认覆盖吗？"
        okText="确认"
        onConfirm={generateJSONObject}
        okButtonColor="yellow"
        cancelText="取消"
      >
        <div className="flex items-center gap-1">
          <Typography.Text link>字段补全</Typography.Text>
          <InfoIconTooltip tooltip="点击自动补全该数据结构内的所有字段"></InfoIconTooltip>
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
