// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import classNames from 'classnames';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { EvaluatorManualScore } from '@cozeloop/shared-components';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  TraceTrigger,
  EvaluatorResultPanel,
  useGlobalEvalConfig,
} from '@cozeloop/evaluate-components';
import { IconButtonContainer } from '@cozeloop/components';
import {
  type Experiment,
  type EvaluatorRecord,
} from '@cozeloop/api-schema/evaluation';
import { IconCozPencil } from '@coze-arch/coze-design/icons';
import { Popover, Toast, Tooltip } from '@coze-arch/coze-design';

// eslint-disable-next-line complexity
export default function EvaluatorScore({
  spaceID,
  traceID,
  align = 'left',
  evaluatorRecordID,
  evaluatorRecord,
  experiment,
  onRefresh,
}: {
  spaceID: Int64;
  traceID: string;
  evaluatorRecordID: Int64;
  evaluatorRecord: EvaluatorRecord | undefined;
  experiment: Experiment | undefined;
  align?: 'left' | 'right';
  onRefresh?: () => void;
}) {
  const [visible, setVisible] = useState(false);
  const [panelVisible, setPanelVisible] = useState(false);
  const { traceEvaluatorPlatformType } = useGlobalEvalConfig();
  const evaluatorResult =
    evaluatorRecord?.evaluator_output_data?.evaluator_result;
  const { score, correction } = evaluatorResult ?? {};
  const hasResult =
    evaluatorResult?.score !== undefined || correction?.score !== undefined;
  const hasCorrection = correction?.score !== undefined;

  const report = () => {
    sendEvent(EVENT_NAMES.cozeloop_experiment_detailsdrawer_trace, {
      from: 'experiment_detail_evaluator_score',
    });
  };
  return (
    <div
      onClick={e => e.stopPropagation()}
      className={classNames(
        'group flex h-5 items-center gap-1',
        align === 'right' ? 'justify-end' : '',
      )}
    >
      {hasResult ? (
        <Popover
          contentClassName="max-h-[500px] overflow-auto"
          stopPropagation={true}
          position="left"
          content={
            <EvaluatorResultPanel
              result={evaluatorResult}
              updateUser={evaluatorRecord?.base_info?.updated_by}
              evaluatorManualScoreProps={{
                spaceID,
                evaluatorRecordID: evaluatorRecordID ?? '',
                visible: panelVisible,
                onVisibleChange: setPanelVisible,
                onSuccess: () => {
                  setPanelVisible(false);
                  Toast.success(I18n.t('update_rating_success'));
                  onRefresh?.();
                },
              }}
            />
          }
          showArrow
        >
          <div className="underline decoration-dotted underline-offset-2 decoration-[var(--coz-fg-secondary)] relative">
            {correction?.score ?? score ?? '-'}
            {hasCorrection ? (
              <div className="absolute right-0 top-[6px] translate-x-[5px] w-1 h-1 rounded-full z-10 bg-[rgb(var(--coze-up-brand-9))]" />
            ) : null}
          </div>
        </Popover>
      ) : (
        '-'
      )}
      <div
        className={classNames(
          'evaluator-score-actions w-10 shrink-0 flex items-center invisible group-hover:visible',
          visible ? '!visible' : '',
        )}
      >
        {hasResult ? (
          <EvaluatorManualScore
            spaceID={spaceID}
            evaluatorRecordID={evaluatorRecordID}
            visible={visible}
            onVisibleChange={setVisible}
            onSuccess={() => {
              setVisible(false);
              Toast.success(I18n.t('update_rating_success'));
              onRefresh?.();
            }}
          >
            <div onClick={() => setVisible(true)}>
              <Tooltip theme="dark" content={I18n.t('manual_calibration')}>
                <div className="h-5">
                  <IconButtonContainer
                    icon={<IconCozPencil />}
                    active={visible}
                  />
                </div>
              </Tooltip>
            </div>
          </EvaluatorManualScore>
        ) : null}
        {traceID ? (
          <div className="h-5" onClick={report}>
            <TraceTrigger
              traceID={traceID}
              platformType={traceEvaluatorPlatformType}
              startTime={experiment?.start_time}
              endTime={experiment?.end_time}
              tooltipProps={{
                content: I18n.t('view_evaluator_trace'),
                theme: 'dark',
              }}
            />
          </div>
        ) : null}
      </div>
    </div>
  );
}
