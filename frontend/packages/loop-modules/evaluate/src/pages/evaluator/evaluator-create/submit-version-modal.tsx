// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable complexity */
import { useEffect, useRef } from 'react';

import { nanoid } from 'nanoid';
import { merge } from 'lodash-es';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
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
  onFail,
}: {
  visible: boolean;
  type: 'create' | 'append';
  evaluator?: Evaluator;
  onCancel: () => void;
  onSuccess?: (evaluatorID?: Int64, newEvaluator?: Evaluator) => void;
  onFail?: (e: any) => void;
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
        try {
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
        } catch (e) {
          console.warn(e);
          onFail?.(e);
          throw e;
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
      title={isAppend ? I18n.t('submit_new_version') : I18n.t('new_evaluator')}
      visible={visible}
      cancelText={I18n.t('cancel')}
      onCancel={onCancel}
      okText={isAppend ? I18n.t('submit') : I18n.t('confirm')}
      onOk={handleOK}
      width={600}
    >
      <Form ref={formRef}>
        <FormInput
          label={{
            text: I18n.t('version'),
            required: true,
            extra: (
              <Tooltip content={I18n.t('version_number_format')}>
                <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
              </Tooltip>
            ),
          }}
          field="current_version.version"
          placeholder={I18n.t('please_input_version_number')}
          rules={[
            {
              validator: (_rule, value) => {
                if (!value) {
                  return new Error(I18n.t('please_input_version_number'));
                }
                const reg = /^\d{1,4}\.\d{1,4}\.\d{1,4}$/;
                if (!reg.test(value)) {
                  return new Error(I18n.t('version_number_format'));
                }
                if (type === 'append') {
                  const latestVersion = evaluator?.latest_version;
                  if (
                    latestVersion &&
                    compareVersions(value, latestVersion) <= 0
                  ) {
                    return new Error(I18n.t('version_number_gt_current'));
                  }
                }

                return true;
              },
            },
          ]}
        />
        <FormTextArea
          label={I18n.t('version_description')}
          field="current_version.description"
          placeholder={I18n.t('version_description')}
          maxCount={200}
          maxLength={200}
        />
      </Form>
    </Modal>
  );
}
