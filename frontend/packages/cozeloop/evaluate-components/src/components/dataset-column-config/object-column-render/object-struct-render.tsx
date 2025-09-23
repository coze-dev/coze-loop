// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useState } from 'react';

import { nanoid } from 'nanoid';
import { cloneDeep } from 'lodash-es';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  IconCozArrowDown,
  IconCozArrowRight,
  IconCozPlus,
} from '@coze-arch/coze-design/icons';
import {
  Collapse,
  Button,
  useFieldApi,
  withField,
} from '@coze-arch/coze-design';

import { validColumnSchema } from '@/utils/jsonschema-convert';
import { getDefaultFieldData } from '@/utils/field-convert';
import { TreeEditor } from '@/components/tree-editor';

import styles from '../index.module.less';
import {
  type ConvertFieldSchema,
  DataType,
  type FieldObjectSchema,
  InputType,
} from '../../dataset-item/type';
import { ObjectCodeEditor } from './object-code-editor-field';
import { JSONSchemaPropertyRender } from './json-schema-property-render';
import { JSONSchemaHeader } from './json-schema-header';
import { useImportDataModal } from './import-data-modal';

interface ObjectStructRenderProps {
  fieldKey: string;
  disabelChangeDatasetType: boolean;
  inputType: InputType;
  showAdditional: boolean;
}
const FormObjectCodeEditor = withField(ObjectCodeEditor);

export const ObjectStructRender = ({
  fieldKey,
  disabelChangeDatasetType,
  inputType,
  showAdditional,
}: ObjectStructRenderProps) => {
  const fieldApi = useFieldApi(fieldKey);
  const fieldValue = fieldApi?.getValue() as ConvertFieldSchema;
  // collapse是否展开
  const [activeKey, setActiveKey] = useState(['1']);
  const { triggerButton, modalNode } = useImportDataModal(fieldKey, inputType);

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
      <div
        onClick={e => e.stopPropagation()}
        className="flex items-center gap-2"
      >
        {!disabelChangeDatasetType ? (
          <>
            {triggerButton}
            {modalNode}
          </>
        ) : null}
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
        {inputType === InputType.Form ? (
          <>
            {fieldValue?.children?.map((item, index) => (
              <div
                key={index}
                className="bg-[var(--coz-bg-plus)] border border-solid border-[var(--coz-stroke-primary)] rounded-[6px] p-[12px] mb-[12px]"
              >
                {fieldValue?.children?.length ? (
                  <JSONSchemaHeader
                    showAdditional={showAdditional}
                    disabelChangeDatasetType={disabelChangeDatasetType}
                  />
                ) : null}
                <TreeEditor
                  onChange={nodeData => {
                    let newChildren = cloneDeep(fieldValue?.children);
                    if (nodeData === null) {
                      newChildren = newChildren?.filter(
                        (child, childIndex) => childIndex !== index,
                      );
                    } else {
                      newChildren[index] = nodeData;
                    }

                    fieldApi.setValue({
                      ...fieldValue,
                      children: newChildren,
                    });
                  }}
                  treeData={item}
                  isShowAddNode={nodeData => {
                    const type = (nodeData as FieldObjectSchema)?.type;
                    return (
                      type === DataType.Object || type === DataType.ArrayObject
                    );
                  }}
                  isShowAction={!disabelChangeDatasetType}
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
                        showAdditional={showAdditional}
                        parentFieldKey={parentKey}
                        fieldKeyPrefix={prefix}
                        level={level}
                        disabled={disabelChangeDatasetType}
                      />
                    );
                  }}
                />
              </div>
            ))}

            {!disabelChangeDatasetType && (
              <Button
                color="primary"
                size="small"
                icon={<IconCozPlus />}
                onClick={() => {
                  fieldApi.setValue({
                    ...fieldValue,
                    children: [
                      ...(fieldValue?.children || []),
                      {
                        ...getDefaultFieldData(),
                        key: nanoid(),
                      },
                    ],
                  });
                }}
              >
                {I18n.t('field')}
              </Button>
            )}
          </>
        ) : null}
        {inputType === InputType.JSON ? (
          <FormObjectCodeEditor
            disabled={disabelChangeDatasetType}
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
