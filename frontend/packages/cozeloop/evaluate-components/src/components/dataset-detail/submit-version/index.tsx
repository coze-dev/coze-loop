import { useRef, useState } from 'react';

import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { GuardPoint, Guard } from '@cozeloop/guard';
import { TooltipWhenDisabled } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type CreateEvaluationSetVersionRequest,
  type EvaluationSet,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import {
  Button,
  Form,
  Modal,
  Toast,
  Tooltip,
  type FormApi,
} from '@coze-arch/coze-design';

export const SubmitVersion = ({
  datasetDetail,
  onSubmit,
}: {
  datasetDetail?: EvaluationSet;
  onSubmit: () => void;
}) => {
  const [visible, setVisible] = useState(false);
  const { spaceID } = useSpace();
  const [loading, setLoading] = useState(false);
  const formRef = useRef<FormApi>();
  const handleSubmit = async (values: CreateEvaluationSetVersionRequest) => {
    try {
      setLoading(true);
      await StoneEvaluationApi.CreateEvaluationSetVersion({
        evaluation_set_id: datasetDetail?.id as string,
        workspace_id: spaceID,
        version: values?.version,
        desc: values?.desc,
      });
      Toast.success('提交成功');
      setVisible(false);
      onSubmit();
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Guard point={GuardPoint['eval.dataset.commit']}>
        <TooltipWhenDisabled
          theme="dark"
          content="暂无修改可提交"
          disabled={!datasetDetail?.change_uncommitted}
        >
          <Button
            color="primary"
            onClick={() => {
              setVisible(true);
              sendEvent(EVENT_NAMES.cozeloop_dataset_submit_version);
            }}
            disabled={!datasetDetail?.change_uncommitted}
          >
            提交新版本
          </Button>
        </TooltipWhenDisabled>
      </Guard>

      <Modal
        visible={visible}
        onCancel={() => setVisible(false)}
        onOk={() => {
          formRef?.current?.submitForm();
        }}
        title="提交新版本"
        okText="提交"
        okButtonProps={{
          loading,
        }}
        cancelText="取消"
      >
        <Form<CreateEvaluationSetVersionRequest>
          getFormApi={formApi => {
            formRef.current = formApi;
          }}
          onSubmit={handleSubmit}
        >
          <Form.Input
            field="version"
            autoComplete="off"
            initValue={getNewVersion(datasetDetail?.latest_version || '')}
            label={
              <div className="inline-flex items-center gap-1">
                版本
                <Tooltip theme="dark" content="版本格式为a.b.c，且每段为0-999">
                  <div className="h-[15px] cursor-pointer">
                    <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
                  </div>
                </Tooltip>
              </div>
            }
            rules={[
              {
                validator: (_, value, callback) => {
                  if (!value) {
                    callback('版本不能为空');
                    return false;
                  }
                  if (
                    !/^([0-9]|[1-9][0-9]{1,2})\.([0-9]|[1-9][0-9]{1,2})\.([0-9]|[1-9][0-9]{1,2})$/.test(
                      value,
                    )
                  ) {
                    callback('请输入正确的版本，格式为a.b.c，且每段为0-999');
                    return false;
                  }
                  if (
                    datasetDetail?.latest_version &&
                    compareVersions(
                      value,
                      datasetDetail?.latest_version || '',
                    ) <= 0
                  ) {
                    callback(
                      `新版本必须大于当前版本：${datasetDetail?.latest_version}`,
                    );
                    return false;
                  }
                  return true;
                },
              },
            ]}
          />
          <Form.TextArea maxCount={200} field="desc" label="版本说明" />
        </Form>
      </Modal>
    </>
  );
};

const getNewVersion = (version?: string) => {
  const versionList = version?.split('.') || [];
  if (versionList.length !== 3) {
    return '0.0.1';
  }
  const [a, b, c] = versionList;
  if (Number(c) < 999) {
    return `${a}.${b}.${Number(c) + 1}`;
  }
  if (Number(b) < 999) {
    return `${a}.${Number(b) + 1}.0`;
  }
  if (Number(a) < 999) {
    return `${Number(a) + 1}.0.0`;
  }
};

export const compareVersions = (version1: string, version2: string): number => {
  const v1Parts = version1.split('.').map(Number);
  const v2Parts = version2.split('.').map(Number);
  const maxLength = Math.max(v1Parts.length, v2Parts.length);

  for (let i = 0; i < maxLength; i++) {
    const num1 = v1Parts[i] || 0;
    const num2 = v2Parts[i] || 0;

    if (num1 > num2) {
      return 1;
    } else if (num1 < num2) {
      return -1;
    }
  }

  return 0;
};
