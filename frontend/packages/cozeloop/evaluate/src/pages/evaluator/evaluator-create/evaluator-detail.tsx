// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useBlocker, useParams } from 'react-router-dom';
import { useEffect, useRef, useState } from 'react';

import { set } from 'lodash-es';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { useBreadcrumb } from '@cozeloop/hooks';
import { GuardPoint, Guard } from '@cozeloop/guard';
import { sourceNameRuleValidator } from '@cozeloop/evaluate-components';
import { RouteBackAction } from '@cozeloop/components';
import { useSpace, useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { EvaluatorType, type Evaluator } from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  Button,
  Form,
  FormInput,
  FormTextArea,
  Spin,
  Modal,
} from '@coze-arch/coze-design';

import { SubmitVersionModal } from './submit-version-modal';
import { generateInputSchemas } from './prompt-field';
import { PromptConfigField } from './prompt-config-field';
import { DebugButton } from './debug-button';

function EvaluatorCreatePage() {
  const { spaceID } = useSpace();
  const { id } = useParams<{ id: string }>();
  const navigateModule = useNavigateModule();
  const [blockLeave, setBlockLeave] = useState(false);
  const [refreshEditorModelKey, setRefreshEditorModelKey] = useState(0);

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      currentLocation.pathname !== nextLocation.pathname && blockLeave,
  );
  useEffect(() => {
    if (blocker.state === 'blocked') {
      Modal.warning({
        title: I18n.t('information_unsaved'),
        content: I18n.t('leave_page_tip'),
        cancelText: I18n.t('cancel'),
        onCancel: blocker.reset,
        okText: I18n.t('confirm'),
        onOk: blocker.proceed,
      });
    }
  }, [blocker.state]);

  useBreadcrumb({
    text: I18n.t('new_evaluator'),
  });

  const formRef = useRef<Form>(null);
  const [submitValues, setSubmitValues] = useState<Evaluator>();

  const sourceService = useRequest(async () => {
    if (id) {
      const { evaluator } = await StoneEvaluationApi.GetEvaluator({
        workspace_id: spaceID,
        evaluator_id: id,
      });
      const sourceName = evaluator?.name || '';
      const copySubfix = '_Copy';
      const newName = sourceName
        .slice(0, 50 - copySubfix.length)
        .concat(copySubfix);
      if (evaluator) {
        return {
          ...evaluator,
          name: newName,
        };
      }
    }
  });
  const handleSubmit = () =>
    formRef.current?.formApi
      ?.validate()
      .then((values: Evaluator) => {
        const inputSchema = generateInputSchemas(
          values.current_version?.evaluator_content?.prompt_evaluator
            ?.message_list,
        );
        const newValues = { ...values };
        set(
          newValues,
          'current_version.evaluator_content.input_schemas',
          inputSchema,
        );

        setSubmitValues(newValues);
      })
      .catch(e => console.warn(e));

  const formContent = (
    <Form
      initValues={
        sourceService.data || {
          evaluator_type: EvaluatorType.Prompt,
        }
      }
      className="flex-1 w-[800px] mx-auto form-default"
      ref={formRef}
      onValueChange={(values, changeValues) => {
        setBlockLeave(true);
      }}
    >
      <div className="h-[28px] mb-3 text-[16px] leading-7 font-medium coz-fg-plus">
        {I18n.t('basic_info')}
      </div>
      <FormInput
        label={I18n.t('name')}
        field="name"
        placeholder={I18n.t('please_input', { field: '' })}
        required
        maxLength={50}
        trigger="blur"
        rules={[
          { required: true, message: I18n.t('please_input_name') },
          { max: 50 },
          { validator: sourceNameRuleValidator },
          {
            asyncValidator: async (_, value: string) => {
              if (value) {
                const { pass } = await StoneEvaluationApi.CheckEvaluatorName({
                  workspace_id: spaceID,
                  name: value,
                });
                if (pass === false) {
                  throw new Error(I18n.t('name_already_exists'));
                }
              }
            },
          },
        ]}
      />
      <FormTextArea
        label={I18n.t('description')}
        field="description"
        placeholder={I18n.t('please_input_description')}
        fieldStyle={{ paddingTop: 8 }}
        maxCount={200}
        maxLength={200}
      />
      <div className="h-7 mt-[10px]" />
      <PromptConfigField refreshEditorModelKey={refreshEditorModelKey} />
    </Form>
  );

  return (
    <div className="flex flex-col h-full">
      <div className="px-6 flex-shrink-0 py-3 h-[56px] flex flex-row items-center">
        <RouteBackAction defaultModuleRoute="evaluation/evaluators" />
        <span className="ml-2 text-[18px] font-medium coz-fg-plus">
          {I18n.t('new_evaluator')}
        </span>
      </div>
      {sourceService.loading ? (
        <div className="flex-1 flex items-center justify-center">
          <Spin spinning={true} />
        </div>
      ) : (
        <>
          <div className="p-6 pt-[12px] flex-1 overflow-y-auto styled-scrollbar pr-[18px]">
            {formContent}
          </div>
          <div className="flex-shrink-0 p-6">
            <div className="w-[800px] mx-auto flex flex-row justify-end gap-2">
              <DebugButton
                formApi={formRef}
                onApplyValue={() => setRefreshEditorModelKey(pre => pre + 1)}
              />
              <Guard point={GuardPoint['eval.evaluator_create.create']}>
                <Button type="primary" onClick={handleSubmit}>
                  {I18n.t('create')}
                </Button>
              </Guard>
            </div>
          </div>
        </>
      )}

      <SubmitVersionModal
        type="create"
        visible={Boolean(submitValues)}
        evaluator={submitValues}
        onCancel={() => setSubmitValues(undefined)}
        onSuccess={(evaluatorID?: Int64) => {
          setBlockLeave(false);
          setTimeout(() => {
            navigateModule(`evaluation/evaluators/${evaluatorID}`, {
              replace: true,
            });
          }, 100);
        }}
      />
    </div>
  );
}

export default EvaluatorCreatePage;
