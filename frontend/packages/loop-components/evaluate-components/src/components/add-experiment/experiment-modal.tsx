// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Version } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import {
  Form,
  type FormApi,
  FormSelect,
  Loading,
  Modal,
  Typography,
} from '@coze-arch/coze-design';

export const ExperimentModal = ({
  datasetDetail,
  currentVersion,
  onOk,
  onCancel,
  isDraftVersion,
}: {
  datasetDetail?: EvaluationSet;
  currentVersion?: Version;
  onOk: (version_id: string) => void;
  onCancel: () => void;
  isDraftVersion?: boolean;
}) => {
  const { spaceID } = useSpace();
  const formRef = useRef<FormApi>();
  const { data, loading } = useRequest(async () => {
    const res = await StoneEvaluationApi.ListEvaluationSetVersions({
      evaluation_set_id: datasetDetail?.id || '',
      workspace_id: spaceID,
      page_number: 1,
      page_size: 200,
    });
    return res.versions;
  });
  const onSubmit = values => {
    onOk(values?.version_id);
  };

  return (
    <Modal
      title={I18n.t('confirm_evaluation_set_version')}
      onOk={() => {
        formRef.current?.submitForm();
      }}
      visible
      width={600}
      height={473}
      onCancel={onCancel}
      okText={I18n.t('confirm')}
      cancelText={I18n.t('cancel')}
    >
      {loading ? (
        <div className="flex justify-center items-center h-full">
          <Loading loading />
        </div>
      ) : (
        <Form
          getFormApi={api => (formRef.current = api)}
          onSubmit={onSubmit}
          initValues={{
            version_id: isDraftVersion ? data?.[0]?.id : currentVersion?.id,
          }}
          onChange={values => {
            console.log(values);
          }}
        >
          {({ formState }) => (
            <>
              <FormSelect
                label={I18n.t('version')}
                className="w-full"
                extraTextPosition="bottom"
                extraText={
                  datasetDetail?.change_uncommitted ? (
                    <Typography.Text
                      icon={<IconCozInfoCircle />}
                      className="!coz-fg-secondary"
                      size="small"
                    >
                      {I18n.t('draft_unsubmitted_tip')}
                    </Typography.Text>
                  ) : null
                }
                field="version_id"
                rules={[{ required: true, message: I18n.t('select_version') }]}
                optionList={data?.map(item => ({
                  label: item.version,
                  value: item.id,
                }))}
                fieldStyle={{ paddingBottom: 8 }}
                filter={true}
              ></FormSelect>
              <Form.Slot label={I18n.t('version_description')}>
                <Typography.Text className="!coz-fg-primary">
                  {data?.find(item => item.id === formState?.values?.version_id)
                    ?.description || '-'}
                </Typography.Text>
              </Form.Slot>
            </>
          )}
        </Form>
      )}
    </Modal>
  );
};
