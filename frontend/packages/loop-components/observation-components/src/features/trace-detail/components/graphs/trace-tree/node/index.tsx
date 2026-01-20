// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
import { isEmpty } from 'lodash-es';
import cls from 'classnames';
import { SpanStatus, SpanType } from '@cozeloop/api-schema/observation';
import {
  IconCozArrowDown,
  IconCozClock,
  IconCozSuccessRate,
} from '@coze-arch/coze-design/icons';
import { Tag, Tooltip, Typography } from '@coze-arch/coze-design';

import { formatTime } from '@/shared/utils/time';
import { useLocale } from '@/i18n';
import { getNodeConfig } from '@/features/trace-detail/utils/span';
import {
  BROKEN_ROOT_SPAN_ID,
  CustomIconWrapper,
  NODE_CONFIG_MAP,
  NORMAL_BROKEN_SPAN_ID,
} from '@/features/trace-detail/constants/span';
import { ReactComponent as TokenTextIcon } from '@/assets/icons/token-text.svg';

import { type SpanNode } from '../type';
import { type TreeNodeExtra } from '../../tree/typing';

import styles from './index.module.less';

interface CustomTreeNodeProps {
  nodeData: TreeNodeExtra;
  onCollapseChange: (id: string) => void;
}

export const CustomTreeNode = ({
  nodeData,
  onCollapseChange,
}: CustomTreeNodeProps) => {
  const { selected, isCurrentNodeOrChildSelected, lineStyle } = nodeData;
  const { spanNode } = nodeData?.extra as {
    spanNode: SpanNode;
  };
  const {
    status,
    span_name,
    duration,
    type,
    custom_tags: { tokens, input_tokens, output_tokens } = {},
    span_id,
    isCollapsed,
    isLeaf,
    children,
    span_type,
  } = spanNode;
  const { t } = useLocale();

  const hasChildren = children?.length && children?.length > 0;
  const isBroken = [BROKEN_ROOT_SPAN_ID, NORMAL_BROKEN_SPAN_ID].includes(
    span_id,
  );
  const nodeConfig = getNodeConfig({
    spanTypeEnum: type ?? 'unknown',
    spanType: span_type,
  });
  const lineColor =
    isCurrentNodeOrChildSelected && !selected
      ? lineStyle?.select?.stroke
      : lineStyle?.normal?.stroke;
  const timeColor =
    Number(duration) > 60000
      ? '#D0292F'
      : Number(duration) > 10000
        ? '#CC8533'
        : '#757a8c';
  const reasoningTokens = spanNode.custom_tags?.reasoning_tokens;
  return (
    <div
      className={cls(
        'flex flex-col gap-[2px] h-[40px]  pt-[6px] pl-[4px] justify-start ',
        styles['node-container'],
      )}
    >
      <div className="flex items-center pt-[4px]">
        <span className={styles['icon-wrapper']}>
          {nodeConfig.icon ? (
            nodeConfig.icon({})
          ) : (
            <CustomIconWrapper color={nodeConfig.color} size={'small'}>
              {nodeConfig.character}
            </CustomIconWrapper>
          )}
        </span>
        <div
          className={cls(styles['trace-tree-node'], {
            [styles.error]: status !== SpanStatus.Success,
            [styles.disabled]: isBroken,
          })}
        >
          <Typography.Text
            className={styles.title}
            ellipsis={{
              showTooltip: true,
            }}
          >
            {span_name}
          </Typography.Text>
        </div>
        {type !== SpanType.Unknown && Boolean(NODE_CONFIG_MAP[type]) && (
          <Tag color="primary" className="m-w-full !px-1 h-[20px]" size="small">
            {NODE_CONFIG_MAP[type].typeName}
          </Tag>
        )}
        <Tag
          type="light"
          className="m-w-full !h-4 !px-1  !bg-transparent"
          prefixIcon={
            <IconCozClock
              style={{ color: timeColor }}
              className="!w-[12px] !h-[12px]"
            />
          }
        >
          <span style={{ color: timeColor }} className="text-[12px]">
            {formatTime(Number(duration))}
          </span>
        </Tag>
        {/* tokens */}
        {tokens !== undefined &&
        Number(tokens) !== 0 &&
        (span_type === 'model' || span_type === 'LLMCall') ? (
          <Tooltip
            theme="dark"
            content={
              <>
                {input_tokens !== undefined && (
                  <div>
                    {t('fornax_analytics_input_tokens_count', {
                      count: Number(input_tokens),
                    })}
                  </div>
                )}
                {output_tokens !== undefined && (
                  <div>
                    {t('fornax_analytics_output_tokens_count', {
                      count: Number(output_tokens),
                    })}
                  </div>
                )}
                {reasoningTokens !== undefined && (
                  <div>
                    {t('fornax_analytics_reasoning_tokens_count', {
                      count: Number(reasoningTokens),
                    })}
                  </div>
                )}
              </>
            }
          >
            <Tag
              color="primary"
              className="m-w-full !h-4 !px-1 !bg-transparent !coz-fg-secondary"
              prefixIcon={
                <TokenTextIcon className="w-[12px] h-[12px] box-border" />
              }
            >
              {Number(tokens)}
            </Tag>
          </Tooltip>
        ) : null}
        {!isEmpty(spanNode.annotations) ? (
          <Tag
            color="primary"
            className="m-w-full !h-4 !px-1 !bg-transparent"
            prefixIcon={
              <IconCozSuccessRate className="w-[12px] h-[12px] box-border !text-[var(--coz-fg-secondary)]" />
            }
          >
            <span className="text-[var(--coz-fg-secondary)]">feedback</span>
          </Tag>
        ) : null}
        {!isLeaf && (
          <Tooltip
            theme="dark"
            content={
              isCollapsed
                ? t('fornax_analytics_extend')
                : t('fornax_analytics_collapse')
            }
            position="right"
          >
            <div
              style={{
                transform: `rotate(${isCollapsed ? -90 : 0}deg)`,
                transition: 'transform 0.2s',
              }}
              className="flex items-center justify-center ml-1"
              onClick={e => {
                e.stopPropagation();
                onCollapseChange(span_id);
              }}
            >
              <IconCozArrowDown className="coz-fg-secondary" />
            </div>
          </Tooltip>
        )}
      </div>
      <div className="flex">
        <div className="w-[16px] h-[12px] flex justify-center">
          {hasChildren && !isCollapsed ? (
            <div
              className="w-[1px] coz-fg-dim"
              style={{
                backgroundColor: lineColor,
              }}
            ></div>
          ) : null}
        </div>
      </div>
    </div>
  );
};
