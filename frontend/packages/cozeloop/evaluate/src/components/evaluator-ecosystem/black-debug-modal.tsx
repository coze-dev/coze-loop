// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { Guard, GuardPoint } from '@cozeloop/guard';
import { getSchemaDefaultValueObj } from '@cozeloop/evaluate-components/src/components/evaluator-ecosystem/utils';
import {
  EvaluatorTestRunResult,
  BlackSchemaEditorGroup,
} from '@cozeloop/evaluate-components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { BenefitBaseBanner } from '@cozeloop/biz-components-adapter';
import {
  type Evaluator,
  type EvaluatorType,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozInfoCircle,
  IconCozPlayCircle,
} from '@coze-arch/coze-design/icons';
import { Button, Modal, Tooltip } from '@coze-arch/coze-design';

export function BlackDebugModal(props: {
  visible?: boolean;
  evaluator?: Evaluator;
  onCancel: () => void;
  evaluatorType?: EvaluatorType;
}) {
  const { visible, onCancel, evaluator } = props;
  const { spaceID } = useSpace();
  const [value, setValue] = useState({ inputValue: '', outputValue: '' });
  const [debugError, setDebugError] = useState<string | undefined>(undefined);

  useEffect(() => {
    if (evaluator && visible) {
      const evaluatorContent = evaluator.current_version?.evaluator_content;
      const inputVs = getSchemaDefaultValueObj(evaluatorContent?.input_schemas);
      const outputVs = getSchemaDefaultValueObj(
        evaluatorContent?.output_schemas,
      );
      setValue({
        inputValue: JSON.stringify(inputVs || '', null, 2),
        outputValue: JSON.stringify(outputVs || '', null, 2),
      });
    }
  }, [evaluator, visible]);

  const service = useRequest(
    async () => {
      try {
        const res = await StoneEvaluationApi.DebugBuiltinEvaluator({
          workspace_id: spaceID,
          evaluator_id: evaluator?.evaluator_id || '',
          input_data: {
            input_fields: {
              ...JSON.parse(value.inputValue || '{}'),
            },
          },
        });

        const error = res.output_data?.evaluator_run_error;
        if (error) {
          throw new Error(error?.message);
        }

        return res.output_data?.evaluator_result;
      } catch (error) {
        return Promise.reject(error);
      }
    },
    {
      manual: true,
      onError: error => {
        // useRequest不会清空error, 错误需要自己管理
        setDebugError(error?.message);
      },
      refreshDeps: [visible],
    },
  );

  const handleOnCancel = () => {
    onCancel?.();
    setValue({ inputValue: '', outputValue: '' });
    setDebugError(undefined);
    service.mutate(undefined);
  };

  return (
    <Modal
      visible={visible}
      height="fill"
      width={'calc(100vw - 160px)'}
      closeOnEsc={false}
      keepDOM={false}
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
      onCancel={handleOnCancel}
    >
      <div className="h-full w-full overflow-hidden flex flex-col coz-stroke-plus">
        <BlackSchemaEditorGroup
          value={value}
          onChange={setValue}
          disableRightPanel={true}
        />
        {debugError || service.data ? (
          <EvaluatorTestRunResult
            errorMsg={debugError || ''}
            // errorMsg={debugError || service.error?.message}
            evaluatorResult={service.data}
            containerStyle={{
              marginTop: '16px',
              border:
                '1px solid var(--coz-stroke-primary, rgba(82, 100, 154, 0.13))',
            }}
            className="!bg-white"
          />
        ) : null}

        <div className="flex-shrink-0 flex-grow flex flex-col pt-0 pb-2">
          <BenefitBaseBanner
            className="mb-3 !rounded-[6px] mt-4"
            description={I18n.t('testrun_require_fee')}
          />

          <div className="flex-shrink-0 flex flex-row gap-2 justify-end mt-3">
            <Button
              color="primary"
              onClick={() => {
                setValue({ ...value, inputValue: '' });
              }}
            >
              {I18n.t('clear')}
            </Button>

            <Guard point={GuardPoint['eval.evaluator_create.debug']} realtime>
              <Button
                color="highlight"
                icon={<IconCozPlayCircle />}
                loading={service.loading}
                onClick={service.run}
              >
                {I18n.t('run')}
              </Button>
            </Guard>
          </div>
        </div>
      </div>
    </Modal>
  );
}
