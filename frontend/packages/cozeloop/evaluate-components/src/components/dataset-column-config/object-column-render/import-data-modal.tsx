import { useState } from 'react';

import jsonGenerator from 'to-json-schema';
import { I18n } from '@cozeloop/i18n-adapter';
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
        Toast.error(I18n.t('evaluation_set_import_error_data_structure_tips'));
        return;
      }
      const schemaObject = convertJSONSchemaToFieldObject(jsonSchema);
      if (schemaObject?.type !== fieldValue?.type) {
        Toast.error(I18n.t('evaluation_set_import_error_data_type_tips'));
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
      Toast.error(I18n.t('sample_data_format_error'));
    }
  };
  const triggerButton = (
    <div className="flex gap-1">
      <Typography.Text link onClick={() => setVisible(true)}>
        {I18n.t('import_sample_data')}
      </Typography.Text>
      <InfoIconTooltip
        tooltip={I18n.t(
          'automatic_extraction_of_data_structure_based_on_sample_data',
        )}
      ></InfoIconTooltip>
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
      title={I18n.t('evaluation_set_builtin_example_data')}
      visible={true}
      width={960}
      onCancel={onCancel}
      footer={
        <div>
          <Button color="primary" onClick={onCancel}>
            {I18n.t('cancel')}
          </Button>
          <Popconfirm
            title={I18n.t('confirm_the_extracted_data_structure')}
            content={I18n.t('extracting_the_data_structure_overwrite_tips')}
            position="top"
            okText={I18n.t('confirm')}
            okButtonColor="yellow"
            showArrow
            cancelText={I18n.t('cancel')}
            onConfirm={() => {
              try {
                const obj = JSON.parse(value);
                onSuccess(obj);
              } catch (error) {
                Toast.error(I18n.t('sample_data_format_error'));
                return;
              }
            }}
          >
            <Button color="brand">{I18n.t('extract_data_structure')}</Button>
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
