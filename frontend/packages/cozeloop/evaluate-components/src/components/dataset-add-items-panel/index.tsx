// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useRef, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, Guard } from '@cozeloop/guard';
import { ResizeSidesheet } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  Anchor,
  Button,
  Form,
  type FormApi,
  Modal,
  Toast,
} from '@coze-arch/coze-design';

import { ImportResultInfo } from '../import-result-info';
import { LoopAnchor } from '../anchor';
import { highlightCollapse } from '../../utils/element-focus';
import { useExpandButton } from '../../hooks/use-expand-button';
import { DATASET_ADD_ITEM_PREFIX } from '../../const';
import { getDefaultEvaSetItem } from './util';
import { DatasetAddItems } from './dataset-add-items';

export const DatasetAddItemsPanel = ({
  onOK,
  onCancel,
  datasetDetail,
}: {
  onOK: () => void;
  onCancel: () => void;
  datasetDetail?: EvaluationSet;
}) => {
  const { spaceID } = useSpace();
  const [loading, setLoading] = useState(false);
  const defaultEvaSetItem = getDefaultEvaSetItem(datasetDetail, spaceID);
  const { ExpandNode, expand } = useExpandButton({
    shrinkTooltip: I18n.t('collapse_all_data_items'),
    expandTooltip: I18n.t('expand_all_data_items'),
  });
  const formApiRef = useRef<FormApi>();

  const handleSubmit = async values => {
    try {
      setLoading(true);
      const res = await StoneEvaluationApi.BatchCreateEvaluationSetItems({
        workspace_id: spaceID,
        evaluation_set_id: datasetDetail?.id as string,
        items: values?.evaSetItems,
        skip_invalid_items: true,
        allow_partial_add: true,
      });
      const successCount = Object.keys(res?.added_items || {}).length;
      if (res?.errors?.length && res?.errors?.length > 0) {
        Modal.info({
          title: I18n.t('execution_result'),
          width: 420,
          content: (
            <div className="mt-[20px]">
              <ImportResultInfo
                errors={res?.errors}
                progress={{
                  added: successCount.toString(),
                  processed: values?.evaSetItems.length,
                }}
              />
            </div>
          ),

          onOk: () => {
            onOK();
          },
          okText: I18n.t('known'),
        });
      } else {
        Toast.success(
          `${I18n.t('successfully_added_{successcount}_data', { successCount })}`,
        );
        onOK();
      }
    } catch (error) {
      console.error(error);
    }
    setLoading(false);
  };
  return (
    <>
      <ResizeSidesheet
        dragOptions={{
          defaultWidth: 880,
          maxWidth: 1382,
          minWidth: 600,
        }}
        showDivider
        bodyStyle={{
          padding: 0,
        }}
        onCancel={onCancel}
        footer={
          <div className="flex  gap-2">
            <Guard point={GuardPoint['eval.dataset.add']}>
              <Button
                color="hgltplus"
                loading={loading}
                onClick={() => {
                  // formApiRef.current?.validate().finally(data => {
                  //   console.log('data', data);
                  // });
                  formApiRef.current?.submitForm();
                }}
                disabled={loading}
              >
                {I18n.t('space_member_role_type_add_btn')}
              </Button>
            </Guard>
            <Button color="primary" onClick={onCancel}>
              {I18n.t('cancel')}
            </Button>
          </div>
        }
        visible={true}
        title={
          <div className="flex items-center justify-between gap-2">
            {I18n.t('add_data')}
            {ExpandNode}
          </div>
        }
      >
        <Form
          className="flex-1 h-full overflow-hidden"
          getFormApi={formApi => (formApiRef.current = formApi)}
          onSubmit={handleSubmit}
          initValues={{
            evaSetItems: [{ ...defaultEvaSetItem, key: '0' }],
          }}
        >
          {({ formState }) => {
            const { evaSetItems } = formState.values;
            return (
              <div
                className="w-full flex h-full overflow-y-auto gap-[16px] pl-[24px] pr-[18px] pt-[24px] styled-scrollbar"
                id="dataset-add-item-container"
              >
                <div className="w-[104px] sticky top-0 h-fit">
                  <LoopAnchor
                    targetOffset={20}
                    showTooltip
                    className="!max-h-[inherit]"
                    offsetTop={30}
                    defaultAnchor={`#${DATASET_ADD_ITEM_PREFIX}-${0}`}
                    getContainer={() =>
                      document.getElementById('dataset-add-item-container') ||
                      document.body
                    }
                    onClick={(_e, link) => {
                      highlightCollapse(
                        document.getElementById(link?.slice(1)),
                      );
                    }}
                  >
                    {evaSetItems?.map((_item, index) => (
                      <Anchor.Link
                        key={index}
                        href={`#${DATASET_ADD_ITEM_PREFIX}-${index}`}
                        title={`${I18n.t('data_item')} ${index + 1}`}
                      />
                    ))}
                  </LoopAnchor>
                </div>
                <div className="flex-1">
                  <DatasetAddItems
                    datasetDetail={datasetDetail}
                    defaultItem={defaultEvaSetItem}
                    expand={expand}
                  />
                </div>
              </div>
            );
          }}
        </Form>
      </ResizeSidesheet>
    </>
  );
};
