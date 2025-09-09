// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ExperimentTurnPayload } from '@cozeloop/api-schema/evaluation';
import { IconCozClock, IconCozText } from '@coze-arch/coze-design/icons';
import { Divider } from '@coze-arch/coze-design';

import ExperimentItemRunStatus from './previews/experiment-item-run-status';

export default function ExperimentRunDataSummary({
  result,
  latencyHidden = false,
  tokenHidden = false,
  statusHidden = false,
}: {
  result: ExperimentTurnPayload | undefined;
  latencyHidden?: boolean;
  tokenHidden?: boolean;
  statusHidden?: boolean;
}) {
  return (
    <div className="flex items-center gap-2 text-xs text-[var(--coz-fg-secondary)]">
      {latencyHidden ? null : (
        <div className="flex items-center gap-1">
          <IconCozClock />
          <span>{3400}ms</span>
        </div>
      )}
      {tokenHidden || latencyHidden ? null : (
        <Divider layout="vertical" style={{ height: 12 }} />
      )}
      {tokenHidden ? null : (
        <div className="flex items-center gap-1">
          <IconCozText />
          <span>{1000}</span>
        </div>
      )}
      {statusHidden || (latencyHidden && tokenHidden) ? null : (
        <Divider layout="vertical" style={{ height: 12 }} />
      )}
      {statusHidden ? null : (
        <div className="flex items-center gap-1">
          <ExperimentItemRunStatus
            status={result?.system_info?.turn_run_state}
            useTag={false}
          />
        </div>
      )}
    </div>
  );
}
