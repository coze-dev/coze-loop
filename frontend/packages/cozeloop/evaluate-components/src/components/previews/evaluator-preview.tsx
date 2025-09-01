import classNames from 'classnames';
import { JumpIconButton } from '@cozeloop/components';
import { useBaseURL } from '@cozeloop/biz-hooks-adapter';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Tag, Tooltip, type TagProps } from '@coze-arch/coze-design';

import { TypographyText } from '../text-ellipsis';

/** 评测集预览 */
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
  const { name, current_version, evaluator_id } = evaluator ?? {};
  const { baseURL } = useBaseURL();
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
          window.open(
            `${baseURL}/evaluation/evaluators/${evaluator_id}?version=${current_version?.id}`,
          );
        }
      }}
    >
      <TypographyText style={nameStyle}>{name ?? '-'}</TypographyText>
      {current_version?.version ? (
        <Tag
          size="small"
          color="primary"
          {...tagProps}
          className={classNames('shrink-0 font-normal', tagProps.className)}
        >
          {current_version?.version}
        </Tag>
      ) : null}
      {enableLinkJump ? (
        <Tooltip theme="dark" content="查看详情">
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
