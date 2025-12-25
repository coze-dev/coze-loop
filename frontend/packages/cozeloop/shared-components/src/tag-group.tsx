// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/prefer-for-of */
import React, { useLayoutEffect, useRef, useState } from 'react';

import { Popover, Tag, type TagProps } from '@coze-arch/coze-design';

const GAP = 8; // 对应 Tailwind 的 `gap-2`
const POPOVER_TAG_WIDTH = 40; // "+N" 标签的预估宽度

/**
 * TagGroup 组件
 * 动态计算并显示标签，当标签超出容器宽度时，将其余标签放入 Popover 中显示。
 */
const TagGroup = ({
  tagList = [],
  showPopover = false,
}: {
  tagList: TagProps[];
  showPopover?: boolean;
}) => {
  // 用于获取可见容器的引用
  const containerRef = useRef<HTMLDivElement>(null);
  // 用于获取隐藏的测量容器的引用
  const measurementRef = useRef<HTMLDivElement>(null);
  // 状态：存储可见标签的数量
  const [visibleCount, setVisibleCount] = useState(tagList.length);

  useLayoutEffect(() => {
    const container = containerRef.current;
    const measurementContainer = measurementRef.current;
    if (!container || !measurementContainer) {
      return;
    }

    // 处理容器尺寸变化的函数
    const handleResize = () => {
      const containerWidth = container.offsetWidth;
      const tagNodes = Array.from(
        measurementContainer.children,
      ) as HTMLElement[];

      let totalWidth = 0;
      let count = 0;

      // 第一步：计算在不考虑 Popover 的情况下，能容纳多少个标签
      for (const tagNode of tagNodes) {
        totalWidth += tagNode.offsetWidth + GAP;
        if (totalWidth > containerWidth) {
          break;
        }
        count++;
      }

      // 第二步：如果并非所有标签都可见，则重新计算，并为 "+N" 的 Popover 标签预留空间
      if (tagNodes.length > count) {
        let widthWithPopover = 0;
        let countWithPopover = 0;
        for (let i = 0; i < tagNodes.length; i++) {
          const newWidth = widthWithPopover + tagNodes[i].offsetWidth + GAP;
          // 如果加上 Popover 标签的宽度后超出容器宽度，则停止计算
          if (newWidth + POPOVER_TAG_WIDTH > containerWidth) {
            break;
          }
          widthWithPopover = newWidth;
          countWithPopover++;
        }
        setVisibleCount(countWithPopover);
      } else {
        // 如果所有标签都能放下，则全部显示
        setVisibleCount(tagNodes.length);
      }
    };

    handleResize();

    // 使用 ResizeObserver 监听容器尺寸变化
    const resizeObserver = new ResizeObserver(handleResize);
    resizeObserver.observe(container);

    // 清理函数：组件卸载时停止监听
    return () => {
      resizeObserver.disconnect();
    };
  }, [tagList]);

  // 根据计算出的可见数量，分割标签列表
  const visibleTagList = tagList.slice(0, visibleCount);
  const unVisibleTagList = tagList.slice(visibleCount);

  return (
    <div className="relative">
      {/* 可见的标签容器 */}
      <div ref={containerRef} className="flex flex-nowrap gap-2">
        {visibleTagList.map((tag, idx) => (
          <Tag key={tag?.tagKey || idx} size="small" color="primary">
            {tag.children}
          </Tag>
        ))}
        {/* 如果有不可见的标签，则显示 Popover */}
        {showPopover && unVisibleTagList.length > 0 ? (
          <Popover
            position="top"
            content={
              <div className="flex flex-wrap gap-2 p-2">
                {unVisibleTagList.map((tag, idx) => (
                  <Tag key={tag?.tagKey || idx} size="small" color="primary">
                    {tag.children}
                  </Tag>
                ))}
              </div>
            }
          >
            <Tag size="small" className="cursor-pointer" color="primary">
              +{unVisibleTagList.length}
            </Tag>
          </Popover>
        ) : null}
      </div>
      {/* 用于宽度测量的隐藏容器 */}
      <div
        ref={measurementRef}
        className="absolute -z-10 flex flex-nowrap gap-2"
        style={{ top: 0, left: 0, visibility: 'hidden' }}
      >
        {tagList.map((tag, idx) => (
          <Tag key={tag?.tagKey || idx} size="small" color="primary">
            {tag.children}
          </Tag>
        ))}
      </div>
    </div>
  );
};

export { TagGroup };
