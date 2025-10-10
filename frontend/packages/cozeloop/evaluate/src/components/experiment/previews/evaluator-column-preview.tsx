// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  EvaluatorIcon,
  getEvaluatorJumpUrl,
  TypographyText,
} from '@cozeloop/evaluate-components';
import { JumpIconButton } from '@cozeloop/components';
import { useBaseURL, useOpenWindow } from '@cozeloop/biz-hooks-adapter';
import { type ColumnEvaluator } from '@cozeloop/api-schema/evaluation';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tag, Tooltip, type TagProps } from '@coze-arch/coze-design';

/** 评测集预览 */
export default function EvaluatorColumnPreview({
  evaluator,
  tagProps = {},
  enableLinkJump,
  defaultShowLinkJump,
  enableDescTooltip,
  className = '',
  style,
}: {
  evaluator: ColumnEvaluator | undefined;
  tagProps?: TagProps;
  enableLinkJump?: boolean;
  defaultShowLinkJump?: boolean;
  enableDescTooltip?: boolean;
  className?: string;
  style?: React.CSSProperties;
}) {
  const { name, version, evaluator_id, evaluator_version_id, evaluator_type } =
    evaluator ?? {};

  const jumpUrl = useMemo(
    () =>
      getEvaluatorJumpUrl({
        evaluatorType: evaluator_type,
        evaluatorId: evaluator_id,
        evaluatorVersionId: evaluator_version_id,
      }),
    [evaluator_type, evaluator_id, evaluator_version_id],
  );

  const { openBlank } = useOpenWindow();
  if (!evaluator) {
    return <>-</>;
  }
  return (
    <div
      className={`group inline-flex items-center overflow-hidden gap-1 max-w-[100%] ${className}`}
      style={style}
      onClick={e => {
        if (enableLinkJump && evaluator_id) {
          e.stopPropagation();
          openBlank(jumpUrl);
        }
      }}
    >
      <EvaluatorIcon evaluatorType={evaluator_type} iconSize={14} />
      <TypographyText>{name ?? '-'}</TypographyText>
      <Tag
        color="primary"
        size="small"
        {...tagProps}
        className={classNames('shrink-0', tagProps.className)}
      >
        {version}
      </Tag>
      {enableLinkJump ? (
        <Tooltip theme="dark" content={I18n.t('detail')}>
          <div>
            <JumpIconButton
              className={defaultShowLinkJump ? '' : '!hidden group-hover:!flex'}
            />
          </div>
        </Tooltip>
      ) : null}
      {enableDescTooltip && evaluator?.description ? (
        <Tooltip theme="dark" content={evaluator?.description}>
          <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
        </Tooltip>
      ) : null}
    </div>
  );
}
