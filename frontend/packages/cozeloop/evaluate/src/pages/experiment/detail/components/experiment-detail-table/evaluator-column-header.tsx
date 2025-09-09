// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { type ColumnEvaluator } from '@cozeloop/api-schema/evaluation';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { type TagProps, Tooltip } from '@coze-arch/coze-design';

import { EvaluatorColumnPreview } from '@/components/experiment';

export const EvaluatorColumnHeader = ({
  evaluator,
  tagProps,
}: {
  evaluator: ColumnEvaluator | undefined;
  tagProps?: TagProps;
}) => {
  const hasDesc = Boolean(evaluator?.description);
  return (
    <div className="flex items-center justify-end overflow-hidden">
      <EvaluatorColumnPreview evaluator={evaluator} tagProps={tagProps} />
      {hasDesc ? (
        <Tooltip content={evaluator?.description} theme="dark">
          <div className="ml-1 flex items-center justify-center w-5 shrink-0 cursor-pointer text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]">
            <IconCozInfoCircle />
          </div>
        </Tooltip>
      ) : null}
      {/* 这里是占位为了让标题和分数右对齐 */}
      <div className={classNames('shrink-0', hasDesc ? 'w-5' : 'w-10')} />
    </div>
  );
};
