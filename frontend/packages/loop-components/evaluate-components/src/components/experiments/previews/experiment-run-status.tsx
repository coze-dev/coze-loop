// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { type Experiment, ExptStatus } from '@cozeloop/api-schema/evaluation';
import { Divider, Popover, Tag, type TagProps } from '@coze-arch/coze-design';

import {
  type ExperimentRunStatusInfo,
  experimentRunStatusInfoList,
} from '../../../constants/experiment-status';

const statusMap = experimentRunStatusInfoList.reduce(
  (prev, item) => ({ ...prev, [item.status]: item }),
  {} as unknown as Record<string | number, ExperimentRunStatusInfo>,
);
/** 实验运行状态标签 */
// eslint-disable-next-line complexity
export function ExperimentRunStatus({
  status,
  showIcon = true,
  experiment,
  enableOnClick = true,
  showProcess,
  ...rest
}: {
  status: ExptStatus | undefined;
  showIcon?: boolean;
  experiment?: Experiment | undefined;
  enableOnClick?: boolean; // 是否显示执行进度
  showProcess?: boolean;
} & TagProps) {
  const statusInfo = statusMap[status ?? ''];
  const processing = status === ExptStatus.Processing;
  if (statusInfo) {
    const tagContent = (
      <Tag
        size="small"
        color={statusInfo.tagColor}
        prefixIcon={showIcon ? statusInfo.icon : null}
        onClick={enableOnClick ? e => e.stopPropagation() : undefined}
        {...rest}
      >
        <span
          className={
            showProcess && processing
              ? 'underline decoration-dotted underline-offset-2 decoration-[var(rgb(var(--coze-blue-50)))]'
              : ''
          }
        >
          {statusInfo.name}
        </span>
      </Tag>
    );

    if (showProcess && processing && experiment) {
      const {
        success_turn_cnt,
        fail_turn_cnt,
        pending_turn_cnt,
        processing_turn_cnt,
        terminated_turn_cnt,
      } = experiment?.expt_stats ?? {};
      const totalCount =
        Number(pending_turn_cnt) +
        Number(success_turn_cnt) +
        Number(fail_turn_cnt) +
        Number(processing_turn_cnt) +
        Number(terminated_turn_cnt);
      return (
        <Popover
          stopPropagation
          position="top"
          content={
            <div className="px-2 py-1">
              <div>
                {I18n.t('total_number')} {totalCount || 0}
              </div>
              <div>
                {I18n.t('success')} {success_turn_cnt}
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
                {I18n.t('to_be_executed')} {pending_turn_cnt}
              </div>
            </div>
          }
        >
          {tagContent}
        </Popover>
      );
    }
    return tagContent;
  }
  return '-';
}
