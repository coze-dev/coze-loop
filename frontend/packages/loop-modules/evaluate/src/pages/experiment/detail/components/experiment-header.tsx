// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode, useState } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { TypographyText } from '@cozeloop/shared-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, Guard } from '@cozeloop/guard';
import {
  RefreshButton,
  ExperimentRunStatus,
} from '@cozeloop/evaluate-components';
import { EditIconButton } from '@cozeloop/components';
import { RouteBackAction } from '@cozeloop/base-with-adapter-components';
import { type Experiment } from '@cozeloop/api-schema/evaluation';
import { Divider, Tag } from '@coze-arch/coze-design';

import { isTraceTargetExpr } from '@/utils/experiment';
import {
  CreateContrastExperiment,
  ExperimentInfoEditFormModal,
  ExportMenu,
  RetryButton,
  KillButton,
} from '@/components/experiment';

export default function ExperimentHeader({
  experiment,
  spaceID,
  onRefreshExperiment,
  onRefresh,
  defaultModuleRoute = 'evaluation/experiments',
  defaultContrastRoute,
  renderExtraButtons,
}: {
  experiment?: Experiment;
  spaceID: string;
  onRefreshExperiment?: () => void;
  onRefresh?: () => void;
  defaultModuleRoute?: string;
  defaultContrastRoute?: string;
  renderExtraButtons?: (experiment?: Experiment) => ReactNode;
}) {
  const [editModalVisible, setEditModalVisible] = useState(false);
  const isTraceTarget = isTraceTargetExpr(experiment);
  const { name, status, expt_stats, id } = experiment ?? {};
  const {
    success_turn_cnt,
    fail_turn_cnt,
    terminated_turn_cnt,
    processing_turn_cnt,
    pending_turn_cnt,
  } = expt_stats ?? {};
  const totalCount =
    Number(success_turn_cnt ?? 0) +
    Number(fail_turn_cnt ?? 0) +
    Number(terminated_turn_cnt ?? 0) +
    Number(pending_turn_cnt ?? 0) +
    Number(processing_turn_cnt ?? 0);
  return (
    <header className="flex items-center shrink-0 h-14 px-6 gap-2 text-xs py-3">
      <RouteBackAction defaultModuleRoute={defaultModuleRoute} />
      <div className="flex items-center h-6">
        <div className="text-[16px] font-bold max-w-[240px]">
          <TypographyText className="!coz-fg-plus !font-medium !text-[18px] !leading-[22px]">
            {name}
          </TypographyText>
        </div>
        <Guard point={GuardPoint['eval.experiment.edit_meta']}>
          <EditIconButton
            className="ml-1 mr-3"
            onClick={() => setEditModalVisible(true)}
          />
        </Guard>

        <ExperimentRunStatus
          status={status}
          size="small"
          experiment={experiment}
          enableOnClick={false}
        />

        <Tag color="primary" size="small" className="ml-2">
          {I18n.t('total_number')} {totalCount || 0} ({I18n.t('success')}{' '}
          {success_turn_cnt}
          <Divider
            layout="vertical"
            style={{ marginLeft: 8, marginRight: 8, height: 12 }}
          />
          {I18n.t('failure')} {fail_turn_cnt}
          <Divider
            layout="vertical"
            style={{ marginLeft: 8, marginRight: 8, height: 12 }}
          />
          {terminated_turn_cnt ? (
            <>
              {I18n.t('abort')} {terminated_turn_cnt}
              <Divider
                layout="vertical"
                style={{ marginLeft: 8, marginRight: 8, height: 12 }}
              />
            </>
          ) : null}
          {processing_turn_cnt ? (
            <>
              {I18n.t('status_running')} {processing_turn_cnt}
              <Divider
                layout="vertical"
                style={{ marginLeft: 8, marginRight: 8, height: 12 }}
              />
            </>
          ) : null}
          {I18n.t('to_be_executed')} {pending_turn_cnt})
        </Tag>
      </div>

      <div className="flex items-center gap-2 ml-auto">
        <RefreshButton onRefresh={onRefresh} />
        <ExportMenu experiment={experiment} source="expt_detail" />
        <RetryButton
          spaceID={spaceID}
          status={status}
          expt_id={id}
          onRefresh={onRefresh}
        />

        <KillButton
          spaceID={spaceID}
          status={status}
          expt_id={id}
          onRefresh={onRefresh}
        />

        <CreateContrastExperiment
          baseExperiment={experiment}
          disabled={isTraceTarget}
          onClick={() => {
            sendEvent(EVENT_NAMES.cozeloop_experimen_open_compare_modal, {
              from: 'detail',
            });
          }}
          onReportCompare={s => {
            sendEvent(EVENT_NAMES.cozeloop_experiment_compare_count, {
              from: 'expt_detail',
              status: s ?? 'success',
            });
          }}
          defaultContrastRoute={defaultContrastRoute}
        />

        {renderExtraButtons?.(experiment)}
      </div>
      {editModalVisible ? (
        <ExperimentInfoEditFormModal
          visible={editModalVisible}
          spaceID={spaceID}
          experiment={experiment}
          onClose={() => setEditModalVisible(false)}
          onSuccess={onRefreshExperiment}
        />
      ) : null}
    </header>
  );
}
