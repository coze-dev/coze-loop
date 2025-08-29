import { useState } from 'react';

import { cloneDeep } from 'lodash-es';
import { useUpdateEffect } from 'ahooks';
import { TooltipWhenDisabled } from '@cozeloop/components';
import {
  type EvaluationSet,
  type EvaluationSetItem,
} from '@cozeloop/api-schema/evaluation';
import {
  IconCozTrashCan,
  IconCozCopy,
  IconCozPlus,
  IconCozArrowDown,
  IconCozArrowRight,
} from '@coze-arch/coze-design/icons';
import {
  Collapse,
  Button,
  Typography,
  Toast,
  useFieldApi,
} from '@coze-arch/coze-design';

import { DatasetItemRenderList } from '../dataset-item-panel/item-list';
import { elementFocus } from '../../utils/element-focus';
import { createUuid } from '../../utils';
import { DATASET_ADD_ITEM_PREFIX } from '../../const';

import styles from './index.module.less';
interface DatasetAddItemsProps {
  datasetDetail?: EvaluationSet;
  defaultItem: EvaluationSetItem;
  expand?: boolean;
}

export const DatasetAddItems = ({
  defaultItem,
  datasetDetail,
  expand,
}: DatasetAddItemsProps) => {
  const fieldSchemas =
    datasetDetail?.evaluation_set_version?.evaluation_set_schema?.field_schemas;
  const evalsetItemsField = useFieldApi('evaSetItems');
  const evaSetItems = evalsetItemsField.getValue();
  const onDelete = (index: number) => {
    const newItems = evaSetItems?.filter((_, i) => i !== index);
    if (newItems.length === 0) {
      Toast.error('至少保留一个数据项');
      return;
    }
    evalsetItemsField.setValue(newItems);
  };
  const [activeKey, setActiveKey] = useState<string[]>(['0']);
  useUpdateEffect(() => {
    if (expand) {
      setActiveKey(evaSetItems?.map((_, index) => index.toString()));
    } else {
      setActiveKey([]);
    }
  }, [expand]);
  const onCopy = (index: number) => {
    if (evaSetItems?.length >= 10) {
      Toast.error('单次最多添加10条数据项');
      return;
    }
    const newItems = [
      ...evaSetItems.slice(0, index + 1),
      cloneDeep(evaSetItems[index]),
      ...evaSetItems.slice(index + 1),
    ];
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
    evalsetItemsField.setValue(newItems);
    elementFocus(`${DATASET_ADD_ITEM_PREFIX}-${index + 1}`);
  };
  return (
    <div>
      <Collapse
        activeKey={activeKey}
        onChange={newActiveKey => setActiveKey(newActiveKey as string[])}
        className={styles.collapse}
        keepDOM
      >
        {evaSetItems?.map((item, index) => (
          <div
            data-key={item?.key}
            key={item?.key ?? `${index}`}
            id={`${DATASET_ADD_ITEM_PREFIX}-${index}`}
          >
            <Collapse.Panel
              header={
                <div className="flex w-full justify-between items-center">
                  <div className="flex items-center gap-[4px]">
                    <Typography.Text className="!font-semibold">
                      {`数据项 ${index + 1}`}
                    </Typography.Text>
                    {activeKey.includes(`${index}`) ? (
                      <IconCozArrowDown
                        onClick={() =>
                          setActiveKey(
                            activeKey.filter(key => key !== `${index}`),
                          )
                        }
                        className="cursor-pointer w-[16px] h-[16px]"
                      />
                    ) : (
                      <IconCozArrowRight
                        onClick={() => setActiveKey([...activeKey, `${index}`])}
                        className="cursor-pointer w-[16px] h-[16px]"
                      />
                    )}
                  </div>
                  <div onClick={e => e.stopPropagation()}>
                    <Button
                      color="secondary"
                      size="small"
                      icon={<IconCozCopy />}
                      onClick={() => onCopy(index)}
                    ></Button>
                    <Button
                      icon={<IconCozTrashCan />}
                      color="secondary"
                      size="small"
                      onClick={() => onDelete(index)}
                    ></Button>
                  </div>
                </div>
              }
              itemKey={`${index}`}
              showArrow={false}
              className="mb-[20px]"
            >
              <DatasetItemRenderList
                fieldSchemas={fieldSchemas}
                isEdit={true}
                fieldKey={`evaSetItems[${index}].turns[0]`}
                turn={item.turns?.[0]}
              />
            </Collapse.Panel>
          </div>
        ))}
      </Collapse>
      <TooltipWhenDisabled
        theme="dark"
        content="单次最多添加10条数据项"
        disabled={evaSetItems?.length >= 10}
      >
        <div>
          <Button
            icon={<IconCozPlus />}
            color="primary"
            className="!w-full mb-[20px]"
            disabled={evaSetItems?.length >= 10}
            onClick={() => {
              const newItems = [
                ...evaSetItems,
                { ...defaultItem, key: createUuid(8) },
              ];
              evalsetItemsField.setValue(newItems);
              setActiveKey([...activeKey, `${evaSetItems?.length}`]);
              elementFocus(`${DATASET_ADD_ITEM_PREFIX}-${evaSetItems?.length}`);
            }}
          >
            添加数据项
            <Typography.Text className="ml-2 !coz-fg-dim" size="small">
              {evaSetItems?.length}/10
            </Typography.Text>
          </Button>
        </div>
      </TooltipWhenDisabled>
    </div>
  );
};
