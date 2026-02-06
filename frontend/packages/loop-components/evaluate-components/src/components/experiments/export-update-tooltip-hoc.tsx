// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useCallback, useMemo } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { useUIStore, UIEvent } from '@cozeloop/stores';
import { I18n } from '@cozeloop/i18n-adapter';
import { Tooltip } from '@coze-arch/coze-design';

/**
 * 导出功能升级提示高阶组件
 *
 * 用于包装导出相关的按钮或组件，当用户没有导出权限时（如个人版用户），
 * 会显示升级提示的 Tooltip，引导用户升级到企业团队版套餐。
 *
 * @param props 组件属性
 * @param props.isShowTooltip 是否显示升级提示的 Tooltip
 * @param props.children 被包装的子组件
 *
 * @example
 * ```tsx
 * <ExportUpdateTooltipHoc isShowTooltip={!hasExportPermission}>
 *   <Button>导出</Button>
 * </ExportUpdateTooltipHoc>
 * ```
 */
export const ExportUpdateTooltipHoc = ({
  isShowTooltip,
  children,
}: {
  /** 是否显示升级提示的 Tooltip */ isShowTooltip: boolean;
  /** 被包装的子组件 */ children: React.ReactNode;
}) => {
  // 获取 UI 事件发布器，用于触发订阅模态框
  const { publish } = useUIStore(
    useShallow(state => ({ publish: state.publish$ })),
  );

  /**
   * 处理升级链接点击事件
   * 阻止事件冒泡并触发订阅模态框的打开
   */
  const handleUpgradeClick = useCallback(
    (e: React.MouseEvent) => {
      // 确保阻止所有事件传播
      e.preventDefault();
      e.stopPropagation();

      // 使用 setTimeout 确保事件处理完成后再发布事件
      setTimeout(() => {
        // 发布打开订阅模态框的事件
        publish.emit(UIEvent.OPEN_SUBSCRIPTION_MODAL);
      }, 0);
    },
    [publish],
  );

  /**
   * 升级提示的内容
   * 包含升级链接和说明文字
   */
  const tooltipContent = useMemo(
    () => (
      <div className="h-full">
        <span
          className="text-primary text-[#B0B9FF] mr-1 cursor-pointer inline-block"
          onMouseDown={e => {
            // 使用 onMouseDown 确保更早的事件拦截
            e.preventDefault();
            e.stopPropagation();
          }}
          onClick={e => {
            handleUpgradeClick(e);
          }}
        >
          {I18n.t('upgrade_enterprise_team_edition_package')}
        </span>
        <span>{I18n.t('you_can_use_this_function_later')}</span>
      </div>
    ),

    [handleUpgradeClick],
  );

  // 根据是否需要显示提示来决定是否包装 Tooltip
  if (isShowTooltip) {
    return (
      <Tooltip content={tooltipContent} keepDOM={true} theme="dark">
        {children}
      </Tooltip>
    );
  }

  // 如果不需要显示提示，直接返回子组件
  return children;
};
