import { forwardRef, useImperativeHandle, useState } from 'react';

import { cloneDeep } from 'lodash-es';
import cs from 'classnames';
import { useUpdateEffect } from 'ahooks';
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
      if (fieldSchema?.length >= 50) {
        Toast.error('最多支持50列');
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
                    Toast.error('至少保留一个列');
                    return;
                  }
                  formApi.validate().catch(err => {
                    console.error(err);
                  });
                  fieldApi.setValue(fieldSchema?.filter((_, i) => i !== index));
                }}
                onCopy={() => {
                  if (fieldSchema?.length >= 50) {
                    Toast.error('最多支持50列');
                    return;
                  }
                  //往index+1位置插入一个item
                  fieldApi.setValue([
                    ...fieldSchema.slice(0, index + 1),
                    {
                      ...cloneDeep(item),
                      key: undefined,
                      name: `${item.name || ''}_copy`.slice(0, 50),
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
            content="最多支持50列"
            theme="dark"
            disabled={fieldSchema?.length >= 50}
          >
            <Button
              className="w-full"
              icon={<IconCozPlus />}
              color="primary"
              disabled={fieldSchema?.length >= 50}
              onClick={addColumn}
            >
              添加列
            </Button>
          </TooltipWhenDisabled>
        ) : null}
      </div>
    );
  },
);
