// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, useState } from 'react';

import classNames from 'classnames';
import { getEvaluatorJumpUrl } from '@cozeloop/evaluate-components';
import { useOpenWindow } from '@cozeloop/biz-hooks-adapter';
import { EvaluatorType } from '@cozeloop/api-schema/evaluation';
import {
  IconCozAiFill,
  IconCozArrowRight,
  IconCozCode,
} from '@coze-arch/coze-design/icons';
import { Tag } from '@coze-arch/coze-design';

import { type EvaluatorPro } from '@/types/experiment/experiment-create';

import { OpenDetailButton } from './open-detail-button';
import { EvaluatorContentRenderer } from './evaluator-content-renderer';

// eslint-disable-next-line complexity
export function EvaluateItemRender({
  evaluatorPro,
}: {
  evaluatorPro: EvaluatorPro;
}) {
  const [open, setOpen] = useState(true);
  const { evaluator } = evaluatorPro;
  const { evaluator_type } = evaluator ?? {};
  const { openBlank } = useOpenWindow();

  const icon = useMemo(
    () =>
      evaluator_type === EvaluatorType.Code ? (
        <IconCozCode style={{ marginRight: '2px' }} />
      ) : (
        <IconCozAiFill style={{ marginRight: '2px' }} />
      ),
    [evaluator_type],
  );

  const jumpUrl = useMemo(
    () =>
      getEvaluatorJumpUrl({
        evaluatorType: evaluator?.evaluator_type,
        evaluatorId: evaluator?.evaluator_id,
        evaluatorVersionId: evaluatorPro?.evaluatorVersion?.id,
      }),
    [
      evaluator?.evaluator_id,
      evaluator?.evaluator_type,
      evaluatorPro?.evaluatorVersion?.id,
    ],
  );

  return (
    <div className="border border-solid coz-stroke-primary rounded-[6px]">
      <div
        className="h-11 px-4 flex flex-row items-center coz-bg-primary cursor-pointer"
        onClick={() => setOpen(pre => !pre)}
      >
        <div className="flex flex-row items-center flex-1 text-sm font-semibold coz-fg-plus gap-1">
          <span className="truncate max-w-[698px]">
            {evaluatorPro?.evaluator?.name}
          </span>
          {evaluatorPro?.evaluatorVersion?.version ? (
            <Tag color="primary" className="!h-5 !px-2 !py-[2px] rounded-[3px]">
              {icon}
              {evaluatorPro.evaluatorVersion.version}
            </Tag>
          ) : null}

          <OpenDetailButton
            url={jumpUrl}
            customOpen={() => openBlank(jumpUrl)}
          />

          <IconCozArrowRight
            className={classNames(
              'h-4 w-4 coz-fg-primary transition-transform',
              open ? 'rotate-90' : '',
            )}
          />
        </div>
      </div>

      <div className={open ? 'p-4' : 'hidden'}>
        <EvaluatorContentRenderer
          evaluatorPro={evaluatorPro}
          evaluatorType={evaluator_type}
        />
      </div>
    </div>
  );
}
