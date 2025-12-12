// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { useState } from 'react';

import cls from 'classnames';
import { SpanStatus } from '@cozeloop/api-schema/observation';
import {
  IconCozCrossFill,
  IconCozArrowLeft,
  IconCozArrowRight,
  IconCozLink,
  IconCozExpand,
  IconCozCopy,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Typography,
  Divider,
  Tooltip,
  Tag,
  Popover,
} from '@coze-arch/coze-design';

import { BIZ } from '@/shared/constants';
import { useLocale } from '@/i18n';
import { useDetailCopy } from '@/features/trace-detail/hooks/use-detail-copy';
import { useCustomComponents } from '@/features/trace-detail/hooks/use-custom-components';
import {
  BROKEN_ROOT_SPAN_ID,
  NORMAL_BROKEN_SPAN_ID,
} from '@/features/trace-detail/constants/span';
import { useConfigContext } from '@/config-provider';
import { ReactComponent as IconCozShouqi } from '@/assets/icons/coz_shouqi.svg';

import { type TraceHeaderProps } from '../typing';

import styles from './index.module.less';

export const HorizontalTraceHeader = ({
  rootSpan,
  advanceInfo,
  className: propsClassName,
  showClose,
  onClose,
  switchConfig,
  extraRender,
  showFullscreenButton,
  onFullscreen,
  renderHeaderCopyNode,
}: TraceHeaderProps & {
  showTraceId?: boolean;
  showFullscreenButton?: boolean;
  onFullscreen?: () => void;
}) => {
  const { t } = useLocale();
  const { bizId } = useConfigContext();
  const { span_name = '', span_id, status } = rootSpan || {};
  const handleCopy = useDetailCopy();
  const isBroken = [NORMAL_BROKEN_SPAN_ID, BROKEN_ROOT_SPAN_ID].includes(
    span_id || '',
  );
  const [isFullscreen, setIsFullscreen] = useState(false);
  const { StatusSuccessIcon, StatusErrorIcon } = useCustomComponents();

  const traceName = isBroken ? 'Unknown Trace' : span_name;

  return (
    <div className={cls(styles['horizontal-header'], propsClassName)}>
      <div className={styles['trace-profile']}>
        <div className={styles.desc}>
          <div className={styles.name}>
            <Typography.Text
              ellipsis={{
                showTooltip: {
                  type: 'tooltip',
                  opts: {
                    position: 'bottom',
                    theme: 'dark',
                  },
                },
              }}
              className="coz-fg-plus text-[16px]"
            >
              {traceName}
            </Typography.Text>
          </div>
        </div>
        <Tag
          color={status === SpanStatus.Error ? 'red' : 'green'}
          className="flex items-center gap-x-2 font-medium"
        >
          {status === SpanStatus.Error ? (
            <StatusErrorIcon />
          ) : (
            <StatusSuccessIcon />
          )}
          <span className="ml-1">
            {status === SpanStatus.Error ? 'Error' : 'Success'}
          </span>
        </Tag>
        <Tag color="primary" className="flex items-center font-medium">
          <span> Latency: {rootSpan?.duration} ms</span>
          <Divider
            layout="vertical"
            margin="4px"
            className="!h-[12px] !w-[1px] coz-fg-secondary bg-[var(--coz-fg-secondary)]"
          />
          <span>
            Tokens:
            {(
              Number(advanceInfo?.tokens?.input ?? 0) +
              Number(advanceInfo?.tokens?.output ?? 0)
            ).toLocaleString()}
          </span>
        </Tag>

        {extraRender ? extraRender(rootSpan) : null}
      </div>
      <div className="flex justify-end items-center">
        {showFullscreenButton ? (
          <Tooltip
            content={!isFullscreen ? t('fullscreen') : t('exit_fullscreen')}
            theme="dark"
          >
            <Button
              color="secondary"
              className="coz-fg-secondary"
              icon={
                isFullscreen ? (
                  <IconCozShouqi className="w-[16px] h-[16px]" />
                ) : (
                  <IconCozExpand className="w-[16px] h-[16px]" />
                )
              }
              onClick={() => {
                setIsFullscreen(!isFullscreen);
                onFullscreen?.();
              }}
            />
          </Tooltip>
        ) : null}
        {bizId === BIZ.Cozeloop || bizId === BIZ.Fornax ? (
          <Tooltip content={t('copy_detail_link')} theme="dark">
            <Button
              color="secondary"
              className="coz-fg-secondary"
              icon={<IconCozLink className="w-[16px] h-[16px]" />}
              onClick={() => {
                handleCopy(window.location.href);
              }}
            />
          </Tooltip>
        ) : null}
        <Popover
          trigger="hover"
          position="bottomLeft"
          showArrow={false}
          className="!border-none !rounded-[6px]"
          contentClassName="!border-none"
          content={
            <div className="flex min-w-[240px] flex-col gap-y-[2px] p-4 gap-4">
              <div>
                <div className="flex gap-2 items-center">
                  <span className="font-medium">Trace ID </span>
                  <div
                    className="coz-fg-secondary cursor-pointer !text-[12px] flex items-center "
                    onClick={() => {
                      handleCopy(rootSpan?.trace_id ?? '');
                    }}
                  >
                    <IconCozCopy /> {t('copy')}
                  </div>
                </div>
                <Typography.Text className="!coz-fg-primary !text-[12px]">
                  {rootSpan?.trace_id}
                </Typography.Text>
              </div>

              {rootSpan?.logid ? (
                <div>
                  <div className="flex gap-2 items-center">
                    <span className="font-medium">Log ID </span>
                    <div
                      className="coz-fg-secondary cursor-pointer !text-[12px] flex items-center "
                      onClick={() => {
                        handleCopy(rootSpan?.logid ?? '');
                      }}
                    >
                      <IconCozCopy /> {t('copy')}
                    </div>
                    {rootSpan ? renderHeaderCopyNode?.(rootSpan) : null}
                  </div>
                  <Typography.Text className="!coz-fg-primary !text-[12px]">
                    {rootSpan?.logid}
                  </Typography.Text>
                </div>
              ) : null}
            </div>
          }
        >
          <Button color="secondary" className="coz-fg-secondary mr-2">
            <IconCozCopy className="w-[16px] h-[16px]" />
          </Button>
        </Popover>
        {switchConfig ? (
          <>
            <Tooltip content={t('prev_item')} theme="dark">
              <Button
                icon={<IconCozArrowLeft className="w-[16px] h-[16px]" />}
                color="secondary"
                className="coz-fg-primary"
                disabled={!switchConfig?.canSwitchPre}
                onClick={() => {
                  switchConfig?.onSwitch('pre');
                }}
              />
            </Tooltip>
            <Tooltip content={t('next_item')} theme="dark">
              <Button
                icon={<IconCozArrowRight className="w-[16px] h-[16px]" />}
                iconPosition="right"
                color="secondary"
                className="ml-2 coz-fg-primary"
                disabled={!switchConfig?.canSwitchNext}
                onClick={() => {
                  switchConfig?.onSwitch('next');
                }}
              />
            </Tooltip>
          </>
        ) : null}

        {switchConfig ? (
          <Divider
            layout="vertical"
            style={{
              height: '12px',
              margin: '0 8px',
            }}
          />
        ) : null}

        {showClose ? (
          <Button
            color="secondary"
            onClick={onClose}
            icon={<IconCozCrossFill className="w-[16px] h-[16px]" />}
          />
        ) : null}
      </div>
    </div>
  );
};
