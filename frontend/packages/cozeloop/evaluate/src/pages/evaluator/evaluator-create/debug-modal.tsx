// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useEffect, useRef, useState } from 'react';

import { useDebounceFn, useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { Guard, GuardPoint, useGuard } from '@cozeloop/guard';
import {
  EvaluatorTestRunResult,
  parseMessagesVariables,
} from '@cozeloop/evaluate-components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  BenefitBanner,
  BenefitBannerScene,
  BenefitBaseBanner,
} from '@cozeloop/biz-components-adapter';
import { VariableType, type VariableDef } from '@cozeloop/api-schema/prompt';
import {
  type EvaluatorInputData,
  type Evaluator,
  EvaluatorType,
  type Content,
  ContentType,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozIllusEmpty } from '@coze-arch/coze-design/illustrations';
import {
  IconCozInfoCircle,
  IconCozPlayCircle,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  EmptyState,
  Form,
  Modal,
  Tooltip,
  FormInput,
  withField,
} from '@coze-arch/coze-design';

import { MultiPartEdit } from './multi-part-editor';
import { ConfigContent } from './config-content';

import styles from './debug-modal.module.less';

const FormMultiPartEdit = withField(MultiPartEdit);

function hasMultipartVar(vars: VariableDef[]) {
  return vars?.some(v => v.type === VariableType.MultiPart);
}

