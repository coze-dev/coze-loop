// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useRef, useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { IconButtonContainer, JumpIconButton } from '@cozeloop/components';
import { useBaseURL } from '@cozeloop/biz-hooks-adapter';
import {
  type EvaluatorResult,
  type UserInfo,
  type Experiment,
  type ColumnEvaluator,
} from '@cozeloop/api-schema/evaluation';
import { IconCozPencil } from '@coze-arch/coze-design/icons';
import { Divider, Popover, Tag, Toast, Tooltip } from '@coze-arch/coze-design';

import { useGlobalEvalConfig } from '@/stores/eval-global-config';

import { TypographyText } from '../text-ellipsis';
import { CozeUser } from '../common/coze-user';
import { TraceTrigger } from './trace-trigger';
import {
  EvaluatorManualScore,
  type CustomSubmitManualScore,
  type EvaluatorManualScoreProps,
} from './evaluator-manual-score';

interface NameScoreTagProps {
  name: string | undefined;
  evaluatorResult: EvaluatorResult | undefined;
  updateUser?: UserInfo;
  version: string | undefined;
  showVersion?: boolean;
  evaluatorID?: Int64;
  evaluatorVersionID?: Int64;
  evaluatorRecordID?: Int64;
  spaceID?: Int64;
  traceID?: Int64;
  startTime?: Int64;
  endTime?: Int64;
  enableLinkJump?: boolean;
  enableTrace?: boolean;
  enableEditScore?: boolean;
  enableCorrectScorePopover?: boolean;
  defaultShowAction?: boolean;
  border?: boolean;
  onSuccess?: () => void;
  onReportCalibration?: () => void;
  onReportEvaluatorTrace?: () => void;
  customSubmitManualScore?: (values: CustomSubmitManualScore) => Promise<void>;
}

export function EvaluatorResultPanel({
  result,
  updateUser,
}: {
  result: EvaluatorResult | undefined;
  updateUser: UserInfo | undefined;
  // 暂时不支持手动校准，后续支持
  evaluatorManualScoreProps: EvaluatorManualScoreProps;
}) {
  const { score, reasoning, correction } = result ?? {};
  return (
    <div className="w-80">
      <div className="font-bold mb-1 flex items-center">
        {I18n.t('evaluate_task_score')}
        {correction ? (
          <Tag
            color="brand"
            size="small"
            className="ml-1 rounded-[3px] font-normal"
          >
            {I18n.t('manual_calibration')}
          </Tag>
        ) : null}
      </div>
      {correction ? (
        <div className="flex items-center gap-1 mb-4">
          <div className="line-through text-[var(--coz-fg-dim)]">{score}</div>
          <div>{correction?.score}</div>
          <div className="ml-auto max-w-[160px] overflow-hidden">
            <CozeUser user={updateUser} />
          </div>
        </div>
      ) : (
        <div>{score}</div>
      )}
      <div className="mt-3">
        <div className="font-bold mb-1">{I18n.t('reason')}</div>
        <div>{(correction ? correction?.explain : reasoning) || '-'}</div>
      </div>
    </div>
  );
}

