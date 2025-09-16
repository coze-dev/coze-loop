// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { forwardRef, useImperativeHandle, useState } from 'react';

import { cloneDeep } from 'lodash-es';
import cs from 'classnames';
import { useUpdateEffect } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { TooltipWhenDisabled } from '@cozeloop/components';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  Button,
  Collapse,
  Toast,
  useFieldApi,
  useFieldState,
  useFormApi,
} from '@coze-arch/coze-design';

import { type ConvertFieldSchema } from '../dataset-item/type';
import { DEFAULT_COLUMN_SCHEMA } from '../dataset-create-form/type';
import { elementFocus } from '../../utils/element-focus';
import { ColumnRender } from './column-render';

import styles from './index.module.less';

interface DatasetColumnConfigProps {
  fieldKey: string;
  size?: 'large' | 'small';
  expand?: boolean;
  showAddButton?: boolean;
  disabledDataTypeSelect?: boolean;
}
export interface DatasetColumnConfigRef {
  addColumn: () => void;
}

const MAX_COLUMN_NUM = 50;

export const DatasetColumnConfig = forwardRef(
  (
    {
      fieldKey,
      size = 'large',
      expand,
      showAddButton = false,
      disabledDataTypeSelect = false,
    }: DatasetColumnConfigProps,
    ref: React.Ref<DatasetColumnConfigRef>,
  ) => {
    const field = useFieldState(fieldKey);
    const fieldApi = useFieldApi(fieldKey);
    const fieldSchema = field.value;
    const formApi = useFormApi();
    const [activeKey, setActiveKey] = useState<string[]>(
      fieldSchema?.map((_, index) => `${index}` || []),
    );
    useUpdateEffect(() => {
      if (expand) {
        setActiveKey(fieldSchema?.map((_, index) => `${index}`));
      } else {
        setActiveKey([]);
      }
    }, [expand]);

    const addColumn = () => {
      if (fieldSchema?.length >= MAX_COLUMN_NUM) {
        Toast.error(
          I18n.t('supports_up_to_{num}_columns', { num: MAX_COLUMN_NUM }),
        );
        return;
      }
      fieldApi.setValue([...fieldSchema, DEFAULT_COLUMN_SCHEMA]);
      setActiveKey([...activeKey, `${fieldSchema?.length}`]);
      elementFocus(`column-${fieldSchema?.length}`);
    };

    useImperativeHandle(ref, () => ({
      addColumn,
    }));

    return (
      <div className="pb-[1px]">
        <Collapse
          activeKey={activeKey}
          onChange={key => {
            setActiveKey(key as string[]);
          }}
          clickHeaderToExpand={false}
          className={cs(styles.collapse, styles[size])}
          keepDOM
        >
          {fieldSchema?.map((item: ConvertFieldSchema, index) => (
            <div id={`column-${index}`} key={index}>
              <ColumnRender
                key={index}
                size={size}
                disabledDataTypeSelect={disabledDataTypeSelect}
                activeKey={activeKey}
                setActiveKey={setActiveKey}
                onDelete={() => {
                  if (fieldSchema?.length === 1) {
                    Toast.error(I18n.t('retain_at_least_one_column'));
                    return;
                  }
                  formApi.validate().catch(err => {
                    console.error(err);
                  });
                  fieldApi.setValue(fieldSchema?.filter((_, i) => i !== index));
                }}
                onCopy={() => {
                  if (fieldSchema?.length >= MAX_COLUMN_NUM) {
                    Toast.error(
                      I18n.t('supports_up_to_{num}_columns', {
                        num: MAX_COLUMN_NUM,
                      }),
                    );
                    return;
                  }
                  //往index+1位置插入一个item
                  fieldApi.setValue([
                    ...fieldSchema.slice(0, index + 1),
                    {
                      ...cloneDeep(item),
                      key: undefined,
                      name: `${item.name || ''}_copy`.slice(0, MAX_COLUMN_NUM),
                      type: item.type,
                      description: item.description,
                      content_type: item.content_type,
                    },
                    ...fieldSchema.slice(index + 1),
                  ]);
                  formApi.validate().catch(err => {
                    console.error(err);
                  });
                  setActiveKey(
                    activeKey
                      .map(key => {
                        if (Number(key) > index) {
                          return `${Number(key) + 1}`;
                        }
                        return key;
                      })
                      .concat(`${index + 1}`),
                  );
                  elementFocus(`column-${index + 1}`);
                }}
                fieldKey={fieldKey}
                index={index}
              ></ColumnRender>
            </div>
          ))}
        </Collapse>
        {showAddButton ? (
          <TooltipWhenDisabled
            content={I18n.t('supports_up_to_{num}_columns', {
              num: MAX_COLUMN_NUM,
            })}
            theme="dark"
            disabled={fieldSchema?.length >= MAX_COLUMN_NUM}
          >
            <Button
              className="w-full"
              icon={<IconCozPlus />}
              color="primary"
              disabled={fieldSchema?.length >= MAX_COLUMN_NUM}
              onClick={addColumn}
            >
              {I18n.t('add_column')}
            </Button>
          </TooltipWhenDisabled>
        ) : null}
      </div>
    );
  },
);