export function DebugModal({
  initValue,
  disableConfig,
  onCancel,
  onSubmit,
}: {
  initValue?: Evaluator;
  disableConfig?: boolean;
  onCancel: () => void;
  onSubmit: (newValue: Evaluator) => void;
}) {
  const { spaceID } = useSpace();
  const evaluatorFormRef = useRef<Form<Evaluator>>(null);
  const inputFormRef = useRef<Form<EvaluatorInputData>>(null);
  const [variables, setVariables] = useState<VariableDef[]>([]);

  const guard = useGuard({
    point: GuardPoint['eval.evaluator_create.debug'],
  });

  const guard2 = useGuard({
    point: GuardPoint['eval.evaluator_create.preview_debug'],
  });

  const calcVariables = useDebounceFn(
    () => {
      const evaluator = evaluatorFormRef.current?.formApi?.getValues();
      const messageList =
        evaluator?.current_version?.evaluator_content?.prompt_evaluator
          ?.message_list;
      const newVariables = parseMessagesVariables(messageList ?? []);
      setVariables(originVars => {
        // 存在多部分变量，但是新的变量中没有多部分变量，触发一次表单校验校验
        if (hasMultipartVar(originVars) && !hasMultipartVar(newVariables)) {
          evaluatorFormRef.current?.formApi
            ?.validate()
            .catch(e => console.warn(e));
        }
        return newVariables;
      });
    },
    { wait: 500 },
  );

  const service = useRequest(
    async () => {
      const evaluator = await evaluatorFormRef.current?.formApi
        ?.validate()
        .catch(e => console.warn(e));
      const evaluatorContent = evaluator?.current_version?.evaluator_content;
      const inputData = inputFormRef.current?.formApi?.getValues();
      if (evaluatorContent) {
        const inputFields: Record<string, Content> = {};

        Object.entries(inputData || {}).forEach(([key, value]) => {
          if (key && value) {
            if (value?.content_type) {
              inputFields[key] = value;
            } else {
              inputFields[key] = {
                content_type: ContentType.Text,
                text: value,
              };
            }
          }
        });

        const res = await StoneEvaluationApi.DebugEvaluator({
          workspace_id: spaceID,
          evaluator_type: EvaluatorType.Prompt,
          evaluator_content: evaluatorContent,
          input_data: {
            input_fields: inputFields,
          },
        });

        const error = res.evaluator_output_data?.evaluator_run_error;
        if (error) {
          throw new Error(error?.message);
        }

        return res.evaluator_output_data?.evaluator_result;
      }
    },
    {
      manual: true,
    },
  );

  useEffect(() => {
    if (initValue) {
      calcVariables.run();
    }
  }, []);

  return (
    <Modal
      className={styles.modal}
      visible={Boolean(initValue)}
      height="fill"
      width={'calc(100vw - 160px)'}
      closeOnEsc={false}
      title={
        <div className="flex flex-row items-center text-xl font-medium coz-fg-plus">
          {I18n.t('preview_and_debug')}
          <Tooltip content={I18n.t('construct_data_to_preview')}>
            <div className="w-4 h-4 ml-1">
              <IconCozInfoCircle className="w-4 h-4 coz-fg-secondary" />
            </div>
          </Tooltip>
        </div>
      }
      onCancel={() => {
        const values = evaluatorFormRef.current?.formApi?.getValues();
        values && onSubmit(values);
        onCancel();
      }}
    >
      <div className="h-full w-full overflow-hidden flex flex-row rounded-lg border border-solid coz-stroke-plus">
        <div className="w-1/2 flex flex-col border-0 border-r border-solid coz-stroke-plus">
          <div className="flex-shrink-0 h-9 px-4 coz-bg-secondary flex items-center text-sm coz-fg-plus font-semibold">
            {I18n.t('config_info')}
          </div>
          <div className="flex-1 overflow-y-auto px-4 pt-1 pb-6 styled-scrollbar pr-[10px]">
            <Form
              ref={evaluatorFormRef}
              initValues={initValue}
              onChange={calcVariables.run}
            >
              <ConfigContent disabled={guard2.data.readonly || disableConfig} />
            </Form>
          </div>
        </div>

        <div className="w-1/2 flex flex-col">
          <div className="flex-shrink-0 h-9 px-4 coz-bg-secondary flex items-center text-sm coz-fg-plus font-semibold">
            {I18n.t('construct_test_data')}
          </div>
          {variables.length ? (
            <div className="flex-1 overflow-hidden flex flex-col">
              <div className="flex-shrink overflow-y-auto mb-0 p-4 pb-2 styled-scrollbar pr-[10px]">
                <Form
                  ref={inputFormRef}
                  className={styles['input-form']}
                  disabled={guard2.data.readonly}
                >
                  {variables.map(variable =>
                    variable?.type === VariableType.MultiPart ? (
                      <FormMultiPartEdit
                        key={variable?.key}
                        variable={variable}
                        field={variable?.key ?? ''}
                        noLabel
                      />
                    ) : (
                      <FormInput
                        key={variable?.key}
                        label={
                          <div className="text-xs coz-fg-plus font-bold ml-3">
                            {variable?.key}
                          </div>
                        }
                        labelPosition="inset"
                        field={variable?.key ?? ''}
                        className="w-full"
                      />
                    ),
                  )}
                </Form>
              </div>
              <div className="p-4 flex-shrink-0 flex-grow flex flex-col pt-0 pb-2">
                {guard.data.readonly ? (
                  <BenefitBanner
                    className="mb-3 !rounded-[6px]"
                    closable={false}
                    scene={BenefitBannerScene.EvaluatorDebug}
                  />
                ) : (
                  <BenefitBaseBanner
                    className="mb-3 !rounded-[6px]"
                    description={I18n.t('testrun_require_fee')}
                  />
                )}

                <div className="flex-shrink-0 flex flex-row gap-2 justify-end">
                  <Button
                    color="primary"
                    onClick={() => {
                      inputFormRef.current?.formApi?.setValues(
                        {},
                        {
                          isOverride: true,
                        },
                      );
                    }}
                  >
                    {I18n.t('clear')}
                  </Button>

                  <Guard
                    point={GuardPoint['eval.evaluator_create.debug']}
                    realtime
                  >
                    <Button
                      icon={<IconCozPlayCircle />}
                      loading={service.loading}
                      onClick={service.run}
                    >
                      {I18n.t('run')}
                    </Button>
                  </Guard>
                </div>
                <div className="flex-grow"></div>
                {service.error || service.data ? (
                  <EvaluatorTestRunResult
                    errorMsg={service.error?.message}
                    evaluatorResult={service.data}
                    className="!bg-white"
                  />
                ) : null}
              </div>
              <div className="self-center text-[var(--coz-fg-dim)] text-xs leading-4 mb-6">
                {I18n.t('generated_by_ai_tip')}
              </div>
            </div>
          ) : (
            <EmptyState
              size="full_screen"
              icon={<IconCozIllusEmpty />}
              title={I18n.t('evaluator_lacks_input')}
            />
          )}
        </div>
      </div>
    </Modal>
  );
}
