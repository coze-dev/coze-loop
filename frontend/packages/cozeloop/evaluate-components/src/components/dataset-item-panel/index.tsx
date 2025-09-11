/* eslint-disable @coze-arch/max-line-per-function */
import { useEffect, useRef, useState } from 'react';

import { GuardPoint, Guard } from '@cozeloop/guard';
import { ResizeSidesheet } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type FieldSchema,
  type EvaluationSetItem,
  type EvaluationSet,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozArrowLeft,
  IconCozArrowRight,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Form,
  type FormApi,
  Modal,
  Toast,
} from '@coze-arch/coze-design';

import IDWithCopy from '../id-with-copy';
import { fillTurnData } from '../../utils';
import { useViewMode } from './use-view-mode';
import { PopconfirmSave } from './popconfirm-save';
import { DatasetItemRenderList } from './item-list';

interface DatasetItemPanelProps {
  datasetItem: EvaluationSetItem;
  datasetDetail?: EvaluationSet;
  fieldSchemas?: FieldSchema[];
  isEdit: boolean;
  onCancel: () => void;
  onSave: (newItemData: EvaluationSetItem) => void;
  switchConfig?: {
    canSwithPre: boolean;
    canSwithNext: boolean;
    onSwith: (type: 'pre' | 'next') => void;
  };
}

export const DatasetItemPanel = ({
  datasetItem,
  datasetDetail,
  isEdit: isEditProps,
  fieldSchemas,
  onCancel,
  onSave,
  switchConfig,
}: DatasetItemPanelProps) => {
  const { spaceID } = useSpace();
  const [hasChange, setHasChange] = useState(false);
  const [isEdit, setIsEdit] = useState(isEditProps);
  const [loading, setLoading] = useState(false);
  const { isAuto, ViewModeNode } = useViewMode();
  const formRef = useRef<FormApi>();
  const handleSubmit = async values => {
    try {
      setLoading(true);
      const newTurnsData = values?.turns?.map(turn => ({
        ...turn,
        field_data_list: turn.field_data_list?.map(field => ({
          ...field,
          content: {
            content_type: field.content?.content_type,
            text: field.content?.text,
            ...field.content,
          },
        })),
      }));
      await StoneEvaluationApi.UpdateEvaluationSetItem({
        evaluation_set_id: datasetItem?.evaluation_set_id || '',
        item_id: datasetItem?.item_id || '',
        turns: newTurnsData,
        workspace_id: spaceID,
      });
      Toast.success('保存成功');
      setHasChange(false);
      setLoading(false);
      return newTurnsData;
    } catch (error) {
      console.error(error);
    }
    setLoading(false);
  };
  const defaultTurnsData = fillTurnData({
    turns: datasetItem?.turns,
    fieldSchemas,
  });

  useEffect(() => {
    setHasChange(false);
  }, [isEdit, datasetItem?.id]);

  const onConfirmChange = async (action: 'pre' | 'next') => {
    const turnsData = await handleSubmit(formRef.current?.getValues());
    if (turnsData) {
      onSave?.({ ...datasetItem, turns: turnsData });
    }
    switchConfig?.onSwith(action);
  };

  const onClose = () => {
    if (!hasChange) {
      onCancel();
    } else {
      Modal.confirm({
        title: '信息未保存',
        content: '离开后信息将不会保存',
        onOk: onCancel,
        okButtonColor: 'red',
        okText: '确认',
        cancelText: '取消',
      });
    }
  };
  return (
    <ResizeSidesheet
      showDivider
      visible={true}
      onCancel={() => {
        onClose();
      }}
      dragOptions={{
        defaultWidth: 880,
        maxWidth: 1382,
        minWidth: 600,
      }}
      bodyStyle={{
        padding: 0,
      }}
      footer={
        <div className="flex gap-2">
          {isEdit ? (
            <Guard point={GuardPoint['eval.dataset.edit']}>
              <Button
                loading={loading}
                color="hgltplus"
                onClick={() => {
                  formRef.current?.submitForm();
                }}
                disabled={loading || !hasChange}
              >
                保存
              </Button>
            </Guard>
          ) : (
            <Button color="primary" onClick={() => setIsEdit(true)}>
              编辑
            </Button>
          )}
          <Button color="primary" onClick={() => onClose()}>
            关闭
          </Button>
        </div>
      }
      title={
        <div className="text-[18px] font-medium flex items-center gap-2">
          <div className="flex">
            {isEdit ? '编辑数据项：' : '查看数据项：'}
            <IDWithCopy id={datasetItem?.id ?? ''} />
          </div>
          {switchConfig ? (
            <div className="flex-1 flex items-center justify-end">
              {!isEdit && (
                <>
                  {ViewModeNode}
                  <Divider layout="vertical" className="h-[12px] ml-2" />
                </>
              )}

              <PopconfirmSave
                needConfirm={hasChange}
                onConfirm={() => {
                  onConfirmChange('pre');
                }}
                onCancel={() => {
                  switchConfig?.onSwith('pre');
                }}
              >
                <Button
                  icon={<IconCozArrowLeft />}
                  color="secondary"
                  disabled={!switchConfig?.canSwithPre}
                  className="text-[13px] !coz-fg-secondary"
                  onClick={() => {
                    if (!hasChange) {
                      switchConfig?.onSwith('pre');
                    }
                  }}
                >
                  上一条
                </Button>
              </PopconfirmSave>
              <PopconfirmSave
                needConfirm={hasChange}
                onConfirm={() => {
                  onConfirmChange('next');
                }}
                onCancel={() => {
                  switchConfig?.onSwith('next');
                }}
              >
                <Button
                  icon={<IconCozArrowRight />}
                  iconPosition="right"
                  className="text-[13px] !coz-fg-secondary ml-2"
                  color="secondary"
                  disabled={!switchConfig?.canSwithNext}
                  onClick={() => {
                    if (!hasChange) {
                      switchConfig?.onSwith('next');
                    }
                  }}
                >
                  下一条
                </Button>
              </PopconfirmSave>
            </div>
          ) : null}
        </div>
      }
    >
      <Form
        className="h-full"
        key={datasetItem?.id}
        onSubmit={handleSubmit}
        getFormApi={api => {
          formRef.current = api;
        }}
        initValues={{
          turns: defaultTurnsData,
        }}
        onValueChange={values => {
          setHasChange(true);
        }}
      >
        {({ formState }) => {
          const { turns } = formState.values;
          return (
            <div className="h-full flex flex-col pl-[24px] pr-[18px] py-[16px] overflow-auto styled-scrollbar">
              <DatasetItemRenderList
                datasetDetail={datasetDetail}
                itemMaxHeightAuto={isAuto}
                fieldSchemas={fieldSchemas}
                isEdit={isEdit}
                turn={turns?.[0] || []}
                fieldKey="turns[0]"
              />
            </div>
          );
        }}
      </Form>
    </ResizeSidesheet>
  );
};
