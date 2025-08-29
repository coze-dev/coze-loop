import { useState } from 'react';

import jsonGenerator from 'to-json-schema';
import { CodeEditor } from '@cozeloop/components';
import {
  Button,
  Modal,
  Popconfirm,
  Toast,
  Typography,
  useFieldApi,
} from '@coze-arch/coze-design';

import { convertJSONSchemaToFieldObject } from '@/utils/jsonschema-convert';
import { validateJsonSchemaV7Strict } from '@/utils/field-convert';
import { useEditorLoading } from '@/components/dataset-item/use-editor-loading';
import {
  InputType,
  type ConvertFieldSchema,
} from '@/components/dataset-item/type';
import styles from '@/components/dataset-item/text/string/index.module.less';
import { codeOptionsConfig } from '@/components/dataset-item/text/string/code/config';
import { InfoIconTooltip } from '@/components/common/info-icon-tooltip';

export const useImportDataModal = (fieldKey: string, inputType: InputType) => {
  const [visible, setVisible] = useState(false);
  const fieldApi = useFieldApi(fieldKey);
  const onCancel = () => {
    setVisible(false);
  };
  const onModalSuccess = (value: Object) => {
    try {
      const fieldValue = fieldApi?.getValue() as ConvertFieldSchema;
      const options = {
        objects: {
          additionalProperties: false,
        },
      };
      const jsonSchema = jsonGenerator(value, options);
      const isValid = validateJsonSchemaV7Strict(jsonSchema);
      if (!isValid) {
        Toast.error('数据结构不符合要求，无法导入');
        return;
      }
      const schemaObject = convertJSONSchemaToFieldObject(jsonSchema);
      if (schemaObject?.type !== fieldValue?.type) {
        Toast.error('导入数据类型与列的数据类型不一致，无法导入');
        return;
      }
      fieldApi.setValue({
        ...fieldValue,
        ...(inputType === InputType.JSON
          ? { schema: JSON.stringify(jsonSchema, null, 2) }
          : { children: schemaObject?.children }),
      });
      setVisible(false);
    } catch (error) {
      Toast.error('样例数据格式错误');
    }
  };
  const triggerButton = (
    <div className="flex gap-1">
      <Typography.Text link onClick={() => setVisible(true)}>
        导入样例数据
      </Typography.Text>
      <InfoIconTooltip tooltip="基于样例数据自动提取数据结构"></InfoIconTooltip>
    </div>
  );

  const modalNode = visible ? (
    <CodeEditorModal onSuccess={onModalSuccess} onCancel={onCancel} />
  ) : null;

  return {
    triggerButton,
    modalNode,
  };
};

export const CodeEditorModal = ({
  onSuccess,
  onCancel,
}: {
  onSuccess: (value: Object) => void;
  onCancel: () => void;
}) => {
  const [value, setValue] = useState('');
  const { LoadingNode, onEditorMount } = useEditorLoading();
  return (
    <Modal
      title="样例数据"
      visible={true}
      width={960}
      onCancel={onCancel}
      footer={
        <div>
          <Button color="primary" onClick={onCancel}>
            取消
          </Button>
          <Popconfirm
            title="确认提取数据结构"
            content="提取数据结构将覆盖原有的字段定义"
            position="top"
            okText="确定"
            okButtonColor="yellow"
            showArrow
            cancelText="取消"
            onConfirm={() => {
              try {
                const obj = JSON.parse(value);
                onSuccess(obj);
              } catch (error) {
                Toast.error('样例数据格式错误');
                return;
              }
            }}
          >
            <Button color="brand">提取数据结构</Button>
          </Popconfirm>
        </div>
      }
    >
      <div className={styles['code-editor']} style={{ height: 460 }}>
        {LoadingNode}
        <CodeEditor
          language={'json'}
          onMount={onEditorMount}
          value={value}
          options={codeOptionsConfig}
          theme="vs-dark"
          onChange={newValue => {
            setValue(newValue || '');
          }}
        />
      </div>
    </Modal>
  );
};
