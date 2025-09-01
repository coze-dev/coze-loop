/* eslint-disable complexity */
import { useEffect, useRef } from 'react';

import { nanoid } from 'nanoid';
import { merge } from 'lodash-es';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import {
  Form,
  FormInput,
  FormTextArea,
  Modal,
  Tooltip,
} from '@coze-arch/coze-design';

import { compareVersions, incrementVersion } from '@/utils/version';

export function SubmitVersionModal({
  visible,
  type,
  evaluator,
  onCancel,
  onSuccess,
}: {
  visible: boolean;
  type: 'create' | 'append';
  evaluator?: Evaluator;
  onCancel: () => void;
  onSuccess?: (evaluatorID?: Int64, newEvaluator?: Evaluator) => void;
}) {
  const { spaceID } = useSpace();
  const formRef = useRef<Form>(null);
  const isAppend = type === 'append';

  const handleOK = async () => {
    if (evaluator) {
      const values = await formRef.current?.formApi
        ?.validate()
        .catch(e => console.warn(e));
      if (values) {
        if (isAppend) {
          const { evaluator: newEvaluator } =
            await StoneEvaluationApi.SubmitEvaluatorVersion({
              workspace_id: spaceID,
              evaluator_id: evaluator.evaluator_id || '',
              version: values.current_version.version,
              description: values.current_version.description,
              cid: nanoid(),
            });
          onSuccess?.(newEvaluator?.evaluator_id, newEvaluator);
        } else {
          const newEvaluator = merge<Evaluator, Evaluator, Evaluator>(
            {
              workspace_id: spaceID,
            },
            evaluator,
            values,
          );
          const { evaluator_id } = await StoneEvaluationApi.CreateEvaluator({
            evaluator: newEvaluator,
            cid: nanoid(),
          });
          if (evaluator_id) {
            const { prompt_evaluator } =
              newEvaluator?.current_version?.evaluator_content || {};
            const { prompt_template_name = '' } = prompt_evaluator || {};
            // 新建评估器, 是否使用模板, 使用到模板的名称
            sendEvent(EVENT_NAMES.cozeloop_rule_template, {
              is_from_template: prompt_template_name ? true : false,
              template_name: prompt_template_name,
            });
            onSuccess?.(evaluator_id);
          }
        }
      }
    }
  };

  useEffect(() => {
    if (visible) {
      let version = '0.0.1';
      const latestVersion = evaluator?.latest_version;
      if (isAppend && latestVersion) {
        version = incrementVersion(latestVersion);
      }

      formRef.current?.formApi?.setValues({
        current_version: {
          version,
        },
      });
    }
  }, [visible]);

  return (
    <Modal
      title={isAppend ? '提交新版本' : '创建评估器'}
      visible={visible}
      cancelText={'取消'}
      onCancel={onCancel}
      okText={isAppend ? '提交' : '确定'}
      onOk={handleOK}
      width={600}
    >
      <Form ref={formRef}>
        <FormInput
          label={{
            text: '版本',
            required: true,
            extra: (
              <Tooltip content={'版本号格式为a.b.c，且每段为0-9999'}>
                <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
              </Tooltip>
            ),
          }}
          field="current_version.version"
          placeholder={'请输入版本号'}
          rules={[
            {
              validator: (_rule, value) => {
                if (!value) {
                  return new Error('请输入版本号');
                }
                const reg = /^\d{1,4}\.\d{1,4}\.\d{1,4}$/;
                if (!reg.test(value)) {
                  return new Error('版本号格式为a.b.c，且每段为0-9999');
                }
                if (type === 'append') {
                  const latestVersion = evaluator?.latest_version;
                  if (
                    latestVersion &&
                    compareVersions(value, latestVersion) <= 0
                  ) {
                    return new Error(
                      `版本号必须大于当前版本号：${latestVersion}`,
                    );
                  }
                }

                return true;
              },
            },
          ]}
        />
        <FormTextArea
          label="版本说明"
          field="current_version.description"
          placeholder={'请输入版本说明'}
          maxCount={200}
          maxLength={200}
        />
      </Form>
    </Modal>
  );
}
