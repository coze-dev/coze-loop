import { useEffect, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  IconCozArrowDown,
  IconCozArrowRight,
} from '@coze-arch/coze-design/icons';
import {
  Collapse,
  Typography,
  useFieldApi,
  withField,
} from '@coze-arch/coze-design';

import { validColumnSchema } from '@/utils/jsonschema-convert';
import { getDefaultFieldData } from '@/utils/field-convert';
import {
  DEFAULT_MULTIPART_SCHEMA,
  DEFAULT_MULTIPART_SCHEMA_OBJ,
} from '@/const/evaluate-target';
import { TreeEditor } from '@/components/tree-editor';

import { ObjectCodeEditor } from '../object-column-render/object-code-editor-field';
import { JSONSchemaPropertyRender } from '../object-column-render/json-schema-property-render';
import { JSONSchemaHeader } from '../object-column-render/json-schema-header';
import styles from '../index.module.less';
import { type ConvertFieldSchema, InputType } from '../../dataset-item/type';

interface MultipartRenderProps {
  inputType: InputType;
}
const FormObjectCodeEditor = withField(ObjectCodeEditor);
const fieldKey = 'tempMultipartFieldKey';
export const MultipartRender = ({ inputType }: MultipartRenderProps) => {
  const fieldApi = useFieldApi(fieldKey);
  const fieldValue = fieldApi?.getValue() as ConvertFieldSchema;
  // collapse是否展开
  const [activeKey, setActiveKey] = useState(['1']);
  useEffect(() => {
    fieldApi.setValue({
      children: DEFAULT_MULTIPART_SCHEMA_OBJ,
      schema: DEFAULT_MULTIPART_SCHEMA,
    });
  }, []);
  const getHeader = () => (
    <div className="flex w-full justify-between">
      <div className="flex items-center gap-[4px]">
        {I18n.t('cozeloop_open_evaluate_data_structure')}
        {activeKey?.length ? (
          <IconCozArrowDown
            onClick={() => setActiveKey([])}
            className="cursor-pointer w-[16px] h-[16px]"
          />
        ) : (
          <IconCozArrowRight
            onClick={() => setActiveKey(['1'])}
            className="cursor-pointer w-[16px] h-[16px]"
          />
        )}
      </div>
    </div>
  );
  return (
    <Collapse
      className={styles['object-collapse']}
      clickHeaderToExpand={false}
      activeKey={activeKey}
      keepDOM
    >
      <Collapse.Panel itemKey={'1'} header={getHeader()} showArrow={false}>
        <Typography.Text className="coz-fg-secondary mb-2 block -mt-1">
          {I18n.t('cozeloop_open_evaluate_preset_array_object_data_type')}
        </Typography.Text>
        {inputType === InputType.Form ? (
          <>
            {fieldValue?.children?.map((item, index) => (
              <div
                key={index}
                className="bg-[var(--coz-bg-plus)] border border-solid border-[var(--coz-stroke-primary)] rounded-[6px] p-[12px] mb-[12px]"
              >
                {fieldValue?.children?.length ? (
                  <JSONSchemaHeader
                    showAdditional={false}
                    disabelChangeDatasetType={true}
                  />
                ) : null}
                <TreeEditor
                  treeData={item}
                  isShowAction={false}
                  defaultNodeData={getDefaultFieldData()}
                  labelRender={({ path, parentPath }) => {
                    const prefix = `${fieldKey}.children[${index}]${path ? `.${path}` : ''}`;
                    let parentKey = fieldKey;
                    if (path) {
                      parentKey = `${fieldKey}.children[${index}]${parentPath ? `.${parentPath}` : ''}`;
                    }
                    const level = path?.split('children')?.length || 0;
                    return (
                      <JSONSchemaPropertyRender
                        showAdditional={false}
                        parentFieldKey={parentKey}
                        fieldKeyPrefix={prefix}
                        level={level}
                        disabled={true}
                      />
                    );
                  }}
                />
              </div>
            ))}
          </>
        ) : null}
        {inputType === InputType.JSON ? (
          <FormObjectCodeEditor
            disabled={true}
            key={inputType}
            noLabel
            rules={[
              {
                validator: (rule, val: string, cb) =>
                  validColumnSchema({
                    schema: val,
                    type: fieldValue.type,
                    cb,
                  }),
              },
            ]}
            field={`${fieldKey}.schema`}
          />
        ) : null}
      </Collapse.Panel>
    </Collapse>
  );
};
