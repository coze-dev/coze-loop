// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { useMemo } from 'react';

import classNames from 'classnames';
import { TypographyText } from '@cozeloop/shared-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { JumpIconButton } from '@cozeloop/components';
import { useOpenWindow } from '@cozeloop/biz-hooks-adapter';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tag, Tooltip, type TagProps } from '@coze-arch/coze-design';

import { getEvaluatorJumpUrlV2 } from '../evaluator/utils';
import EvaluatorIcon from '../evaluator/evaluator-icon';

/** 评估器预览 */
export function EvaluatorPreview({
  evaluator,
  tagProps = {},
  enableLinkJump,
  defaultShowLinkJump,
  enableDescTooltip,
  className = '',
  style,
  nameStyle,
}: {
  evaluator: Evaluator | undefined;
  tagProps?: TagProps;
  enableLinkJump?: boolean;
  defaultShowLinkJump?: boolean;
  enableDescTooltip?: boolean;
  className?: string;
  style?: React.CSSProperties;
  nameStyle?: React.CSSProperties;
}) {
  const { name, current_version, evaluator_id, evaluator_type, builtin } =
    evaluator ?? {};

  const jumpUrl = useMemo(() => getEvaluatorJumpUrlV2(evaluator), [evaluator]);

  const { openBlank } = useOpenWindow();
  if (!evaluator) {
    return <>-</>;
  }

  return (
    <div
      className={`group inline-flex items-center gap-1 cursor-pointer max-w-[100%] ${className}`}
      style={style}
      onClick={e => {
        if (enableLinkJump && evaluator_id) {
          e.stopPropagation();
          openBlank(jumpUrl);
        }
      }}
    >
      <EvaluatorIcon evaluatorType={evaluator_type} />
      <TypographyText style={nameStyle}>{name ?? '-'}</TypographyText>
      {current_version?.version ? (
        <Tag
          size="small"
          color="primary"
          {...tagProps}
          className={classNames('shrink-0 font-normal', tagProps.className)}
        >
          {builtin ? 'latest' : current_version?.version}
        </Tag>
      ) : null}
      {enableLinkJump ? (
        <Tooltip theme="dark" content={I18n.t('view_detail')}>
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
