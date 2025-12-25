// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef } from 'react';

import cls from 'classnames';
import { useHover } from 'ahooks';
import { type Model } from '@cozeloop/api-schema/llm-manage';
import { IconCozDiamondFill } from '@coze-arch/coze-design/icons';
import {
  Avatar,
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import styles from './index.module.less';

export interface ModelItemProps extends Model {
  series?: {
    /** series name */
    name?: string;
    /** series icon url */
    icon?: string;
    /** vendor name */
    vendor?: string;
  };
  /** model icon url */
  icon?: string;
  /** model tags */
  tags?: Array<string>;
  disabled?: boolean;
  status?: Int64;
  statusInfo?: React.ReactNode;
}

export type ModelOptionProps = {
  model: ModelItemProps;
  selected?: boolean;
  disabled?: boolean;
  className?: string;
  /** 返回是否切换成功 */
  onClick?: () => boolean;
} & (
  | {
      enableConfig?: false;
    }
  | {
      enableConfig: true;
      onConfigClick: () => void;
    }
) &
  (
    | {
        enableJumpDetail?: false;
      }
    | {
        enableJumpDetail: true;
        /**
         * 点击跳转模型管理页面
         *
         * 因为该组件定位是纯 UI 组件，且不同模块 space id 获取的方式不尽相同，因此跳转行为和 url 的拼接就不内置了
         */
        onDetailClick: (modelId: string) => void;
      }
  );

export function ModelOption({
  model,
  selected,
  disabled,
  onClick,
  className,
  ...props
}: ModelOptionProps) {
  /** 这个 ref 纯粹为了判断是否 hover */
  const ref = useRef<HTMLElement>(null);
  const isHovering = useHover(ref);

  const modelTags = model?.tags || [];

  const renderOverflow = (items: string[]) =>
    items?.length ? (
      <Tooltip
        content={
          <Space wrap spacing={3}>
            {items?.map(item => <Tag>{item}</Tag>)}
          </Space>
        }
      >
        <Tag
          style={{ flex: '0 0 auto' }}
          size="mini"
          color="primary"
          className="!bg-transparent !border border-solid"
        >
          +{items.length}
        </Tag>
      </Tooltip>
    ) : null;

  return (
    <article
      ref={ref}
      className={cls(
        'pl-[16px] pr-[12px] w-full relative',
        'flex gap-[16px] items-center rounded-[12px]',
        'min-w-[480px] max-w-[570px]',
        disabled ? 'cursor-not-allowed' : 'cursor-pointer',
        styles['model-option'],
        { [styles['model-option_selected']]: selected },
        className,
      )}
      onClick={() => {
        onClick?.();
      }}
    >
      <Avatar
        className="shrink-0 rounded-[6px] border border-solid coz-stroke-primary"
        shape="square"
        src={model.series?.icon}
        size="default"
      />
      <div
        className={cls(
          'h-[70px] py-[14px] w-full',
          'flex flex-col overflow-hidden',
          'border-0 border-b border-solid coz-stroke-primary',
          styles['model-info-border'],
        )}
        style={
          isHovering
            ? {
                mask: calcMaskStyle([
                  props.enableConfig,
                  props.enableJumpDetail,
                ]),
              }
            : undefined
        }
      >
        <div className="w-full flex items-center gap-[6px] overflow-hidden">
          <Typography.Title fontSize="14px" ellipsis={{ showTooltip: true }}>
            {model.name}
          </Typography.Title>
          <div className="shrink-0 flex gap-[6px]">
            {disabled ? (
              <div className="h-[16px] leading-[16px]">
                <IconCozDiamondFill className="coz-fg-hglt text-[12px]" />
              </div>
            ) : null}
            {/* {model.commercialModelStatusDetail?.isNewModel ? (
              <Tooltip content="Pro 版用户专享" theme="dark">
                <Tag size="mini" color="brand">
                  新模型
                </Tag>
              </Tooltip>
            ) : null}
            {model.commercialModelStatusDetail?.isAdvancedModel ? (
              <Tooltip content="专业版用户专享" theme="dark">
                <Tag size="mini" color="blue">
                  高级
                </Tag>
              </Tooltip>
            ) : null} */}
            {model.statusInfo}
            {modelTags?.length ? (
              <Space spacing={4}>
                {modelTags.slice(0, 3).map(item => (
                  <Tag
                    key={item}
                    size="mini"
                    color="primary"
                    className="!bg-transparent !border border-solid coz-stroke-plus !rounded-[4px]"
                  >
                    {item}
                  </Tag>
                ))}
                {renderOverflow(modelTags.slice(3))}
              </Space>
            ) : null}
          </div>
        </div>
        <Typography.Text
          className="mt-[4px] text-[12px] leading-[16px] coz-fg-secondary"
          ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
        >
          {model.desc}
        </Typography.Text>
      </div>
    </article>
  );
}

/**
 * hover 展示若干图标（比如跳转模型详情页、详细配置）时，要对图标下的内容有个渐变遮罩效果
 * 该方法用于计算遮罩样式
 */
function calcMaskStyle(buttonVisible: Array<boolean | undefined>) {
  const btnNum = buttonVisible.reduce(
    (prevNum, showBtn) => prevNum + (showBtn ? 1 : 0),
    0,
  );
  if (btnNum === 0) {
    return 'none';
  }

  const BTN_WIDTH = 32;
  const BTN_GAP = 3;
  /** 不随按钮数量变化的遮罩固定宽度 */
  const PRESET_PADDING = 16;
  /** 遮罩的渐变宽度 */
  const MASK_WIDTH = 24;

  const gradientStart =
    btnNum * BTN_WIDTH + (btnNum - 1) * BTN_GAP + PRESET_PADDING;
  const gradientEnd = gradientStart + MASK_WIDTH;
  return `linear-gradient(to left, rgba(0,0,0,0), rgba(0,0,0,0) ${gradientStart}px, #fff ${gradientEnd}px)`;
}