// eslint-disable-next-line complexity
export function EvaluatorNameScoreTag({
  name,
  evaluatorResult,
  updateUser,
  version,
  showVersion = true,
  evaluatorID,
  evaluatorVersionID,
  evaluatorRecordID,
  spaceID = '',
  traceID,
  startTime,
  endTime,
  enableLinkJump,
  enableTrace,
  enableEditScore,
  enableCorrectScorePopover,
  defaultShowAction = false,
  border = true,
  onSuccess,
  onReportCalibration,
  onReportEvaluatorTrace,
  customSubmitManualScore,
}: NameScoreTagProps) {
  const [visible, setVisible] = useState(false);
  const [panelVisible, setPanelVisible] = useState(false);
  const { baseURL } = useBaseURL();
  const scoreRef = useRef<HTMLDivElement>(null);
  const { traceEvaluatorPlatformType } = useGlobalEvalConfig();
  const { score, correction } = evaluatorResult ?? {};

  const borderClass = border
    ? 'border border-solid border-[var(--coz-stroke-primary)] cursor-pointer hover:bg-[var(--coz-mg-primary)] hover:border-[var(--coz-stroke-plus)]'
    : '';
  const scoreValue = correction?.score ?? score;
  const hasResult = scoreValue !== undefined;
  const hasCorrection = correction?.score !== undefined;
  const hasAction =
    enableLinkJump ||
    (enableTrace && traceID) ||
    (enableEditScore && hasResult);
  return (
    <div className={'group flex items-center text-[var(--coz-fg-primary)]'}>
      <div
        className={`flex items-center h-5 px-2 rounded-[3px] gap-1 text-xs font-medium ${borderClass}`}
      >
        <TypographyText className="max-w-10">{name ?? '-'}</TypographyText>
        {showVersion ? (
          <>
            <Tag size="mini" color="primary" className="shrink-0">
              {version ?? '-'}
            </Tag>
            <Divider layout="vertical" style={{ height: 12 }} />
          </>
        ) : null}

        {enableCorrectScorePopover && scoreValue !== undefined ? (
          <Popover
            showArrow
            position="top"
            stopPropagation
            content={
              <EvaluatorResultPanel
                result={evaluatorResult}
                updateUser={updateUser}
                evaluatorManualScoreProps={{
                  spaceID,
                  evaluatorRecordID: evaluatorRecordID ?? '',
                  visible: panelVisible,
                  onVisibleChange: setPanelVisible,
                  onSuccess: () => {
                    setPanelVisible(false);
                    onSuccess?.();
                  },
                }}
              />
            }
          >
            <div
              ref={scoreRef}
              className="underline decoration-dotted decoration-[var(--coz-fg-secondary)] underline-offset-2 relative"
            >
              {scoreValue}
              {hasCorrection ? (
                <div className="absolute right-0 top-1 translate-x-[5px] w-1 h-1 rounded-full z-10 bg-[rgb(var(--coze-up-brand-9))]" />
              ) : null}
            </div>
          </Popover>
        ) : (
          (scoreValue ?? '-')
        )}
      </div>
      <div className={classNames('flex items-center', hasAction ? 'ml-1' : '')}>
        {enableLinkJump ? (
          <Tooltip theme="dark" content={I18n.t('view_evaluator_details')}>
            <div className="flex items-center">
              <JumpIconButton
                className={defaultShowAction ? '' : 'hidden group-hover:flex'}
                onClick={() => {
                  window.open(
                    `${baseURL}/evaluation/evaluators/${evaluatorID}?version=${evaluatorVersionID}`,
                  );
                }}
              />
            </div>
          </Tooltip>
        ) : null}
        {enableTrace && traceID ? (
          <div
            className="flex items-center"
            onClick={() => onReportEvaluatorTrace?.()}
          >
            <TraceTrigger
              traceID={traceID ?? ''}
              className={defaultShowAction ? '' : 'hidden group-hover:flex'}
              platformType={traceEvaluatorPlatformType}
              startTime={startTime}
              endTime={endTime}
              tooltipProps={{
                content: I18n.t('view_evaluator_trace'),
                theme: 'dark',
              }}
            />
          </div>
        ) : null}
        {enableEditScore && hasResult ? (
          <EvaluatorManualScore
            spaceID={spaceID}
            evaluatorRecordID={evaluatorRecordID ?? ''}
            visible={visible}
            onVisibleChange={setVisible}
            customSubmitManualScore={customSubmitManualScore}
            onSuccess={() => {
              setVisible(false);
              Toast.success(I18n.t('update_score_successful'));
              onSuccess?.();
            }}
          >
            <div className="flex items-center">
              <Tooltip theme="dark" content={I18n.t('manual_calibration')}>
                <div
                  className={
                    defaultShowAction ? 'h-5' : 'h-5 !hidden group-hover:!flex'
                  }
                  onClick={() => {
                    onReportCalibration?.();
                  }}
                >
                  <IconButtonContainer
                    icon={<IconCozPencil />}
                    active={visible}
                  />
                </div>
              </Tooltip>
            </div>
          </EvaluatorManualScore>
        ) : null}
      </div>
    </div>
  );
}

