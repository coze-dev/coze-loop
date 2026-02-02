// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useRef, useState } from 'react';

import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { ResizeSidesheet } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type EvaluationSet,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  Anchor,
  Button,
  Divider,
  Form,
  type FormApi,
  Typography,
} from '@coze-arch/coze-design';

import {
  DatasetColumnConfig,
  type DatasetColumnConfigRef,
} from '../../dataset-column-config';
import { LoopAnchor } from '../../anchor';
import {
  convertDataTypeToSchema,
  convertSchemaToDataType,
} from '../../../utils/field-convert';
import { highlightCollapse } from '../../../utils/element-focus';
import { useExpandButton } from '../../../hooks/use-expand-button';

interface ColumnForm {
  columns: FieldSchema[];
}

export const useDatasetColumnEdit = ({
  datasetDetail,
  onRefresh,
  totalItemCount,
}: {
  datasetDetail?: EvaluationSet;
  onRefresh: () => void;
  totalItemCount?: number;
}) => {
  const fieldSchemas =
    datasetDetail?.evaluation_set_version?.evaluation_set_schema?.field_schemas;

  const formApiRef = useRef<FormApi>();
  const { spaceID } = useSpace();
  const columnConfigRef = useRef<DatasetColumnConfigRef>(null);
  const [visible, setVisible] = useState(false);
  const guard = useGuard({ point: GuardPoint['eval.dataset.edit_col'] });

  const [currentColumnNum, setCurrentColumnNum] = useState(
    fieldSchemas?.length,
  );
  const { expand, ExpandNode } = useExpandButton({
    shrinkTooltip: I18n.t('collapse_all_columns'),
    expandTooltip: I18n.t('expand_all_columns'),
  });
  const [loading, setLoading] = useState(false);
  const handleSubmit = async (values: ColumnForm) => {
    try {
      setLoading(true);
      const columns = values?.columns?.map(item =>
        convertDataTypeToSchema(item),
      );
      await StoneEvaluationApi.UpdateEvaluationSetSchema({
        evaluation_set_id: datasetDetail?.id as string,
        fields: columns,
        workspace_id: spaceID,
      });
      onRefresh();
      setVisible(false);
    } catch (error) {
      console.error(error);
    } finally {
      setLoading(false);
    }
  };
  const ColumnEditButton = (
    <Button
      color="primary"
      onClick={() => {
        setVisible(true);
        sendEvent(EVENT_NAMES.cozeloop_dataset_column_edit);
      }}
    >
      {I18n.t('edit_column')}
    </Button>
  );

  const ColumnEditModal = (
    <>
      <ResizeSidesheet
        dragOptions={{
          defaultWidth: 880,
          maxWidth: 1382,
          minWidth: 600,
        }}
        showDivider
        onCancel={() => {
          setVisible(false);
        }}
        bodyStyle={{
          padding: 0,
        }}
        visible={visible}
        title={
          <div className="flex w-full items-center justify-between">
            <div className="flex items-center gap-2">
              {I18n.t('edit_column')}
              <Typography.Text className="!coz-fg-secondary">
                {I18n.t('column_count_info', { currentColumnNum })}
              </Typography.Text>
            </div>
            {ExpandNode}
          </div>
        }
        footer={
          <div className="flex items-center gap-2">
            <Button
              disabled={guard.data.readonly}
              loading={loading}
              onClick={() => {
                formApiRef.current?.submitForm();
              }}
            >
              {I18n.t('save')}
            </Button>
            <Button color="primary" onClick={() => setVisible(false)}>
              {I18n.t('cancel')}
            </Button>
            <Divider layout="vertical" className="h-[12px] mx-[9px]" />
            <Button
              icon={<IconCozPlus />}
              color="primary"
              onClick={() => {
                columnConfigRef.current?.addColumn();
              }}
            >
              {I18n.t('add_column')}
            </Button>
          </div>
        }
      >
        <Form<ColumnForm>
          className="flex-1 h-full overflow-hidden"
          getFormApi={formApi => (formApiRef.current = formApi)}
          onSubmit={handleSubmit}
          initValues={{
            columns:
              fieldSchemas?.map(item => convertSchemaToDataType(item)) || [],
          }}
          onValueChange={values => {
            console.log('values', values);
            setCurrentColumnNum(values?.columns?.length || 0);
          }}
        >
          {({ formState }) => {
            const { columns } = formState.values ?? {};
            return (
              <div
                className="w-full flex h-full py-[16px]  pl-[24px]  pr-[18px] overflow-y-auto styled-scrollbar gap-[16px]"
                id="dataset-column-edit-container"
              >
                <div className="w-[133px] sticky top-0 h-fit">
                  <LoopAnchor
                    targetOffset={20}
                    showTooltip
                    offsetTop={30}
                    className="!max-h-[inherit]"
                    defaultAnchor={'#column-0'}
                    getContainer={() =>
                      document.getElementById(
                        'dataset-column-edit-container',
                      ) || document.body
                    }
                    onClick={(e, link) => {
                      highlightCollapse(
                        document.getElementById(link?.slice(1)),
                      );
                    }}
                  >
                    {columns?.map((item, index) => (
                      <Anchor.Link
                        key={index}
                        href={`#column-${index}`}
                        title={`${item.name || `${I18n.t('column')} ${index + 1}`}`}
                      />
                    ))}
                  </LoopAnchor>
                </div>
                <div className="flex-1">
                  <DatasetColumnConfig
                    ref={columnConfigRef}
                    disabledDataTypeSelect={totalItemCount !== 0}
                    fieldKey="columns"
                    size="small"
                    expand={expand}
                    showAddButton={false}
                  ></DatasetColumnConfig>
                </div>
              </div>
            );
          }}
        </Form>
      </ResizeSidesheet>
    </>
  );

  return {
    ColumnEditButton,
    ColumnEditModal,
  };
};