export function EvaluatorNameScore({
  evaluator,
  evaluatorResult,
  experiment,
  updateUser,
  spaceID,
  traceID,
  evaluatorRecordID,
  enablePopover = false,
  enableEditScore = true,
  showVersion,
  border = true,
  defaultShowAction,
  popoverNameScoreTagProps = {},
  onEditScoreSuccess,
  onReportCalibration,
  onReportEvaluatorTrace,
}: {
  evaluator: ColumnEvaluator | undefined;
  experiment: Experiment | undefined;
  evaluatorResult: EvaluatorResult | undefined;
  updateUser?: UserInfo;
  spaceID?: Int64;
  traceID?: Int64;
  evaluatorRecordID?: Int64;
  enablePopover?: boolean;
  enableEditScore?: boolean;
  border?: boolean;
  showVersion?: boolean;
  defaultShowAction?: boolean;
  popoverNameScoreTagProps?: Pick<
    NameScoreTagProps,
    'enableLinkJump' | 'enableTrace' | 'enableEditScore'
  >;
  onEditScoreSuccess?: () => void;
  onReportCalibration?: () => void;
  onReportEvaluatorTrace?: () => void;
}) {
  const {
    evaluator_id,
    name,
    version,
    evaluator_version_id: versionId,
  } = evaluator ?? {};

  if (!enablePopover) {
    return (
      <EvaluatorNameScoreTag
        name={name}
        evaluatorResult={evaluatorResult}
        updateUser={updateUser}
        version={version}
        evaluatorID={evaluator_id}
        evaluatorVersionID={versionId}
        evaluatorRecordID={evaluatorRecordID}
        spaceID={spaceID}
        traceID={traceID}
        startTime={experiment?.start_time}
        endTime={experiment?.end_time}
        enableLinkJump={true}
        enableTrace={true}
        enableEditScore={enableEditScore}
        enableCorrectScorePopover={true}
        defaultShowAction={defaultShowAction}
        border={border}
        showVersion={showVersion}
        onSuccess={onEditScoreSuccess}
        onReportCalibration={onReportCalibration}
        onReportEvaluatorTrace={onReportEvaluatorTrace}
      />
    );
  }
  return (
    <Popover
      position="top"
      trigger="click"
      stopPropagation
      content={
        <div className="p-1" style={{ color: 'var(--coz-fg-secondary)' }}>
          <EvaluatorNameScoreTag
            name={name}
            evaluatorResult={evaluatorResult}
            updateUser={updateUser}
            version={version}
            evaluatorID={evaluator_id}
            evaluatorVersionID={versionId}
            evaluatorRecordID={evaluatorRecordID}
            spaceID={spaceID}
            traceID={traceID}
            startTime={experiment?.start_time}
            endTime={experiment?.end_time}
            enableLinkJump={true}
            enableTrace={true}
            enableEditScore={enableEditScore}
            defaultShowAction={true}
            enableCorrectScorePopover={true}
            border={false}
            onSuccess={onEditScoreSuccess}
            onReportCalibration={onReportCalibration}
            onReportEvaluatorTrace={onReportEvaluatorTrace}
            {...popoverNameScoreTagProps}
          />
        </div>
      }
    >
      <div>
        <EvaluatorNameScoreTag
          name={name}
          evaluatorResult={evaluatorResult}
          updateUser={updateUser}
          version={version}
          border={border}
          showVersion={showVersion}
          evaluatorID={evaluator_id}
          evaluatorVersionID={versionId}
          evaluatorRecordID={evaluatorRecordID}
          spaceID={spaceID}
          traceID={traceID}
          startTime={experiment?.start_time}
          endTime={experiment?.end_time}
          defaultShowAction={defaultShowAction}
          onReportCalibration={onReportCalibration}
          onReportEvaluatorTrace={onReportEvaluatorTrace}
          enableCorrectScorePopover={false}
        />
      </div>
    </Popover>
  );
}
